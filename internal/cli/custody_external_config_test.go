package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// extCLIRun runs one custody subcommand against root and returns its error.
func extCLIRun(t *testing.T, root string, args ...string) error {
	t.Helper()
	cwd := root
	cmd := NewCustodyCmd(&cwd)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// extCLIRefusal asserts err is a structured custody refusal with the given code and a message
// containing substr. The substring stops a generic refusal from passing for the external-path one.
func extCLIRefusal(t *testing.T, err error, code, substr string) {
	t.Helper()
	var refusal *custodyCLIError
	require.True(t, errors.As(err, &refusal), "expected a structured custody refusal, got %v", err)
	assert.Equal(t, code, refusal.Code)
	assert.True(t, strings.Contains(refusal.Message, substr), "message %q must contain %q", refusal.Message, substr)
}

func TestExternalCLI_OperatorPinsPath(t *testing.T) {
	repo := u7NewCustodyRepo(t)
	inside := filepath.Join(repo.root, ".backlogit", "operator-pins.yaml")
	require.NoError(t, os.WriteFile(inside, []byte("pins: []\n"), 0o600))
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "operator-pins.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("pins:\n  - fingerprint: fp-1\n    role: operator\n"), 0o600))

	t.Run("verify_flag_refuses_inside_workspace", func(t *testing.T) {
		err := extCLIRun(t, repo.root, "verify", repo.stageShip, "--operator-pins-file", inside)
		extCLIRefusal(t, err, "validation_failed", "inside workspace root")
	})
	t.Run("verify_env_refuses_inside_workspace", func(t *testing.T) {
		t.Setenv(core.CustodyOperatorPinsEnv, inside)
		err := extCLIRun(t, repo.root, "verify", repo.stageShip)
		extCLIRefusal(t, err, "validation_failed", "inside workspace root")
	})
	t.Run("verify_flag_overrides_env", func(t *testing.T) {
		t.Setenv(core.CustodyOperatorPinsEnv, inside)
		err := extCLIRun(t, repo.root, "verify", repo.stageShip, "--operator-pins-file", outside)
		require.NoError(t, err, "the flag must override the environment, so the outside pins file is used")
	})
}

func TestExternalCLI_NonceStorePath(t *testing.T) {
	repo := u7NewCustodyRepo(t)
	inside := filepath.Join(repo.root, ".backlogit", "nonces.jsonl")

	t.Run("accept_flag_refuses_inside_workspace", func(t *testing.T) {
		err := extCLIRun(t, repo.root, "accept", repo.stageShip,
			"--receipt-id", "rcpt-stage-ship-1", "--by", "ship", "--dry-run", "--nonce-store-path", inside)
		extCLIRefusal(t, err, "validation_failed", "inside workspace root")
	})
	t.Run("accept_env_refuses_inside_workspace", func(t *testing.T) {
		t.Setenv(core.CustodyNonceStoreEnv, inside)
		err := extCLIRun(t, repo.root, "accept", repo.stageShip,
			"--receipt-id", "rcpt-stage-ship-1", "--by", "ship", "--dry-run")
		extCLIRefusal(t, err, "validation_failed", "inside workspace root")
	})
}
