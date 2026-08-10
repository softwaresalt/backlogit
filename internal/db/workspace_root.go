package db

import (
	"os"
	"path/filepath"
)

var workspaceRootCandidates = [...]string{".backlog", ".backlogit"}

func workspaceStorageRoot(workspacePath string) string {
	cleanPath := filepath.Clean(workspacePath)
	base := filepath.Base(cleanPath)
	for _, candidate := range workspaceRootCandidates {
		if base == candidate {
			return cleanPath
		}
	}

	for _, candidate := range workspaceRootCandidates {
		storageRoot := filepath.Join(cleanPath, candidate)
		info, err := os.Stat(filepath.Join(storageRoot, "config.yaml"))
		if err == nil && !info.IsDir() {
			return storageRoot
		}
	}

	return filepath.Join(cleanPath, workspaceRootCandidates[len(workspaceRootCandidates)-1])
}
