package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
)

// ResolveTargetDir determines the filesystem directory for an artifact.
func ResolveTargetDir(registry *config.RegistryConfig, artifactType string, status string) string {
	for _, rule := range registry.Directories {
		for _, s := range rule.Condition.Status {
			if s == status {
				return rule.Path
			}
		}
		for _, t := range rule.Condition.Type {
			if t == artifactType {
				return rule.Path
			}
		}
	}
	return "queue"
}

// MoveArtifactFile relocates an artifact file within the .backlogit storage root atomically.
func MoveArtifactFile(_ context.Context, storageRoot string, currentPath string, newDir string) (string, error) {
	if _, err := SafeResolve(storageRoot, currentPath); err != nil {
		// currentPath may already be absolute; validate it is inside root using filepath.Rel.
		absRoot, absErr := filepath.Abs(storageRoot)
		if absErr != nil {
			return "", fmt.Errorf("resolve storage root: %w", absErr)
		}
		rel, relErr := filepath.Rel(absRoot, filepath.Clean(currentPath))
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("current path escapes storage root: %s", currentPath)
		}
	}

	newDirAbs := filepath.Join(storageRoot, newDir)
	if err := os.MkdirAll(newDirAbs, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", newDirAbs, err)
	}

	newPath := filepath.Join(newDirAbs, filepath.Base(currentPath))
	if err := os.Rename(currentPath, newPath); err != nil {
		return "", fmt.Errorf("move file: %w", err)
	}
	return newPath, nil
}
