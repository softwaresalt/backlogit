package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupCLIReconcileWorkspace creates a minimal backlogit workspace for reconcile
// CLI surface tests.  It uses config.WriteDefaults so that header-def.yaml and
// all other required config files carry valid default content.  It also seeds an
// archived task item with archived_status "active" so the reconcile command has a
// real item to process.
func setupCLIReconcileWorkspace(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "archive"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	archivedContent := "---\nid: 001-T\ntitle: Test task\nstatus: archived\nartifact_type: task\n" +
		"archived_from: .backlogit/queue/001-T.md\narchived_status: active\n---\nTest body\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogDir, "archive", "001-T.md"), []byte(archivedContent), 0o644,
	))
	return tmpDir
}

// TestReconcileCommand_HasRequiredFlags verifies that the reconcile subcommand is
// registered on the root command and exposes all four expected flags.
func TestReconcileCommand_HasRequiredFlags(t *testing.T) {
	cmd := cli.NewRootCommand()
	sub, _, err := cmd.Find([]string{"reconcile"})
	require.NoError(t, err)
	require.NotNil(t, sub, "reconcile subcommand must be registered with the root command")

	assert.NotNil(t, sub.Flags().Lookup("reason"), "reconcile must have --reason flag")
	assert.NotNil(t, sub.Flags().Lookup("actor"), "reconcile must have --actor flag")
	assert.NotNil(t, sub.Flags().Lookup("target-status"), "reconcile must have --target-status flag")
	assert.NotNil(t, sub.Flags().Lookup("idempotency-key"), "reconcile must have --idempotency-key flag")
}

// TestReconcileCommand_MissingReason_Error verifies that omitting the required
// --reason flag returns an error from cobra's required-flag validation.
func TestReconcileCommand_MissingReason_Error(t *testing.T) {
	tmpDir := t.TempDir()
	var outBuf bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"reconcile", "--cwd", tmpDir, "--actor", "test-actor", "001-T"})
	err := cmd.Execute()
	assert.Error(t, err, "missing --reason must return an error")
}

// TestReconcileCommand_MissingActor_Error verifies that omitting the required
// --actor flag returns an error from cobra's required-flag validation.
func TestReconcileCommand_MissingActor_Error(t *testing.T) {
	tmpDir := t.TempDir()
	var outBuf bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"reconcile", "--cwd", tmpDir, "--reason", "lifecycle fix", "001-T"})
	err := cmd.Execute()
	assert.Error(t, err, "missing --actor must return an error")
}

// TestReconcileCommand_MissingItemID_Error verifies that providing no positional
// item ID arguments returns an error from cobra's MinimumNArgs(1) validation.
func TestReconcileCommand_MissingItemID_Error(t *testing.T) {
	tmpDir := t.TempDir()
	var outBuf bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"reconcile", "--cwd", tmpDir, "--reason", "lifecycle fix", "--actor", "test-actor"})
	err := cmd.Execute()
	assert.Error(t, err, "missing item ID positional argument must return an error")
}

// TestReconcileCommand_ValidRequest_OutputsJSON verifies that a valid reconcile
// invocation against a properly prepared archived item produces JSON output
// containing a real reconciliation outcome ("completed" or "no_op").
//
// RED: this test fails against the stub because the stub discards the real
// core.ReconcileArchivedLifecycle result and prints {"outcome":"not_implemented"}
// instead of the actual JSON result.
func TestReconcileCommand_ValidRequest_OutputsJSON(t *testing.T) {
	tmpDir := setupCLIReconcileWorkspace(t)

	// Pre-populate the database so CheckChildrenTerminal and downstream DB
	// operations have a consistent record.  The connection is closed before the
	// CLI command opens its own connection to avoid SQLite write-lock contention.
	dbPath := filepath.Join(tmpDir, ".backlogit", "backlogit.db")
	database, dbErr := db.Open(dbPath)
	require.NoError(t, dbErr)
	require.NoError(t, db.EnsureSchema(database))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID:           "001-T",
		Title:        "Test task",
		Status:       models.StatusArchived,
		ArtifactType: "task",
	}))
	require.NoError(t, database.Close())

	var outBuf bytes.Buffer
	cmd := cli.NewRootCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{
		"reconcile",
		"--cwd", tmpDir,
		"--reason", "P-001 lifecycle fix",
		"--actor", "test-agent",
		"001-T",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	out := outBuf.String()
	assert.True(t,
		strings.Contains(out, "completed") || strings.Contains(out, "no_op"),
		"output must contain a real reconciliation outcome (completed or no_op); got: %s", out,
	)
}
