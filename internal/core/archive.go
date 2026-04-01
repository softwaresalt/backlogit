package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/backlogit/backlogit/internal/db"
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
	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	currentPath, err := findFileAnywhere(backlogDir, itemID)
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

	// Upsert DB record so the archive path is reflected in the index.
	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err == nil {
		if upsertErr := db.UpsertItem(ctx, database, artifact); upsertErr != nil {
			return nil, fmt.Errorf("sync archive state: %w", upsertErr)
		}
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
	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")
	archivePath := filepath.Join(archiveDir, itemID+".md")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archived artifact not found: %s", itemID)
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
		return fmt.Errorf("archived_from not set in %s: cannot restore", itemID)
	}

	// F-006: Validate the restore path is contained within the workspace.
	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	rel, relErr := filepath.Rel(ws.RootPath, originalPath)
	if relErr != nil || len(rel) >= 2 && rel[:2] == ".." {
		return fmt.Errorf("archived_from path %q escapes workspace: cannot restore", originalPath)
	}
	_ = backlogDir // containment validated via filepath.Rel above

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
	items, err := db.QueryItems(ctx, database, db.QueryFilters{})
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

// findFileAnywhere walks the .backlogit directory (including hidden subdirs) to
// locate the Markdown file for the given artifact ID.
func findFileAnywhere(backlogDir, id string) (string, error) {
	var found string
	walkErr := filepath.WalkDir(backlogDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		fm, _, parseErr := models.ParseFrontmatter(string(data))
		if parseErr != nil {
			return nil
		}
		if idVal, ok := fm["id"].(string); ok && idVal == id {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("walk workspace: %w", walkErr)
	}
	if found == "" {
		return "", fmt.Errorf("artifact not found: %s", id)
	}
	return found, nil
}
