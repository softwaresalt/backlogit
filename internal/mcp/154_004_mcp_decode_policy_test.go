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

// 154.004-T (U2c) MCP harness: end-to-end integration test for the full
// pipeline once PlanMigration report-and-continue (U2c) is wired.
//
// Pre-U2c: PlanMigration hard-aborts on the malformed file and the handler
// returns InternalError → error-type assertions fail RED ✓.
// Post-U2b+U2c: full path wired → plan.Findings populated → apply returns
// ErrPlanHasFindings → handler maps to plan_has_findings → GREEN.

// writeMCPU2cFixture writes content to root/rel, creating parent dirs.
// Reuses writeMCPGuardFixture's body; consolidation into a shared test helper
// is a P2-5 follow-up from the U2b adversarial review.
func writeMCPU2cFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	writeMCPGuardFixture(t, root, rel, content)
}

// TestU2cMCP_MixedCorpus_ApplyReturnsPlanHasFindings is the end-to-end MCP
// integration test for the full U2b+U2c pipeline: a corpus with one malformed
// file causes PlanMigration to report decode_error and continue, and the
// subsequent apply is rejected with error_type==plan_has_findings carrying a
// discrete findings array.
//
// RED until U2c wires PlanMigration report-and-continue (so the malformed file
// populates plan.Findings rather than aborting).
func TestU2cMCP_MixedCorpus_ApplyReturnsPlanHasFindings(t *testing.T) {
	t.Setenv(docsApplyAllowEnv, "1")
	root := t.TempDir()
	s := NewServerForRoot(root)

	// Malformed-frontmatter file + a valid file in the same scoped path.
	writeMCPU2cFixture(t, root, "docs/decisions/broken.md",
		"---\ntitle: [unclosed yaml\n---\nBody.\n")
	writeMCPU2cFixture(t, root, "docs/decisions/good.md",
		"---\ndoc_type: decision\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n")

	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"apply": true,
		"path":  "docs/decisions",
	}
	res, err := s.handleDocsMigrate(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	// The result must be an error result (ErrPlanHasFindings guard fired).
	require.True(t, res.IsError,
		"handleDocsMigrate must return IsError:true when plan has findings (ErrPlanHasFindings guard)")

	tc, ok := res.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &payload))

	// error field must be plan_has_findings — not validation_failed or internal.
	var errorType string
	require.NoError(t, json.Unmarshal(payload["error"], &errorType))
	assert.Equal(t, "plan_has_findings", errorType,
		"error field must be 'plan_has_findings'")
	assert.NotEqual(t, "internal", errorType,
		"ErrPlanHasFindings must not fall through to InternalError")

	// Discrete top-level findings array (not flattened into message).
	findingsRaw, hasFindingsKey := payload["findings"]
	require.True(t, hasFindingsKey,
		"response must carry a top-level 'findings' field")
	var findings []docline.FindingReport
	require.NoError(t, json.Unmarshal(findingsRaw, &findings))
	require.NotEmpty(t, findings,
		"findings must be non-empty (the broken.md decode_error)")
	assert.Equal(t, "docs/decisions/broken.md", findings[0].File)
	assert.Equal(t, docline.RuleDecodeError, findings[0].Rule)

	// ZERO writes: good.md must be unchanged (corpus all-or-nothing).
	goodPath := filepath.Join(root, "docs", "decisions", "good.md")
	after, readErr := os.ReadFile(goodPath)
	require.NoError(t, readErr)
	assert.Equal(t,
		"---\ndoc_type: decision\nsource: docs/decisions/good.md\ntitle: Good\n---\nBody.\n",
		string(after),
		"ApplyMigration must write ZERO files when plan carries Findings")
}
