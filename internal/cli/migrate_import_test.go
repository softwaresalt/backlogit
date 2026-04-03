package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/config"
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
	assert.Len(t, markdownFiles, 5, "expected config templates plus exactly one imported artifact markdown file")
}
