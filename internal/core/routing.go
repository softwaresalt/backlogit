package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/backlogit/backlogit/internal/config"
)

// ResolveTargetDir determines the filesystem directory for an artifact.
func ResolveTargetDir(registry *config.RegistryConfig, artifactType string, status string) string {
	for _, rule := range registry.Directories {
		for _, t := range rule.Condition.Type {
			if t == artifactType {
				return rule.Path
			}
		}
		for _, s := range rule.Condition.Status {
			if s == status {
				return rule.Path
			}
		}
	}
	return "items"
}

// MoveArtifactFile relocates an artifact file to a new directory atomically.
func MoveArtifactFile(_ context.Context, workspaceRoot string, currentPath string, newDir string) (string, error) {
	if _, err := SafeResolve(workspaceRoot, currentPath); err != nil {
		// currentPath may already be absolute; validate it is inside root
		cleanRoot := filepath.Clean(workspaceRoot)
		cleanPath := filepath.Clean(currentPath)
		if len(cleanPath) <= len(cleanRoot) {
			return "", fmt.Errorf("current path escapes workspace: %s", currentPath)
		}
	}

	newDirAbs := filepath.Join(workspaceRoot, newDir)
	if err := os.MkdirAll(newDirAbs, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", newDirAbs, err)
	}

	newPath := filepath.Join(newDirAbs, filepath.Base(currentPath))
	if err := os.Rename(currentPath, newPath); err != nil {
		return "", fmt.Errorf("move file: %w", err)
	}
	return newPath, nil
}
