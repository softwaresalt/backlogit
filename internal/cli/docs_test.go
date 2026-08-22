package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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

// TestDocsLint_CIGateContract pins the exit-code contract that the docs-lint CI
// gate and the `make docs-lint` target depend on: `docs lint` returns
// errLintViolations (which the binary maps to a non-zero process exit) when an
// in-scope doc violates the authoring profile, and returns nil (exit 0) when the
// tree is clean. If this contract ever regresses — e.g. lint starts exiting zero
// on violations — the CI gate would silently stop blocking non-compliant docs,
// so this test fails closed to protect 065.010-T's enforcement guarantee.
func TestDocsLint_CIGateContract(t *testing.T) {
	t.Run("fails closed on an in-scope violation", func(t *testing.T) {
		root := t.TempDir()
		// Missing source + doc_type → authoring-profile violation.
		writeFixtureDoc(t, root, "docs/reviews/bad.md", "---\ntitle: Bad\n---\nBody.\n")

		// Invoked exactly as the CI gate runs it: default profile.
		_, err := runDocs(t, root, "docs", "lint")
		require.Error(t, err, "lint must fail closed (non-zero exit) on violations")
		assert.True(t, errors.Is(err, errLintViolations),
			"the gate relies on errLintViolations to map to a non-zero exit code")
	})

	t.Run("passes on a clean tree", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureDoc(t, root, "docs/decisions/good.md",
			"---\nchunk_strategy: h1-h2-h3\ndescription: \"\"\ndoc_type: decision\ningested_at: \"2026-06-01T00:00:00Z\"\nschema_version: \"1.0\"\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n")

		_, err := runDocs(t, root, "docs", "lint")
		require.NoError(t, err, "a clean tree must exit zero so CI passes")
	})
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

	// --apply --yes with a path that resolves to the workspace root must also be
	// refused (closes the `--path .` / `--path docs/..` whole-tree bypass).
	_, err = runDocs(t, root, "docs", "migrate", "--apply", "--yes", "--path", ".")
	require.Error(t, err, "--path . must be refused as a whole-tree apply")

	_, err = runDocs(t, root, "docs", "migrate", "--apply", "--yes", "--path", "docs/..")
	require.Error(t, err, "--path docs/.. (resolves to root) must be refused")
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

func TestDocsLint_RejectsUnknownProfile(t *testing.T) {
	root := docsFixtureTree(t)
	// A profile typo must fail fast rather than silently defaulting to authoring.
	_, err := runDocs(t, root, "docs", "lint", "--profile", "authroing", "--format", "json")
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

func TestDocs_RejectsUnknownFormat(t *testing.T) {
	root := docsFixtureTree(t)
	// An unrecognized --format must fail fast rather than silently falling back
	// to text. Covers every surface that resolves a format.
	for _, args := range [][]string{
		{"docs", "scope", "--format", "jsn"},
		{"docs", "lint", "--format", "jsn"},
		{"docs", "migrate", "--format", "jsn"},
	} {
		_, err := runDocs(t, root, args...)
		require.Errorf(t, err, "args=%v", args)
		assert.ErrorContainsf(t, err, "invalid --format", "args=%v", args)
	}
}

// docsDegradedCorpus builds a fixture tree containing one file whose
// frontmatter cannot be decoded plus one ordinary contract violation, for the
// 146.020-T (U9a) degraded-corpus behavioral scenarios.
func docsDegradedCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureDoc(t, root, "docs/decisions/broken.md", "---\ntitle: [unclosed\n---\nBody.\n")
	writeFixtureDoc(t, root, "docs/decisions/missing-source.md", "---\ntitle: Missing Source\n---\nBody.\n")
	return root
}

// TestDocsLint_DegradedCorpus_ExitsNonZeroWithDecodeErrorFinding is scenario 1
// of 146.020-T (U9a): `backlogit docs lint` still exits non-zero and prints a
// report containing the decode_error finding for a degraded corpus.
func TestDocsLint_DegradedCorpus_ExitsNonZeroWithDecodeErrorFinding(t *testing.T) {
	root := docsDegradedCorpus(t)

	out, err := runDocs(t, root, "docs", "lint", "--format", "json")
	require.Error(t, err, "a degraded corpus must still exit non-zero")

	var report struct {
		Valid    bool `json:"valid"`
		Findings []struct {
			File string `json:"file"`
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &report), "the report must still be rendered even though a file failed to decode")
	assert.False(t, report.Valid)

	var sawDecodeError bool
	for _, f := range report.Findings {
		if f.File == "docs/decisions/broken.md" && f.Rule == "decode_error" {
			sawDecodeError = true
		}
	}
	assert.True(t, sawDecodeError, "the report must contain a decode_error finding for the undecodable file")
}

// TestDocsLint_DegradedCorpus_FindingsArrayPresentAndNonNull is scenario 2 of
// 146.020-T (U9a): the marshalled JSON is inspected directly for a present
// and non-null findings array using a .([]any) type assertion, which fails
// for both an absent key and a JSON null.
func TestDocsLint_DegradedCorpus_FindingsArrayPresentAndNonNull(t *testing.T) {
	root := docsDegradedCorpus(t)

	out, err := runDocs(t, root, "docs", "lint", "--format", "json")
	require.Error(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &raw))
	findings, ok := raw["findings"].([]any)
	require.True(t, ok, "findings must be present and decode as a non-null JSON array")
	assert.NotEmpty(t, findings)
}

// TestDocsLint_PathEscape_NoReportRendered is scenario 4 of 146.020-T (U9a):
// a corpus whose --path escapes the workspace makes the CLI exit non-zero
// with NO report rendered and NO decode_error finding. Green before and
// after 146.018-T (U8): this locks the existing containment mapping against
// U8's edits to the producing path.
func TestDocsLint_PathEscape_NoReportRendered(t *testing.T) {
	root := docsDegradedCorpus(t)

	out, err := runDocs(t, root, "docs", "lint", "--path", "../escape", "--format", "json")
	require.Error(t, err)
	assert.Empty(t, out, "a containment failure must render no report at all")
	assert.NotContains(t, out, "decode_error")
}
