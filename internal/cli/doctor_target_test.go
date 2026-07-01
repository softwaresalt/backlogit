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
	slow := func(_ *core.Workspace, target string) (*core.DoctorTargetResult, error) {
		time.Sleep(250 * time.Millisecond)
		return core.NewDoctorTargetResult(target, core.DoctorTargetPass), nil
	}
	var buf bytes.Buffer
	code, res := runDoctorTargetWithTimeout(context.Background(), nil, "x.md", "text",
		5*time.Millisecond, slow, &buf)
	assert.Equal(t, 2, code)
	assert.Equal(t, core.DoctorTargetTimeout, res.Kind)
}

// TestRunDoctorTarget_BusyIsExit4 covers the busy → exit 4 mapping through the
// runner (a held task lock elsewhere yields the busy kind).
func TestRunDoctorTarget_BusyIsExit4(t *testing.T) {
	busy := func(_ *core.Workspace, target string) (*core.DoctorTargetResult, error) {
		return core.NewDoctorTargetResult(target, core.DoctorTargetBusy), nil
	}
	var buf bytes.Buffer
	code, res := runDoctorTargetWithTimeout(context.Background(), nil, "x.md", "text",
		doctorTargetTimeout, busy, &buf)
	assert.Equal(t, 4, code)
	assert.Equal(t, core.DoctorTargetBusy, res.Kind)
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
