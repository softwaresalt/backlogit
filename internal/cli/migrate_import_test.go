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
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.Contains(filepath.ToSlash(path), "/backlog/") {
			return nil
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
	assert.True(t, found, "expected imported artifact markdown file outside the source backlog workspace")
}
