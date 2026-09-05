package docline

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/softwaresalt/backlogit/internal/core"
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

// ValidateClassifyPath validates path before classification. It explicitly
// rejects:
//
//   - empty or whitespace-only paths,
//   - absolute paths (leading / or \ on any platform),
//   - volume- or UNC-qualified paths (C:\, D:/, \\host),
//
// and then calls core.SafeResolve for traversal/escape validation.
//
// This function is called by BOTH the CLI and MCP surfaces to enforce identical
// input rejection (155.003-T / U3). core.SafeResolve validates only the joined
// result and does not reject empty or absolute raw path inputs before joining,
// so the explicit checks above are required to cover those cases.
func ValidateClassifyPath(root string, path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("classify path must not be empty")
	}
	// Reject absolute paths (leading / or \).
	if path[0] == '/' || path[0] == '\\' {
		return fmt.Errorf("classify path must be relative, got absolute path: %q", path)
	}
	// Reject volume-qualified (C:\ or C:/) and UNC (\\host) forms.
	// The UNC case (\\) is already caught by the backslash check above,
	// but the volume-letter check is platform-neutral and covers both slash forms.
	if len(path) >= 2 && path[1] == ':' {
		return fmt.Errorf("classify path must be relative, got volume-qualified path: %q", path)
	}
	// After explicit raw-input checks, call SafeResolve for traversal/escape.
	if _, err := core.SafeResolve(root, path); err != nil {
		return err
	}
	return nil
}
