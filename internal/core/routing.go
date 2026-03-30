package core

import (
	"context"

	"github.com/backlogit/backlogit/internal/config"
)

// ResolveTargetDir determines the filesystem directory for an artifact.
//
// Worker: Implement registry-based directory routing.
func ResolveTargetDir(registry *config.RegistryConfig, artifactType string, status string) string {
	panic("not implemented: Worker: Implement directory routing from registry config")
}

// MoveArtifactFile relocates an artifact file to a new directory.
//
// Worker: Implement atomic file move with SafeResolve validation.
func MoveArtifactFile(ctx context.Context, workspaceRoot string, currentPath string, newDir string) (string, error) {
	panic("not implemented: Worker: Implement artifact file relocation")
}
