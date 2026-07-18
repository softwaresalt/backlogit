package integration_test

// 107.001-T: Live tracked-corpus guard for the docline soft keys.
//
// Docline defaults make a missing chunk_strategy or schema_version invisible to
// the authoring validator. This guard walks the live, Git-tracked, in-scope
// markdown corpus (bounded by the exported docline.Scope() descriptor) and
// requires every document to declare the canonical soft keys with the exact
// production values chunk_strategy: h1-h2-h3 and schema_version: "1.0" (a YAML
// string). Untracked scratch files are excluded because the inventory is driven
// by `git ls-files`.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/softwaresalt/backlogit/internal/docline"
	"github.com/stretchr/testify/require"
)

const (
	softKeyChunkStrategy = "chunk_strategy"
	softKeySchemaVersion = "schema_version"
	wantChunkStrategy    = "h1-h2-h3"
	wantSchemaVersion    = "1.0"
)

// doclineInScope replicates the production in-scope decision using only the
// exported docline.Scope() descriptor, so the guard cannot drift from the
// service's own scope rules.
func doclineInScope(rel string, sc docline.ScopeDescriptor) bool {
	for _, f := range sc.ExcludeFiles {
		if rel == f {
			return false
		}
	}
	for _, d := range sc.ExcludeDirs {
		if strings.HasPrefix(rel, d) {
			return false
		}
	}
	for _, f := range sc.IncludeFiles {
		if rel == f {
			return true
		}
	}
	for _, d := range sc.IncludeDirs {
		if strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}

// inScopeTrackedMarkdown returns the sorted, repo-relative POSIX paths of the
// Git-tracked, in-scope markdown files under root. It is the shared inventory
// helper used by both the live-corpus guard and the hermetic value test.
func inScopeTrackedMarkdown(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	require.NoErrorf(t, err, "git ls-files must succeed in %s", root)

	sc := docline.Scope()
	var files []string
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}
		if !doclineInScope(rel, sc) {
			continue
		}
		files = append(files, rel)
	}
	sort.Strings(files)
	return files
}

// softKeyViolations checks a decoded frontmatter map against the canonical soft
// keys and returns one message per violation (missing, wrong type, wrong value).
// It is the shared value helper used by both the live-corpus and hermetic tests.
func softKeyViolations(fm map[string]any) []string {
	want := []struct{ key, val string }{
		{softKeyChunkStrategy, wantChunkStrategy},
		{softKeySchemaVersion, wantSchemaVersion},
	}
	var violations []string
	for _, w := range want {
		raw, ok := fm[w.key]
		if !ok {
			violations = append(violations, w.key+": missing")
			continue
		}
		s, ok := raw.(string)
		if !ok {
			violations = append(violations, fmt.Sprintf("%s: not a string (got %T)", w.key, raw))
			continue
		}
		if s != w.val {
			violations = append(violations, fmt.Sprintf("%s: want %q, got %q", w.key, w.val, s))
		}
	}
	return violations
}

// softKeyOffenders decodes each file through the production docline codec and
// returns a map of repo-relative path to its ordered soft-key violations. A
// decode error or an absent frontmatter block is itself a violation.
func softKeyOffenders(t *testing.T, root string, files []string) map[string][]string {
	t.Helper()
	offenders := map[string][]string{}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoErrorf(t, err, "read %s", rel)
		md, err := docline.Decode(raw)
		if err != nil {
			offenders[rel] = []string{"decode error: " + err.Error()}
			continue
		}
		if !md.HasFrontmatter {
			offenders[rel] = []string{"no frontmatter block"}
			continue
		}
		if v := softKeyViolations(md.Frontmatter); len(v) > 0 {
			offenders[rel] = v
		}
	}
	return offenders
}

// TestDoclineSoftKeys_LiveTrackedCorpus is the live regression guard. It fails
// with a deterministic, path-specific report whenever a tracked in-scope doc
// omits or misdeclares a canonical soft key.
func TestDoclineSoftKeys_LiveTrackedCorpus(t *testing.T) {
	root := findRepoRoot(t)

	// Pin the guard's expected literals to docline's exported production
	// defaults so the corpus contract cannot silently drift from the codec's
	// own canonical soft-key values.
	require.Equal(t, docline.DefaultChunkStrategy, wantChunkStrategy,
		"guard chunk_strategy literal must equal docline.DefaultChunkStrategy")
	require.Equal(t, docline.DefaultSchemaVersion, wantSchemaVersion,
		"guard schema_version literal must equal docline.DefaultSchemaVersion")

	// Compute the inventory and offenders eagerly at parent scope so each
	// subtest stays valid under -run filtering instead of depending on sibling
	// state; a filtered subtest must never pass vacuously.
	files := inScopeTrackedMarkdown(t, root)
	offenders := softKeyOffenders(t, root, files)

	t.Run("git_scope_inventory", func(t *testing.T) {
		require.NotEmpty(t, files, "expected a non-empty in-scope tracked markdown corpus")
	})

	t.Run("deterministic_path_report", func(t *testing.T) {
		if len(offenders) == 0 {
			return
		}
		paths := make([]string, 0, len(offenders))
		for p := range offenders {
			paths = append(paths, p)
		}
		sort.Strings(paths)

		var b strings.Builder
		for _, p := range paths {
			b.WriteString("\n  " + p + ":")
			for _, v := range offenders[p] {
				b.WriteString("\n    - " + v)
			}
		}
		t.Errorf("%d tracked in-scope doc(s) violate the docline soft-key contract (%s=%q, %s=%q):%s",
			len(offenders), softKeyChunkStrategy, wantChunkStrategy, softKeySchemaVersion, wantSchemaVersion, b.String())
	})
}
