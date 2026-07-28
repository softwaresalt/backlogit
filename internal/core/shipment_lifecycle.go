package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

var deliberationIDPattern = regexp.MustCompile(`\b(?:DL\d+|[0-9]+(?:\.[0-9]+)*-DL)\b`)

// CommitMetadata captures the merge or release commit that closed a shipment.
type CommitMetadata struct {
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message,omitempty"`
	Author  string `json:"author,omitempty"`
}

// ShipShipmentResult summarizes shipment release hygiene actions.
type ShipShipmentResult struct {
	ShipmentID     string   `json:"shipment_id"`
	ShipmentStatus string   `json:"shipment_status"`
	ArchivedIDs    []string `json:"archived_ids"`
	ReturnedIDs    []string `json:"returned_ids"`
	CommitSHA      string   `json:"commit_sha,omitempty"`
}

// ClaimShipment moves a queued shipment to active and marks the included work
// scope active. Activation is all-or-nothing: if any item fails to load or
// activate mid-flight, the shipment and every already-activated item (plus any
// cascade-activated parent) are restored to their pre-claim state so no
// partial/torn activation is left behind.
func ClaimShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	// Snapshot the pre-claim shipment before any mutation so a mid-flight
	// failure can be rolled back to a fully queued state.
	current, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}
	preClaimShipment := cloneArtifact(current)

	if err := MoveShipmentStatus(ctx, ws, shipmentID, ShipmentActive); err != nil {
		return nil, err
	}

	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		// The shipment is active but no items have been activated yet; restore
		// it to queued so a failed read-back does not leave a torn state.
		return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, nil,
			fmt.Errorf("reload shipment after activation: %w", err))
	}

	// activatedIDs records every item this claim transitioned to active, in
	// application order, so rollback can revert them (newest first) on failure.
	var activatedIDs []string
	for _, itemID := range NormalizeShipmentItems(shipment) {
		item, loadErr := loadArtifact(ctx, ws, itemID)
		if loadErr != nil {
			return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, activatedIDs,
				fmt.Errorf("load item %s: %w", itemID, loadErr))
		}
		if item.Status == models.StatusQueued {
			// Record the item as activation-attempted *before* mutating it:
			// setArtifactStatus persists the item active before cascading parent
			// statuses, so a failure mid-call can leave the item active on disk.
			// Tracking it up front guarantees rollback reverts it; a queued->queued
			// revert is a safe no-op when activation never landed.
			activatedIDs = append(activatedIDs, itemID)
			if _, setErr := setArtifactStatus(ctx, ws, itemID, models.StatusActive, "shipment claimed"); setErr != nil {
				return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, activatedIDs,
					fmt.Errorf("activate item %s: %w", itemID, setErr))
			}
		}
	}

	// The shipment artifact itself is not mutated by item activation (its
	// manifest items are children of the feature, not of the shipment), so the
	// snapshot loaded above already reflects the post-activation truth. Return
	// it directly rather than performing another read-back: a read-back here
	// could fail after every item is active, leaving a torn state with no
	// remaining operation to roll back. Eliminating it keeps the claim
	// all-or-nothing by construction.
	return shipment, nil
}

// rollbackShipmentClaim reverts a partially applied shipment claim. Each item
// the claim activated is returned to queued in reverse order so child statuses
// settle before their parents are recomputed by the cascade, and the shipment
// is restored to its pre-claim snapshot. The original claim error is wrapped
// together with any rollback error so the caller sees the full failure context.
func rollbackShipmentClaim(ctx context.Context, ws *Workspace, shipmentID string, preClaimShipment *models.Artifact, activatedIDs []string, claimErr error) error {
	// Guard against a nil triggering error: rollback must never collapse to a
	// nil return that silently drops the failure (a future caller that passes
	// nil would otherwise hide a torn-state rollback behind a success).
	if claimErr == nil {
		claimErr = fmt.Errorf("claim rollback invoked without a triggering error")
	}
	var rollbackErrs []error
	for i := len(activatedIDs) - 1; i >= 0; i-- {
		if _, err := setArtifactStatus(ctx, ws, activatedIDs[i], models.StatusQueued, "shipment claim rolled back"); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("revert item %s: %w", activatedIDs[i], err))
		}
	}
	if preClaimShipment != nil {
		if err := persistArtifact(ctx, ws, preClaimShipment, true); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore shipment %s: %w", shipmentID, err))
		}
	}

	slog.WarnContext(ctx, "shipment claim rolled back",
		"shipment_id", shipmentID, "reverted_items", len(activatedIDs), "error", claimErr)
	appendItemEvent(ctx, ws, shipmentID, "shipment_claim_rolled_back", map[string]any{
		"reverted_items": len(activatedIDs),
	})

	if len(rollbackErrs) > 0 {
		return fmt.Errorf("claim shipment %s: %w; rollback failed: %w", shipmentID, claimErr, errors.Join(rollbackErrs...))
	}
	return fmt.Errorf("claim shipment %s: %w", shipmentID, claimErr)
}

// ShipShipment closes a shipped scope, returns untouched descendants to backlog,
// archives the released artifacts, and records the closing commit in item logs.
func ShipShipment(ctx context.Context, ws *Workspace, shipmentID string, commit *CommitMetadata) (*ShipShipmentResult, error) {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}
	if shipment.Status != models.StatusActive {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, blerrors.ErrShipmentConflict)
	}

	// Fire pre-ship hooks (top-level).
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(shipment.Status)},
			NewValues:    map[string]any{"status": string(ShipmentShipped)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookShipShipment, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-ship hook: %w", err)
		}
	}

	explicitScope := uniqueNonEmptyStrings(NormalizeShipmentItems(shipment))
	releaseScope, err := releaseScopeItemIDs(ctx, ws, explicitScope)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: resolve release scope: %w", shipmentID, err)
	}

	// Two-level shipment gate (082-F ST4.2): validate member-task gate evidence
	// and run a shipment-level autoharness gate check over the full diff BEFORE
	// completing the release scope, so an ungated member is never auto-completed.
	// A refusal leaves shipment state unchanged.
	if err := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope); err != nil {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, err)
	}

	if err := completeReleaseScope(ctx, ws, releaseScope); err != nil {
		return nil, fmt.Errorf("ship shipment %s: complete release scope: %w", shipmentID, err)
	}

	featureIDs, err := featureScopeRoots(ctx, ws, explicitScope)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: resolve feature scope: %w", shipmentID, err)
	}

	returnedIDs := make([]string, 0)
	releaseScopeSet := toIDSet(releaseScope)
	for _, featureID := range featureIDs {
		returned, returnErr := returnUnreleasedFeatureItems(ctx, ws, featureID, releaseScopeSet)
		if returnErr != nil {
			return nil, fmt.Errorf("ship shipment %s: return unreleased feature items for %s: %w", shipmentID, featureID, returnErr)
		}
		returnedIDs = append(returnedIDs, returned...)
		if _, setErr := setArtifactStatus(ctx, ws, featureID, models.StatusDone, "feature released"); setErr != nil {
			return nil, fmt.Errorf("ship shipment %s: mark feature %s done: %w", shipmentID, featureID, setErr)
		}
	}

	if err := moveShipmentStatusWithTopLevel(ctx, ws, shipmentID, ShipmentShipped, false); err != nil {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, err)
	}

	archiveIDs, err := collectArchiveCandidateIDs(ctx, ws, shipmentID, releaseScope, featureIDs, returnedIDs)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: collect archive scope: %w", shipmentID, err)
	}

	if err := attachCommitToItems(ctx, ws, archiveIDs, commit); err != nil {
		return nil, fmt.Errorf("ship shipment %s: record commit traceability: %w", shipmentID, err)
	}

	archivedIDs, err := archiveItems(ctx, ws, archiveIDs)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: archive release scope: %w", shipmentID, err)
	}

	if err := VerifyPostShipConsistency(ctx, ws, archivedIDs); err != nil {
		return nil, fmt.Errorf("ship shipment %s: post-ship consistency: %w", shipmentID, err)
	}

	// Fire post-ship hooks (top-level).
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(shipment.Status)},
			NewValues:    map[string]any{"status": string(ShipmentShipped)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookShipShipment, hookCtx)
	}

	return &ShipShipmentResult{
		ShipmentID:     shipmentID,
		ShipmentStatus: string(ShipmentShipped),
		ArchivedIDs:    archivedIDs,
		ReturnedIDs:    uniqueNonEmptyStrings(returnedIDs),
		CommitSHA:      commitSHA(commit),
	}, nil
}

func completeReleaseScope(ctx context.Context, ws *Workspace, releaseScope []string) error {
	for _, itemID := range depthSortedIDs(releaseScope) {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return fmt.Errorf("load item %s: %w", itemID, err)
		}
		if item.Status == models.StatusBlocked {
			return fmt.Errorf("item %s is blocked and cannot ship: %w", itemID, blerrors.ErrShipmentConflict)
		}
		if isTerminalReleaseStatus(item.Status) {
			continue
		}
		if _, err := setArtifactStatus(ctx, ws, itemID, models.StatusDone, "shipment released"); err != nil {
			return fmt.Errorf("mark item %s done: %w", itemID, err)
		}
	}
	return nil
}

func returnUnreleasedFeatureItems(ctx context.Context, ws *Workspace, featureID string, releaseScope map[string]struct{}) ([]string, error) {
	descendants, err := descendantItems(ctx, ws, featureID)
	if err != nil {
		return nil, err
	}

	var returned []string
	for _, item := range descendants {
		if _, ok := releaseScope[item.ID]; ok {
			continue
		}
		if isTerminalReleaseStatus(item.Status) {
			continue
		}
		if item.Status != models.StatusQueued {
			if _, err := setArtifactStatus(ctx, ws, item.ID, models.StatusQueued, "returned to backlog after release"); err != nil {
				return nil, err
			}
		}
		// Clear parent_id so the orphaned item is visible as unparented backlog.
		// The hierarchical ID prefix preserves provenance without implying ownership.
		if err := clearParentID(ctx, ws, item.ID); err != nil {
			return nil, err
		}
		appendItemEvent(ctx, ws, item.ID, "returned_to_backlog", map[string]any{
			"feature_id": featureID,
		})
		returned = append(returned, item.ID)
	}
	return uniqueNonEmptyStrings(returned), nil
}

func collectArchiveCandidateIDs(ctx context.Context, ws *Workspace, shipmentID string, releaseScope, featureIDs, returnedIDs []string) ([]string, error) {
	candidates := []string{shipmentID}
	returnedSet := toIDSet(returnedIDs)

	for _, itemID := range releaseScope {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return nil, err
		}
		if _, returned := returnedSet[itemID]; returned || item.Status == models.StatusArchived {
			continue
		}
		if isTerminalReleaseStatus(item.Status) {
			candidates = append(candidates, item.ID)
		}
	}

	for _, featureID := range featureIDs {
		feature, err := loadArtifact(ctx, ws, featureID)
		if err != nil {
			return nil, err
		}
		if feature.Status != models.StatusArchived {
			candidates = append(candidates, feature.ID)
		}

		descendants, err := descendantItems(ctx, ws, featureID)
		if err != nil {
			return nil, err
		}
		for _, item := range descendants {
			if _, returned := returnedSet[item.ID]; returned || item.Status == models.StatusArchived {
				continue
			}
			if isTerminalReleaseStatus(item.Status) {
				candidates = append(candidates, item.ID)
			}
		}

		deliberations, err := linkedDeliberationIDs(ctx, ws, feature)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, deliberations...)
	}

	return uniqueNonEmptyStrings(candidates), nil
}

func attachCommitToItems(ctx context.Context, ws *Workspace, itemIDs []string, commit *CommitMetadata) error {
	if commit == nil || strings.TrimSpace(commit.SHA) == "" {
		return nil
	}
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		// Load from Markdown (source of truth): one authoritative read for
		// both the archived-status guard and the mutate-then-persist operation.
		// The DB fast-path carries neither item_links (stored in the separate
		// item_links table) nor archive provenance (unindexed), so using it
		// would drop those fields on re-persist and could give a stale status
		// on the archived-skip guard when the index has not been rehydrated.
		artifact, err := findArtifact(ctx, ws, itemID)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				return fmt.Errorf("reload item %s from markdown: %w", itemID, blerrors.ErrNotFound)
			}
			return fmt.Errorf("reload item %s from markdown: %w", itemID, err)
		}
		// 129.001-T: skip already-archived items — stamping a new shipment
		// commit on a pre-existing archived artifact is semantically wrong
		// (the artifact belonged to an earlier shipment), and the write-
		// boundary guard would refuse the re-persist without provenance anyway.
		if artifact.Status == models.StatusArchived {
			continue
		}
		// 129.002-T: the Markdown-loaded artifact carries item_links and
		// archive provenance; set commit and persist it so those fields survive
		// the rewrite (mirrors the MoveInQueue / serializer_provenance precedent).
		artifact.Commit = commit.SHA
		artifact.UpdatedAt = models.NowUTC()
		if err := persistArtifact(ctx, ws, artifact, false); err != nil {
			return fmt.Errorf("persist item %s commit: %w", itemID, err)
		}
		if err := LinkCommit(ctx, ws.DB, ws, itemID, commit.SHA, commit.Message, commit.Author); err != nil {
			return fmt.Errorf("link commit for %s: %w", itemID, err)
		}
	}
	return nil
}

func archiveItems(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	ordered := depthSortedIDs(itemIDs)
	archived := make([]string, 0, len(ordered))
	for _, itemID := range ordered {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return nil, fmt.Errorf("load item %s for archive: %w", itemID, err)
		}
		if item.Status == models.StatusArchived {
			continue
		}
		if _, err := ArchiveItem(ctx, ws.DB, ws, itemID, WithTopLevel(false)); err != nil {
			return nil, fmt.Errorf("archive item %s: %w", itemID, err)
		}
		archived = append(archived, itemID)
	}
	return archived, nil
}

func releaseScopeItemIDs(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	ids := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		if _, ok := seen[itemID]; !ok {
			seen[itemID] = struct{}{}
			ids = append(ids, itemID)
		}
		descendants, err := descendantItems(ctx, ws, itemID)
		if err != nil {
			return nil, err
		}
		for _, child := range descendants {
			if _, ok := seen[child.ID]; ok {
				continue
			}
			seen[child.ID] = struct{}{}
			ids = append(ids, child.ID)
		}
	}
	return ids, nil
}

func featureScopeRoots(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	seen := map[string]struct{}{}
	var featureIDs []string
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		currentID := itemID
		for currentID != "" {
			item, err := loadArtifact(ctx, ws, currentID)
			if err != nil {
				return nil, err
			}
			if item.ArtifactType == "feature" {
				if _, ok := seen[item.ID]; !ok {
					seen[item.ID] = struct{}{}
					featureIDs = append(featureIDs, item.ID)
				}
			}
			currentID = item.ParentID
		}
	}
	return featureIDs, nil
}

func linkedDeliberationIDs(ctx context.Context, ws *Workspace, feature *models.Artifact) ([]string, error) {
	if feature == nil {
		return nil, nil
	}
	var ids []string
	if feature.CustomFields != nil {
		if value, ok := feature.CustomFields["source_deliberation_id"].(string); ok && value != "" {
			ids = append(ids, value)
		}
	}
	ids = append(ids, deliberationIDPattern.FindAllString(feature.Description, -1)...)
	for _, ref := range feature.References {
		ids = append(ids, deliberationIDPattern.FindAllString(ref, -1)...)
	}

	unique := uniqueNonEmptyStrings(ids)
	valid := make([]string, 0, len(unique))
	for _, id := range unique {
		item, err := loadArtifact(ctx, ws, id)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if item.ArtifactType == "deliberation" {
			valid = append(valid, id)
		}
	}
	return valid, nil
}

func descendantItems(ctx context.Context, ws *Workspace, parentID string) ([]*models.Artifact, error) {
	queue := []string{parentID}
	var descendants []*models.Artifact
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		children, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{
			ParentID:        current,
			IncludeArchived: true,
		})
		if err != nil {
			return nil, fmt.Errorf("query children for %s: %w", current, err)
		}
		for _, child := range children {
			descendants = append(descendants, child)
			queue = append(queue, child.ID)
		}
	}
	return descendants, nil
}

func setArtifactStatus(ctx context.Context, ws *Workspace, itemID string, newStatus models.ArtifactStatus, reason string) (*models.Artifact, error) {
	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, err
	}
	if artifact.Status == newStatus {
		return artifact, nil
	}

	previous := artifact.Status
	artifact.Status = newStatus
	artifact.UpdatedAt = models.NowUTC()
	clearStaleBlockedReason(artifact, previous)
	if err := persistArtifact(ctx, ws, artifact, shouldRelocateOnStatusChange(previous, newStatus)); err != nil {
		return nil, err
	}
	appendItemEvent(ctx, ws, itemID, "status_changed", map[string]any{
		"from":   string(previous),
		"to":     string(newStatus),
		"reason": reason,
	})
	if err := cascadePersistedParentStatuses(ctx, ws, itemID); err != nil {
		return nil, err
	}
	return artifact, nil
}

func cascadePersistedParentStatuses(ctx context.Context, ws *Workspace, itemID string) error {
	item, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return err
	}
	if item.ParentID == "" {
		return nil
	}

	newStatus, err := ComputeParentStatus(ctx, ws.DB, item.ParentID)
	if err != nil {
		return err
	}
	parent, err := loadArtifact(ctx, ws, item.ParentID)
	if err != nil {
		return err
	}
	if parent.Status == newStatus {
		return nil
	}

	previous := parent.Status
	parent.Status = newStatus
	parent.UpdatedAt = models.NowUTC()
	clearStaleBlockedReason(parent, previous)
	if err := persistArtifact(ctx, ws, parent, shouldRelocateOnStatusChange(previous, newStatus)); err != nil {
		return err
	}
	appendItemEvent(ctx, ws, parent.ID, "status_changed", map[string]any{
		"from":   string(previous),
		"to":     string(newStatus),
		"reason": "child status rollup",
	})
	return cascadePersistedParentStatuses(ctx, ws, parent.ID)
}

// clearParentID removes the parent_id from an artifact, making it an
// unparented backlog item. The hierarchical ID prefix is preserved as
// provenance; only the active ownership link is severed.
func clearParentID(ctx context.Context, ws *Workspace, itemID string) error {
	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("clear parent_id for %s: %w", itemID, err)
	}
	if artifact.ParentID == "" {
		return nil
	}
	artifact.ParentID = ""
	artifact.UpdatedAt = models.NowUTC()
	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		return fmt.Errorf("persist cleared parent_id for %s: %w", itemID, err)
	}
	return nil
}

// AdoptItemResult summarizes the outcome of an adopt operation.
type AdoptItemResult struct {
	ItemID               string   `json:"item_id"`
	NewID                string   `json:"new_id,omitempty"`
	NewParentID          string   `json:"new_parent_id"`
	OriginFeature        string   `json:"origin_feature,omitempty"`
	IsOrphan             bool     `json:"was_orphan"`
	RewrittenArtifactIDs []string `json:"rewritten_artifact_ids,omitempty"`
}

// AdoptItem sets an orphaned or unparented item's parent_id to a new feature,
// atomically rewriting its hierarchical ID, renaming files, updating dependency
// and link edges, and syncing the index. The return value includes the new ID
// so callers can update their own references. Adoption rewrites internal
// backlogit references only; external references are the caller's responsibility.
func AdoptItem(ctx context.Context, ws *Workspace, itemID, newParentID string) (*AdoptItemResult, error) {
	if newParentID == "" {
		return nil, fmt.Errorf("adopt item %s: new_parent_id is required", itemID)
	}

	// Validate the new parent exists.
	if _, err := loadArtifact(ctx, ws, newParentID); err != nil {
		return nil, fmt.Errorf("adopt item %s: load new parent %s: %w", itemID, newParentID, err)
	}

	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("adopt item %s: %w", itemID, err)
	}

	if artifact.Status == models.StatusArchived {
		return nil, fmt.Errorf("adopt item %s: cannot adopt an archived item", itemID)
	}

	oldParentID := artifact.ParentID

	// Fire pre-adopt hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"parent_id": oldParentID},
			NewValues:    map[string]any{"parent_id": newParentID},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookAdoptItem, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-adopt hook: %w", err)
		}
	}

	wasOrphan := IsOrphan(artifact)

	// Record origin_feature from the ID prefix if not already set.
	originFeature := extractOriginFeatureID(ws, itemID)
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	if _, exists := artifact.CustomFields["origin_feature"]; !exists && originFeature != "" {
		artifact.CustomFields["origin_feature"] = originFeature
	}

	// Step 1: Generate the new hierarchical ID under the new parent.
	oldID := artifact.ID
	newID := oldID // fallback: keep old ID if we can't generate a new one
	if ws.Config != nil && ws.Config.QueueLayout != nil {
		typeCfg, typeOK := ws.Config.ArtifactTypes[artifact.ArtifactType]
		if typeOK && typeCfg != nil {
			generatedID, idErr := NextTypedHierarchicalID(
				ctx, ws.DB, newParentID, artifact.ArtifactType,
				typeCfg, ws.Config.QueueLayout,
			)
			if idErr == nil {
				newID = generatedID
			}
		}
	}

	// Update the artifact with new parent and ID.
	artifact.ParentID = newParentID
	artifact.ID = newID
	artifact.UpdatedAt = models.NowUTC()

	// Scan for other artifacts that reference oldID in their frontmatter.
	// This is done outside the transaction (read-only) so the walk does not
	// contend with the write transaction that follows.
	crossRefs, crossRefErr := findCrossArtifactReferences(ctx, ws, oldID, newID)
	if crossRefErr != nil {
		return nil, fmt.Errorf("adopt item %s: scan cross-references: %w", oldID, crossRefErr)
	}

	// Step 2: Begin DB transaction for edge rewrites and index sync.
	// durSyncErr accumulates post-mutation directory-fsync failures from the
	// ID-change branch. It is surfaced as ErrWriteIndeterminate just before the
	// successful return (commit-then-surface) so the completed adopt is never
	// rolled back yet the durability signal is not silently discarded.
	var durSyncErr error
	if ws.DB != nil && newID != oldID {
		tx, txErr := ws.DB.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, fmt.Errorf("adopt item %s: begin tx: %w", oldID, txErr)
		}
		defer tx.Rollback() //nolint:errcheck

		// Compute log paths: absolute paths for filesystem ops, relative
		// (to .backlogit/) paths for the DB to match IndexEvent's convention.
		logsDir := WorkspaceLogsRoot(ws.RootPath)
		oldLogPath := filepath.Join(logsDir, oldID+".jsonl")
		newLogPath := filepath.Join(logsDir, newID+".jsonl")
		newRelLogPath := filepath.ToSlash(filepath.Join("logs", newID+".jsonl"))

		// Rewrite dependency and link edges.
		if err := bldb.RewriteDependencyEdges(ctx, tx, oldID, newID); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}
		if err := bldb.RewriteLinkEdges(ctx, tx, oldID, newID); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}

		// Rewrite ancillary references (commit_links, stash_links, item_logs,
		// item_log_entries) so the index remains fully self-consistent.
		if err := bldb.RewriteAncillaryReferences(ctx, tx, oldID, newID, newRelLogPath); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}

		// Delete old index row and insert new one.
		// Use a non-cascading delete — edges are already rewritten above.
		if _, delErr := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, oldID); delErr != nil {
			return nil, fmt.Errorf("adopt item %s: delete old index: %w", oldID, delErr)
		}
		if err := bldb.UpsertItemTx(ctx, tx, artifact); err != nil {
			return nil, fmt.Errorf("adopt item %s: upsert new index: %w", oldID, err)
		}

		// Step 3: File operations — rename .md and .jsonl files.
		oldMDPath, findErr := FindArtifactPath(ctx, ws, oldID)
		if findErr != nil && !errors.Is(findErr, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("adopt item %s: find old md: %w", oldID, findErr)
		}

		var renamedMD, renamedLog bool
		var newMDPath string
		// 060.004-T: Snapshot the old file content before overwriting with the new
		// ID so every rollback path can restore the exact original file content.
		// Without this, the rename-based rollback places a file with frontmatter
		// "id: newID" at oldMDPath, making the artifact undiscoverable by oldID.
		var oldMDRaw []byte

		if findErr == nil {
			// Snapshot the original content before any file operations.
			oldMDRaw, err = os.ReadFile(oldMDPath)
			if err != nil {
				return nil, fmt.Errorf("adopt item %s: read old md: %w", oldID, err)
			}

			// Compute new filename using the configured naming resolver to
			// respect artifact_types[*].file_name_format when configured.
			dir := filepath.Dir(oldMDPath)
			newFileName := newID // default: use the artifact ID as filename
			if ws.Config != nil {
				if typeCfg, ok := ws.Config.ArtifactTypes[artifact.ArtifactType]; ok && typeCfg != nil {
					newFileName = ResolveFileName(typeCfg, newID, artifact.Title, ws.Config.MaxSlugLength)
				}
			}
			newMDPath = filepath.Join(dir, newFileName+".md")

			// Write updated artifact content (with new ID in frontmatter) to new path.
			if writeErr := WriteArtifactFileWithOptions(artifact, newMDPath, WorkspaceDurableWrites(ws)); writeErr != nil {
				return nil, fmt.Errorf("adopt item %s: write new md: %w", oldID, writeErr)
			}
			if newMDPath != oldMDPath {
				if rmErr := os.Remove(oldMDPath); rmErr != nil && !os.IsNotExist(rmErr) {
					// Rollback: remove the new file we just wrote
					_ = os.Remove(newMDPath)
					return nil, fmt.Errorf("adopt item %s: remove old md: %w", oldID, rmErr)
				}
				renamedMD = true
				// Durable same-directory rename: the artifact write fsynced the new
				// dirent, but the old-ID entry was just removed afterward and that
				// removal is not durable until the directory is fsynced again. This
				// runs after the in-tx DB mutations, so a failure is NOT rolled back
				// (the rename likely persisted); it is accumulated and surfaced as
				// ErrWriteIndeterminate after tx.Commit (commit-then-surface).
				if e := durableSyncDirDetailed(ws, filepath.Dir(oldMDPath), "adopt md rename"); e != nil {
					durSyncErr = errors.Join(durSyncErr, e)
				}
			}
		}

		// Rewrite frontmatter in other artifacts that reference oldID.
		if applyErr := applyCrossArtifactRewrites(ctx, tx, ws, crossRefs); applyErr != nil {
			if renamedMD {
				rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
			}
			return nil, fmt.Errorf("adopt item %s: apply cross-artifact rewrites: %w", oldID, applyErr)
		}

		// Rename log file if it exists.
		if _, statErr := os.Stat(oldLogPath); statErr == nil {
			if renameErr := os.Rename(oldLogPath, newLogPath); renameErr != nil {
				// Rollback MD rename
				if renamedMD {
					rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
				}
				return nil, fmt.Errorf("adopt item %s: rename log: %w", oldID, renameErr)
			}
			renamedLog = true
			// Durable log rename: both the new and removed dirents live in logsDir;
			// fsync it so the rename is durable. Same commit-then-surface handling
			// as the MD rename above: a failure is accumulated and surfaced as
			// ErrWriteIndeterminate after commit, never rolled back.
			if e := durableSyncDirDetailed(ws, filepath.Dir(oldLogPath), "adopt log rename"); e != nil {
				durSyncErr = errors.Join(durSyncErr, e)
			}
		}

		// Step 4: Commit the transaction now that all file ops succeeded.
		if commitErr := tx.Commit(); commitErr != nil {
			// Rollback file operations
			if renamedLog {
				_ = os.Rename(newLogPath, oldLogPath)
			}
			if renamedMD {
				rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
			}
			for _, u := range crossRefs {
				tmp := u.filePath + ".rollback-tmp"
				if writeErr := os.WriteFile(tmp, u.snapshotRaw, 0o644); writeErr != nil {
					slog.Warn("adopt item: rollback cross-ref write failed", "path", u.filePath, "error", writeErr)
					continue
				}
				if renameErr := os.Rename(tmp, u.filePath); renameErr != nil {
					slog.Warn("adopt item: rollback cross-ref rename failed", "path", u.filePath, "error", renameErr)
					_ = os.Remove(tmp)
				}
			}
			return nil, fmt.Errorf("adopt item %s: commit tx: %w", oldID, commitErr)
		}
	} else {
		// No ID change or no DB — just persist the artifact with updated parent.
		if err := persistArtifact(ctx, ws, artifact, false); err != nil {
			return nil, fmt.Errorf("adopt item %s: persist: %w", itemID, err)
		}
		if ws.DB != nil {
			if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
				return nil, fmt.Errorf("adopt item %s: index: %w", itemID, err)
			}
		}
	}

	// Build list of rewritten artifact IDs for the event delta and result.
	rewrittenIDs := make([]string, 0, len(crossRefs))
	for _, u := range crossRefs {
		rewrittenIDs = append(rewrittenIDs, u.artifact.ID)
	}

	appendItemEvent(ctx, ws, newID, "adopted", map[string]any{
		"old_id":                 oldID,
		"new_id":                 newID,
		"new_parent_id":          newParentID,
		"origin_feature":         originFeature,
		"was_orphan":             wasOrphan,
		"rewritten_artifact_ids": rewrittenIDs,
	})

	// Fire post-adopt hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       newID,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"parent_id": oldParentID, "id": oldID},
			NewValues:    map[string]any{"parent_id": newParentID, "id": newID},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookAdoptItem, hookCtx)
	}

	return &AdoptItemResult{
		ItemID:               oldID,
		NewID:                newID,
		NewParentID:          newParentID,
		OriginFeature:        originFeature,
		IsOrphan:             wasOrphan,
		RewrittenArtifactIDs: rewrittenIDs,
	}, adoptDurabilityErr(newID, durSyncErr)
}

// adoptDurabilityErr returns a wrapped ErrWriteIndeterminate when a post-mutation
// directory fsync failed during a committed adopt, and nil otherwise. The adopt
// already applied (files renamed, tx committed, event appended), so the caller
// still receives the fully-built result; wrapping the sentinel lets an err-only
// caller and blerrors.IsWriteIndeterminate see the honest durability signal
// without triggering a rollback of the completed operation.
func adoptDurabilityErr(newID string, durSyncErr error) error {
	if durSyncErr == nil {
		return nil
	}
	return fmt.Errorf("adopt item %s applied but durability is indeterminate: %w: %w",
		newID, blerrors.ErrWriteIndeterminate, durSyncErr)
}

// IsOrphan returns true when an item has no parent_id but its hierarchical ID
// suggests it originated under a parent (contains a dot separator).
func IsOrphan(a *models.Artifact) bool {
	return a.ParentID == "" && strings.Contains(a.ID, ".")
}

// rollbackMDFile undoes a WriteArtifactFile + os.Remove pair that advanced
// the artifact from oldMDPath → newMDPath. It removes newMDPath and restores
// oldMDPath to its original byte content. If oldMDRaw is empty it falls back
// to a simple rename (pre-060.004-T behaviour).
func rollbackMDFile(newMDPath, oldMDPath string, oldMDRaw []byte) {
	if len(oldMDRaw) == 0 {
		// Fallback: rename may place new-ID content at oldMDPath, but it is
		// better than leaving the workspace with no file at all.
		if renameErr := os.Rename(newMDPath, oldMDPath); renameErr != nil {
			slog.Warn("adopt item: rollback md rename failed", "from", newMDPath, "to", oldMDPath, "error", renameErr)
		}
		return
	}

	if removeErr := os.Remove(newMDPath); removeErr != nil && !os.IsNotExist(removeErr) {
		slog.Warn("adopt item: rollback md remove failed", "path", newMDPath, "error", removeErr)
	}
	if restoreErr := os.WriteFile(oldMDPath, oldMDRaw, 0o644); restoreErr != nil {
		slog.Warn("adopt item: rollback md content restore failed", "path", oldMDPath, "error", restoreErr)
	}
}

// extractIDPrefix returns the portion of a hierarchical ID before the last dot
// segment. For "F015.T009" it returns "F015". For "T009" it returns "".
func extractIDPrefix(id string) string {
	idx := strings.LastIndex(id, ".")
	if idx < 0 {
		return ""
	}
	return id[:idx]
}

func extractOriginFeatureID(ws *Workspace, id string) string {
	prefix := extractIDPrefix(id)
	if prefix == "" {
		return ""
	}

	root := prefix
	if idx := strings.Index(prefix, "."); idx >= 0 {
		root = prefix[:idx]
	}
	if strings.Contains(root, "-") {
		return root
	}

	digits := leadingDigits(root)
	if digits == "" || ws == nil || ws.Config == nil {
		return prefix
	}
	featureCfg, ok := ws.Config.ArtifactTypes["feature"]
	if !ok || featureCfg == nil || featureCfg.Suffix == "" {
		return prefix
	}
	return digits + featureCfg.Suffix
}

func shouldRelocateOnStatusChange(previous models.ArtifactStatus, next models.ArtifactStatus) bool {
	return previous != next
}

func isTerminalReleaseStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusDone, models.StatusAccepted, models.StatusRejected, models.StatusArchived:
		return true
	default:
		return false
	}
}

// isDescopeEligibleStatus reports whether a member archived FROM the given status
// is a GENUINE DESCOPE — scaffolded then removed from the release before shipping a
// deliverable — and is therefore exempt from the per-member F4 gate-evidence
// requirement. Two status classes qualify:
//
//   - In-flight statuses (queued, active, blocked, review) never reached completion.
//   - Non-completion terminals (abandoned, rejected) ended the item WITHOUT shipping
//     a deliverable, so there is no completion contract to gate.
//
// COMPLETION statuses (done, accepted, shipped) are NEVER descope-eligible: a member
// driven to completion and then archived MUST still present valid gate evidence, or
// the F4 fail-open evidence predicate would be bypassed (a completed member whose only
// "pass" is an EventGatePassed{ran:false} carries no valid evidence yet could be
// archived after the fact). The archived sink status is excluded because it is not a
// pre-archive provenance value. This predicate is distinct from isTerminalReleaseStatus
// (which governs relocation and lifecycle transitions and MUST NOT change): terminality
// and descope-eligibility are orthogonal — rejected is terminal yet descope-eligible,
// abandoned is non-terminal yet descope-eligible, and shipped is non-terminal yet a
// completion (never descope-eligible).
func isDescopeEligibleStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked,
		models.StatusReview, models.StatusAbandoned, models.StatusRejected:
		return true
	default:
		return false
	}
}

// isRecognizedReleaseStatus reports whether status is one of the known artifact
// lifecycle statuses. Unrecognized (malformed/typo) provenance must be treated as
// unknown so safety-critical callers can fail closed rather than misclassify it:
// isTerminalReleaseStatus returns false for any unknown value, so an exemption gated
// only on !isTerminalReleaseStatus would wrongly treat garbage provenance as a
// non-terminal descope.
func isRecognizedReleaseStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked,
		models.StatusReview, models.StatusDone, models.StatusAccepted,
		models.StatusRejected, models.StatusArchived, models.StatusShipped,
		models.StatusAbandoned:
		return true
	default:
		return false
	}
}

func toIDSet(itemIDs []string) map[string]struct{} {
	set := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		set[itemID] = struct{}{}
	}
	return set
}

func depthSortedIDs(itemIDs []string) []string {
	ids := uniqueNonEmptyStrings(itemIDs)
	sort.Slice(ids, func(i, j int) bool {
		leftDepth := strings.Count(ids[i], ".")
		rightDepth := strings.Count(ids[j], ".")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return ids[i] < ids[j]
	})
	return ids
}

func commitSHA(commit *CommitMetadata) string {
	if commit == nil {
		return ""
	}
	return strings.TrimSpace(commit.SHA)
}
