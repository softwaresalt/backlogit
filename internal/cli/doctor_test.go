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

// writeOverArchivedFeatureFixture reproduces the 133.005-T (Unit 3)
// over-archived covering-feature signature at the file level: a feature
// closed (status done) that was never itself an explicit shipment manifest
// member, with a sibling task's returned_to_backlog event naming it as the
// source feature. Mirrors the core-level fixture in
// TestDoctor_OverArchivedFeatureAudit_FlagsWronglyClosedCoveringFeature.
func writeOverArchivedFeatureFixture(t *testing.T, tmp string) {
	t.Helper()
	archiveDir := filepath.Join(tmp, ".backlogit", "archive")
	logsDir := filepath.Join(tmp, ".backlogit", "logs")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "001-F.md"), []byte(`---
id: "001-F"
title: "Over-archived covering feature"
status: done
artifact_type: feature
level: 1
hierarchy_path: "001"
---
Covering feature.
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "001-S.md"), []byte(`---
id: "001-S"
title: "Partial-feature shipment"
status: shipped
artifact_type: shipment
level: 1
hierarchy_path: "001-S"
custom_fields:
    items:
        - 001.001-T
---
Shipment record.
`), 0o644))

	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z","delta":{"feature_id":"001-F"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "001.002-T.jsonl"), []byte(logContent), 0o644))
}

// TestDoctorCommand_CheckOverArchivedFeaturesFlag is the Constitution
// Reviewer follow-up for 133.005-T: internal/core/doctor_test.go exercised
// core.DoctorOptions.CheckOverArchivedFeatures directly, but no test
// confirmed the CLI actually wires --check-over-archived-features through to
// that option. Verifies both halves of the CLI boundary: (1) the check is
// opt-in and silent by default, matching --check-gate-evidence's
// default-false convention, and (2) the finding surfaces via the CLI only
// once the flag is explicitly supplied.
func TestDoctorCommand_CheckOverArchivedFeaturesFlag(t *testing.T) {
	tmp := t.TempDir()
	initTestWorkspace(t, tmp)
	writeOverArchivedFeatureFixture(t, tmp)

	bufDefault := new(bytes.Buffer)
	cmdDefault := cli.NewRootCommand()
	cmdDefault.SetOut(bufDefault)
	cmdDefault.SetErr(bufDefault)
	cmdDefault.SetArgs([]string{"doctor", "--cwd", tmp})
	require.NoError(t, cmdDefault.Execute())
	assert.NotContains(t, bufDefault.String(), "over_archived_covering_feature",
		"check must be opt-in and silent by default")

	bufFlag := new(bytes.Buffer)
	cmdFlag := cli.NewRootCommand()
	cmdFlag.SetOut(bufFlag)
	cmdFlag.SetErr(bufFlag)
	cmdFlag.SetArgs([]string{"doctor", "--cwd", tmp, "--check-over-archived-features"})
	require.NoError(t, cmdFlag.Execute())
	assert.Contains(t, bufFlag.String(), "over_archived_covering_feature", "finding type must be reported")
	assert.Contains(t, bufFlag.String(), "001-F", "over-archived covering feature must be identified via the CLI flag")
}

func TestDoctorCommand_CheckWorkspaceRootConflictFlag(t *testing.T) {
	tmp := t.TempDir()
	for _, candidate := range []string{".backlog", ".backlogit"} {
		dir := filepath.Join(tmp, candidate)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, config.WriteDefaults(dir))
	}

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"doctor",
		"--cwd", tmp,
		"--check-workspace-root-conflict",
		"--check-orphans=false",
		"--check-duplicates=false",
	})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "workspace_root_conflict")
	assert.Contains(t, buf.String(), ".backlog")
	assert.Contains(t, buf.String(), ".backlogit")
}
