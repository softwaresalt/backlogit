package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/backlogit/backlogit/internal/config"
)

// RelocateArtifactFile moves an artifact's Markdown file from its current location
// to the directory mapped by the new (type, status) pair in registry.yaml.
// artifactType is passed by the caller to avoid a DB lookup when the in-memory
// artifact is already available, keeping this function DB-cache-independent.
func RelocateArtifactFile(ctx context.Context, ws *Workspace, artifactType, artifactID, newStatus string) (string, error) {
	backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	targetDir := ResolveTargetDir(registry, artifactType, newStatus)

	currentPath, err := FindArtifactPath(ctx, ws, artifactID)
	if err != nil {
		return "", fmt.Errorf("find artifact path: %w", err)
	}

	// Compare the current directory relative to the workspace root with the target
	// directory from the registry, which may be a multi-segment path.
	currentRel, err := filepath.Rel(ws.RootPath, filepath.Dir(currentPath))
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	if filepath.Clean(currentRel) == filepath.Clean(targetDir) {
		return currentPath, nil
	}

	newPath, err := MoveArtifactFile(ctx, ws.RootPath, currentPath, targetDir)
	if err != nil {
		return "", fmt.Errorf("move artifact file: %w", err)
	}

	return newPath, nil
}
