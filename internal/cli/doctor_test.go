package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
)

// initTestWorkspace initializes a minimal .backlogit workspace for CLI tests.
func initTestWorkspace(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "archive"), 0o755))
	require.NoError(t, config.WriteDefaults(dir))
}

func TestDoctorCommand_CleanWorkspace(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "No issues found")
}

func TestDoctorCommand_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp, "--format", "json"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"findings"`)
	assert.Contains(t, buf.String(), `"checked_at"`)
}

func TestDoctorCommand_DetectsOrphan(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)

	// Write an orphaned child task directly (bypassing the tool surface).
	queueDir := filepath.Join(tmp, ".backlogit", "queue")
	content := `---
id: "999.001-T"
title: "Orphaned task"
status: active
artifact_type: task
level: 2
hierarchy_path: "999/999.001"
---
`
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "999.001-T.md"), []byte(content), 0o644))

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "999.001-T")
	assert.Contains(t, buf.String(), "orphaned")
}

func TestDoctorCommand_InvalidFormat(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp, "--format", "xml"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}
