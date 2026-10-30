package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ComplexityMutation is the presence-aware command for complexity/provenance
// updates. A nil Complexity is invalid; an empty-string Complexity clears the
// complexity value together with both provenance fields.
type ComplexityMutation struct {
	Complexity     *string
	Source         *string
	RulesetVersion *string
	Actor          ActorContext
}

const (
	complexityKey               = "complexity"
	complexitySourceKey         = "complexity_source"
	complexityRulesetVersionKey = "complexity_ruleset_version"
)

// SetArtifactComplexity sets or clears the task complexity metadata. It is a
// provenance-free compatibility wrapper over SetArtifactComplexityWithProvenance:
// a non-empty value clears any stale provenance, an empty value clears all three
// complexity fields.
func SetArtifactComplexity(ctx context.Context, ws *Workspace, id, complexity string) (*models.Artifact, error) {
	return SetArtifactComplexityWithProvenance(ctx, ws, id, ComplexityMutation{Complexity: &complexity, Actor: ActorContextHuman})
}

// SetArtifactComplexityWithProvenance is the single seam for complexity and
// provenance mutation. It validates before any write, appends the fail-closed
// estimate_history (dimension: complexity) audit event before the durable
// body-preserving frontmatter write, and keeps the SQLite index in sync.
func SetArtifactComplexityWithProvenance(ctx context.Context, ws *Workspace, id string, m ComplexityMutation) (artifact *models.Artifact, retErr error) {
	unlock, err := lockArtifactMutation(ctx, ws, id)
	if err != nil {
		if errors.Is(err, ErrTaskBusy) {
			return nil, err
		}
		return nil, fmt.Errorf("lock task %s: %w", id, err)
	}
	defer func() {
		if err := unlock(); err != nil && retErr == nil {
			retErr = fmt.Errorf("unlock task %s: %w", id, err)
		}
	}()

	path, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("find artifact %s: %w", id, err)
	}
	ioPath, err := resolveContainedArtifactPath(ws, path)
	if err != nil {
		return nil, fmt.Errorf("resolve contained path for %s: %w", id, err)
	}

	raw, err := os.ReadFile(ioPath)
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
	if m.Complexity == nil {
		return nil, fmt.Errorf("complexity mutation for artifact %s requires a complexity value (empty string clears): %w", id, blerrors.ErrValidation)
	}
	complexity := *m.Complexity
	if err := validateComplexityMutation(ws, artifactType, complexity); err != nil {
		return nil, fmt.Errorf("validate complexity for artifact %s: %w", id, err)
	}
	if err := validateComplexityProvenance(complexity, m); err != nil {
		return nil, fmt.Errorf("validate complexity provenance for artifact %s: %w", id, err)
	}

	actor := m.Actor
	if actor == "" {
		actor = ActorContextHuman
	}
	delta := map[string]any{"actor": string(actor), "dimension": "complexity", complexityKey: complexity}
	provenanceSupplied := m.Source != nil || m.RulesetVersion != nil
	if provenanceSupplied {
		delta[complexitySourceKey] = *m.Source
		delta[complexityRulesetVersionKey] = *m.RulesetVersion
	}
	// Event-before-write, fail-closed: refuse the durable write when the audit
	// append fails so no persisted change lacks its event.
	if err := appendItemEventWithActorErr(ctx, ws, id, string(actor), events.EventEstimateHistory, delta); err != nil {
		return nil, fmt.Errorf("append estimate-history event for %s: %w", id, err)
	}

	switch {
	case complexity == "":
		clearDecodedCustomField(mdDoc, complexityKey)
		clearDecodedCustomField(mdDoc, complexitySourceKey)
		clearDecodedCustomField(mdDoc, complexityRulesetVersionKey)
	case provenanceSupplied:
		setDecodedCustomField(mdDoc, complexityKey, complexity)
		setDecodedCustomField(mdDoc, complexitySourceKey, *m.Source)
		setDecodedCustomField(mdDoc, complexityRulesetVersionKey, *m.RulesetVersion)
	default:
		// Value-only update: never retain provenance that described a prior value.
		setDecodedCustomField(mdDoc, complexityKey, complexity)
		clearDecodedCustomField(mdDoc, complexitySourceKey)
		clearDecodedCustomField(mdDoc, complexityRulesetVersionKey)
	}

	out, err := mdDoc.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode artifact %s: %w", id, err)
	}
	if err := writeComplexitySeamDurable(ioPath, out, WorkspaceDurableWrites(ws)); err != nil {
		return nil, fmt.Errorf("write artifact %s: %w", id, err)
	}

	artifact, err = models.ArtifactFromFrontmatter(mdDoc.Frontmatter, string(mdDoc.Body))
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

// validateComplexityProvenance enforces provenance completeness: source and
// ruleset must be supplied together, the source must be a known actor context,
// the ruleset must be non-blank without control characters, and provenance is
// rejected when the mutation clears complexity.
func validateComplexityProvenance(complexity string, m ComplexityMutation) error {
	// Presence, not non-emptiness: an explicitly supplied empty value is invalid
	// input and must never be treated as omitted.
	srcSet, rvSet := m.Source != nil, m.RulesetVersion != nil
	if !srcSet && !rvSet {
		return nil
	}
	if complexity == "" {
		return fmt.Errorf("complexity_source/complexity_ruleset_version cannot accompany a complexity clear: %w", blerrors.ErrValidation)
	}
	if !srcSet {
		return fmt.Errorf("complexity_ruleset_version requires an accompanying complexity_source: %w", blerrors.ErrValidation)
	}
	switch ActorContext(*m.Source) {
	case ActorContextHuman, ActorContextAgent, ActorContextDerived:
	default:
		return fmt.Errorf("invalid complexity_source %q: must be one of [human agent derived]: %w", *m.Source, blerrors.ErrValidation)
	}
	if !rvSet || strings.TrimSpace(*m.RulesetVersion) == "" {
		return fmt.Errorf("complexity_source %q requires an accompanying non-empty complexity_ruleset_version: %w", *m.Source, blerrors.ErrValidation)
	}
	for _, r := range *m.RulesetVersion {
		if unicode.IsControl(r) {
			return fmt.Errorf("complexity_ruleset_version must not contain control characters: %w", blerrors.ErrValidation)
		}
	}
	return nil
}

// ValidateComplexityValue confirms complexity is a member of the type's
// header-def complexity enum.
func ValidateComplexityValue(ws *Workspace, artifactType, complexity string) error {
	return validateComplexityMutation(ws, artifactType, complexity)
}

func validateComplexityMutation(ws *Workspace, artifactType, complexity string) error {
	if artifactType != "task" {
		return fmt.Errorf("complexity is task-only; artifact type %q cannot store complexity: %w", artifactType, blerrors.ErrValidation)
	}
	if ws.HeaderDef == nil {
		return fmt.Errorf("cannot validate complexity: header-def not loaded: %w", blerrors.ErrConfig)
	}
	schema, err := ws.HeaderDef.ResolveFieldSchema(artifactType)
	if err != nil {
		return fmt.Errorf("resolve schema for %q: %w", artifactType, err)
	}
	def, ok := schema["complexity"]
	if !ok {
		return fmt.Errorf("artifact type %q does not define a complexity field: %w", artifactType, blerrors.ErrValidation)
	}
	if complexity == "" {
		return nil
	}
	for _, v := range def.Values {
		if v == complexity {
			return nil
		}
	}
	return fmt.Errorf("invalid complexity %q: must be one of %v: %w", complexity, def.Values, blerrors.ErrValidation)
}

func clearDecodedCustomField(mdDoc *mdfront.Markdown, key string) {
	cf, ok := mdDoc.Frontmatter["custom_fields"].(map[string]any)
	if !ok || cf == nil {
		return
	}
	delete(cf, key)
	if len(cf) == 0 {
		delete(mdDoc.Frontmatter, "custom_fields")
		return
	}
	mdDoc.Frontmatter["custom_fields"] = cf
}

func writeComplexitySeamDurable(path string, data []byte, durable bool) error {
	err := atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: durable})
	if err == nil {
		return nil
	}
	if blerrors.IsWriteNotApplied(err) {
		return atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: durable})
	}
	return err
}

// IsTaskRouteEligible reports whether a task carries complete, valid size and
// complexity provenance and is therefore eligible for model routing.
func IsTaskRouteEligible(item *models.Artifact) bool { return db.IsTaskRouteEligible(item) }
