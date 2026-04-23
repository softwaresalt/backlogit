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

// ArchiveRecord tracks the metadata for a single archived artifact.
type ArchiveRecord struct {
	ID           string    `json:"id"`
	ArchivedAt   time.Time `json:"archived_at"`
	ArchivedBy   string    `json:"archived_by"`
	OriginalPath string    `json:"original_path"`
	ArchivePath  string    `json:"archive_path"`
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

// ArchiveItem moves an artifact from its active directory to the archive directory,
// updating the SQLite index and storing the original path in frontmatter for restoration.
func ArchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string, opts ...ArchiveOpt) (*ArchiveRecord, error) {
	var cfg archiveConfig
	for _, opt := range opts {
		opt(&cfg)
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
		if filepath.Clean(currentPath) != filepath.Clean(archivePath) {
			// Write the original content back to restore pre-archive state.
			if restoreErr := os.WriteFile(currentPath, raw, 0o644); restoreErr != nil {
				slog.Error("archive: failed to restore file after DB error",
					"archive_path", archivePath, "original_path", currentPath, "error", restoreErr)
			} else {
				os.Remove(archivePath) //nolint:errcheck
			}
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

	return &ArchiveRecord{
		ID:           itemID,
		ArchivedAt:   time.Now(),
		OriginalPath: currentPath,
		ArchivePath:  archivePath,
	}, nil
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
