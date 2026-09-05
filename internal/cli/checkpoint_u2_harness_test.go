package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/cli/format"
)

// TestU2_ExecuteHandlesCheckpointUnknownFieldStructured asserts that
// internal/cli/root.go Execute() handles CheckpointUnknownFieldError with a
// structured data envelope rather than the generic string-only path
// (155.002-T / U2). Source-shape harness: compiles before implementation but
// fails the assertion because Execute() does not yet reference those identifiers.
func TestU2_ExecuteHandlesCheckpointUnknownFieldStructured(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "root.go", nil, 0)
	require.NoError(t, err)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "Execute" {
			funcDecl = fn
			break
		}
	}
	require.NotNil(t, funcDecl, "Execute function not found in root.go")

	// Check if Execute body references "CheckpointUnknownFieldError" or "WrapErrorData" —
	// either identifier confirms the structured-error path is wired.
	referencesStructured := false
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "CheckpointUnknownFieldError" || ident.Name == "WrapErrorData" {
				referencesStructured = true
			}
		}
		return !referencesStructured
	})
	assert.True(t, referencesStructured,
		"Execute() must reference CheckpointUnknownFieldError or WrapErrorData for structured error handling")
}

// TestU2_CLICheckpointCreateUnknownFieldStructuredDataOnWire verifies that
// backlogit checkpoint create --jsonrpc with an unknown field emits a structured
// JSON-RPC error with data.unknown_fields on the wire (155.002-T / U2).
// The structured data must carry all four bounded-projection keys.
func TestU2_CLICheckpointCreateUnknownFieldStructuredDataOnWire(t *testing.T) {
	root := setupCLIWorkspace(t)
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","novel_key":"v"}`

	output, execErr := u2ExecuteWithCapturedStdout(t,
		"--jsonrpc", "--cwd", root,
		"checkpoint", "create",
		"--state-dump", stateDump,
	)
	// Execute returns the original command error even on the JSON-RPC path.
	assert.Error(t, execErr, "Execute must return error for unknown field dump")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace([]byte(output)), &resp),
		"raw stdout: %s", output)

	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok, "response must have error object; got: %v", resp)
	assert.Equal(t, float64(format.ErrCodeServerError), errObj["code"])

	data, ok := errObj["data"].(map[string]any)
	require.True(t, ok, "error object must carry structured data field; errObj: %v", errObj)

	_, hasFields := data["unknown_fields"]
	assert.True(t, hasFields, "data must have unknown_fields")
	_, hasTruncated := data["unknown_fields_truncated"]
	assert.True(t, hasTruncated, "data must have unknown_fields_truncated")
	_, hasOmitted := data["unknown_fields_omitted"]
	assert.True(t, hasOmitted, "data must have unknown_fields_omitted")
	_, hasShortened := data["unknown_fields_shortened"]
	assert.True(t, hasShortened, "data must have unknown_fields_shortened")
}

// TestU2_CLIMCPParityIdenticalBoundedShape verifies that the CLI JSON-RPC
// data.unknown_fields* scalars are identical to the MCP top-level
// unknown_fields* scalars for the same state dump (155.002-T / U2 parity).
// Both surfaces call BoundedFields() on the same error; this test confirms
// neither surface diverges in its projection.
func TestU2_CLIMCPParityIdenticalBoundedShape(t *testing.T) {
	root := setupCLIWorkspace(t)
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","novel_key":"v"}`

	// --- CLI path: package-level Execute() with --jsonrpc ---
	cliOutput, _ := u2ExecuteWithCapturedStdout(t,
		"--jsonrpc", "--cwd", root,
		"checkpoint", "create",
		"--state-dump", stateDump,
	)
	var cliResp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace([]byte(cliOutput)), &cliResp),
		"CLI raw stdout: %s", cliOutput)
	cliErrObj, ok := cliResp["error"].(map[string]any)
	require.True(t, ok, "CLI response must have error object")
	cliData, ok := cliErrObj["data"].(map[string]any)
	require.True(t, ok, "CLI error must have structured data field (structured path not yet wired?)")

	// --- MCP path: InvokeTool via backlogit_create_checkpoint ---
	srv := newMCPServerForRoot(t, root)
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_checkpoint"
	request.Params.Arguments = map[string]any{"state_dump": stateDump}
	result, invokeErr := srv.InvokeTool(context.Background(), "backlogit_create_checkpoint", request)
	require.NoError(t, invokeErr)
	require.NotNil(t, result)
	require.True(t, result.IsError, "MCP create_checkpoint must fail for dump with unknown field")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var mcpBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &mcpBody))

	// The CLI data.unknown_fields* must be byte-identical to MCP top-level unknown_fields*.
	assert.Equal(t, mcpBody["unknown_fields"], cliData["unknown_fields"],
		"CLI data.unknown_fields must be identical to MCP unknown_fields")
	assert.Equal(t, mcpBody["unknown_fields_truncated"], cliData["unknown_fields_truncated"],
		"CLI data.unknown_fields_truncated must be identical to MCP unknown_fields_truncated")
	assert.Equal(t, mcpBody["unknown_fields_omitted"], cliData["unknown_fields_omitted"],
		"CLI data.unknown_fields_omitted must be identical to MCP unknown_fields_omitted")
	assert.Equal(t, mcpBody["unknown_fields_shortened"], cliData["unknown_fields_shortened"],
		"CLI data.unknown_fields_shortened must be identical to MCP unknown_fields_shortened")
}

// u2ExecuteWithCapturedStdout invokes the package-level cli.Execute() with the
// supplied args, redirecting os.Stdout to a temp file so the JSON-RPC output
// written to origOut (which defaults to os.Stdout) can be captured. Both
// os.Args and os.Stdout are restored via t.Cleanup so they do not leak across
// test cases.
func u2ExecuteWithCapturedStdout(t *testing.T, args ...string) (string, error) {
	t.Helper()

	oldArgs := os.Args
	oldStdout := os.Stdout

	outputFile, createErr := os.CreateTemp(t.TempDir(), "backlogit-u2-stdout-*.txt")
	require.NoError(t, createErr)

	os.Args = append([]string{"backlogit"}, args...)
	os.Stdout = outputFile
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	})

	executeErr := cli.Execute()
	require.NoError(t, outputFile.Close())

	output, readErr := os.ReadFile(outputFile.Name())
	require.NoError(t, readErr)

	return string(output), executeErr
}
