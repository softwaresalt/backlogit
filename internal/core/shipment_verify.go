package core

// shipment_verify.go provides post-shipment consistency checks.
// 025.018-T (Unit 6): After ShipShipment archives items, VerifyPostShipConsistency
// confirms that all archived item IDs have been removed from the queue directory.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// VerifyPostShipConsistency verifies that all items in archivedIDs are absent
// from non-archive workspace directories. It returns an error listing any stale
// artifact IDs found, indicating a partial or failed archive operation.
func VerifyPostShipConsistency(_ context.Context, ws *Workspace, archivedIDs []string) error {
	if ws == nil {
		return fmt.Errorf("VerifyPostShipConsistency: workspace is required")
	}
	if len(archivedIDs) == 0 {
		return nil
	}

	idSet := make(map[string]bool, len(archivedIDs))
	for _, id := range archivedIDs {
		if id != "" {
			idSet[id] = true
		}
	}
	if len(idSet) == 0 {
		return nil
	}

	backlogDir := WorkspaceStorageRoot(ws.RootPath)
	archiveDir := filepath.Clean(filepath.Join(backlogDir, "archive"))

	// Use registry-aware directory enumeration to cover all active routing paths,
	// not just QueueLayout.RootDir. Skip the archive directory itself since items
	// are expected to be there after a successful archive operation.
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return fmt.Errorf("verify post-ship consistency: enumerate dirs: %w", err)
	}

	var staleIDs []string
	for _, dir := range searchDirs {
		if filepath.Clean(dir) == archiveDir {
			continue
		}
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			a, _, parseErr := parseFile(path)
			if parseErr != nil || a.ID == "" {
				return nil
			}
			if idSet[a.ID] {
				staleIDs = append(staleIDs, a.ID)
			}
			return nil
		})
		if walkErr != nil {
			return fmt.Errorf("verify post-ship consistency: walk %s: %w", dir, walkErr)
		}
	}

	if len(staleIDs) > 0 {
		return fmt.Errorf("post-ship consistency: stale queue files found for archived items: %v", staleIDs)
	}
	return nil
}
