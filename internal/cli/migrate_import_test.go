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
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestMigrateCommand_ImportStructuredBacklogWorkspace_DryRun(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, "backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "tasks", "back-101 - Example-task.md"), []byte(`---
id: BACK-101
title: Example task
status: To Do
assignee:
  - '@alice'
labels: ["infra"]
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Imported body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Migrated")
	assert.Contains(t, buf.String(), "BACK-101")
}

func TestMigrateCommand_ImportDotBacklogWorkspace_DryRun(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "tasks", "task-001 - Example-task.md"), []byte(`---
id: TASK-001
title: Example task
status: Done
parent_task_id:
labels: ["infra"]
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Imported body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Migrated")
	assert.Contains(t, buf.String(), "TASK-001")
}

func TestMigrateCommand_ImportStructuredBacklogWorkspace_WritesArtifacts(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, "backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "tasks", "back-101 - Example-task.md"), []byte(`---
id: BACK-101
title: Example task
status: To Do
assignee:
  - '@alice'
labels: ["infra"]
dependencies: []
priority: medium
milestone: M1
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Imported body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Imported")

	found := false
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, filepath.WalkDir(storageRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" {
			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			if bytes.Contains(data, []byte("Example task")) {
				found = true
			}
		}
		return nil
	}))
	assert.True(t, found, "expected imported artifact markdown file inside the .backlogit workspace")
	_, err = os.Stat(filepath.Join(root, "tasks"))
	assert.True(t, os.IsNotExist(err), "expected no repo-root tasks directory after migration")
	_, err = os.Stat(filepath.Join(root, "queue"))
	assert.True(t, os.IsNotExist(err), "expected no repo-root queue directory after migration")
}

func TestMigrateCommand_ImportStructuredBacklogWorkspace_IsIdempotent(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, "backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "tasks", "back-101 - Example-task.md"), []byte(`---
id: BACK-101
title: Example task
status: To Do
labels: ["infra"]
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Imported body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	run := func() string {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md"})
		require.NoError(t, cmd.Execute())
		return buf.String()
	}

	first := run()
	second := run()

	assert.Contains(t, first, "Imported 1 artifacts, skipped 0, failed 0")
	assert.Contains(t, second, "Imported 0 artifacts, skipped 1, failed 0")

	var markdownFiles []string
	require.NoError(t, filepath.WalkDir(filepath.Join(root, ".backlogit"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" && !strings.HasSuffix(path, ".stash.md") {
			markdownFiles = append(markdownFiles, path)
		}
		return nil
	}))
	assert.Len(t, markdownFiles, 7, "expected config templates plus exactly one imported artifact markdown file")
}

// TestMigrateCommand_ImportArchivedItem_PreservesProvenance is a regression
// guard for the archive-provenance hardening: source items under archive/ map to
// status "archived", which CreateArtifact and the write boundary now reject. The
// migration path must therefore import them in a restorable status and archive
// them through ArchiveItem so the record lands in .backlogit/archive/ with intact
// archived_from/archived_status (invertible), rather than being reported failed.
func TestMigrateCommand_ImportArchivedItem_PreservesProvenance(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, "backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "archive"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "archive", "back-900 - Archived-task.md"), []byte(`---
id: BACK-900
title: Archived task
status: Done
labels: ["infra"]
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Archived body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Imported 1 artifacts, skipped 0, failed 0",
		"archived source item must import successfully, not fail")

	archiveDir := filepath.Join(root, ".backlogit", "archive")
	require.DirExists(t, archiveDir, "archived import must create the archive directory")

	var archivedFile string
	require.NoError(t, filepath.WalkDir(archiveDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		if bytes.Contains(data, []byte("Archived task")) {
			archivedFile = path
		}
		return nil
	}))
	require.NotEmpty(t, archivedFile, "archived source item must be imported into .backlogit/archive")

	data, err := os.ReadFile(archivedFile)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "status: archived", "imported item must be archived")
	assert.Contains(t, content, "archived_from:", "imported archived item must carry archived_from provenance")
	assert.Contains(t, content, "archived_status:", "imported archived item must carry archived_status provenance")
}

// TestMigrateCommand_ImportArchivedItem_ResumesAfterInterruptedArchive verifies
// that an archived-source import whose ArchiveItem step was interrupted (create
// succeeded, archive did not) is resumed on the next migration run instead of
// being permanently skipped by buildExistingImportIndex.
func TestMigrateCommand_ImportArchivedItem_ResumesAfterInterruptedArchive(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))

	sourceDir := filepath.Join(root, "backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "archive"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "archive", "back-901 - Archived-task.md"), []byte(`---
id: BACK-901
title: Archived task resume
status: Done
labels: ["infra"]
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Archived body
<!-- SECTION:DESCRIPTION:END -->
`), 0o644))

	runMigrate := func() string {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--cwd", root, "migrate", "--source", sourceDir, "--adapter", "backlog-md"})
		require.NoError(t, cmd.Execute())
		return buf.String()
	}

	// First run imports and archives the source item, stamping provenance.
	out := runMigrate()
	require.Contains(t, out, "Imported 1 artifacts, skipped 0, failed 0")

	archiveDir := filepath.Join(root, ".backlogit", "archive")
	mintedID := findArtifactIDContaining(t, archiveDir, "Archived task resume")
	require.NotEmpty(t, mintedID, "first import must archive the source item")

	// Simulate an interrupted prior run: unarchive restores the item to its
	// pre-archive status while retaining the backlog_md_source_path provenance
	// that buildExistingImportIndex keys on — the exact state a create-succeeded
	// / archive-failed run would leave behind.
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	require.NoError(t, core.UnarchiveItem(ctx, ws.DB, ws, mintedID))
	require.NoError(t, ws.Close())

	// Second run must resume ArchiveItem for the already-created source, not skip
	// it. Skipping would strand the item in a non-archived status forever.
	out = runMigrate()
	require.Contains(t, out, "Imported 1 artifacts, skipped 0, failed 0",
		"interrupted archived import must resume via ArchiveItem, not skip")

	archivedFile := findArtifactFileContaining(t, archiveDir, "Archived task resume")
	require.NotEmpty(t, archivedFile, "resumed item must be re-archived into .backlogit/archive")
	resumedData, readErr := os.ReadFile(archivedFile)
	require.NoError(t, readErr)
	content := string(resumedData)
	assert.Contains(t, content, "status: archived", "resumed item must be archived")
	assert.Contains(t, content, "archived_from:", "resumed item must carry archived_from provenance")
	assert.Contains(t, content, "archived_status:", "resumed item must carry archived_status provenance")
}

// findArtifactFileContaining returns the path of the first .md file under dir
// whose contents include marker.
func findArtifactFileContaining(t *testing.T, dir, marker string) string {
	t.Helper()
	var found string
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		fileData, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.Contains(fileData, []byte(marker)) {
			found = path
		}
		return nil
	}))
	return found
}

// findArtifactIDContaining returns the minted frontmatter id of the artifact
// file under dir whose contents include marker.
func findArtifactIDContaining(t *testing.T, dir, marker string) string {
	t.Helper()
	path := findArtifactFileContaining(t, dir, marker)
	if path == "" {
		return ""
	}
	fileData, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, line := range strings.Split(string(fileData), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
		}
	}
	return ""
}
