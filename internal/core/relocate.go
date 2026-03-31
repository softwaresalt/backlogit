package core

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/backlogit/backlogit/internal/config"
	dbpkg "github.com/backlogit/backlogit/internal/db"
)

// RelocateArtifactFile moves an artifact's Markdown file from its current location
// to the directory mapped by the new (type, status) pair in registry.yaml.
func RelocateArtifactFile(ctx context.Context, database *sql.DB, ws *Workspace, artifactID, newStatus string) (string, error) {
	artifact, err := dbpkg.GetItem(ctx, database, artifactID)
	if err != nil {
		return "", fmt.Errorf("get artifact %s: %w", artifactID, err)
	}

	backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	targetDir := ResolveTargetDir(registry, artifact.ArtifactType, newStatus)

	currentPath, err := FindArtifactPath(ctx, ws, artifactID)
	if err != nil {
		return "", fmt.Errorf("find artifact path: %w", err)
	}

	// If already in the right directory, no move needed.
	currentDir := filepath.Base(filepath.Dir(currentPath))
	if currentDir == targetDir {
		return currentPath, nil
	}

	newPath, err := MoveArtifactFile(ctx, ws.RootPath, currentPath, targetDir)
	if err != nil {
		return "", fmt.Errorf("move artifact file: %w", err)
	}

	return newPath, nil
}
