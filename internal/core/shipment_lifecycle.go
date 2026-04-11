package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	bldb "github.com/backlogit/backlogit/internal/db"
	blerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
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

// ClaimShipment moves a queued shipment to active and marks the included work scope active.
func ClaimShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	if err := MoveShipmentStatus(ctx, ws, shipmentID, ShipmentActive); err != nil {
		return nil, err
	}

	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}

	for _, itemID := range shipmentItems(shipment) {
		item, loadErr := loadArtifact(ctx, ws, itemID)
		if loadErr != nil {
			return nil, fmt.Errorf("claim shipment %s: load item %s: %w", shipmentID, itemID, loadErr)
		}
		if item.Status == models.StatusQueued {
			if _, setErr := setArtifactStatus(ctx, ws, itemID, models.StatusActive, "shipment claimed"); setErr != nil {
				return nil, fmt.Errorf("claim shipment %s: activate item %s: %w", shipmentID, itemID, setErr)
			}
		}
	}

	return GetShipment(ctx, ws, shipmentID)
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

	explicitScope := uniqueNonEmptyStrings(shipmentItems(shipment))
	releaseScope, err := releaseScopeItemIDs(ctx, ws, explicitScope)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: resolve release scope: %w", shipmentID, err)
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

	if err := MoveShipmentStatus(ctx, ws, shipmentID, ShipmentShipped); err != nil {
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
		artifact, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return fmt.Errorf("load item %s for commit link: %w", itemID, err)
		}
		artifact.Commit = commit.SHA
		artifact.UpdatedAt = time.Now()
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
		if _, err := ArchiveItem(ctx, ws.DB, ws, itemID); err != nil {
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
	artifact.UpdatedAt = time.Now()
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
	parent.UpdatedAt = time.Now()
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
	artifact.UpdatedAt = time.Now()
	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		return fmt.Errorf("persist cleared parent_id for %s: %w", itemID, err)
	}
	return nil
}

// AdoptItemResult summarizes the outcome of an adopt operation.
type AdoptItemResult struct {
	ItemID        string `json:"item_id"`
	NewParentID   string `json:"new_parent_id"`
	OriginFeature string `json:"origin_feature,omitempty"`
	IsOrphan      bool   `json:"was_orphan"`
}

// AdoptItem sets an orphaned or unparented item's parent_id to a new feature,
// preserving the original hierarchical ID as provenance. If the item had a
// previous parent encoded in its ID prefix, that prefix is recorded in the
// origin_feature custom field for lineage.
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

	wasOrphan := IsOrphan(artifact)
	previousParent := artifact.ParentID

	// Record origin_feature from the ID prefix if not already set.
	originFeature := extractOriginFeatureID(ws, itemID)
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	if _, exists := artifact.CustomFields["origin_feature"]; !exists && originFeature != "" {
		artifact.CustomFields["origin_feature"] = originFeature
	}

	artifact.ParentID = newParentID
	artifact.UpdatedAt = time.Now()

	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		return nil, fmt.Errorf("adopt item %s: persist: %w", itemID, err)
	}

	appendItemEvent(ctx, ws, itemID, "adopted", map[string]any{
		"new_parent_id":   newParentID,
		"previous_parent": previousParent,
		"origin_feature":  originFeature,
	})

	return &AdoptItemResult{
		ItemID:        itemID,
		NewParentID:   newParentID,
		OriginFeature: originFeature,
		IsOrphan:      wasOrphan,
	}, nil
}

// IsOrphan returns true when an item has no parent_id but its hierarchical ID
// suggests it originated under a parent (contains a dot separator).
func IsOrphan(a *models.Artifact) bool {
	return a.ParentID == "" && strings.Contains(a.ID, ".")
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
