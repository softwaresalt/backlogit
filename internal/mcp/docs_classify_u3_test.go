package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU3_DocsClassifyToolRegistered asserts that backlogit_docs_classify is
// registered in the server tool set (155.003-T / U3).
func TestU3_DocsClassifyToolRegistered(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	names := map[string]bool{}
	for _, td := range s.ToolDefs() {
		names[td.Name] = true
	}
	assert.True(t, names["backlogit_docs_classify"],
		"backlogit_docs_classify must be registered in the MCP server tool set")
}

// TestU3_DocsClassifyRejectsEmpty asserts that an empty path returns validation_failed.
func TestU3_DocsClassifyRejectsEmpty(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": ""}
	res, err := s.handleDocsClassify(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "empty path must produce an error result")
	text := docsResultText(t, res)
	assert.Contains(t, text, "validation_failed")
}

// TestU3_DocsClassifyRejectsAbsolute asserts that an absolute path returns validation_failed.
func TestU3_DocsClassifyRejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": "/etc/passwd"}
	res, err := s.handleDocsClassify(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "absolute path must produce an error result")
	text := docsResultText(t, res)
	assert.Contains(t, text, "validation_failed")
}

// TestU3_DocsClassifyReturnsDocType asserts that a valid relative path returns
// a JSON object with a doc_type field.
func TestU3_DocsClassifyReturnsDocType(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": "docs/decisions/001-arch.md"}
	res, err := s.handleDocsClassify(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError, "valid path must not produce an error result")

	text := docsResultText(t, res)
	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(text), &payload), "response must be valid JSON")
	docType, ok := payload["doc_type"]
	assert.True(t, ok, "response must contain doc_type field")
	assert.Equal(t, "decision", docType, "docs/decisions/... must classify as decision")
}
