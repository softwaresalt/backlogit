package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBacklogitDocsLint_ContractText_DecodeErrorSuccessfulResult is scenario
// 1 of 146.024-T (U9c): the registered backlogit_docs_lint tool description
// must name the decode_error rule value and state that a malformed document
// is returned as a finding in a SUCCESSFUL tool result rather than failing
// the tool call. Assertions read the registered description through the
// exported server handle, never a package-level literal, so text that is
// written but never registered cannot produce a false green. This is
// 146.018-T (U8b)'s own red harness, authored and observed failing before
// U8b.
func TestBacklogitDocsLint_ContractText_DecodeErrorSuccessfulResult(t *testing.T) {
	s, _ := setupBugFixServer(t)
	desc := strings.ToLower(findToolDescription(t, s, "backlogit_docs_lint"))

	require.NotEmpty(t, desc)
	assert.Contains(t, desc, "decode_error", "the registered description must name the decode_error rule value")
	assert.True(t,
		strings.Contains(desc, "successful") || strings.Contains(desc, "success"),
		"the registered description must state that a malformed document is returned in a successful tool result",
	)
	assert.False(t, strings.Contains(desc, "fail the tool call") || strings.Contains(desc, "fails the tool call"),
		"the registered description must not claim a malformed document fails the tool call")
}

// TestBacklogitDocsLint_ContractText_NoExitCodeMention is scenario 2 of
// 146.024-T (U9c): the registered backlogit_docs_lint description must never
// mention exit codes — MCP has none, and importing a shell concept into an
// agent-facing tool contract is a leaky abstraction (R11, agent-facing
// half).
func TestBacklogitDocsLint_ContractText_NoExitCodeMention(t *testing.T) {
	s, _ := setupBugFixServer(t)
	desc := strings.ToLower(findToolDescription(t, s, "backlogit_docs_lint"))

	assert.NotContains(t, desc, "exit code")
	assert.NotContains(t, desc, "non-zero")
	assert.NotContains(t, desc, "nonzero")
}
