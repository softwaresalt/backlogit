package docline

import (
	"regexp"
	"strings"
)

// uriSchemeRE matches a leading RFC 3986 scheme followed by "://" (e.g.
// "https://"), used to distinguish a full origin URI source from a
// repo-relative POSIX path.
var uriSchemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*://`)

// hasURIScheme reports whether s begins with a URI scheme followed by "://".
// Per the Q2 sign-off (task 065.002-T), a source that is a full origin URI (a
// known online source) is preserved by the normalizer instead of being
// rewritten to a repo-relative POSIX path.
func hasURIScheme(s string) bool {
	return uriSchemeRE.MatchString(s)
}

// toPOSIX normalizes any path separators to forward slashes deterministically
// across platforms. filepath.ToSlash only rewrites the OS-native separator, so
// a Windows-derived path (backslashes) reaching this code on Linux would keep
// its backslashes; replacing explicitly makes classification and source
// derivation platform-independent.
func toPOSIX(relPath string) string {
	return strings.ReplaceAll(relPath, `\`, "/")
}

// Classify returns the doc_type for a repo-relative POSIX path. Classification
// is purely directory-based (longest-prefix), then explicit root-file
// overrides, then the docs/*.md direct-child "guide" default — see the
// 065.001-T decision doc §2. The path is authoritative: an existing legacy
// `type` field never overrides it; the normalizer folds that legacy key under
// docline.type (move, never drop).
func Classify(relPath string) DocType {
	return classifyDocType(toPOSIX(relPath))
}

// DeriveSource returns the repo-relative POSIX `source` value for a path. Per
// the Q2 recommendation (065.002-T), source is a repo-relative POSIX path.
// Backslashes are converted to forward slashes so Windows-derived paths
// normalize deterministically across platforms.
func DeriveSource(relPath string) string {
	return toPOSIX(relPath)
}
