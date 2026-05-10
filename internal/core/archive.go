package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ArchiveRecord tracks the metadata for a single archived artifact,
// including any cascaded children and failures.
type ArchiveRecord struct {
	ID            string           `json:"id"`
	ArchivedAt    time.Time        `json:"archived_at"`
	ArchivedBy    string           `json:"archived_by"`
	OriginalPath  string           `json:"original_path"`
	ArchivePath   string           `json:"archive_path"`
	CascadedItems []string         `json:"cascaded_items,omitempty"`
	FailedItems   []ArchiveFailure `json:"failed_items,omitempty"`
}

// ArchiveFailure records a child item that failed to archive during cascade.
type ArchiveFailure struct {
	ItemID string `json:"item_id"`
	Error  string `json:"error"`
}

// ArchivePolicy defines rules for automatic archiving of completed artifacts.
type ArchivePolicy struct {
	TerminalStatuses []string `json:"terminal_statuses" yaml:"terminal_statuses"`
	RetentionDays    int      `json:"retention_days" yaml:"retention_days"`
	ArchiveDir       string   `json:"archive_dir" yaml:"archive_dir"`
}

// ArchiveOpt configures optional behavior for ArchiveItem.
type ArchiveOpt func(*archiveConfig)

type archiveConfig struct {
	commitSHA string
	topLevel  *bool // nil means default true
	cascade   bool  // when true, archive children recursively before the parent
}

// WithCommitSHA attaches a git commit SHA to the archive event for traceability.
func WithCommitSHA(sha string) ArchiveOpt {
	return func(c *archiveConfig) { c.commitSHA = sha }
}

// WithTopLevel sets whether this archive call is a top-level operation.
// When false, post-hooks that emit external events (event emission, webhooks)
// skip to prevent duplicate notifications from nested operations.
// Default is true when not specified.
func WithTopLevel(topLevel bool) ArchiveOpt {
	return func(c *archiveConfig) { c.topLevel = &topLevel }
}

// WithCascade enables recursive archiving of child items before archiving
// the target item. Children are archived deepest-first (subtasks before tasks).
// Default is false.
func WithCascade(cascade bool) ArchiveOpt {
	return func(c *archiveConfig) { c.cascade = cascade }
}

// ArchiveItem moves an artifact from its active directory to the archive directory,
// updating the SQLite index and storing the original path in frontmatter for restoration.
// When WithCascade(true) is set, child items are archived bottom-up before the parent.
func ArchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string, opts ...ArchiveOpt) (*ArchiveRecord, error) {
	var cfg archiveConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	// Cascade: archive all descendants bottom-up before archiving the target.
	var cascadedItems []string
	var failedItems []ArchiveFailure
	if cfg.cascade {
		cascadedItems, failedItems = archiveDescendants(ctx, database, ws, itemID, cfg)
	}

	backlogDir := WorkspaceStorageRoot(ws.RootPath)
	currentPath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("find artifact: %w", err)
	}

	archiveDir := filepath.Join(backlogDir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return nil, fmt.Errorf("create archive dir: %w", err)
	}

	// Stamp the original path into frontmatter for later restoration.
	raw, err := os.ReadFile(currentPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}
	oldStatus, _ := fm["status"].(string)
	isTopLevel := cfg.topLevel == nil || *cfg.topLevel // default true

	// Fire pre-archive hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: fmArtifactType(fm),
			OldValues:    map[string]any{"status": oldStatus},
			NewValues:    map[string]any{"status": string(models.StatusArchived), "archived": true},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     isTopLevel,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookArchiveItem, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-archive hook: %w", err)
		}
	}

	fm["archived_from"] = workspaceRelativePath(ws.RootPath, currentPath)
	fm["status"] = string(models.StatusArchived)
	newContent := models.SerializeFrontmatter(fm, body)

	archivePath := filepath.Join(archiveDir, filepath.Base(currentPath))

	// Atomic write: write to a temp file then rename.
	tmpPath := archivePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("write archive file: %w", err)
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		os.Remove(tmpPath) //nolint:errcheck
		return nil, fmt.Errorf("rename archive file: %w", err)
	}
	// Only remove the source file when it differs from the archive destination.
	// When the registry routes a terminal status (e.g. "done") to the archive
	// directory, the item may already reside there before ArchiveItem is called.
	// Removing currentPath in that case would delete the file we just wrote.
	if filepath.Clean(currentPath) != filepath.Clean(archivePath) {
		if err := os.Remove(currentPath); err != nil {
			os.Remove(archivePath) //nolint:errcheck
			return nil, fmt.Errorf("remove original: %w", err)
		}
	}

	// Update DB status to archived. On failure, restore the file to its
	// original path so filesystem and DB index remain consistent.
	if _, dbErr := database.ExecContext(ctx, "UPDATE items SET status = ? WHERE id = ?", string(models.StatusArchived), itemID); dbErr != nil {
		// Always restore original content to currentPath regardless of whether
		// paths differ: the file may have been overwritten with archive frontmatter
		// even when currentPath == archivePath (already-archived items).
		if restoreErr := os.WriteFile(currentPath, raw, 0o644); restoreErr != nil {
			slog.Error("archive: failed to restore file after DB error",
				"archive_path", archivePath, "original_path", currentPath, "error", restoreErr)
		} else if filepath.Clean(currentPath) != filepath.Clean(archivePath) {
			// Remove the stale archive copy only when it is a distinct file.
			os.Remove(archivePath) //nolint:errcheck
		}
		return nil, fmt.Errorf("sync archive state: %w", dbErr)
	}

	// Best-effort: log archive event to the item's JSONL log (non-fatal on failure).
	// Errors are logged for diagnosability, matching the pattern in commits.go.
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	ew := events.NewEventWriter(logsDir)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "archived",
		Delta:     map[string]any{"archive_path": workspaceRelativePath(ws.RootPath, archivePath)},
		CommitSHA: cfg.commitSHA,
	}
	if evErr := ew.AppendEvent(ctx, event); evErr != nil {
		slog.Warn("archive item: failed to append event to item log", "item_id", itemID, "error", evErr)
	} else if indexErr := db.IndexEvent(ctx, database, logsDir, event); indexErr != nil {
		slog.Warn("archive item: failed to index event", "item_id", itemID, "error", indexErr)
	}

	// Fire post-archive hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: fmArtifactType(fm),
			OldValues:    map[string]any{"status": oldStatus},
			NewValues:    map[string]any{"status": string(models.StatusArchived), "archived": true},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     isTopLevel,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookArchiveItem, hookCtx)
	}

	// Archive any stash entries linked to this item (best-effort).
	if n, stashErr := ArchiveLinkedStashEntries(ctx, ws, itemID); stashErr != nil {
		slog.Warn("archive item: failed to archive linked stash entries",
			"item_id", itemID, "error", stashErr)
	} else if n > 0 {
		slog.Info("archive item: archived linked stash entries",
			"item_id", itemID, "count", n)
	}

	return &ArchiveRecord{
		ID:            itemID,
		ArchivedAt:    time.Now(),
		OriginalPath:  currentPath,
		ArchivePath:   archivePath,
		CascadedItems: cascadedItems,
		FailedItems:   failedItems,
	}, nil
}

// archiveDescendants collects all descendants of parentID bottom-up and archives
// each one individually. It returns the IDs of successfully archived items and
// any failures. Already-archived children are silently skipped.
func archiveDescendants(ctx context.Context, database *sql.DB, ws *Workspace, parentID string, cfg archiveConfig) ([]string, []ArchiveFailure) {
	descendants := collectDescendants(ctx, database, parentID, make(map[string]bool), 0, maxCascadeDepth(ws))
	// descendants are collected parent-first; reverse for bottom-up archive order.
	for i, j := 0, len(descendants)-1; i < j; i, j = i+1, j-1 {
		descendants[i], descendants[j] = descendants[j], descendants[i]
	}

	var cascaded []string
	var failures []ArchiveFailure
	for _, childID := range descendants {
		_, err := ArchiveItem(ctx, database, ws, childID, WithTopLevel(false), WithCommitSHA(cfg.commitSHA))
		if err != nil {
			// Skip already-archived items silently.
			if isAlreadyArchived(ctx, database, childID) {
				continue
			}
			failures = append(failures, ArchiveFailure{ItemID: childID, Error: err.Error()})
			slog.Warn("cascade archive: child failed", "parent_id", parentID, "child_id", childID, "error", err)
			continue
		}
		cascaded = append(cascaded, childID)
	}
	return cascaded, failures
}

// collectDescendants recursively finds all child item IDs for parentID using
// the DB index. Items are returned parent-first (caller reverses for bottom-up).
func collectDescendants(ctx context.Context, database *sql.DB, parentID string, visited map[string]bool, depth, maxDepth int) []string {
	if depth >= maxDepth || visited[parentID] {
		return nil
	}
	visited[parentID] = true

	children, err := db.QueryItems(ctx, database, db.QueryFilters{ParentID: parentID})
	if err != nil {
		slog.Warn("cascade archive: failed to query children", "parent_id", parentID, "error", err)
		return nil
	}

	var result []string
	for _, child := range children {
		result = append(result, child.ID)
		result = append(result, collectDescendants(ctx, database, child.ID, visited, depth+1, maxDepth)...)
	}
	return result
}

// maxCascadeDepth returns a safe maximum recursion depth for cascade operations.
func maxCascadeDepth(ws *Workspace) int {
	if ws != nil && ws.Config != nil && ws.Config.QueueLayout != nil && len(ws.Config.QueueLayout.Levels) > 0 {
		return len(ws.Config.QueueLayout.Levels) + 1
	}
	return 10 // fallback
}

// isAlreadyArchived checks whether an item has status=archived in the DB.
func isAlreadyArchived(ctx context.Context, database *sql.DB, itemID string) bool {
	item, err := db.GetItem(ctx, database, itemID)
	if err != nil {
		return false
	}
	return item.Status == models.StatusArchived
}

// UnarchiveItem restores an artifact from the archive back to its original path.
func UnarchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string) error {
	backlogDir := WorkspaceStorageRoot(ws.RootPath)
	archivePath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("find archived artifact: %w", err)
	}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("read archive file: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return fmt.Errorf("parse archive file: %w", err)
	}

	originalPath, _ := fm["archived_from"].(string)
	if originalPath == "" {
		status, _ := fm["status"].(string)
		if status != string(models.StatusArchived) {
			return fmt.Errorf("item %s is not archived", itemID)
		}
		return fmt.Errorf("archived item %s is missing archived_from metadata", itemID)
	}
	originalPath = resolveWorkspacePath(ws.RootPath, originalPath)

	// F-006: Validate the restore path is contained within .backlogit to prevent
	// path traversal when restoring artifacts from archive.
	rel, relErr := filepath.Rel(backlogDir, originalPath)
	if relErr != nil || len(rel) >= 2 && rel[:2] == ".." {
		return fmt.Errorf("archived_from path %q escapes workspace storage: cannot restore", originalPath)
	}

	// Restore frontmatter without the archived_from field.
	delete(fm, "archived_from")
	restored := models.SerializeFrontmatter(fm, body)

	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		return fmt.Errorf("create restore dir: %w", err)
	}

	// Atomic write: write to a temp file then rename.
	tmpPath := originalPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(restored), 0o644); err != nil {
		return fmt.Errorf("write restored file: %w", err)
	}
	if err := os.Rename(tmpPath, originalPath); err != nil {
		os.Remove(tmpPath) //nolint:errcheck
		return fmt.Errorf("rename restored file: %w", err)
	}
	// Only remove the archive file when it differs from the restored path.
	// When archived_from stored an archive-dir path (because the file was already
	// there before ArchiveItem ran), originalPath == archivePath and the rename
	// above updated the file in place — removing it here would undo that write.
	if filepath.Clean(archivePath) != filepath.Clean(originalPath) {
		if err := os.Remove(archivePath); err != nil {
			return fmt.Errorf("remove archive file: %w", err)
		}
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err == nil {
		if upsertErr := db.UpsertItem(ctx, database, artifact); upsertErr != nil {
			return fmt.Errorf("sync unarchive state: %w", upsertErr)
		}
	}
	return nil
}

// fmArtifactType extracts artifact_type from frontmatter for hook context.
func fmArtifactType(fm map[string]any) string {
	if t, ok := fm["artifact_type"].(string); ok {
		return t
	}
	if t, ok := fm["type"].(string); ok {
		return t
	}
	return ""
}

func workspaceRelativePath(rootPath string, target string) string {
	rel, err := filepath.Rel(rootPath, target)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(target))
	}
	return filepath.ToSlash(rel)
}

func resolveWorkspacePath(rootPath string, trackedPath string) string {
	if trackedPath == "" {
		return ""
	}
	if filepath.IsAbs(trackedPath) {
		return filepath.Clean(trackedPath)
	}
	return filepath.Join(rootPath, filepath.FromSlash(trackedPath))
}

// AutoArchive scans for items in terminal statuses past the retention window and
// archives them according to the policy.
func AutoArchive(ctx context.Context, database *sql.DB, ws *Workspace, policy *ArchivePolicy) (int, error) {
	if len(policy.TerminalStatuses) == 0 {
		return 0, nil
	}

	threshold := time.Now().AddDate(0, 0, -policy.RetentionDays)
	items, err := db.QueryItems(ctx, database, db.QueryFilters{IncludeArchived: true})
	if err != nil {
		return 0, fmt.Errorf("query items: %w", err)
	}

	terminalSet := make(map[string]bool, len(policy.TerminalStatuses))
	for _, s := range policy.TerminalStatuses {
		terminalSet[s] = true
	}

	count := 0
	for _, item := range items {
		if !terminalSet[string(item.Status)] {
			continue
		}
		if item.UpdatedAt.After(threshold) {
			continue
		}
		if _, archiveErr := ArchiveItem(ctx, database, ws, item.ID); archiveErr != nil {
			continue
		}
		count++
	}
	return count, nil
}
