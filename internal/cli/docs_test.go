package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runDocs executes the root command with the given args against root dir and
// returns stdout and the command error.
func runDocs(t *testing.T, rootDir string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	full := append([]string{"--cwd", rootDir}, args...)
	cmd.SetArgs(full)
	err := cmd.Execute()
	return out.String(), err
}

func writeFixtureDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

func docsFixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Compliant doc (normalized canonical form).
	writeFixtureDoc(t, root, "docs/decisions/good.md",
		"---\nchunk_strategy: h1-h2-h3\ndescription: \"\"\ndoc_type: decision\ningested_at: \"2026-06-01T00:00:00Z\"\nschema_version: \"1.0\"\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n")
	// Non-compliant doc (missing source + doc_type).
	writeFixtureDoc(t, root, "docs/reviews/bad.md", "---\ntitle: Bad\n---\nBody.\n")
	// Out-of-scope memory doc.
	writeFixtureDoc(t, root, "docs/memory/skip.md", "---\ntitle: Skip\n---\nBody.\n")
	return root
}

func TestDocsLint_PassesOnCompliantFixture(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "docs/decisions/good.md",
		"---\nchunk_strategy: h1-h2-h3\ndescription: \"\"\ndoc_type: decision\ningested_at: \"2026-06-01T00:00:00Z\"\nschema_version: \"1.0\"\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n")

	out, err := runDocs(t, root, "docs", "lint", "--format", "json")
	require.NoError(t, err)

	var res struct {
		Valid          bool `json:"valid"`
		ViolationCount int  `json:"violation_count"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.True(t, res.Valid)
	assert.Equal(t, 0, res.ViolationCount)
}

func TestDocsLint_FailsOnMissingRequired(t *testing.T) {
	root := docsFixtureTree(t)

	out, err := runDocs(t, root, "docs", "lint", "--format", "json")
	require.Error(t, err, "lint must exit non-zero when violations exist")

	var res struct {
		Valid          bool `json:"valid"`
		ViolationCount int  `json:"violation_count"`
		Findings       []struct {
			File  string `json:"file"`
			Field string `json:"field"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.False(t, res.Valid)
	assert.Greater(t, res.ViolationCount, 0)

	// Out-of-scope memory doc must never appear.
	for _, f := range res.Findings {
		assert.NotEqual(t, "docs/memory/skip.md", f.File)
	}
}

func TestDocsMigrate_DryRunWritesNothing(t *testing.T) {
	root := docsFixtureTree(t)
	badPath := filepath.Join(root, "docs", "reviews", "bad.md")
	before, err := os.ReadFile(badPath)
	require.NoError(t, err)

	out, err := runDocs(t, root, "docs", "migrate", "--format", "json")
	require.NoError(t, err)

	var res struct {
		DryRun  bool `json:"dry_run"`
		Changes []struct {
			File   string `json:"file"`
			Action string `json:"action"`
		} `json:"changes"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.True(t, res.DryRun)
	assert.NotEmpty(t, res.Changes)

	after, err := os.ReadFile(badPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "dry-run must not write")
}

func TestDocsMigrate_ApplyRequiresYesAndPath(t *testing.T) {
	root := docsFixtureTree(t)

	// --apply without --yes
	_, err := runDocs(t, root, "docs", "migrate", "--apply", "--path", "docs/reviews")
	require.Error(t, err)

	// --apply --yes without --path (refuses whole-tree apply)
	_, err = runDocs(t, root, "docs", "migrate", "--apply", "--yes")
	require.Error(t, err)
}

func TestDocsMigrate_ApplyWritesScopedPath(t *testing.T) {
	root := docsFixtureTree(t)
	_, err := runDocs(t, root, "docs", "migrate", "--apply", "--yes", "--path", "docs/reviews", "--format", "json")
	require.NoError(t, err)

	migrated, err := os.ReadFile(filepath.Join(root, "docs", "reviews", "bad.md"))
	require.NoError(t, err)
	assert.Contains(t, string(migrated), "doc_type: review")
	assert.Contains(t, string(migrated), "source: docs/reviews/bad.md")
}

func TestDocsMigrate_RejectsPathEscape(t *testing.T) {
	root := docsFixtureTree(t)
	_, err := runDocs(t, root, "docs", "migrate", "--path", "../outside", "--format", "json")
	require.Error(t, err)
}

func TestDocsClassify_PrintsDocType(t *testing.T) {
	root := t.TempDir()
	out, err := runDocs(t, root, "docs", "classify", "docs/decisions/x.md")
	require.NoError(t, err)
	assert.Contains(t, out, "decision")
}

func TestDocsScope_PrintsScope(t *testing.T) {
	root := t.TempDir()
	out, err := runDocs(t, root, "docs", "scope", "--format", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "doc_types")
}
