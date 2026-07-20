package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ErrSizeEstimationNotImplemented marks the 108-F red-phase sizing harness.
var ErrSizeEstimationNotImplemented = errors.New("TODO: implement 108-F size estimation")

// ActorContext identifies the trusted transport actor for size provenance.
type ActorContext string

const (
	// ActorContextHuman records a trusted human/CLI-authored size mutation.
	ActorContextHuman ActorContext = "human"
	// ActorContextAgent records an agent/MCP-authored size mutation.
	ActorContextAgent ActorContext = "agent"
	// ActorContextDerived records a derived size mutation.
	ActorContextDerived ActorContext = "derived"
)

// SizeMutation is the presence-aware command for size/provenance updates.
type SizeMutation struct {
	Size           *string
	Source         *string
	RulesetVersion *string
	Actor          ActorContext
}

// SetArtifactSize sets the logical `size` field on an artifact via the typed
// provenance seam. It is a thin compatibility wrapper that constructs a
// provenance-free SizeMutation (human actor context) and delegates to
// SetArtifactSizeWithProvenance. Callers that need to record `size_source` /
// `size_ruleset_version` must use SetArtifactSizeWithProvenance directly.
func SetArtifactSize(ctx context.Context, ws *Workspace, id, size string) (*models.Artifact, error) {
	return SetArtifactSizeWithProvenance(ctx, ws, id, SizeMutation{Size: &size, Actor: ActorContextHuman})
}

// sizeSeamWriteFailureHook, when non-nil, is invoked after the estimate-history
// audit event append succeeds but before the durable frontmatter write. Tests use
// it to simulate a process crash between the append and the durable write so the
// orphan-crash-residue-event-ignored-on-read contract (108-F SE-3b, D4) can be
// exercised. It is nil in production.
var sizeSeamWriteFailureHook func() error

// SetArtifactSizeWithProvenance is the SINGLE seam for size + provenance mutation
// (108-F SE-3a). It deliberately bypasses the generic UpdateArtifact rebuild path.
//
// The sequence is: (a) enum-validate the supplied size against the type's
// header-def schema and validate provenance completeness BEFORE any write (a
// targeted check — global enum enforcement is intentionally NOT retrofitted into
// ValidateArtifactFields, which would regress legacy artifacts); (b) acquire the
// per-task advisory lock (non-blocking; a held lock returns ErrTaskBusy);
// (c) append the estimate-history audit event under event-before-write,
// fail-closed ordering — if the append fails, the durable write is refused so no
// persisted size/provenance change ever lacks its event (the durable
// custom_fields.size remains the sole source of truth; the audit stream is never
// read back as truth); (d) merge-set only the provided reserved keys in the
// decoded frontmatter, leaving every other frontmatter key and the ENTIRE body
// bytes untouched; (e) mdfront.Encode -> atomicfile.WriteFileAtomic; (f) keep the
// SQLite index in sync via db.UpsertItem with a FULLY-POPULATED artifact
// reconstructed from the same decode (UpsertItem is INSERT OR REPLACE on the full
// row, so a partial stub would null non-size columns and re-open markdown<->DB
// drift).
func SetArtifactSizeWithProvenance(ctx context.Context, ws *Workspace, id string, m SizeMutation) (*models.Artifact, error) {
	path, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("find artifact %s: %w", id, err)
	}

	// Acquire the per-task lock before any read/write. Non-blocking: a concurrent
	// holder yields ErrTaskBusy so gate consumers get deterministic contention.
	unlock, err := lockTaskFile(path)
	if err != nil {
		if errors.Is(err, ErrTaskBusy) {
			return nil, err
		}
		return nil, fmt.Errorf("lock task %s: %w", id, err)
	}
	defer func() { _ = unlock() }()

	// Re-validate path containment UNDER THE LOCK, immediately before the read, to
	// close the TOCTOU window between FindArtifactPath's walk-time containment check
	// and this locked read/write: a symlink at path could otherwise be swapped after
	// lookup to resolve outside the workspace storage root.
	if err := ensureArtifactLookupContained(ws, path); err != nil {
		return nil, fmt.Errorf("revalidate containment for %s: %w", id, err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", id, err)
	}
	mdDoc, err := mdfront.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decode artifact %s: %w", id, err)
	}
	if !mdDoc.HasFrontmatter {
		return nil, fmt.Errorf("artifact %s has no frontmatter", id)
	}

	artifactType, _ := mdDoc.Frontmatter["artifact_type"].(string)
	priorSize := decodedCustomField(mdDoc, "size")

	// Targeted validation BEFORE any audit append or durable write.
	if err := validateSizeMutation(ws, artifactType, priorSize, m); err != nil {
		return nil, err
	}

	// Event-before-write, fail-closed: append the estimate-history audit event
	// first; refuse the durable write if the append fails so no persisted change
	// lacks its event (gate-evidence precedent).
	delta := map[string]any{"actor": string(m.Actor)}
	if m.Size != nil {
		delta["size"] = *m.Size
	}
	if m.Source != nil {
		delta["size_source"] = *m.Source
	}
	if m.RulesetVersion != nil {
		delta["size_ruleset_version"] = *m.RulesetVersion
	}
	if err := appendItemEventWithActorErr(ctx, ws, id, string(m.Actor), events.EventEstimateHistory, delta); err != nil {
		return nil, fmt.Errorf("append estimate-history event for %s: %w", id, err)
	}

	// Test-only fault injection: simulate a crash between the successful audit
	// append and the durable write, leaving an orphan crash-residue event.
	if sizeSeamWriteFailureHook != nil {
		if err := sizeSeamWriteFailureHook(); err != nil {
			return nil, fmt.Errorf("write artifact %s: %w", id, err)
		}
	}

	// Merge-not-replace: set only the reserved keys the caller supplied, leaving
	// all other frontmatter keys and the entire body bytes untouched.
	if m.Size != nil {
		setDecodedCustomField(mdDoc, "size", *m.Size)
	}
	if m.Source != nil {
		setDecodedCustomField(mdDoc, "size_source", *m.Source)
	}
	if m.RulesetVersion != nil {
		setDecodedCustomField(mdDoc, "size_ruleset_version", *m.RulesetVersion)
	}

	out, err := mdDoc.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode artifact %s: %w", id, err)
	}
	if err := atomicfile.WriteFileAtomic(path, out); err != nil {
		return nil, fmt.Errorf("write artifact %s: %w", id, err)
	}

	// Reconstruct a fully-populated artifact from the SAME decode so the full-row
	// UpsertItem preserves every non-size column.
	artifact, err := models.ArtifactFromFrontmatter(mdDoc.Frontmatter, string(mdDoc.Body))
	if err != nil {
		return nil, fmt.Errorf("reconstruct artifact %s: %w", id, err)
	}
	if ws.DB != nil {
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			return nil, fmt.Errorf("upsert artifact %s: %w", id, err)
		}
	}
	return artifact, nil
}

// reservedSizingKeys are the size/provenance custom_fields keys that only the
// size seam may author. Generic create/update paths must not write them off-seam.
var reservedSizingKeys = []string{"size", "size_source", "size_ruleset_version"}

// validateSizeMutation performs the targeted pre-write validation for the size
// seam: the size value against the type's enum, size_source against the fixed
// provenance set, provenance completeness (an explicit size_source must be
// accompanied by a size_ruleset_version so a recorded estimate always states the
// ruleset it was produced under), and effective-size presence (any provenance
// field requires an accompanying size — from this mutation or already stored — so
// provenance is never recorded on an unsized artifact). All failures wrap
// ErrValidation so the MCP layer surfaces them as validation_failed (422) rather
// than opaque internal (500) errors.
func validateSizeMutation(ws *Workspace, artifactType, priorSize string, m SizeMutation) error {
	if m.Size != nil {
		if err := validateSizeValue(ws, artifactType, *m.Size); err != nil {
			return err
		}
	}
	if m.Source != nil {
		switch ActorContext(*m.Source) {
		case ActorContextHuman, ActorContextAgent, ActorContextDerived:
		default:
			return fmt.Errorf("invalid size_source %q: must be one of [human agent derived]: %w", *m.Source, blerrors.ErrValidation)
		}
		// Treat an empty/whitespace ruleset as missing so CLI (`--size-ruleset-version ""`)
		// and MCP (omitted argument) enforce provenance completeness identically.
		if m.RulesetVersion == nil || strings.TrimSpace(*m.RulesetVersion) == "" {
			return fmt.Errorf("size_source %q requires an accompanying non-empty size_ruleset_version: %w", *m.Source, blerrors.ErrValidation)
		}
	}
	// Provenance accompanies a size: if any provenance field is being written, the
	// effective post-mutation size (this mutation's size, else the already-stored
	// size) must be non-empty. Otherwise provenance would be recorded on an unsized
	// task, leaving meaningless reserved fields with no size they describe.
	if m.Source != nil || m.RulesetVersion != nil {
		effectiveSize := priorSize
		if m.Size != nil {
			effectiveSize = *m.Size
		}
		if strings.TrimSpace(effectiveSize) == "" {
			return fmt.Errorf("size provenance (size_source/size_ruleset_version) requires an accompanying size: %w", blerrors.ErrValidation)
		}
	}
	return nil
}

// rejectReservedSizingKeysOnCreate enforces sole-writer integrity at the generic
// create boundary (108-F SE-3a, Copilot G7): the generic create path may NEVER
// author size or provenance. Any reserved sizing key (size, size_source,
// size_ruleset_version) present on a generic create is refused so an initial size
// is never recorded eventless, unvalidated, and off-seam. All initial sizing must
// route through the audited SetArtifactSizeWithProvenance seam, which validates the
// value + provenance completeness and appends the fail-closed estimate_history
// event.
//
// A prior variant permitted a provenanced size (size + size_source) to ride through
// for "migration/import", but that permit-branch bypassed validateSizeMutation
// (allowing an invalid enum value or a source without a ruleset) and emitted no
// audit event. No production caller is affected by the stricter fail-closed rule:
// the backlog.md migration adapter prefixes every source frontmatter key with
// `backlog_md_` (internal/parser/adapter.go), so a migrated `size` arrives as
// `backlog_md_size`, never the reserved `size` key. The reserved keys therefore only
// reach this boundary via a direct programmatic WithFields, which must use the seam.
func rejectReservedSizingKeysOnCreate(fields map[string]any) error {
	if fields == nil {
		return nil
	}
	for _, k := range reservedSizingKeys {
		if _, ok := fields[k]; ok {
			return fmt.Errorf("create carrying reserved sizing key %q must route through the audited size seam: %w", k, blerrors.ErrValidation)
		}
	}
	return nil
}

// mergePreserveReservedSizingKeys enforces sole-writer integrity for the reserved
// sizing keys on the generic update path (108-F SE-3a). The generic path may never
// author size/provenance: any caller-supplied reserved key is dropped (an off-seam
// write would bypass validateSizeMutation and the fail-closed estimate_history
// event), and the prior seam-owned value is always carried forward so a generic
// update never silently drops or overwrites the size the size seam owns.
func mergePreserveReservedSizingKeys(prior, incoming map[string]any) map[string]any {
	if incoming == nil {
		incoming = map[string]any{}
	}
	for _, k := range reservedSizingKeys {
		delete(incoming, k)
		if v, ok := prior[k]; ok {
			incoming[k] = v
		}
	}
	return incoming
}

// validateSizeValue confirms size is a member of the type's header-def `size`
// enum. It is a targeted check used only by the size-mutation seam.
func validateSizeValue(ws *Workspace, artifactType, size string) error {
	if ws.HeaderDef == nil {
		return fmt.Errorf("cannot validate size: header-def not loaded: %w", blerrors.ErrConfig)
	}
	schema, err := ws.HeaderDef.ResolveFieldSchema(artifactType)
	if err != nil {
		return fmt.Errorf("resolve schema for %q: %w", artifactType, err)
	}
	def, ok := schema["size"]
	if !ok {
		// User-correctable: this artifact type simply has no size field. Wrap
		// ErrValidation so the MCP layer surfaces it as validation_failed (422)
		// rather than an opaque internal (500) error.
		return fmt.Errorf("artifact type %q does not define a size field: %w", artifactType, blerrors.ErrValidation)
	}
	for _, v := range def.Values {
		if v == size {
			return nil
		}
	}
	// User-correctable: the supplied value is not in the enum. Wrap
	// ErrValidation for a deterministic validation_failed (422) MCP response.
	return fmt.Errorf("invalid size %q: must be one of %v: %w", size, def.Values, blerrors.ErrValidation)
}

// setDecodedCustomField sets key=value inside the decoded frontmatter's
// custom_fields map, creating the map when absent and leaving all other keys
// untouched.
func setDecodedCustomField(mdDoc *mdfront.Markdown, key, value string) {
	cf, ok := mdDoc.Frontmatter["custom_fields"].(map[string]any)
	if !ok || cf == nil {
		cf = map[string]any{}
	}
	cf[key] = value
	mdDoc.Frontmatter["custom_fields"] = cf
}

// decodedCustomField returns the string value of custom_fields[key] in the
// decoded frontmatter, or "" when the map or key is absent or non-string.
func decodedCustomField(mdDoc *mdfront.Markdown, key string) string {
	cf, ok := mdDoc.Frontmatter["custom_fields"].(map[string]any)
	if !ok || cf == nil {
		return ""
	}
	if v, ok := cf[key].(string); ok {
		return v
	}
	return ""
}
