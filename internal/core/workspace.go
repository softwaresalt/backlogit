package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/db"
)

// Workspace coordinates cross-store operations across Markdown, SQLite, and JSONL.
type Workspace struct {
	RootPath  string
	Config    *config.WorkspaceConfig
	DB        *sql.DB
	HeaderDef *config.HeaderDefConfig
	Templates []*config.TemplateConfig
}

// WorkspaceStorageRoot returns the .backlogit directory for a workspace root.
func WorkspaceStorageRoot(rootPath string) string {
	return filepath.Join(rootPath, ".backlogit")
}

// WorkspaceLogsRoot returns the .backlogit\logs directory for a workspace root.
func WorkspaceLogsRoot(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "logs")
}

// NewWorkspace creates a workspace, loads config, opens DB, and ensures schema.
func NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	resolvedRoot, err := resolveWorkspaceRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	backlogitDir := WorkspaceStorageRoot(resolvedRoot)
	cfg, err := config.Load(ctx, backlogitDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dbPath := filepath.Join(backlogitDir, "backlogit.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Load header-def (optional — nil if file is absent).
	headerDef, hdErr := config.LoadHeaderDef(backlogitDir)
	if hdErr != nil {
		if !errors.Is(hdErr, os.ErrNotExist) {
			database.Close()
			return nil, fmt.Errorf("load header-def: %w", hdErr)
		}
		headerDef = nil
	}

	// Load templates (optional — nil if templates dir is absent).
	templates, templatesErr := config.LoadTemplates(filepath.Join(backlogitDir, "templates"))
	if templatesErr != nil {
		database.Close()
		return nil, fmt.Errorf("load templates: %w", templatesErr)
	}

	if headerDef != nil {
		if err := db.EnsureSchemaWithExtensions(database, headerDef); err != nil {
			database.Close()
			return nil, fmt.Errorf("ensure schema: %w", err)
		}
	} else {
		if err := db.EnsureSchema(database); err != nil {
			database.Close()
			return nil, fmt.Errorf("ensure schema: %w", err)
		}
	}

	return &Workspace{
		RootPath:  resolvedRoot,
		Config:    cfg,
		DB:        database,
		HeaderDef: headerDef,
		Templates: templates,
	}, nil
}

func resolveWorkspaceRoot(rootPath string) (string, error) {
	cleanRoot := filepath.Clean(rootPath)

	if hasWorkspaceConfig(WorkspaceStorageRoot(cleanRoot)) {
		return cleanRoot, nil
	}
	if filepath.Base(cleanRoot) == ".backlogit" && hasWorkspaceConfig(cleanRoot) {
		return filepath.Dir(cleanRoot), nil
	}

	entries, err := os.ReadDir(cleanRoot)
	if err != nil {
		return cleanRoot, nil
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		candidateRoot := filepath.Join(cleanRoot, entry.Name())
		if hasWorkspaceConfig(WorkspaceStorageRoot(candidateRoot)) {
			matches = append(matches, candidateRoot)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	return cleanRoot, nil
}

func hasWorkspaceConfig(storageRoot string) bool {
	info, err := os.Stat(filepath.Join(storageRoot, "config.yaml"))
	if err != nil {
		return false
	}
	return !info.IsDir()
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

	normalizedTarget := strings.ReplaceAll(target, "\\", string(filepath.Separator))
	abs, err := filepath.Abs(filepath.Join(absRoot, normalizedTarget))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !strings.HasPrefix(abs, cleanRoot+string(filepath.Separator)) && abs != cleanRoot {
		return "", fmt.Errorf("path escapes workspace boundary: %s", target)
	}
	return abs, nil
}
