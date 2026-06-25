package core

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
// across the queue and archive directories.
func scanCanonicalArtifacts(ws *Workspace) (map[string][]artifactRef, error) {
	// TODO(066.001-T): implement the shared recursive canonical-ID scanner.
	return nil, nil
}
