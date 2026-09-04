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

// 154.003-T (U2b) MCP harness.
//
// TestU2bMCPGuard_PlanHasFindingsHelperShape is the unit-level acceptance test
// for planHasFindingsResult: it must return IsError:true, error=="plan_has_findings"
// (NOT "validation_failed" or "internal"), and a discrete top-level findings array.
// This test is reachable pre-U2c (pure unit test of the new MCP helper).
//
// RED until planHasFindingsResult is implemented in docs_tools.go.
//
// The end-to-end integration test (full pipeline with a real malformed corpus,
// PlanMigration report-and-continue + ApplyMigration rejection) lives in the
// Wave 3 harness (154_004_decode_policy_test.go) because it also requires U2c.

// writeMCPGuardFixture writes content to root/rel, creating parent dirs.
func writeMCPGuardFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

// TestU2bMCPGuard_PlanHasFindingsHelperShape verifies the planHasFindingsResult
// helper function directly (reachable pre-U2c). RED before U2b adds the helper.
func TestU2bMCPGuard_PlanHasFindingsHelperShape(t *testing.T) {
	t.Parallel()

	plan := docline.MigrationPlan{
		Findings: []docline.Finding{
			{
				File:     "docs/decisions/broken.md",
				Rule:     docline.RuleDecodeError,
				Severity: docline.SeverityError,
				Fix:      "malformed frontmatter",
			},
		},
	}
	res := planHasFindingsResult(plan)
	require.NotNil(t, res)

	// The result must be an error (IsError == true).
	require.True(t, res.IsError, "planHasFindingsResult must return IsError:true")

	tc, ok := res.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &payload))

	// error field must be plan_has_findings.
	var errorType string
	require.NoError(t, json.Unmarshal(payload["error"], &errorType))
	assert.Equal(t, "plan_has_findings", errorType,
		"error field must be 'plan_has_findings', not 'validation_failed' or 'internal'")
	assert.NotEqual(t, "internal", errorType,
		"ErrPlanHasFindings must not fall through to InternalError")

	// The response must carry a discrete top-level findings array.
	findingsRaw, hasFindingsKey := payload["findings"]
	require.True(t, hasFindingsKey, "response must carry a top-level 'findings' field")
	var findings []docline.FindingReport
	require.NoError(t, json.Unmarshal(findingsRaw, &findings))
	require.Len(t, findings, 1)
	assert.Equal(t, "docs/decisions/broken.md", findings[0].File)
	assert.Equal(t, docline.RuleDecodeError, findings[0].Rule)
}
