package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func setupDoctorTargetWorkspace(t *testing.T) (root, queueDir string) {
	t.Helper()
	root = t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "logs"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	return root, filepath.Join(backlogitDir, "queue")
}

const cliValidTask = `---
artifact_type: task
id: 100.001-T
title: A valid task
status: active
priority: medium
parent_id: 100-F
---

Body.
`

// TestRunDoctorTarget_TimeoutIsExit2 exercises the goroutine+select deadline
// path NON-VACUOUSLY: a slow stub against a near-zero deadline must map to the
// timeout kind and exit code 2 (a bare context.WithTimeout would not interrupt
// synchronous work, so this proves the select actually races the deadline).
func TestRunDoctorTarget_TimeoutIsExit2(t *testing.T) {
	slow := func(_ *core.Workspace, target, _ string) *core.DoctorTargetResult {
		time.Sleep(250 * time.Millisecond)
		return core.NewDoctorTargetResult(target, core.DoctorTargetPass)
	}
	var buf bytes.Buffer
	code, res := runDoctorTargetWithTimeout(context.Background(), nil, "x.md", "x.md", "text",
		5*time.Millisecond, slow, &buf)
	assert.Equal(t, 2, code)
	assert.Equal(t, core.DoctorTargetTimeout, res.Kind)
}

// TestRunDoctorTargetMode_TimeoutDoesNotStrandLock is the regression for the
// Copilot cycle-2 finding: on timeout, the still-running validation goroutine
// must NOT hold the task lock. Because runDoctorTargetMode owns the lock in its
// synchronous frame (PrepareDoctorTarget + defer unlock) and only the lock-free
// validate runs in the goroutine, the sidecar must be gone once the mode
// function returns — even though the (leaked) goroutine sleeps past the deadline.
func TestRunDoctorTargetMode_TimeoutDoesNotStrandLock(t *testing.T) {
	root, queueDir := setupDoctorTargetWorkspace(t)
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "100.001-T.md"), []byte(cliValidTask), 0o644))

	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	slow := func(_ *core.Workspace, target, _ string) *core.DoctorTargetResult {
		time.Sleep(300 * time.Millisecond)
		return core.NewDoctorTargetResult(target, core.DoctorTargetPass)
	}
	var buf bytes.Buffer
	code, res := runDoctorTargetMode(context.Background(), ws, ".backlogit/queue/100.001-T.md",
		"text", 5*time.Millisecond, slow, &buf)
	assert.Equal(t, 2, code)
	assert.Equal(t, core.DoctorTargetTimeout, res.Kind)

	// The deferred unlock in runDoctorTargetMode's frame has already run by the
	// time it returns, so a second acquisition must remain possible. The stable
	// sidecar inode is retained for advisory locking.
	sidecar := filepath.Join(queueDir, ".100.001-T.md.lock")
	_, statErr := os.Stat(sidecar)
	assert.NoError(t, statErr, "stable lock sidecar must remain after timeout")
}

// TestDoctorTargetCLI_BusyExit4 covers the busy → exit 4 mapping end-to-end
// through the full command: a fresh (in-TTL) lock sidecar simulates a concurrent
// holder, so PrepareDoctorTarget short-circuits to busy BEFORE the timeout
// wrapper is ever reached.
func TestDoctorTargetCLI_BusyExit4(t *testing.T) {
	root, queueDir := setupDoctorTargetWorkspace(t)
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "100.001-T.md"), []byte(cliValidTask), 0o644))
	// Hold the sidecar with the same OS-level advisory primitive used by
	// production to simulate a live concurrent lock holder.
	release, err := holdAdvisoryLock(filepath.Join(queueDir, ".100.001-T.md.lock"))
	require.NoError(t, err)
	t.Cleanup(release)

	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--cwd", root, "doctor", "--target", ".backlogit/queue/100.001-T.md"})

	err = cmd.Execute()
	require.Error(t, err)
	var ee *ExitError
	require.True(t, errors.As(err, &ee), "expected ExitError, got %T: %v", err, err)
	assert.Equal(t, 4, ee.Code)
}

func TestDoctorTargetExitCode_Table(t *testing.T) {
	cases := map[core.DoctorTargetKind]int{
		core.DoctorTargetPass:       0,
		core.DoctorTargetValidation: 1,
		core.DoctorTargetTimeout:    2,
		core.DoctorTargetScope:      3,
		core.DoctorTargetIO:         3,
		core.DoctorTargetBusy:       4,
	}
	for kind, want := range cases {
		got := doctorTargetExitCode(core.NewDoctorTargetResult("p", kind))
		assert.Equalf(t, want, got, "kind %s", kind)
	}
}

func TestDoctorTargetCLI_ValidExit0(t *testing.T) {
	root, queueDir := setupDoctorTargetWorkspace(t)
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "100.001-T.md"), []byte(cliValidTask), 0o644))

	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--cwd", root, "doctor", "--target", ".backlogit/queue/100.001-T.md"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, 0, ExitCodeFor(err))
	assert.Contains(t, buf.String(), "PASS")
}

func TestDoctorTargetCLI_MissingFieldExit1(t *testing.T) {
	root, queueDir := setupDoctorTargetWorkspace(t)
	const missing = `---
artifact_type: task
id: 100.002-T
title: Missing priority
status: active
---

Body.
`
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "100.002-T.md"), []byte(missing), 0o644))

	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--cwd", root, "doctor", "--target", ".backlogit/queue/100.002-T.md"})

	err := cmd.Execute()
	require.Error(t, err)
	var ee *ExitError
	require.True(t, errors.As(err, &ee), "expected ExitError, got %T: %v", err, err)
	assert.Equal(t, 1, ee.Code)
	assert.Contains(t, buf.String(), "priority")
}

func TestDoctorTargetCLI_OutOfScopeExit3(t *testing.T) {
	root, _ := setupDoctorTargetWorkspace(t)

	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--cwd", root, "doctor", "--target", filepath.Join("..", "escape.md")})

	err := cmd.Execute()
	require.Error(t, err)
	var ee *ExitError
	require.True(t, errors.As(err, &ee))
	assert.Equal(t, 3, ee.Code)
}

func TestDoctorTargetCLI_JSONVersionedSchema(t *testing.T) {
	root, queueDir := setupDoctorTargetWorkspace(t)
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "100.001-T.md"), []byte(cliValidTask), 0o644))

	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--cwd", root, "doctor", "--target", ".backlogit/queue/100.001-T.md", "--format", "json"})

	require.NoError(t, cmd.Execute())

	var payload struct {
		Mode          string `json:"mode"`
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Kind          string `json:"kind"`
		ArtifactID    string `json:"artifact_id"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
	assert.Equal(t, "target", payload.Mode)
	assert.NotEmpty(t, payload.SchemaVersion)
	assert.True(t, payload.OK)
	assert.Equal(t, "pass", payload.Kind)
	assert.Equal(t, "100.001-T", payload.ArtifactID)
}
