package core

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/db"
)

// Workspace coordinates cross-store operations across Markdown, SQLite, and JSONL.
type Workspace struct {
	RootPath string
	Config   *config.WorkspaceConfig
	DB       *sql.DB
}

// NewWorkspace creates a workspace, loads config, opens DB, and ensures schema.
func NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	cfg, err := config.Load(ctx, rootPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dbPath := filepath.Join(rootPath, ".backlogit", "backlogit.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.EnsureSchema(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	return &Workspace{
		RootPath: rootPath,
		Config:   cfg,
		DB:       database,
	}, nil
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
func SafeResolve(workspaceRoot, target string) (string, error) {
	// Convert workspaceRoot to absolute first so both sides of the comparison are absolute.
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	cleanRoot := filepath.Clean(absRoot)

	abs, err := filepath.Abs(filepath.Join(absRoot, target))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !strings.HasPrefix(abs, cleanRoot+string(filepath.Separator)) && abs != cleanRoot {
		return "", fmt.Errorf("path escapes workspace boundary: %s", target)
	}
	return abs, nil
}
