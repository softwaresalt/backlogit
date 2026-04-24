package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCheckpointRecoveryFlow exercises the full checkpoint lifecycle through CLI.
func TestCheckpointRecoveryFlow(t *testing.T) {
	root := setupIntegrationWorkspace(t)

	// Step 1: Seed a V1 checkpoint file directly in the workspace for the subsequent CLI flows.
	checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

	now := time.Now().UTC()
	cp := &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "integration-test-session",
		Phase:         "build-execution",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		Context: events.CheckpointContext{
			ShipmentID: "044-S",
			FeatureID:  "045-F",
			TaskIDs:    []string{"045.003-T", "045.004-T"},
			Branch:     "feat/045-agent-session-disaster-recovery",
		},
		Progress: &events.CheckpointProgress{
			TasksCompleted: []string{"045.003-T"},
			TasksRemaining: []string{"045.004-T"},
			FilesModified:  []string{"internal/events/checkpoint_schema.go"},
			Decisions:      []string{"Used package-level validator cache"},
		},
		ResumeHint: "Continue with lifecycle functions",
	}
	cpData, err := json.Marshal(cp)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(checkpointDir, "checkpoint-20260423-100000.json"), cpData, 0o644))

	// Step 2: List via CLI — verify discovery.
	output, err := runCLI(t, root, "checkpoint", "list")
	require.NoError(t, err)
	assert.Contains(t, output, "integration-test-session")
	assert.Contains(t, output, "ship")

	// Step 3: List with agent filter.
	output, err = runCLI(t, root, "checkpoint", "list", "--agent", "ship")
	require.NoError(t, err)
	assert.Contains(t, output, "ship")

	// Step 4: Get via CLI — verify parse + validation.
	output, err = runCLI(t, root, "checkpoint", "get", "checkpoint-20260423-100000.json")
	require.NoError(t, err)
	assert.Contains(t, output, "integration-test-session")
	assert.Contains(t, output, "044-S")
	assert.Contains(t, output, "045-F")

	// Step 5: Resolve via CLI.
	output, err = runCLI(t, root, "checkpoint", "resolve", "checkpoint-20260423-100000.json")
	require.NoError(t, err)
	assert.Contains(t, output, "resolved")

	// Step 6: Verify resolved via get.
	output, err = runCLI(t, root, "checkpoint", "get", "checkpoint-20260423-100000.json")
	require.NoError(t, err)
	assert.Contains(t, output, "resolved")

	// Step 7: Cleanup via CLI.
	output, err = runCLI(t, root, "checkpoint", "cleanup", "--retention-days", "7")
	require.NoError(t, err)
	assert.Contains(t, output, "archived_count")

	// Verify archived.
	archiveDir := filepath.Join(root, ".backlogit", "archive", "checkpoints")
	assert.FileExists(t, filepath.Join(archiveDir, "checkpoint-20260423-100000.json"))
	assert.NoFileExists(t, filepath.Join(checkpointDir, "checkpoint-20260423-100000.json"))
}

// TestCheckpointRecoveryFlow_QuarantineCorrupt verifies corrupt checkpoints are quarantined.
func TestCheckpointRecoveryFlow_QuarantineCorrupt(t *testing.T) {
	root := setupIntegrationWorkspace(t)
	checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

	// Write corrupt file.
	require.NoError(t, os.WriteFile(
		filepath.Join(checkpointDir, "checkpoint-corrupt.json"),
		[]byte("{not valid json"),
		0o644,
	))

	// Write valid file.
	now := time.Now().UTC()
	valid := &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "stage",
		SessionID:     "valid-session",
		Phase:         "triage",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	validData, err := json.Marshal(valid)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(checkpointDir, "checkpoint-20260423-120000.json"),
		validData,
		0o644,
	))

	// List — both should appear; corrupt with validation error.
	output, err := runCLI(t, root, "checkpoint", "list")
	require.NoError(t, err)
	assert.Contains(t, output, "valid-session")
	assert.Contains(t, output, "quarantined")

	// Verify corrupt file was quarantined.
	quarantineDir := filepath.Join(root, ".backlogit", "quarantine", "checkpoints")
	assert.FileExists(t, filepath.Join(quarantineDir, "checkpoint-corrupt.json"))
}

// TestCheckpointRecoveryFlow_MultipleCheckpoints tests recovery selection from multiple checkpoints.
func TestCheckpointRecoveryFlow_MultipleCheckpoints(t *testing.T) {
	root := setupIntegrationWorkspace(t)
	checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

	now := time.Now().UTC()

	// Create older checkpoint.
	cp1 := &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "old-session",
		Phase:         "harness-complete",
		Status:        "active",
		CreatedAt:     now.Add(-2 * time.Hour),
		UpdatedAt:     now.Add(-2 * time.Hour),
		Context:       events.CheckpointContext{ShipmentID: "043-S"},
	}
	cp1Data, err := json.Marshal(cp1)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(checkpointDir, "checkpoint-20260423-080000.json"), cp1Data, 0o644))

	// Create recent checkpoint.
	cp2 := &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "recent-session",
		Phase:         "build-execution",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		Context:       events.CheckpointContext{ShipmentID: "044-S"},
		ResumeHint:    "Continue build-execution for 044-S",
	}
	cp2Data, err := json.Marshal(cp2)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(checkpointDir, "checkpoint-20260423-100000.json"), cp2Data, 0o644))

	// List — most recent should be first.
	output, err := runCLI(t, root, "checkpoint", "list", "--agent", "ship", "--status", "active")
	require.NoError(t, err)
	assert.Contains(t, output, "recent-session")
	assert.Contains(t, output, "old-session")

	// Parse output to verify order.
	var result struct {
		Total       int                        `json:"total"`
		Checkpoints []events.CheckpointSummary `json:"checkpoints"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	require.Equal(t, 2, result.Total)
	assert.Equal(t, "recent-session", result.Checkpoints[0].SessionID)
	assert.Equal(t, "old-session", result.Checkpoints[1].SessionID)

	// Resolve old session, keep recent.
	_, err = runCLI(t, root, "checkpoint", "resolve", "checkpoint-20260423-080000.json")
	require.NoError(t, err)

	// List active only — should show only recent.
	output, err = runCLI(t, root, "checkpoint", "list", "--status", "active")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "recent-session", result.Checkpoints[0].SessionID)
}
