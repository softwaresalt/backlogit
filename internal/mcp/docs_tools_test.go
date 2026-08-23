package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

func writeMCPDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

func docsToolTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMCPDoc(t, root, "docs/reviews/bad.md", "---\ntitle: Bad\n---\nBody.\n")
	writeMCPDoc(t, root, "docs/memory/skip.md", "---\ntitle: Skip\n---\nBody.\n")
	return root
}

func callDocsTool(t *testing.T, handler func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error), args map[string]any) *mcplib.CallToolResult {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	res, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

// resultText extracts the text payload from a tool result.
func docsResultText(t *testing.T, res *mcplib.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent")
	return tc.Text
}

func TestDocsLintTool_SuccessWithFindings(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsLint, map[string]any{"profile": "authoring"})
	assert.False(t, res.IsError, "lint returns a success envelope even with violations")

	var report docline.LintReport
	require.NoError(t, json.Unmarshal([]byte(docsResultText(t, res)), &report))
	assert.False(t, report.Valid)
	assert.Greater(t, report.ViolationCount, 0)
	for _, f := range report.Findings {
		assert.NotEqual(t, "docs/memory/skip.md", f.File, "out-of-scope file excluded")
	}
}

func TestDocsMigrateTool_DefaultDryRunNoWrites(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)
	badPath := filepath.Join(root, "docs", "reviews", "bad.md")
	before, err := os.ReadFile(badPath)
	require.NoError(t, err)

	res := callDocsTool(t, s.handleDocsMigrate, map[string]any{})
	assert.False(t, res.IsError)

	var report docline.MigrateReport
	require.NoError(t, json.Unmarshal([]byte(docsResultText(t, res)), &report))
	assert.True(t, report.DryRun)
	assert.NotEmpty(t, report.Changes)

	after, err := os.ReadFile(badPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "dry-run must not write")
}

func TestDocsMigrateTool_ApplyGatedByDefault(t *testing.T) {
	t.Setenv(docsApplyAllowEnv, "") // ensure disabled
	root := docsToolTree(t)
	s := NewServerForRoot(root)
	badPath := filepath.Join(root, "docs", "reviews", "bad.md")
	before, err := os.ReadFile(badPath)
	require.NoError(t, err)

	res := callDocsTool(t, s.handleDocsMigrate, map[string]any{"apply": true, "path": "docs/reviews"})
	assert.True(t, res.IsError, "apply must be rejected when the gate is disabled")
	assert.Contains(t, docsResultText(t, res), "apply_not_permitted")

	after, err := os.ReadFile(badPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "no writes when apply is gated")
}

func TestDocsMigrateTool_ApplyRequiresPathWhenEnabled(t *testing.T) {
	t.Setenv(docsApplyAllowEnv, "1")
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	// apply enabled but no path → whole-tree apply refused (validation error).
	res := callDocsTool(t, s.handleDocsMigrate, map[string]any{"apply": true})
	assert.True(t, res.IsError)
	assert.Contains(t, docsResultText(t, res), "validation_failed")

	// apply enabled with path="." (resolves to workspace root) must also be
	// refused — closes the whole-tree apply bypass an agent could trigger.
	badPath := filepath.Join(root, "docs", "reviews", "bad.md")
	before, err := os.ReadFile(badPath)
	require.NoError(t, err)

	res = callDocsTool(t, s.handleDocsMigrate, map[string]any{"apply": true, "path": "."})
	assert.True(t, res.IsError, `path="." must be refused as a whole-tree apply`)
	assert.Contains(t, docsResultText(t, res), "validation_failed")

	after, err := os.ReadFile(badPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "no writes on refused whole-tree apply")
}

func TestDocsMigrateTool_ApplyWritesWhenEnabledAndScoped(t *testing.T) {
	t.Setenv(docsApplyAllowEnv, "1")
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsMigrate, map[string]any{"apply": true, "path": "docs/reviews"})
	assert.False(t, res.IsError)

	migrated, err := os.ReadFile(filepath.Join(root, "docs", "reviews", "bad.md"))
	require.NoError(t, err)
	assert.Contains(t, string(migrated), "doc_type: review")
}

func TestDocsScopeTool_ReturnsScope(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsScope, map[string]any{})
	assert.False(t, res.IsError)

	var sc docline.ScopeDescriptor
	require.NoError(t, json.Unmarshal([]byte(docsResultText(t, res)), &sc))
	assert.NotEmpty(t, sc.DocTypes)
	assert.NotEmpty(t, sc.Profiles)
}

func TestDocsTools_RejectPathEscape(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsLint, map[string]any{"path": "../outside"})
	assert.True(t, res.IsError)
	assert.Contains(t, docsResultText(t, res), "validation_failed")
}

func TestDocsLintTool_RejectsUnknownProfile(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	// A profile typo must fail fast (validation error), not silently default.
	res := callDocsTool(t, s.handleDocsLint, map[string]any{"profile": "authroing"})
	assert.True(t, res.IsError)
	assert.Contains(t, docsResultText(t, res), "validation_failed")
}

func TestDocsTools_DiscoverableViaListTools(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)
	// NewServerForRoot already calls RegisterTools() during construction; do not
	// re-register here or every tool would be registered twice.

	names := map[string]bool{}
	for _, td := range s.ToolDefs() {
		names[td.Name] = true
	}
	assert.True(t, names["backlogit_docs_lint"])
	assert.True(t, names["backlogit_docs_migrate"])
	assert.True(t, names["backlogit_docs_scope"])
}

// TestDocsTools_CLIParity asserts the MCP lint payload is structurally identical
// to the shared docline report the CLI also marshals.
func TestDocsTools_CLIParity(t *testing.T) {
	root := docsToolTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsLint, map[string]any{})
	mcpJSON := docsResultText(t, res)

	// Compute the expected report directly from the service (the same path the
	// CLI uses) and compare canonical JSON.
	findings, err := docline.LintTree(docline.Options{Root: root, Profile: docline.ProfileAuthoring})
	require.NoError(t, err)
	expected, err := json.Marshal(docline.NewLintReport(findings))
	require.NoError(t, err)

	var gotMap, wantMap any
	require.NoError(t, json.Unmarshal([]byte(mcpJSON), &gotMap))
	require.NoError(t, json.Unmarshal(expected, &wantMap))
	assert.Equal(t, wantMap, gotMap, "MCP and CLI must marshal identical lint payloads")
}

// docsToolDegradedTree builds a fixture tree containing one file whose
// frontmatter cannot be decoded plus one ordinary contract violation, for the
// 146.020-T (U9a) degraded-corpus behavioral scenarios.
func docsToolDegradedTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMCPDoc(t, root, "docs/decisions/broken.md", "---\ntitle: [unclosed\n---\nBody.\n")
	writeMCPDoc(t, root, "docs/decisions/missing-source.md", "---\ntitle: Missing Source\n---\nBody.\n")
	return root
}

// TestDocsLintTool_DegradedCorpus_SuccessfulResultNotInternalError is
// scenario 3 of 146.020-T (U9a): MCP returns a successful tool result
// carrying the same findings rather than InternalError, and the MCP finding
// payload for the same corpus is byte-identical to docline.LintTree's own
// output — the same underlying service call the CLI transport also makes
// (mirroring TestDocsTools_CLIParity's cross-surface comparison method,
// since neither this file nor that one invokes the compiled CLI binary
// directly).
func TestDocsLintTool_DegradedCorpus_SuccessfulResultNotInternalError(t *testing.T) {
	root := docsToolDegradedTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsLint, map[string]any{})
	require.False(t, res.IsError, "a degraded corpus must still return a successful tool result, never InternalError")

	mcpJSON := docsResultText(t, res)
	var report docline.LintReport
	require.NoError(t, json.Unmarshal([]byte(mcpJSON), &report))

	var sawDecodeError bool
	for _, f := range report.Findings {
		if f.File == "docs/decisions/broken.md" && f.Rule == "decode_error" {
			sawDecodeError = true
		}
	}
	assert.True(t, sawDecodeError, "the MCP result must contain a decode_error finding for the undecodable file")

	// Cross-surface parity: docline.LintTree is the same underlying call the
	// CLI's `docs lint` command makes (see internal/cli/docs.go), so
	// comparing against it directly is the established parity methodology in
	// this file (see TestDocsTools_CLIParity above).
	findings, err := docline.LintTree(docline.Options{Root: root, Profile: docline.ProfileAuthoring})
	require.NoError(t, err)
	expected, err := json.Marshal(docline.NewLintReport(findings))
	require.NoError(t, err)

	var gotMap, wantMap any
	require.NoError(t, json.Unmarshal([]byte(mcpJSON), &gotMap))
	require.NoError(t, json.Unmarshal(expected, &wantMap))
	assert.Equal(t, wantMap, gotMap, "MCP and the underlying docline service must marshal identical lint payloads for a degraded corpus")
}

// TestDocsLintTool_PathEscapeOnDegradedCorpus_ValidationFailed is scenario 5
// of 146.020-T (U9a): the same degraded-corpus input driven through
// backlogit_docs_lint with an escaping path returns a structured
// validation_failed result — never InternalError and never a successful
// result carrying a finding. Green before and after 146.018-T (U8): it locks
// the existing docline.ErrPathEscapesWorkspace -> validation_failed mapping
// against U8's edits to the producing path.
func TestDocsLintTool_PathEscapeOnDegradedCorpus_ValidationFailed(t *testing.T) {
	root := docsToolDegradedTree(t)
	s := NewServerForRoot(root)

	res := callDocsTool(t, s.handleDocsLint, map[string]any{"path": "../escape"})
	require.True(t, res.IsError, "an escaping path must fail as validation_failed, never succeed")
	text := docsResultText(t, res)
	assert.Contains(t, text, "validation_failed")
	assert.NotContains(t, text, "decode_error")
}
