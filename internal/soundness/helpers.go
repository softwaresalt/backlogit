package soundness

import (
	"strings"
)

// helpers shared by the consistency rules.

func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func normalizePath(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "\\", "/")))
	return strings.TrimPrefix(s, "./")
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
