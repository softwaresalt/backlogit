package core

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// SetArtifactSize sets the logical `size` field on an artifact, physically stored
// under custom_fields.size, via a body-preserving write. It is the SINGLE seam for
// size mutation and deliberately bypasses the generic UpdateArtifact rebuild path.
//
// The sequence is: (a) enum-validate size against the type's header-def schema
// BEFORE any write (a targeted check — global enum enforcement is intentionally
// NOT retrofitted into ValidateArtifactFields, which would regress legacy
// artifacts); (b) acquire the per-task advisory lock (non-blocking; a held lock
// returns ErrTaskBusy); (c) mdfront.Decode the file and set custom_fields.size,
// leaving every other frontmatter key and the ENTIRE body bytes untouched;
// (d) mdfront.Encode -> atomicfile.WriteFileAtomic; (e) keep the SQLite index in
// sync via db.UpsertItem with a FULLY-POPULATED artifact reconstructed from the
// same decode. The reconstruction is critical: UpsertItem is INSERT OR REPLACE on
// the full row, so a partial {ID, CustomFields} stub would null title/status/
// priority and re-open markdown<->DB drift.
//
// SetArtifactSize intentionally emits no mutation hook event (size-only changes
// bypass the generic HookUpdateArtifact chain); this is documented as acceptable
// because the only pre-hook is a no-op when status is unchanged.
func SetArtifactSize(ctx context.Context, ws *Workspace, id, size string) (*models.Artifact, error) {
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

	// Targeted enum validation BEFORE any write.
	if err := validateSizeValue(ws, artifactType, size); err != nil {
		return nil, err
	}

	// Set custom_fields.size in the decoded frontmatter, preserving all other
	// frontmatter keys and the entire body bytes.
	setDecodedCustomField(mdDoc, "size", size)

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

// validateSizeValue confirms size is a member of the type's header-def `size`
// enum. It is a targeted check used only by the size-mutation seam.
func validateSizeValue(ws *Workspace, artifactType, size string) error {
	if ws.HeaderDef == nil {
		return fmt.Errorf("cannot validate size: header-def not loaded")
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
