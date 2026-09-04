package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// 154.003-T (U2b) MCP harness: handleDocsMigrate with apply=true on a corpus
// that causes ApplyMigration to return ErrPlanHasFindings must return an
// IsError:true result with error field == "plan_has_findings" (NOT
// "validation_failed", NOT "internal") and a discrete top-level "findings"
// array via a dedicated response struct.
//
// RED until docs_tools.go maps errors.Is(err, ErrPlanHasFindings) to IsError:true
// with error_type "plan_has_findings" and a discrete findings array.

// writeMCPGuardFixture writes content to root/rel, creating parent dirs.
func writeMCPGuardFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

// TestU2bMCPGuard_PlanHasFindingsResultShape verifies that when
// handleDocsMigrate is called with apply=true on a corpus that leads to
// plan.Findings being non-empty (after U2c wires report-and-continue), the
// MCP response has IsError:true, error=="plan_has_findings" (not
// "validation_failed" or "internal"), and a top-level "findings" array.
//
// Pre-U2b: the handler does not map ErrPlanHasFindings, so it returns
// InternalError or ValidationFailed → error-type assertions fail RED ✓.
// Pre-U2c: PlanMigration hard-aborts on the malformed file so plan.Findings is
// never populated; the plan_has_findings guard is never triggered → test would
// still fail RED (wrong error type or no error at all).
// Post-U2b+U2c: full path is wired and all assertions pass GREEN.
func TestU2bMCPGuard_PlanHasFindingsResultShape(t *testing.T) {
	t.Setenv(docsApplyAllowEnv, "1")
	root := t.TempDir()
	s := NewServerForRoot(root)

	// A malformed-frontmatter file alongside a valid file: once U2c wires
	// report-and-continue, PlanMigration will populate plan.Findings with a
	// decode_error Finding for broken.md, and ApplyMigration will return
	// ErrPlanHasFindings (U2b guard). Pre-U2c, PlanMigration hard-aborts on
	// broken.md and the handler returns InternalError.
	writeMCPGuardFixture(t, root, "docs/decisions/broken.md",
		"---\ntitle: [unclosed yaml\n---\nBody.\n")
	writeMCPGuardFixture(t, root, "docs/decisions/good.md",
		"---\ndoc_type: decision\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n")

	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"apply": true,
		"path":  "docs/decisions",
	}
	res, err := s.handleDocsMigrate(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	// The result must be an error result.
	require.True(t, res.IsError,
		"handleDocsMigrate must return IsError:true when plan has findings")

	// The error field must be plan_has_findings — not validation_failed or internal.
	tc, ok := res.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &payload))

	var errorType string
	require.NoError(t, json.Unmarshal(payload["error"], &errorType))
	assert.Equal(t, "plan_has_findings", errorType,
		"error field must be 'plan_has_findings', not 'validation_failed' or 'internal'")
	assert.NotEqual(t, "internal", errorType,
		"ErrPlanHasFindings must not fall through to InternalError")

	// The response must carry a discrete top-level findings array (not flattened
	// into message).
	findingsRaw, hasFindingsKey := payload["findings"]
	require.True(t, hasFindingsKey,
		"response must carry a top-level 'findings' field (not flattened into message)")
	var findings []docline.FindingReport
	require.NoError(t, json.Unmarshal(findingsRaw, &findings),
		"findings field must be a JSON array of FindingReport objects")
}
