package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// NewReadOnlyIndexWorkspace opens a non-recovering workspace whose SQLite index
// is attached read-only. It never runs shipment recovery, never migrates the
// schema, and rejects writes at the connection level, so previews that need the
// index (for example ID prediction) cannot mutate the workspace. When the index
// file does not exist DB stays nil and callers must treat the index as
// unavailable. Callers must Close the workspace.
func NewReadOnlyIndexWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	ws, err := NewReadOnlyWorkspace(ctx, rootPath)
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(ws.StorageRoot, "backlogit.db")
	if _, statErr := os.Stat(dbPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return ws, nil
		}
		return nil, fmt.Errorf("inspect index database: %w", statErr)
	}
	database, err := bldb.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index database read-only: %w", err)
	}
	ws.DB = database
	return ws, nil
}
