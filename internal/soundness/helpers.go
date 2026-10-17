package soundness

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/softwaresalt/backlogit/internal/docline"
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

// driveRelative matches c:foo, a path relative to a drive's current directory.
var driveRelative = regexp.MustCompile(`^[a-z]:`)

// confinedArtifact validates that an artifact path stays inside the workspace
// and returns its normalized key. Absolute and parent-escaping paths are rejected.
// hasRawDotDot reports whether p has a ".." segment before cleaning. Cleaning
// would hide a symlink followed by ".." (link/../x resolves outside the root
// but cleans to x), so such paths are rejected outright.
func hasRawDotDot(p string) bool {
	for _, seg := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if strings.TrimSpace(seg) == ".." {
			return true
		}
	}
	return false
}

func confinedArtifact(p string) (string, error) {
	if hasRawDotDot(p) {
		return "", errors.New("path escapes the workspace root")
	}
	key := normalizePath(p)
	switch {
	case key == "":
		return "", errors.New("empty path")
	case key == ".":
		return "", errors.New("path names the workspace root, not a file")
	case driveRelative.MatchString(key) && !driveLetter.MatchString(key):
		return "", errors.New("drive-relative path is outside the workspace")
	case strings.HasPrefix(key, "/") || driveLetter.MatchString(key):
		return "", errors.New("absolute path is outside the workspace")
	case key == ".." || strings.HasPrefix(key, "../"):
		return "", errors.New("path escapes the workspace root")
	}
	return key, nil
}

// confinedArtifactIn applies confinedArtifact and, when root is set, also
// resolves the deepest existing ancestor of the path so a symlink or reparse
// point that leaves the workspace is rejected. Missing in-root outputs remain
// valid because only the existing prefix is resolved.
func confinedArtifactIn(root, p string) (string, error) {
	key, err := confinedArtifact(p)
	if err != nil || root == "" {
		return key, err
	}
	rel := path.Clean(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
	for rel != "." {
		_, rerr := docline.ResolveWorkspacePath(root, rel)
		switch {
		case rerr == nil:
			return key, nil
		case errors.Is(rerr, docline.ErrAnchorOutsideRoot):
			return "", fmt.Errorf("path escapes the workspace root: %w", rerr)
		case !errors.Is(rerr, docline.ErrAnchorNotFound):
			return "", fmt.Errorf("path cannot be confined to the workspace: %w", rerr)
		}
		// A dangling link is not "missing": it could later resolve outside the root.
		if _, lerr := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); lerr == nil {
			return "", errors.New("path is a dangling link and cannot be confined to the workspace")
		}
		rel = path.Dir(rel)
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

// relation answers "is this unit intentionally linked to that one" for one
// fixed unit. Transitive dependency sets in both directions are computed once,
// lazily, and discarded with the relation, so no per-unit reach sets are kept
// on the context (memory stays O(units) at any instant).
type relation struct {
	ctx  *checkContext
	u    *Unit
	up   map[string]bool // units u transitively depends on
	down map[string]bool // units that transitively depend on u
}

func newRelation(ctx *checkContext, u *Unit) *relation { return &relation{ctx: ctx, u: u} }

// with reports whether other is linked to the relation's unit by a
// parent/child edge, an explicit related edge or a transitive dependency in
// either direction. Dependency cycles terminate.
func (r *relation) with(other *Unit) bool {
	u := r.u
	if u.ParentID == other.ID || other.ParentID == u.ID ||
		contains(u.Related, other.ID) || contains(other.Related, u.ID) {
		return true
	}
	if r.up == nil {
		r.up = transitiveDependencies(r.ctx, u.ID)
		r.down = transitiveDependents(r.ctx, u.ID)
	}
	return r.up[other.ID] || r.down[other.ID]
}

// transitiveDependents returns every unit ID that transitively depends on id.
func transitiveDependents(ctx *checkContext, id string) map[string]bool {
	if ctx.dependents == nil {
		ctx.dependents = map[string][]string{}
		for i := range ctx.plan.Units {
			u := &ctx.plan.Units[i]
			if ctx.byID[u.ID] != u {
				continue
			}
			for _, d := range u.Dependencies {
				ctx.dependents[d] = append(ctx.dependents[d], u.ID)
			}
		}
	}
	ctx.walks++
	return walk(id, func(cur string) []string { return ctx.dependents[cur] })
}

// transitiveDependencies returns every unit ID reachable from id through
// dependency edges. The walk is iterative and cycle-safe and the result is not
// cached; callers hold it only as long as needed.
func transitiveDependencies(ctx *checkContext, id string) map[string]bool {
	ctx.walks++
	return walk(id, func(cur string) []string {
		if u := ctx.byID[cur]; u != nil {
			return u.Dependencies
		}
		return nil
	})
}

func walk(start string, next func(string) []string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range next(cur) {
			if !seen[d] {
				seen[d] = true
				stack = append(stack, d)
			}
		}
	}
	return seen
}
