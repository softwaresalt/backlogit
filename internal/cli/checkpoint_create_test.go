package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U6 (078.006-T): `backlogit checkpoint create --state-dump {{state_dump}}` must
// mirror the MCP backlogit_create_checkpoint tool — it validates a V1 checkpoint
// state dump, writes it to the workspace checkpoints directory via the shared
// events.CreateCheckpoint pipeline, and returns the written path as JSON. The
// resulting file must be readable by the existing `checkpoint get` and
// `checkpoint list` surfaces so recovery tooling stays same-shape across CLI/MCP.

// TestCheckpointCreate_WritesReadableCheckpoint verifies the happy path: a valid
// V1 state dump produces a checkpoint file that list and get can consume.
func TestCheckpointCreate_WritesReadableCheckpoint(t *testing.T) {
	root := setupCLIWorkspace(t)

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-078","phase":"build"}`

	createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)

	var created struct {
		Path string `json:"path"`
	}
	require.NoError(t, json.Unmarshal([]byte(createOut), &created))
	require.NotEmpty(t, created.Path, "create must return the written checkpoint path")

	filename := filepath.Base(created.Path)
	require.True(t, strings.HasPrefix(filename, "checkpoint-"), "unexpected checkpoint filename: %s", filename)
	require.True(t, strings.HasSuffix(filename, ".json"), "unexpected checkpoint filename: %s", filename)

	// The new checkpoint must appear in the list surface.
	listOut := runCLIStdout(t, root, "checkpoint", "list")
	var listed struct {
		Total       int `json:"total"`
		Checkpoints []struct {
			SessionID string `json:"session_id"`
			Agent     string `json:"agent"`
			Phase     string `json:"phase"`
			Status    string `json:"status"`
		} `json:"checkpoints"`
	}
	require.NoError(t, json.Unmarshal([]byte(listOut), &listed))
	require.Equal(t, 1, listed.Total, "created checkpoint must be listable")
	assert.Equal(t, "sess-078", listed.Checkpoints[0].SessionID)
	assert.Equal(t, "ship", listed.Checkpoints[0].Agent)
	assert.Equal(t, "active", listed.Checkpoints[0].Status, "status must default to active when omitted")

	// The new checkpoint must be retrievable and valid via get.
	getOut := runCLIStdout(t, root, "checkpoint", "get", filename)
	var got struct {
		Valid      bool `json:"valid"`
		Checkpoint struct {
			SessionID string `json:"session_id"`
		} `json:"checkpoint"`
	}
	require.NoError(t, json.Unmarshal([]byte(getOut), &got))
	assert.True(t, got.Valid, "created checkpoint must validate")
	assert.Equal(t, "sess-078", got.Checkpoint.SessionID)
}

// TestCheckpointCreate_MissingStateDump verifies the required-input contract.
func TestCheckpointCreate_MissingStateDump(t *testing.T) {
	root := setupCLIWorkspace(t)
	err := runCLIErr(t, root, "checkpoint", "create")
	require.Error(t, err, "create must fail when --state-dump is not provided")
}

// TestCheckpointCreate_InvalidSchema verifies that an invalid V1 checkpoint is
// rejected (mirroring the events.CreateCheckpoint validation gate).
func TestCheckpointCreate_InvalidSchema(t *testing.T) {
	root := setupCLIWorkspace(t)
	// schema_version=1 selects V1 validation; agent "bogus" violates oneof.
	stateDump := `{"schema_version":1,"agent":"bogus","session_id":"s","phase":"p"}`
	err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", stateDump)
	require.Error(t, err, "create must reject an invalid V1 checkpoint")
}

func TestCheckpointCreate_MissingWorkspaceStorageRootFailsClosed(t *testing.T) {
	root := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-missing","phase":"build"}`
	err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", stateDump)

	require.Error(t, err, "create must fail when no workspace storage root exists")
	assert.Contains(t, err.Error(), "resolve checkpoint dir")
}
