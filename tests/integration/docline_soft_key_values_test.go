package integration_test

// 107.008-T: Hermetic value semantics for the docline soft keys.
//
// These negatives stay active after the live corpus is compliant. They pin the
// exact-value/type contract (chunk_strategy: h1-h2-h3, schema_version: "1.0" as
// a YAML string) and prove that the guard inspects only Git-tracked files: an
// invalid untracked document is excluded from the inventory entirely.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwaresalt/backlogit/internal/docline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	compliantDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n" +
		"chunk_strategy: h1-h2-h3\nschema_version: \"1.0\"\n---\n\nbody\n"
	missingChunkDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n" +
		"schema_version: \"1.0\"\n---\n\nbody\n"
	missingSchemaDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n" +
		"chunk_strategy: h1-h2-h3\n---\n\nbody\n"
	missingBothDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n---\n\nbody\n"
	wrongValueDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n" +
		"chunk_strategy: h1-h2\nschema_version: \"1.0\"\n---\n\nbody\n"
	// schema_version: 1.0 (unquoted) decodes to a YAML float, not the string "1.0".
	wrongTypeDoc = "---\n" +
		"title: t\nsource: docs/decisions/x.md\ndoc_type: decision\n" +
		"chunk_strategy: h1-h2-h3\nschema_version: 1.0\n---\n\nbody\n"
	// A dangling flow-sequence open bracket is a hard YAML parse error.
	malformedDoc = "---\n" +
		"title: [unterminated\nchunk_strategy: h1-h2-h3\n---\n\nbody\n"
)

// runGit runs a git subcommand in dir and fails the test on a non-zero exit.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(out))
}

// newTempDoclineRepo creates an initialized temp Git repository, writes and
// stages the tracked files, then writes the untracked files without staging.
func newTempDoclineRepo(t *testing.T, tracked, untracked map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "guard@example.com")
	runGit(t, dir, "config", "user.name", "guard")

	write := func(files map[string]string) {
		for rel, content := range files {
			abs := filepath.Join(dir, filepath.FromSlash(rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
			require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
		}
	}
	write(tracked)
	for rel := range tracked {
		runGit(t, dir, "add", "--", rel)
	}
	write(untracked)
	return dir
}

// TestDoclineSoftKeyValues_Hermetic pins the value/type negatives and the
// tracked-versus-untracked exclusion using temporary Git repositories and the
// same production Scope/value helpers as the live guard.
func TestDoclineSoftKeyValues_Hermetic(t *testing.T) {
	t.Run("compliant_and_each_missing_key", func(t *testing.T) {
		cases := []struct {
			name string
			doc  string
			want []string // nil == compliant
		}{
			{"compliant", compliantDoc, nil},
			{"missing_chunk_strategy", missingChunkDoc, []string{"chunk_strategy: missing"}},
			{"missing_schema_version", missingSchemaDoc, []string{"schema_version: missing"}},
			{"missing_both", missingBothDoc, []string{"chunk_strategy: missing", "schema_version: missing"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				md, err := docline.Decode([]byte(tc.doc))
				require.NoError(t, err)
				require.True(t, md.HasFrontmatter)
				got := softKeyViolations(md.Frontmatter)
				assert.ElementsMatch(t, tc.want, got)
			})
		}
	})

	t.Run("wrong_value_type_and_malformed", func(t *testing.T) {
		t.Run("wrong_value", func(t *testing.T) {
			md, err := docline.Decode([]byte(wrongValueDoc))
			require.NoError(t, err)
			got := strings.Join(softKeyViolations(md.Frontmatter), "\n")
			assert.Contains(t, got, `chunk_strategy: want "h1-h2-h3", got "h1-h2"`)
		})
		t.Run("wrong_type", func(t *testing.T) {
			md, err := docline.Decode([]byte(wrongTypeDoc))
			require.NoError(t, err)
			got := strings.Join(softKeyViolations(md.Frontmatter), "\n")
			assert.Contains(t, got, "schema_version: not a string")
		})
		t.Run("malformed_yaml", func(t *testing.T) {
			_, err := docline.Decode([]byte(malformedDoc))
			assert.Error(t, err, "malformed frontmatter YAML must fail decoding")
		})
	})

	t.Run("invalid_tracked_detected_untracked_excluded", func(t *testing.T) {
		const (
			goodTracked  = "docs/decisions/good-tracked.md"
			badTracked   = "docs/decisions/bad-tracked.md"
			badUntracked = "docs/decisions/bad-untracked.md"
		)
		repo := newTempDoclineRepo(t,
			map[string]string{goodTracked: compliantDoc, badTracked: missingBothDoc},
			map[string]string{badUntracked: missingBothDoc},
		)

		files := inScopeTrackedMarkdown(t, repo)
		assert.ElementsMatch(t, []string{goodTracked, badTracked}, files,
			"inventory must include tracked in-scope docs and exclude the untracked one")

		offenders := softKeyOffenders(t, repo, files)
		assert.Contains(t, offenders, badTracked, "invalid tracked doc must be flagged")
		assert.NotContains(t, offenders, goodTracked, "compliant tracked doc must pass")
		assert.NotContains(t, offenders, badUntracked, "invalid untracked doc must never be inventoried")
	})
}
