package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/events"
	"github.com/backlogit/backlogit/internal/models"
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

// ArchiveItem moves an artifact from its active directory to the archive directory,
// updating the SQLite index and storing the original path in frontmatter for restoration.
func ArchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string) (*ArchiveRecord, error) {
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
	fm["archived_from"] = currentPath
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
	if err := os.Remove(currentPath); err != nil {
		os.Remove(archivePath) //nolint:errcheck
		return nil, fmt.Errorf("remove original: %w", err)
	}

	// Update DB status to archived directly — avoids re-parsing frontmatter field
	// mapping discrepancies (e.g. type vs artifact_type).
	if _, dbErr := database.ExecContext(ctx, "UPDATE items SET status = ? WHERE id = ?", string(models.StatusArchived), itemID); dbErr != nil {
		return nil, fmt.Errorf("sync archive state: %w", dbErr)
	}

	// Best-effort: log archive event to the item's JSONL log (non-fatal on failure).
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	ew := events.NewEventWriter(logsDir)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "archived",
		Delta:     map[string]any{"archive_path": archivePath},
	}
	_ = ew.AppendEvent(ctx, event)
	_ = db.IndexEvent(ctx, database, logsDir, event)

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
	if err := os.Remove(archivePath); err != nil {
		return fmt.Errorf("remove archive file: %w", err)
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err == nil {
		if upsertErr := db.UpsertItem(ctx, database, artifact); upsertErr != nil {
			return fmt.Errorf("sync unarchive state: %w", upsertErr)
		}
	}
	return nil
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
