package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupCLIStashCorrectWorkspace creates a minimal workspace for stash correct tests.
func setupCLIStashCorrectWorkspace(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "archive"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	// Write stash archive with an entry that has a harvested_artifact_id.
	stashEntry := `{"id":"STASH001","priority":"medium","kind":"feature","text":"Test stash","archived_at":"2026-01-01T00:00:00Z","reason":"harvested","harvested_artifact_id":"001-F"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "archive", "stash.jsonl"), []byte(stashEntry), 0o644))

	// Write canonical delivery artifact with source_stash_id pointing at STASH001.
	artifactContent := "---\nid: 002-F\ntitle: Canonical delivery\nstatus: active\nartifact_type: feature\ncustom_fields:\n  source_stash_id: STASH001\n---\nBody\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "queue", "002-F.md"), []byte(artifactContent), 0o644))

	// Initialize DB and register the artifact so FindArtifactPath can locate it.
	database, err := db.Open(filepath.Join(backlogDir, "backlogit.db"))
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "002-F", Title: "Canonical delivery", Status: models.StatusActive,
		ArtifactType: "feature", CustomFields: map[string]any{"source_stash_id": "STASH001"},
	}))
	database.Close()

	return tmpDir
}

func TestStashCorrectCommand_HasRequiredFlags(t *testing.T) {
	root := cli.NewRootCommand()
	stashCmd, _, _ := root.Find([]string{"stash"})
	require.NotNil(t, stashCmd)
	correctCmd, _, _ := stashCmd.Find([]string{"correct"})
	require.NotNil(t, correctCmd, "stash correct subcommand must exist")
	assert.NotNil(t, correctCmd.Flag("stash-id"), "must have --stash-id flag")
	assert.NotNil(t, correctCmd.Flag("canonical-delivery"), "must have --canonical-delivery flag")
	assert.NotNil(t, correctCmd.Flag("reason"), "must have --reason flag")
	assert.NotNil(t, correctCmd.Flag("actor"), "must have --actor flag")
}

func TestStashCorrectCommand_MissingReason_Error(t *testing.T) {
	tmpDir := setupCLIStashCorrectWorkspace(t)
	root := cli.NewRootCommand()
	root.SetArgs([]string{"stash", "correct", "--cwd", tmpDir,
		"--stash-id", "STASH001",
		"--canonical-delivery", "002-F",
		"--actor", "test-agent",
	})
	var outBuf bytes.Buffer
	root.SetOut(&outBuf)
	err := root.Execute()
	assert.Error(t, err)
}

func TestStashCorrectCommand_ValidRequest_OutputsJSON(t *testing.T) {
	tmpDir := setupCLIStashCorrectWorkspace(t)
	root := cli.NewRootCommand()
	root.SetArgs([]string{"stash", "correct", "--cwd", tmpDir,
		"--stash-id", "STASH001",
		"--canonical-delivery", "002-F",
		"--reason", "test correction",
		"--actor", "test-agent",
	})
	var outBuf bytes.Buffer
	root.SetOut(&outBuf)
	err := root.Execute()
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &result))
	outcome, _ := result["outcome"].(string)
	assert.Equal(t, "corrected", outcome, "output must contain real correction outcome; got: %s", outBuf.String())
}
