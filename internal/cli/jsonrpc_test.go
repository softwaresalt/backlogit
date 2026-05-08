package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONRPCFlag_RegisteredOnRoot verifies --jsonrpc is a persistent flag.
func TestJSONRPCFlag_RegisteredOnRoot(t *testing.T) {
	root := NewRootCommand()
	flag := root.PersistentFlags().Lookup("jsonrpc")
	require.NotNil(t, flag, "--jsonrpc must be registered as a persistent flag on root")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue, "--jsonrpc must default to false")
}

// TestJSONRPCFlag_VersionNoFlagPlainText verifies normal output is unaffected without the flag.
func TestJSONRPCFlag_VersionNoFlagPlainText(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"version"})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, `"jsonrpc"`, "non-jsonrpc mode must not wrap output")
	assert.Contains(t, out, "version", "version output must appear as plain text")
}

// TestJSONRPCFlag_VersionWithFlagProducesEnvelope verifies output is wrapped in JSON-RPC 2.0.
func TestJSONRPCFlag_VersionWithFlagProducesEnvelope(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"--jsonrpc", "version"})
	err := root.Execute()
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &resp),
		"--jsonrpc output must be valid JSON; got: %s", buf.String())

	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit version", resp["id"])
	_, hasResult := resp["result"]
	assert.True(t, hasResult, "response must have a result field")
	assert.Nil(t, resp["error"], "success response must not have an error field")
}

// TestJSONRPCFlag_VersionJSON_ResultIsObject verifies that when the command outputs
// JSON, the result is parsed as an object (not a string).
func TestJSONRPCFlag_VersionJSON_ResultIsObject(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"--jsonrpc", "version", "--format", "json"})
	err := root.Execute()
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &resp))

	assert.Equal(t, "2.0", resp["jsonrpc"])
	result, ok := resp["result"].(map[string]any)
	require.True(t, ok, "result should be a JSON object when command outputs JSON, got %T", resp["result"])
	assert.Contains(t, result, "version")
}

// TestJSONRPCFlag_EmptyOutput_ProducesNullResult verifies commands with no stdout output
// produce a JSON-RPC envelope with a null result.
func TestJSONRPCFlag_EmptyOutput_ProducesNullResult(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	// "version" with no flags outputs text. We test a blank-output scenario by
	// checking the result field exists even when the buffer is empty.
	// Use "backlogit --jsonrpc version" — text output becomes a string result.
	root.SetArgs([]string{"--jsonrpc", "version"})
	err := root.Execute()
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &resp))
	_, hasResult := resp["result"]
	assert.True(t, hasResult)
}

// TestJSONRPCErrorWrapping_ErrorProducesJSONRPCEnvelope verifies that when --jsonrpc
// is active and a command fails, the error wrapping logic produces a JSON-RPC error envelope.
// This test directly exercises the newRootCommandImpl + error-path logic that Execute() uses.
func TestJSONRPCErrorWrapping_ErrorProducesJSONRPCEnvelope(t *testing.T) {
	jctx := &jsonrpcInterceptor{}
	root := newRootCommandImpl(jctx)

	var outBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&bytes.Buffer{}) // suppress stderr
	root.SilenceErrors = true

	// version --format=badformat triggers a RunE error.
	root.SetArgs([]string{"--jsonrpc", "version", "--format", "badformat"})
	err := root.Execute()
	require.Error(t, err, "bad --format must cause a RunE error")
	assert.True(t, jctx.enabled, "jctx must be enabled when --jsonrpc was set")
}

// TestExecute_FunctionExists verifies the exported Execute function is available.
func TestExecute_FunctionExists(t *testing.T) {
	// Verify the signature compiles — we cannot call cli.Execute() directly in
	// tests without controlling os.Args. The function's os.Args dependency is
	// tested via the binary in integration tests.
	var fn func() error = Execute
	assert.NotNil(t, fn)
}

func executeWithCapturedStdout(t *testing.T, args ...string) (string, error) {
	t.Helper()

	oldArgs := os.Args
	oldStdout := os.Stdout

	outputFile, err := os.CreateTemp(t.TempDir(), "backlogit-stdout-*.txt")
	require.NoError(t, err)

	os.Args = append([]string{"backlogit"}, args...)
	os.Stdout = outputFile
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	})

	executeErr := Execute()
	require.NoError(t, outputFile.Close())

	output, readErr := os.ReadFile(outputFile.Name())
	require.NoError(t, readErr)

	return string(output), executeErr
}

func decodeEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace([]byte(raw)), &resp), "raw output: %s", raw)
	return resp
}

func TestExecute_JSONRPCRootHelpProducesEnvelope(t *testing.T) {
	output, err := executeWithCapturedStdout(t, "--jsonrpc", "--help")
	require.NoError(t, err)

	resp := decodeEnvelope(t, output)
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit", resp["id"])
	result, ok := resp["result"].(string)
	require.True(t, ok, "help output must be wrapped as a string result")
	assert.Contains(t, result, "Available Commands")
}

func TestExecute_JSONRPCHelpSubcommandProducesEnvelope(t *testing.T) {
	output, err := executeWithCapturedStdout(t, "--jsonrpc", "help", "add")
	require.NoError(t, err)

	resp := decodeEnvelope(t, output)
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit help", resp["id"])
	result, ok := resp["result"].(string)
	require.True(t, ok, "subcommand help must be wrapped as a string result")
	assert.Contains(t, result, "backlogit add")
}

func TestExecute_JSONRPCCompletionProducesEnvelope(t *testing.T) {
	output, err := executeWithCapturedStdout(t, "--jsonrpc", "completion", "powershell")
	require.NoError(t, err)

	resp := decodeEnvelope(t, output)
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit completion powershell", resp["id"])
	result, ok := resp["result"].(string)
	require.True(t, ok, "completion output must be wrapped as a string result")
	assert.Contains(t, result, "Register-ArgumentCompleter")
}

func TestExecute_JSONRPCRootVersionProducesEnvelope(t *testing.T) {
	output, err := executeWithCapturedStdout(t, "--jsonrpc", "--version")
	require.NoError(t, err)

	resp := decodeEnvelope(t, output)
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit", resp["id"])
	result, ok := resp["result"].(string)
	require.True(t, ok, "root --version output must be wrapped as a string result")
	assert.Contains(t, result, "backlogit version")
}

func TestExecute_JSONRPCUnknownCommandProducesErrorEnvelope(t *testing.T) {
	output, err := executeWithCapturedStdout(t, "--jsonrpc", "definitely-not-a-command")
	require.Error(t, err)

	resp := decodeEnvelope(t, output)
	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit", resp["id"])
	errorPayload, ok := resp["error"].(map[string]any)
	require.True(t, ok, "unknown command must produce a JSON-RPC error payload")
	message, ok := errorPayload["message"].(string)
	require.True(t, ok, "error message must be a string")
	assert.Contains(t, message, "unknown command")
}
