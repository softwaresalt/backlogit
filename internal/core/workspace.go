package core

import (
	"context"
	"database/sql"

	"github.com/backlogit/backlogit/internal/config"
)

// Workspace coordinates cross-store operations across Markdown, SQLite, and JSONL.
type Workspace struct {
	RootPath string
	Config   *config.WorkspaceConfig
	DB       *sql.DB
}

// NewWorkspace creates a workspace, loads config, opens DB, and ensures schema.
//
// Worker: Implement workspace initialization with config loading and DB setup.
func NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	panic("not implemented: Worker: Implement workspace initialization")
}

// Close closes the database connection.
func (ws *Workspace) Close() error {
	if ws.DB != nil {
		return ws.DB.Close()
	}
	return nil
}

// SafeResolve returns an absolute path within the workspace root or an error
// if the target escapes the workspace boundary.
//
// Worker: Implement path traversal validation.
func SafeResolve(workspaceRoot, target string) (string, error) {
	panic("not implemented: Worker: Implement path traversal validation")
}
