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

// writeSelfRefArchiveRecord writes an archive record whose archived_from points at
// its own archive path (the legacy self-referential defect) and returns its path.
func writeSelfRefArchiveRecord(t *testing.T, tmp string) string {
	t.Helper()
	archiveDir := filepath.Join(tmp, ".backlogit", "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	p := filepath.Join(archiveDir, "200-T.md")
	content := "---\n" +
		"id: \"200-T\"\n" +
		"title: \"Self-ref archive record\"\n" +
		"status: done\n" +
		"artifact_type: task\n" +
		"archived_from: \".backlogit/archive/200-T.md\"\n" +
		"---\n" +
		"Body preserved\n"
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

// TestDoctorCommand_DetectsArchivedFromSelfRef verifies the read-only detection
// surface flags self-referential archived_from records by default WITHOUT mutating
// them (no --fix-archived-from supplied).
func TestDoctorCommand_DetectsArchivedFromSelfRef(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)
	recordPath := writeSelfRefArchiveRecord(t, tmp)
	before, err := os.ReadFile(recordPath)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "200-T", "self-ref record must be reported")
	assert.Contains(t, buf.String(), "archived_from", "finding must name the field")
	assert.NotContains(t, buf.String(), "fix:", "no repair without --fix-archived-from")

	after, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "detection must not mutate the record")
}

// TestDoctorCommand_FixArchivedFrom verifies the CLI-only --fix-archived-from flag
// rewrites the self-referential archived_from to its canonical queue restore path.
func TestDoctorCommand_FixArchivedFrom(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)
	recordPath := writeSelfRefArchiveRecord(t, tmp)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp, "--fix-archived-from"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "fix:", "a fix action must be reported")

	after, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	assert.Contains(t, string(after), ".backlogit/queue/200-T.md", "archived_from rewritten to queue path")
	assert.NotContains(t, string(after), "archived_from: \".backlogit/archive/200-T.md\"", "self-ref removed")
	assert.Contains(t, string(after), "Body preserved", "body must be preserved")
}

// TestDoctorCommand_FixArchivedFromFlagExists guards that the destructive repair is
// gated behind an explicit CLI flag (it must be registered on the doctor command).
func TestDoctorCommand_FixArchivedFromFlagExists(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--cwd", tmp, "--help"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "--fix-archived-from", "flag must be documented in help")
}
