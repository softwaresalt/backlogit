package format_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli/format"
)

// TestFormatJSONRPCConstant verifies the constant value.
func TestFormatJSONRPCConstant(t *testing.T) {
	assert.Equal(t, format.Format("jsonrpc"), format.FormatJSONRPC)
}

// TestWrapResult_Structure verifies the full JSON-RPC 2.0 response envelope.
func TestWrapResult_Structure(t *testing.T) {
	data := map[string]string{"id": "001-F", "title": "Alpha"}
	b, err := format.WrapResult("backlogit list", data)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(b, &resp))

	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit list", resp["id"])
	assert.NotNil(t, resp["result"])
	assert.Nil(t, resp["error"])
}

// TestWrapResult_NilResult produces a JSON-RPC response with a null result.
func TestWrapResult_NilResult(t *testing.T) {
	b, err := format.WrapResult("backlogit sync", nil)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(b, &resp))
	assert.Equal(t, "2.0", resp["jsonrpc"])
	_, hasResult := resp["result"]
	assert.True(t, hasResult)
}

// TestWrapError_Structure verifies the JSON-RPC 2.0 error envelope.
func TestWrapError_Structure(t *testing.T) {
	b, err := format.WrapError("backlogit get", -32602, "invalid params")
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(b, &resp))

	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit get", resp["id"])
	assert.Nil(t, resp["result"])

	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(-32602), errObj["code"])
	assert.Equal(t, "invalid params", errObj["message"])
}

// TestJSONRPCRenderer_ImplementsRenderer verifies interface satisfaction.
func TestJSONRPCRenderer_ImplementsRenderer(t *testing.T) {
	var _ format.Renderer = format.NewJSONRPCRenderer("test")
}

// TestJSONRPCRenderer_Render_SuccessEnvelope verifies renderer wraps rows correctly.
func TestJSONRPCRenderer_Render_SuccessEnvelope(t *testing.T) {
	r := format.NewJSONRPCRenderer("backlogit list")
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))

	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit list", resp["id"])

	result, ok := resp["result"].([]any)
	require.True(t, ok, "result should be a JSON array")
	assert.Len(t, result, len(testRows))
	assert.Nil(t, resp["error"])
}

// TestJSONRPCRenderer_Render_OnlyDeclaredColumns verifies column scoping.
func TestJSONRPCRenderer_Render_OnlyDeclaredColumns(t *testing.T) {
	r := format.NewJSONRPCRenderer("backlogit list")
	var buf bytes.Buffer

	cols := []format.Column{{Key: "id", Header: "ID"}}
	rows := []map[string]any{
		{"id": "001-F", "title": "Should be excluded", "status": "active"},
	}
	err := r.Render(&buf, cols, rows)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))

	result := resp["result"].([]any)
	elem := result[0].(map[string]any)
	assert.Equal(t, "001-F", elem["id"])
	_, hasTitle := elem["title"]
	assert.False(t, hasTitle, "undeclared column should be excluded")
}

// TestJSONRPCRenderer_Render_EmptyRows verifies empty result set.
func TestJSONRPCRenderer_Render_EmptyRows(t *testing.T) {
	r := format.NewJSONRPCRenderer("backlogit list")
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, []map[string]any{})
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())
	assert.Contains(t, out, `"result":[]`)
}

// TestNewJSONRPCFormat verifies that NewRenderer dispatches jsonrpc correctly.
func TestNewJSONRPCFormat(t *testing.T) {
	r, err := format.NewRenderer(format.FormatJSONRPC, "backlogit list")
	require.NoError(t, err)
	assert.NotNil(t, r)
}

// TestJSONRPCCodeConstants verifies exported error code constants.
func TestJSONRPCCodeConstants(t *testing.T) {
	assert.Equal(t, -32700, format.ErrCodeParseError)
	assert.Equal(t, -32600, format.ErrCodeInvalidRequest)
	assert.Equal(t, -32601, format.ErrCodeMethodNotFound)
	assert.Equal(t, -32602, format.ErrCodeInvalidParams)
	assert.Equal(t, -32603, format.ErrCodeInternalError)
	assert.Equal(t, -32000, format.ErrCodeServerError)
}
