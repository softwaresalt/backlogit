package soundness

import (
	"errors"
	"path"
	"regexp"
	"strings"
)

// helpers shared by the consistency rules.

func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// normalizePath returns a slash-separated, cleaned, lower-cased path key so
// that equivalent spellings (a/../b.go, ./b.go) compare equal. Case is folded
// deliberately: the linter must not treat Parser.go and parser.go as distinct
// deliverables on case-insensitive file systems. A blank path yields "".
func normalizePath(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "\\", "/")))
	if s == "" {
		return ""
	}
	return path.Clean(s)
}

var driveLetter = regexp.MustCompile(`^[a-z]:[/\\]`)

// confinedArtifact validates that an artifact path stays inside the workspace
// and returns its normalized key. Absolute and parent-escaping paths are rejected.
func confinedArtifact(p string) (string, error) {
	key := normalizePath(p)
	switch {
	case key == "":
		return "", errors.New("empty path")
	case strings.HasPrefix(key, "/") || driveLetter.MatchString(key):
		return "", errors.New("absolute path is outside the workspace")
	case key == ".." || strings.HasPrefix(key, "../"):
		return "", errors.New("path escapes the workspace root")
	}
	return key, nil
}

func contains(list []string, id string) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

// related reports whether two units are intentionally linked by a
// dependency, parent/child relationship or explicit related edge.
func related(a, b *Unit) bool {
	return contains(a.Dependencies, b.ID) || contains(b.Dependencies, a.ID) ||
		contains(a.Related, b.ID) || contains(b.Related, a.ID) ||
		a.ParentID == b.ID || b.ParentID == a.ID
}
