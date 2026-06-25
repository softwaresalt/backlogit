package docline

import "path/filepath"

// Classify returns the doc_type for a repo-relative POSIX path. Classification
// is purely directory-based (longest-prefix), then explicit root-file
// overrides, then the docs/*.md direct-child "guide" default — see the
// 065.001-T decision doc §2. The path is authoritative: an existing legacy
// `type` field never overrides it; the normalizer folds that legacy key under
// docline.type (move, never drop).
func Classify(relPath string) DocType {
	return classifyDocType(filepath.ToSlash(relPath))
}

// DeriveSource returns the repo-relative POSIX `source` value for a path. Per
// the Q2 recommendation (065.002-T), source is a repo-relative POSIX path.
// Backslashes are converted to forward slashes so Windows-derived paths
// normalize deterministically across platforms.
func DeriveSource(relPath string) string {
	return filepath.ToSlash(relPath)
}
