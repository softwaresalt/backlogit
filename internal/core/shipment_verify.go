package core

// shipment_verify.go provides post-shipment consistency checks.
// 025.018-T (Unit 6): After ShipShipment archives items, VerifyPostShipConsistency
// confirms that all archived item IDs have been removed from the queue directory.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/softwaresalt/backlogit/internal/config"
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

	// Derive archive directories from registry status conditions so that a
	// customised registry.yaml (non-default archive path) is respected. Fall
	// back to the conventional "archive" directory when no rule lists "archived"
	// as a routed status.
	archiveDirs := make(map[string]bool)
	if registry, regErr := config.LoadRegistry(backlogDir); regErr == nil {
		for _, rule := range registry.Directories {
			for _, s := range rule.Condition.Status {
				if s == "archived" {
					archiveDirs[filepath.Clean(filepath.Join(backlogDir, rule.Path))] = true
					break
				}
			}
		}
	}
	if len(archiveDirs) == 0 {
		archiveDirs[filepath.Clean(filepath.Join(backlogDir, "archive"))] = true
	}

	// Use registry-aware directory enumeration to cover all active routing paths.
	// Skip any directory that serves archived status so we don't flag items that
	// were correctly moved there.
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return fmt.Errorf("verify post-ship consistency: enumerate dirs: %w", err)
	}

	var staleIDs []string
	for _, dir := range searchDirs {
		if archiveDirs[filepath.Clean(dir)] {
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
