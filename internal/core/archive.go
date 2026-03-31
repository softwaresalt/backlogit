package core

import (
	"context"
	"database/sql"
	"time"
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
// updating the SQLite index and appending an archive event to events.jsonl.
//
// Worker: Validate item exists and is in a terminal status. Resolve archive path
// using registry.yaml routing rules. Move the .md file. Update the DB record path.
// Append an archive event to events.jsonl via EventWriter.
func ArchiveItem(ctx context.Context, db *sql.DB, ws *Workspace, itemID string) (*ArchiveRecord, error) {
	panic("not implemented: Worker: Move completed artifact to archive dir, update DB and emit archive event")
}

// UnarchiveItem restores an artifact from the archive back to its active directory.
//
// Worker: Look up archive record. Move file back to original path. Update DB.
// Append an unarchive event.
func UnarchiveItem(ctx context.Context, db *sql.DB, ws *Workspace, itemID string) error {
	panic("not implemented: Worker: Restore archived artifact to active directory and update DB")
}

// AutoArchive scans for items in terminal statuses past the retention window and
// archives them according to the policy.
//
// Worker: Query items WHERE status IN terminal_statuses AND updated_at < (now - retention_days).
// Call ArchiveItem for each. Return count archived.
func AutoArchive(ctx context.Context, db *sql.DB, ws *Workspace, policy *ArchivePolicy) (int, error) {
	panic("not implemented: Worker: Scan and archive items past retention window per ArchivePolicy")
}
