package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// canonical_scan.go (066.001-T / Unit 1) provides a single reusable, recursive
// scanner over the canonical artifact source files (the full artifactSearchDirs
// set: queue + archive + any registry-routed directories). The same parsed
// result feeds the doctor integrity audit (duplicate / root-ID collision
// detection) and the create-time pre-write uniqueness guard (066.002-T), so a
// single filesystem walk serves both consumers without re-parsing.

// artifactRef carries the canonical file path for an artifact along with the
// frontmatter fields needed by the doctor audit and the create-time uniqueness
// guard. Bundling the parsed fields lets a single walk serve multiple consumers
// without re-reading the file.
type artifactRef struct {
	path         string
	id           string
	artifactType string
	parentID     string
	status       string
	level        int
}

// scanCanonicalArtifacts walks the full artifactSearchDirs(ws) set recursively
// and returns a map from artifact ID to every canonical .md file that declares
// that ID. Files that fail to parse or carry an empty ID are skipped. Returning
// every ref (not just the first) lets callers detect duplicate / colliding IDs
// across the queue and archive directories. The candidate archive directories
// are always included in the scan set (see below) so cross-boundary
// root-ID collisions are detectable even when the registry does not route the
// archive.
func scanCanonicalArtifacts(ws *Workspace) (map[string][]artifactRef, error) {
	dirs, err := artifactSearchDirs(ws)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact search dirs: %w", err)
	}

	// ArchiveItem always relocates items into the resolved storage root's
	// archive directory, but stale or conflicting records can still exist under
	// either supported workspace root name. Force both archive candidates into
	// the scan set so the collision guard and doctor audit never go blind to
	// already-archived IDs.
	for _, candidate := range WorkspaceRootCandidates() {
		archiveDir := filepath.Join(ws.RootPath, candidate, "archive")
		archivePresent := false
		for _, d := range dirs {
			if filepath.Clean(d) == filepath.Clean(archiveDir) {
				archivePresent = true
				break
			}
		}
		if !archivePresent {
			// Guard against path traversal: skip candidate dirs that are
			// symlinks or reparse points before adding their archive subdir.
			candidateDir := filepath.Join(ws.RootPath, candidate)
			if info, statErr := os.Lstat(candidateDir); statErr == nil {
				if isReparse, _ := isSymlinkOrReparse(info, candidateDir); isReparse {
					continue
				}
			}
			dirs = append(dirs, archiveDir)
		}
	}

	refs := make(map[string][]artifactRef)
	for _, dirPath := range dirs {
		if _, statErr := os.Lstat(dirPath); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			// Mirror Rehydrate's contract: the legacy multi-entry stash file is
			// not a canonical artifact and must never participate in ID
			// duplicate/collision detection, even if a future stash format gains
			// a top-level id frontmatter field.
			if strings.EqualFold(filepath.Base(path), ".stash.md") {
				return nil
			}
			a, _, parseErr := parseFile(path)
			if parseErr != nil {
				// Skip unparseable files: without parseable frontmatter we cannot
				// extract a canonical ID, so they cannot participate in ID
				// duplicate/collision detection. They are intentionally excluded
				// here rather than indexed or treated as collisions.
				return nil
			}
			if a.ID == "" {
				return nil
			}
			level := a.Level
			if level == 0 {
				level = levelFromID(a.ID)
			}
			refs[a.ID] = append(refs[a.ID], artifactRef{
				path:         path,
				id:           a.ID,
				artifactType: a.ArtifactType,
				parentID:     a.ParentID,
				status:       string(a.Status),
				level:        level,
			})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", dirPath, walkErr)
		}
	}
	return refs, nil
}
