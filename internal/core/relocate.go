package core

import (
	"context"
	"database/sql"
)

// RelocateArtifactFile moves an artifact's Markdown file from its current location
// to the directory mapped by the new (type, status) pair in registry.yaml.
//
// Worker: Look up the target directory from the workspace's RegistryConfig for
// the given artifact type and new status. If the current file path differs from
// the target, move the file using atomic rename (temp-file-then-rename pattern).
// Create the target directory with os.MkdirAll if it does not exist.
// Return the new file path.
func RelocateArtifactFile(ctx context.Context, db *sql.DB, ws *Workspace, artifactID, newStatus string) (string, error) {
	panic("not implemented: Worker: Look up target directory from registry.yaml routing for (type, status), move file atomically, create target dir if needed, return new path")
}
