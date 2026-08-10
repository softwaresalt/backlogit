package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 136-F/U10: `backlogit checkpoint abandon` and `backlogit checkpoint
// quarantine` are the CLI fallback surfaces for the MCP
// backlogit_abandon_checkpoint and backlogit_quarantine_checkpoint tools.
// Both require --reason; operator resolves via --operator, then
// BACKLOGIT_OPERATOR, then the OS user, and is never defaulted to a fixed
// string.

func writeCLICheckpoint(t *testing.T, root, filename, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestCheckpointAbandon_HappyPath(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-abandon-cli.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	out := runCLIStdout(t, root, "checkpoint", "abandon", "checkpoint-abandon-cli.json", "--reason", "superseded", "--operator", "tester@example.com")

	var result struct {
		Filename    string `json:"filename"`
		Disposition string `json:"disposition"`
		Reason      string `json:"reason"`
		Operator    string `json:"operator"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "checkpoint-abandon-cli.json", result.Filename)
	assert.Equal(t, "abandoned", result.Disposition)
	assert.Equal(t, "superseded", result.Reason)
	assert.Equal(t, "tester@example.com", result.Operator)
}

func TestCheckpointAbandon_MissingReason(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-abandon-cli.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	err := runCLIErr(t, root, "checkpoint", "abandon", "checkpoint-abandon-cli.json", "--operator", "tester@example.com")
	require.Error(t, err)
}

func TestCheckpointAbandon_MissingOperatorFallsBackToEnv(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-abandon-env.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	t.Setenv("BACKLOGIT_OPERATOR", "env-operator@example.com")
	out := runCLIStdout(t, root, "checkpoint", "abandon", "checkpoint-abandon-env.json", "--reason", "superseded")

	var result struct {
		Operator string `json:"operator"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "env-operator@example.com", result.Operator)
}

func TestCheckpointAbandon_MalformedRefusesNamingQuarantine(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-malformed-cli.json", "not-json{")

	err := runCLIErr(t, root, "checkpoint", "abandon", "checkpoint-malformed-cli.json", "--reason", "x", "--operator", "tester@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quarantine")
}

func TestCheckpointQuarantine_HappyPath(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-quarantine-cli.json", "not-json{")

	out := runCLIStdout(t, root, "checkpoint", "quarantine", "checkpoint-quarantine-cli.json", "--reason", "corrupt", "--operator", "tester@example.com")

	var result struct {
		Filename    string `json:"filename"`
		Disposition string `json:"disposition"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "checkpoint-quarantine-cli.json", result.Filename)
	assert.Equal(t, "quarantined", result.Disposition)

	destPath := filepath.Join(root, ".backlogit", "archive", "checkpoints", "checkpoint-quarantine-cli.json")
	assert.FileExists(t, destPath)
}

func TestCheckpointQuarantine_ValidRefusesNamingAbandon(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-valid-cli.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	err := runCLIErr(t, root, "checkpoint", "quarantine", "checkpoint-valid-cli.json", "--reason", "x", "--operator", "tester@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abandon")
}

func TestCheckpointQuarantine_MissingReason(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeCLICheckpoint(t, root, "checkpoint-quarantine-cli.json", "not-json{")

	err := runCLIErr(t, root, "checkpoint", "quarantine", "checkpoint-quarantine-cli.json", "--operator", "tester@example.com")
	require.Error(t, err)
}
