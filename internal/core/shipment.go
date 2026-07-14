package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ShipmentStatus represents the lifecycle state of a shipment.
type ShipmentStatus string

const (
	// ShipmentQueued indicates the shipment is created but not yet started.
	ShipmentQueued ShipmentStatus = "queued"
	// ShipmentActive indicates the shipment is in progress.
	ShipmentActive ShipmentStatus = "active"
	// ShipmentShipped indicates the shipment has been delivered.
	ShipmentShipped ShipmentStatus = "shipped"
	// ShipmentAbandoned indicates the shipment was cancelled.
	ShipmentAbandoned ShipmentStatus = "abandoned"
)

type fileSnapshot struct {
	Path    string
	Exists  bool
	Content []byte
}

type returnBlockedJournal struct {
	Shipment *models.Artifact `json:"shipment"`
	Item     *models.Artifact `json:"item"`
}

// CreateShipment creates a new shipment artifact in the workspace with the given
// title and associates the specified item IDs with it. The shipment owns the items
// list as an aggregate root.
//
// Worker: Create a new shipment Markdown artifact with YAML frontmatter containing
// the items list, set status to queued, generate ID with S prefix, write to queue
// directory, and upsert into the database index.
func CreateShipment(ctx context.Context, ws *Workspace, title string, itemIDs []string) (*models.Artifact, error) {
	items := uniqueNonEmptyStrings(itemIDs)
	if err := validateShipmentItemIDs(ctx, ws, "", items); err != nil {
		return nil, fmt.Errorf("create shipment %q: %w", title, err)
	}

	shipment, err := CreateArtifact(ctx, ws, title, "shipment", WithFields(map[string]any{"items": items}))
	if err != nil {
		return nil, fmt.Errorf("create shipment %q: %w", title, err)
	}
	normalizeShipmentArtifact(shipment)
	if err := bldb.UpsertItem(ctx, ws.DB, shipment); err != nil {
		return nil, fmt.Errorf("create shipment %s: %w", shipment.ID, err)
	}

	appendItemEvent(ctx, ws, shipment.ID, "shipment_created", map[string]any{
		"title": title,
		"items": items,
	})

	return shipment, nil
}

// GetShipment retrieves a shipment artifact by ID from the workspace.
//
// Worker: Look up the shipment by ID in the database index, parse the Markdown
// source file, and return the populated Artifact with items list in CustomFields.
func GetShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	artifact, err := loadArtifact(ctx, ws, shipmentID)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
		}
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, err)
	}
	if artifact.ArtifactType != "shipment" {
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}

	normalizeShipmentArtifact(artifact)
	return artifact, nil
}

// MoveShipmentStatus transitions a shipment's status. Valid transitions:
// queued->active, active->shipped, active->abandoned.
//
// Worker: Validate the transition is legal, update the Markdown frontmatter,
// rewrite the file atomically, and upsert the new status into the database.
// Emit slog.Info for status transition and events.jsonl record.
func MoveShipmentStatus(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus) error {
	return moveShipmentStatusWithTopLevel(ctx, ws, shipmentID, newStatus, true)
}

// moveShipmentStatusWithTopLevel is the internal variant that accepts an explicit
// topLevel flag. Top-level callers use true; nested callers (e.g. ShipShipment)
// use false to suppress duplicate post-hook event emission.
func moveShipmentStatusWithTopLevel(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus, topLevel bool) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}
	oldShipmentStatus := shipment.Status

	if !isValidShipmentTransition(shipment.Status, newStatus) {
		return fmt.Errorf(
			"move shipment %s from %s to %s: %w",
			shipmentID,
			shipment.Status,
			newStatus,
			blerrors.ErrShipmentConflict,
		)
	}

	// Fire pre-move-shipment-status hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(oldShipmentStatus)},
			NewValues:    map[string]any{"status": string(newStatus)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     topLevel,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookMoveShipmentStatus, hookCtx); err != nil {
			return fmt.Errorf("pre-move-shipment-status hook: %w", err)
		}
	}

	shipment.Status = models.ArtifactStatus(newStatus)
	shipment.UpdatedAt = models.NowUTC()
	if err := persistArtifact(ctx, ws, shipment, true); err != nil {
		return fmt.Errorf("move shipment %s: %w", shipmentID, err)
	}

	slog.InfoContext(ctx, "shipment status changed", "shipment_id", shipmentID, "new_status", newStatus)
	appendItemEvent(ctx, ws, shipmentID, "shipment_status_changed", map[string]any{
		"status": string(newStatus),
	})

	// Fire post-move-shipment-status hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(oldShipmentStatus)},
			NewValues:    map[string]any{"status": string(newStatus)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     topLevel,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookMoveShipmentStatus, hookCtx)
	}

	return nil
}

// AddItemToShipment associates an artifact ID with a shipment. The item must not
// already belong to another active shipment.
//
// Worker: Check the item is not already assigned to another shipment (return
// ErrItemAlreadyAssigned if so), append the item ID to the shipment's items list
// in frontmatter, rewrite the file atomically, and upsert into the database.
// Emit slog.Debug for item association and events.jsonl record.
func AddItemToShipment(ctx context.Context, ws *Workspace, shipmentID, itemID string) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}
	if shipmentMutationBlocked(shipment.Status) {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, blerrors.ErrShipmentConflict)
	}

	items := NormalizeShipmentItems(shipment)
	if containsString(items, itemID) {
		return nil
	}
	if err := validateShipmentItemIDs(ctx, ws, shipmentID, []string{itemID}); err != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, err)
	}

	items = append(items, itemID)
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = items
	shipment.UpdatedAt = models.NowUTC()
	if err := persistArtifact(ctx, ws, shipment, false); err != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, err)
	}

	slog.DebugContext(ctx, "shipment item added", "shipment_id", shipmentID, "item_id", itemID)
	appendItemEvent(ctx, ws, shipmentID, "shipment_item_added", map[string]any{
		"item_id": itemID,
	})

	return nil
}

// ReturnBlockedItem removes an item from shipment and sets its status to blocked
// with a reason. This is an explicit operation, not a side effect.
//
// Worker: Remove the item ID from the shipment's items list, set the item's status
// to blocked, append a blocked_reason field to the item's frontmatter, rewrite
// both files atomically, and emit slog.Info + events.jsonl for the return.
// Return ErrCannotReturnItem if the item is not in this shipment.
func ReturnBlockedItem(ctx context.Context, ws *Workspace, shipmentID, itemID, reason string) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}
	if shipmentMutationBlocked(shipment.Status) {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, blerrors.ErrShipmentConflict)
	}

	items := NormalizeShipmentItems(shipment)
	if !containsString(items, itemID) {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, blerrors.ErrCannotReturnItem)
	}

	item, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load item %s: %w", itemID, err)
	}
	originalShipment := cloneArtifact(shipment)
	originalItem := cloneArtifact(item)
	if err := writeReturnBlockedJournal(ws.RootPath, originalShipment, originalItem); err != nil {
		return fmt.Errorf("journal return of item %s from shipment %s: %w", itemID, shipmentID, err)
	}

	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = removeString(items, itemID)
	shipment.UpdatedAt = models.NowUTC()

	if item.CustomFields == nil {
		item.CustomFields = map[string]any{}
	}
	item.Status = models.StatusBlocked
	item.CustomFields["blocked_reason"] = reason
	item.UpdatedAt = models.NowUTC()

	rolledBack, err := persistReturnedBlockedArtifacts(ctx, ws, originalShipment, shipment, originalItem, item)
	if err != nil {
		if rolledBack {
			removeReturnBlockedJournal(ctx, ws.RootPath, originalShipment.ID, originalItem.ID)
		}
		return err
	}
	removeReturnBlockedJournal(ctx, ws.RootPath, originalShipment.ID, originalItem.ID)

	slog.InfoContext(ctx, "shipment item returned blocked", "shipment_id", shipmentID, "item_id", itemID)
	appendItemEvent(ctx, ws, shipmentID, "shipment_item_returned_blocked", map[string]any{
		"item_id": itemID,
		"reason":  reason,
	})
	appendItemEvent(ctx, ws, itemID, "item_blocked", map[string]any{
		"shipment_id":    shipmentID,
		"blocked_reason": reason,
	})

	return nil
}

func appendItemEvent(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) {
	appendItemEventWithCommit(ctx, ws, itemID, eventType, delta, "")
}

// clearStaleBlockedReason removes blocked_reason metadata once an artifact
// leaves the blocked status. The reason is only meaningful while the item is
// blocked; retaining it after the item re-enters the backlog (queued) or is
// re-activated misrepresents the item's current availability.
func clearStaleBlockedReason(artifact *models.Artifact, previousStatus models.ArtifactStatus) {
	if artifact == nil || artifact.CustomFields == nil {
		return
	}
	if previousStatus == models.StatusBlocked && artifact.Status != models.StatusBlocked {
		delete(artifact.CustomFields, "blocked_reason")
	}
}

func appendItemEventWithCommit(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any, commitSHA string) {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: eventType,
		Delta:     delta,
		CommitSHA: commitSHA,
	}

	writer := events.NewEventWriter(logsDir)
	if err := writer.AppendEvent(ctx, event); err != nil {
		slog.WarnContext(ctx, "append shipment event", "item_id", itemID, "event_type", eventType, "error", err)
		return
	}
	if err := bldb.IndexEvent(ctx, ws.DB, logsDir, event); err != nil {
		slog.WarnContext(ctx, "index shipment event", "item_id", itemID, "event_type", eventType, "error", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isValidShipmentTransition(current models.ArtifactStatus, next ShipmentStatus) bool {
	switch current {
	case models.StatusQueued:
		return next == ShipmentActive
	case models.StatusActive:
		return next == ShipmentShipped || next == ShipmentAbandoned
	default:
		return false
	}
}

func loadArtifact(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	artifact, err := bldb.GetItem(ctx, ws.DB, id)
	if err == nil {
		return artifact, nil
	}
	if !errors.Is(err, blerrors.ErrNotFound) {
		return nil, err
	}

	artifact, findErr := findArtifact(ctx, ws, id)
	if findErr != nil {
		if errors.Is(findErr, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("load artifact %s: %w", id, blerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("load artifact %s: %w", id, findErr)
	}
	if upsertErr := bldb.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
		return nil, fmt.Errorf("upsert artifact %s: %w", id, upsertErr)
	}
	return artifact, nil
}

func normalizeShipmentArtifact(artifact *models.Artifact) {
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	artifact.CustomFields["items"] = NormalizeShipmentItems(artifact)
}

func persistArtifact(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) error {
	if err := artifact.Validate(); err != nil {
		return fmt.Errorf("validate artifact: %w", err)
	}

	currentPath, targetPath, err := resolveArtifactPersistPaths(ctx, ws, artifact, relocate)
	if err != nil {
		return err
	}
	currentSnapshot, err := snapshotFile(currentPath)
	if err != nil {
		return fmt.Errorf("snapshot current artifact file: %w", err)
	}
	targetSnapshot, err := snapshotFile(targetPath)
	if err != nil {
		return fmt.Errorf("snapshot target artifact file: %w", err)
	}

	if currentPath != targetPath {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear target artifact path: %w", err)
		}
	}
	if err := WriteArtifactFile(artifact, targetPath); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	if currentPath != targetPath {
		if err := os.Remove(currentPath); err != nil {
			cleanupErr := os.Remove(targetPath)
			if cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				return fmt.Errorf("remove old artifact file: %w; cleanup new artifact file: %v", err, cleanupErr)
			}
			return fmt.Errorf("remove old artifact file: %w", err)
		}
	}
	if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
		if restoreErr := restorePersistedArtifactFiles(currentSnapshot, targetSnapshot, currentPath, targetPath); restoreErr != nil {
			return fmt.Errorf("upsert item: %w; restore files: %v", err, restoreErr)
		}
		return fmt.Errorf("upsert item: %w", err)
	}
	return nil
}

func resolveArtifactPersistPaths(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) (string, string, error) {
	currentPath, err := FindArtifactPath(ctx, ws, artifact.ID)
	if err != nil {
		return "", "", fmt.Errorf("find artifact path: %w", err)
	}
	if !relocate {
		return currentPath, currentPath, nil
	}

	backlogitDir := WorkspaceStorageRoot(ws.RootPath)
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return "", "", fmt.Errorf("load registry: %w", err)
	}
	targetDir := ResolveTargetDir(registry, artifact.ArtifactType, string(artifact.Status))

	currentRel, err := filepath.Rel(backlogitDir, filepath.Dir(currentPath))
	if err != nil {
		return "", "", fmt.Errorf("resolve relative path: %w", err)
	}
	if filepath.Clean(currentRel) == filepath.Clean(targetDir) {
		return currentPath, currentPath, nil
	}

	targetDirAbs := filepath.Join(backlogitDir, targetDir)
	if err := os.MkdirAll(targetDirAbs, 0o755); err != nil {
		return "", "", fmt.Errorf("create directory %s: %w", targetDirAbs, err)
	}
	targetPath := filepath.Join(targetDirAbs, filepath.Base(currentPath))
	return currentPath, targetPath, nil
}

func persistReturnedBlockedArtifacts(ctx context.Context, ws *Workspace, originalShipment, updatedShipment, originalItem, updatedItem *models.Artifact) (bool, error) {
	if err := persistArtifact(ctx, ws, updatedShipment, false); err != nil {
		return false, fmt.Errorf("update shipment %s: %w", updatedShipment.ID, err)
	}
	if err := persistArtifact(ctx, ws, updatedItem, true); err != nil {
		if rollbackErr := rollbackReturnedBlockedArtifacts(ctx, ws, originalShipment, originalItem); rollbackErr != nil {
			return false, fmt.Errorf("update item %s: %w; rollback failed: %w", updatedItem.ID, err, rollbackErr)
		}
		return true, fmt.Errorf("update item %s: %w", updatedItem.ID, err)
	}
	return false, nil
}

func rollbackReturnedBlockedArtifacts(ctx context.Context, ws *Workspace, originalShipment, originalItem *models.Artifact) error {
	var rollbackErrs []error
	if err := persistArtifact(ctx, ws, originalItem, true); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("restore item %s: %w", originalItem.ID, err))
	}
	if err := persistArtifact(ctx, ws, originalShipment, false); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("restore shipment %s: %w", originalShipment.ID, err))
	}
	return errors.Join(rollbackErrs...)
}

func validateShipmentItemIDs(ctx context.Context, ws *Workspace, currentShipmentID string, itemIDs []string) error {
	normalizedIDs := uniqueNonEmptyStrings(itemIDs)
	if len(normalizedIDs) == 0 {
		return nil
	}

	shipments, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{Type: "shipment", IncludeArchived: true})
	if err != nil {
		return fmt.Errorf("list shipments: %w", err)
	}

	activeAssignments := make(map[string]string)
	for _, existing := range shipments {
		if existing.ID == currentShipmentID || !shipmentStatusBlocksAssignment(existing.Status) {
			continue
		}
		for _, assignedItemID := range NormalizeShipmentItems(existing) {
			activeAssignments[assignedItemID] = existing.ID
		}
	}

	for _, itemID := range normalizedIDs {
		if itemID == currentShipmentID {
			return fmt.Errorf("shipment %s cannot include itself: %w", currentShipmentID, blerrors.ErrValidation)
		}

		artifact, loadErr := loadArtifact(ctx, ws, itemID)
		if loadErr != nil {
			return loadErr
		}
		if artifact.ArtifactType == "shipment" {
			return fmt.Errorf("artifact %s is a shipment and cannot be nested in a shipment: %w", itemID, blerrors.ErrValidation)
		}
		if assignedShipmentID, ok := activeAssignments[itemID]; ok {
			return fmt.Errorf("item %s already assigned to shipment %s: %w", itemID, assignedShipmentID, blerrors.ErrItemAlreadyAssigned)
		}
	}

	return nil
}

func shipmentStatusBlocksAssignment(status models.ArtifactStatus) bool {
	return status != models.StatusAbandoned && status != models.StatusShipped && status != models.StatusArchived
}

func shipmentMutationBlocked(status models.ArtifactStatus) bool {
	return status == models.StatusShipped || status == models.StatusAbandoned || status == models.StatusArchived
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

// NormalizeShipmentItems is the single source of truth for reading a shipment's
// custom_fields["items"] into a normalized []string. It is a PURE READ: it does
// NOT mutate the artifact (unlike the mutator normalizeShipmentArtifact, which
// wraps this reader to canonicalize items on the CREATE/GET write path).
//
// It maps the lossy on-the-way-out representations of the SQLite JSON array
// (see docs/compound/go-patterns/f015-shipment-stash-patterns.md — treat
// []interface{} shipment CustomFields as lossy and normalize on every read):
// a []string is cloned, a []any is filtered to its string elements
// order-preserving, and nil/absent/unknown inputs yield an empty slice.
//
// CONTRACT: this function NEVER returns nil. An empty result is a non-nil
// []string{}. This is a JSON wire-shape invariant, not a stylistic choice: a
// nil []string marshals to null, whereas a non-nil empty slice marshals to [].
// The shipment items field is emitted through core.ShipmentView, which is
// marshaled by BOTH the CLI and the MCP list/get surfaces, so a nil here would
// surface as items: null on the wire. The end-to-end guard for this is
// TestListShipments_EmptyItems_NeverNull (internal/mcp). Do NOT "simplify" the
// []string branch back to the nil-able append([]string(nil), ...) form — that
// silently reintroduces the null-on-empty regression this consolidation removed.
func NormalizeShipmentItems(artifact *models.Artifact) []string {
	if artifact == nil || artifact.CustomFields == nil {
		return []string{}
	}

	raw, ok := artifact.CustomFields["items"]
	if !ok || raw == nil {
		return []string{}
	}

	switch items := raw.(type) {
	case []string:
		out := make([]string, len(items))
		copy(out, items)
		return out
	case []any:
		normalized := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := item.(string); ok {
				normalized = append(normalized, value)
			}
		}
		return normalized
	default:
		return []string{}
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneArtifact(artifact *models.Artifact) *models.Artifact {
	if artifact == nil {
		return nil
	}

	clone := *artifact
	clone.Labels = append([]string(nil), artifact.Labels...)
	clone.Dependencies = append([]string(nil), artifact.Dependencies...)
	clone.References = append([]string(nil), artifact.References...)
	if artifact.CustomFields != nil {
		clone.CustomFields = maps.Clone(artifact.CustomFields)
	}
	return &clone
}

func snapshotFile(path string) (fileSnapshot, error) {
	snapshot := fileSnapshot{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return snapshot, err
	}
	if info.IsDir() {
		return snapshot, fmt.Errorf("%s is a directory", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	snapshot.Exists = true
	snapshot.Content = content
	return snapshot, nil
}

func restorePersistedArtifactFiles(currentSnapshot, targetSnapshot fileSnapshot, currentPath, targetPath string) error {
	var errs []error
	if currentPath == targetPath {
		if err := restoreSnapshot(currentSnapshot); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove target %s: %w", targetPath, err))
	}
	if err := restoreSnapshot(currentSnapshot); err != nil {
		errs = append(errs, err)
	}
	if err := restoreSnapshot(targetSnapshot); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func restoreSnapshot(snapshot fileSnapshot) error {
	if snapshot.Path == "" {
		return nil
	}
	if !snapshot.Exists {
		if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", snapshot.Path, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(snapshot.Path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", snapshot.Path, err)
	}
	if err := os.WriteFile(snapshot.Path, snapshot.Content, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", snapshot.Path, err)
	}
	return nil
}

func shipmentOpsRoot(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "ops")
}

func returnBlockedJournalPath(rootPath, shipmentID, itemID string) string {
	return filepath.Join(shipmentOpsRoot(rootPath), fmt.Sprintf("return-blocked-%s-%s.json", shipmentID, itemID))
}

func writeReturnBlockedJournal(rootPath string, shipment, item *models.Artifact) error {
	journalPath := returnBlockedJournalPath(rootPath, shipment.ID, item.ID)
	if err := os.MkdirAll(filepath.Dir(journalPath), 0o755); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}
	payload, err := json.MarshalIndent(returnBlockedJournal{
		Shipment: cloneArtifact(shipment),
		Item:     cloneArtifact(item),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}
	tmpPath := journalPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return fmt.Errorf("write journal temp file: %w", err)
	}
	if err := os.Rename(tmpPath, journalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit journal: %w", err)
	}
	return nil
}

func removeReturnBlockedJournal(ctx context.Context, rootPath, shipmentID, itemID string) {
	journalPath := returnBlockedJournalPath(rootPath, shipmentID, itemID)
	if err := os.Remove(journalPath); err != nil && !os.IsNotExist(err) {
		slog.WarnContext(ctx, "remove return-blocked journal", "path", journalPath, "error", err)
	}
}

func recoverPendingShipmentOperations(ctx context.Context, ws *Workspace) error {
	entries, err := os.ReadDir(shipmentOpsRoot(ws.RootPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read shipment ops directory: %w", err)
	}
	var recoveryErrs []error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		journalPath := filepath.Join(shipmentOpsRoot(ws.RootPath), entry.Name())
		if err := recoverReturnBlockedJournal(ctx, ws, journalPath); err != nil {
			recoveryErrs = append(recoveryErrs, err)
		}
	}
	return errors.Join(recoveryErrs...)
}

func recoverReturnBlockedJournal(ctx context.Context, ws *Workspace, journalPath string) error {
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return fmt.Errorf("read shipment journal %s: %w", journalPath, err)
	}
	var journal returnBlockedJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return fmt.Errorf("parse shipment journal %s: %w", journalPath, err)
	}
	if journal.Shipment == nil || journal.Item == nil {
		return fmt.Errorf("shipment journal %s is incomplete", journalPath)
	}
	if err := persistArtifact(ctx, ws, journal.Shipment, false); err != nil {
		return fmt.Errorf("restore shipment %s from journal: %w", journal.Shipment.ID, err)
	}
	if err := persistArtifact(ctx, ws, journal.Item, true); err != nil {
		return fmt.Errorf("restore item %s from journal: %w", journal.Item.ID, err)
	}
	removeReturnBlockedJournal(ctx, ws.RootPath, journal.Shipment.ID, journal.Item.ID)
	return nil
}
