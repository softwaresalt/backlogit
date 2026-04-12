package core_test

import (
	"path/filepath"
	"strings"
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

func TestWriteCommandMap_WritesInsideBacklogit(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	catalog := &core.MetadataCatalog{
		Workspace: core.MetadataWorkspaceInfo{
			StorageRoot: backlogitDir,
			QueuePath:   filepath.Join(backlogitDir, "queue"),
			ArchivePath: filepath.Join(backlogitDir, "archive"),
			LogsPath:    filepath.Join(backlogitDir, "logs"),
			StashPath:   filepath.Join(backlogitDir, "stash.jsonl"),
		},
	}

	writtenPath, err := core.WriteCommandMap(backlogitDir, "command-map.md", catalog, "markdown")
	require.NoError(t, err)
	assert.FileExists(t, writtenPath)

	// Verify the file was written inside .backlogit/ using filepath.Rel
	// to avoid false positives from prefix matching (e.g. ".backlogit-evil/").
	absWritten, _ := filepath.Abs(writtenPath)
	absBacklogit, _ := filepath.Abs(backlogitDir)
	relPath, relErr := filepath.Rel(absBacklogit, absWritten)
	assert.NoError(t, relErr, "should be able to compute relative path")
	assert.False(t, strings.HasPrefix(relPath, ".."),
		"written path %s should be inside %s (relative: %s)", absWritten, absBacklogit, relPath)
}

func TestWriteCommandMap_RejectsEscapingBacklogit(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")

	// A path that stays within the repo root but escapes .backlogit/
	_, err := core.WriteCommandMap(backlogitDir, filepath.Join("..", ".github", "instructions", "map.md"), &core.MetadataCatalog{}, "markdown")
	require.Error(t, err, "should reject paths escaping .backlogit/ even if within repo root")
}

func TestWriteCommandMap_RejectsEscapingWorkspace(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	_, err := core.WriteCommandMap(backlogitDir, "..\\..\\outside.md", &core.MetadataCatalog{}, "markdown")
	require.Error(t, err)
}
