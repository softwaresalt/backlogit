package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

func validCheckpointV1() *CheckpointV1 {
	now := time.Now().UTC()
	return &CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "test-session-001",
		Phase:         "build-execution",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		Context: CheckpointContext{
			ShipmentID: "044-S",
			FeatureID:  "045-F",
			TaskIDs:    []string{"045.003-T", "045.004-T"},
			Branch:     "feat/045-agent-session-disaster-recovery",
		},
		Progress: &CheckpointProgress{
			TasksCompleted: []string{"045.003-T"},
			TasksRemaining: []string{"045.004-T"},
			FilesModified:  []string{"internal/events/checkpoint_schema.go"},
			Decisions:      []string{"Used go-playground validator at package level"},
		},
		ResumeHint: "Continue with Unit 3 implementation",
	}
}

func TestParseCheckpoint_ValidJSON(t *testing.T) {
	cp := validCheckpointV1()
	data, err := json.Marshal(cp)
	require.NoError(t, err)

	parsed, err := ParseCheckpoint(data)
	require.NoError(t, err)
	assert.Equal(t, 1, parsed.SchemaVersion)
	assert.Equal(t, "ship", parsed.Agent)
	assert.Equal(t, "test-session-001", parsed.SessionID)
	assert.Equal(t, "build-execution", parsed.Phase)
	assert.Equal(t, "active", parsed.Status)
	assert.Equal(t, "044-S", parsed.Context.ShipmentID)
	assert.Equal(t, "045-F", parsed.Context.FeatureID)
	assert.Len(t, parsed.Context.TaskIDs, 2)
	assert.NotNil(t, parsed.Progress)
	assert.Equal(t, "Continue with Unit 3 implementation", parsed.ResumeHint)
}

func TestParseCheckpoint_InvalidJSON(t *testing.T) {
	_, err := ParseCheckpoint([]byte("not-json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointCorrupt)
}

func TestParseCheckpoint_EmptyJSON(t *testing.T) {
	cp, err := ParseCheckpoint([]byte("{}"))
	require.NoError(t, err)
	assert.Equal(t, 0, cp.SchemaVersion)
	assert.Empty(t, cp.Agent)
}

func TestValidateCheckpoint_ValidV1(t *testing.T) {
	cp := validCheckpointV1()
	err := ValidateCheckpoint(cp)
	assert.NoError(t, err)
}

func TestValidateCheckpoint_MissingAgent(t *testing.T) {
	cp := validCheckpointV1()
	cp.Agent = ""
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_InvalidAgent(t *testing.T) {
	cp := validCheckpointV1()
	cp.Agent = "invalid-agent"
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_MissingSessionID(t *testing.T) {
	cp := validCheckpointV1()
	cp.SessionID = ""
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_MissingPhase(t *testing.T) {
	cp := validCheckpointV1()
	cp.Phase = ""
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_InvalidStatus(t *testing.T) {
	cp := validCheckpointV1()
	cp.Status = "unknown"
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_WrongSchemaVersion(t *testing.T) {
	cp := validCheckpointV1()
	cp.SchemaVersion = 2
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestValidateCheckpoint_ResolvedStatus(t *testing.T) {
	cp := validCheckpointV1()
	cp.Status = "resolved"
	err := ValidateCheckpoint(cp)
	assert.NoError(t, err)
}

func TestValidateCheckpoint_StageAgent(t *testing.T) {
	cp := validCheckpointV1()
	cp.Agent = "stage"
	err := ValidateCheckpoint(cp)
	assert.NoError(t, err)
}

func TestCheckpointV1_JSONRoundTrip(t *testing.T) {
	original := validCheckpointV1()
	data, err := json.Marshal(original)
	require.NoError(t, err)

	parsed, err := ParseCheckpoint(data)
	require.NoError(t, err)

	assert.Equal(t, original.SchemaVersion, parsed.SchemaVersion)
	assert.Equal(t, original.Agent, parsed.Agent)
	assert.Equal(t, original.SessionID, parsed.SessionID)
	assert.Equal(t, original.Phase, parsed.Phase)
	assert.Equal(t, original.Status, parsed.Status)
	assert.Equal(t, original.Context.ShipmentID, parsed.Context.ShipmentID)
	assert.Equal(t, original.Context.FeatureID, parsed.Context.FeatureID)
	assert.Equal(t, original.Context.TaskIDs, parsed.Context.TaskIDs)
	assert.Equal(t, original.Context.Branch, parsed.Context.Branch)
	assert.Equal(t, original.Progress.TasksCompleted, parsed.Progress.TasksCompleted)
	assert.Equal(t, original.Progress.TasksRemaining, parsed.Progress.TasksRemaining)
	assert.Equal(t, original.Progress.FilesModified, parsed.Progress.FilesModified)
	assert.Equal(t, original.Progress.Decisions, parsed.Progress.Decisions)
	assert.Equal(t, original.ResumeHint, parsed.ResumeHint)
}

func TestCheckpointV1_NilProgress(t *testing.T) {
	cp := validCheckpointV1()
	cp.Progress = nil

	data, err := json.Marshal(cp)
	require.NoError(t, err)

	parsed, err := ParseCheckpoint(data)
	require.NoError(t, err)
	assert.Nil(t, parsed.Progress)

	err = ValidateCheckpoint(parsed)
	assert.NoError(t, err)
}

func TestCheckpointV1_EmptyContext(t *testing.T) {
	cp := validCheckpointV1()
	cp.Context = CheckpointContext{}

	err := ValidateCheckpoint(cp)
	assert.NoError(t, err)
}

func TestCheckpointV1_MissingCreatedAt(t *testing.T) {
	cp := validCheckpointV1()
	cp.CreatedAt = time.Time{}
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestCheckpointV1_MissingUpdatedAt(t *testing.T) {
	cp := validCheckpointV1()
	cp.UpdatedAt = time.Time{}
	err := ValidateCheckpoint(cp)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}
