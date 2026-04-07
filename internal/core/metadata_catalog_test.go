package core_test

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
)

func TestBuildMetadataCatalog_ReturnsUnifiedCatalog(t *testing.T) {
	root := t.TempDir()
	ws := &core.Workspace{
		RootPath:  root,
		Config:    config.DefaultConfig(),
		HeaderDef: testHeaderDef(),
		Templates: testTemplates(),
	}

	cliRoot := &cobra.Command{Use: "backlogit", Short: "root"}
	cliRoot.AddCommand(&cobra.Command{Use: "metadata", Short: "metadata"})

	catalog, err := core.BuildMetadataCatalog(
		ws,
		[]core.TemplateInfo{{TypeName: "task", DisplayName: "task-template"}},
		config.DefaultRegistry(),
		config.DefaultMigrationConfig(),
		cliRoot,
		[]core.ToolInfo{{Name: "backlogit_get_metadata_catalog", Description: "catalog"}},
	)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, ".backlogit"), catalog.Workspace.StorageRoot)
	assert.NotEmpty(t, catalog.ArtifactTypes)
	assert.NotEmpty(t, catalog.CLI)
	assert.NotEmpty(t, catalog.MCPTools)
	assert.Contains(t, catalog.Stash.SupportedKinds, "feature")
	assert.Contains(t, catalog.Stash.SupportedPriorities, "critical")
	assert.Equal(t, "medium", catalog.Stash.DefaultPriority)
	assert.True(t, catalog.Stash.SupportsDeliberation)
	assert.Equal(t, "deliberation", catalog.Stash.DeliberationType)
}

func TestWriteCommandMap_WritesInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	catalog := &core.MetadataCatalog{
		Workspace: core.MetadataWorkspaceInfo{
			StorageRoot: filepath.Join(root, ".backlogit"),
			QueuePath:   filepath.Join(root, ".backlogit", "queue"),
			ArchivePath: filepath.Join(root, ".backlogit", "archive"),
			LogsPath:    filepath.Join(root, ".backlogit", "logs"),
			StashPath:   filepath.Join(root, ".backlogit", "stash.jsonl"),
		},
	}

	writtenPath, err := core.WriteCommandMap(root, filepath.Join(".github", "instructions", "backlogit-command-map.md"), catalog, "markdown")
	require.NoError(t, err)
	assert.FileExists(t, writtenPath)
}

func TestWriteCommandMap_RejectsEscapingWorkspace(t *testing.T) {
	root := t.TempDir()
	_, err := core.WriteCommandMap(root, "..\\outside.md", &core.MetadataCatalog{}, "markdown")
	require.Error(t, err)
}
