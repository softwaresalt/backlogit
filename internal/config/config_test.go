package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  task:
    prefix: T
    name_format: "{prefix}{NNN}-{title_slug}"
    allowed_children: []
fields:
  status:
    type: enum
    values: [todo, in_progress, done]
    default: todo
max_slug_length: 60
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Contains(t, cfg.ArtifactTypes, "task")
}

func TestLoad_MissingFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	_, err := config.Load(context.Background(), ws)

	// Assert
	assert.Error(t, err)
}

func TestDefaultConfig_ReturnsValid(t *testing.T) {
	// Act
	cfg := config.DefaultConfig()

	// Assert
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ArtifactTypes)
	assert.Contains(t, cfg.ArtifactTypes, "deliberation")
}

func TestLoad_ConfigWithSuffixAndBugLevel(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  bug:
    prefix: B
    suffix: "-B"
    name_format: "{NNN}{suffix}"
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 3
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task]
    - level: 3
      types: [subtask]
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	require.Contains(t, cfg.ArtifactTypes, "feature")
	assert.Equal(t, "-F", cfg.ArtifactTypes["feature"].Suffix)
	assert.Equal(t, 3, cfg.BugLevel)
	assert.Contains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	level, levelErr := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, levelErr)
	assert.Equal(t, 3, level)
}

func TestLoad_InvalidBugLevelRejected(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 4
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	_, err := config.Load(context.Background(), ws)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bug_level")
}

func TestDefaultConfig_UsesNumericSuffixFormats(t *testing.T) {
	// Act
	cfg := config.DefaultConfig()

	// Assert
	require.Contains(t, cfg.ArtifactTypes, "feature")
	require.Contains(t, cfg.ArtifactTypes, "task")
	assert.Equal(t, "-F", cfg.ArtifactTypes["feature"].Suffix)
	assert.Equal(t, "{NNN}{suffix}", cfg.ArtifactTypes["feature"].NameFormat)
	assert.Equal(t, "-T", cfg.ArtifactTypes["task"].Suffix)
	assert.Equal(t, "{NNN}{suffix}", cfg.ArtifactTypes["task"].NameFormat)
	assert.Equal(t, 3, cfg.BugLevel)
	assert.Contains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	level, err := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, err)
	assert.Equal(t, 3, level)
}

func TestLoad_BugLevelTwoMovesBugParentingAndHierarchy(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  subtask:
    prefix: ST
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
  bug:
    prefix: B
    suffix: "-B"
    name_format: "{NNN}{suffix}"
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 2
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task]
    - level: 3
      types: [subtask]
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	level, levelErr := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, levelErr)
	assert.Equal(t, 2, level)
}

func TestDefaultRegistry_ReturnsValid(t *testing.T) {
	// Act
	reg := config.DefaultRegistry()

	// Assert
	assert.NotNil(t, reg)
	assert.NotEmpty(t, reg.Directories)
}

func TestWriteDefaults_CreatesFiles(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	err := config.WriteDefaults(ws)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(ws, "config.yaml"))
	assert.FileExists(t, filepath.Join(ws, "registry.yaml"))
	assert.FileExists(t, filepath.Join(ws, "stash.jsonl"))
	assert.FileExists(t, filepath.Join(ws, "templates", "deliberation.md"))
}

func TestWorkspaceConfig_ToolsDirValidation(t *testing.T) {
	t.Run("direct-validate", func(t *testing.T) {
		absDir := filepath.Join(t.TempDir(), "tools")
		cases := []struct {
			name     string
			toolsDir string
			wantErr  bool
		}{
			{name: "valid-dot-tools-bin", toolsDir: ".tools/bin"},
			{name: "valid-dot-bin", toolsDir: ".bin"},
			{name: "valid-nested", toolsDir: "custom/tools"},
			{name: "whitespace-only", toolsDir: "   ", wantErr: true},
			{name: "leading-whitespace", toolsDir: " .tools", wantErr: true},
			{name: "trailing-whitespace", toolsDir: ".tools ", wantErr: true},
			{name: "nul-byte", toolsDir: "tools\x00bin", wantErr: true},
			{name: "traversal", toolsDir: "../outside", wantErr: true},
			{name: "dot", toolsDir: ".", wantErr: true},
			{name: "absolute", toolsDir: absDir, wantErr: true},
			{name: "volume-name", toolsDir: "C:tools", wantErr: true},
			{name: "volume-name-lower", toolsDir: "c:tools", wantErr: true},
			{name: "volume-rooted-backslash", toolsDir: `C:\tools`, wantErr: true},
			{name: "volume-rooted-slash", toolsDir: "C:/tools", wantErr: true},
			{name: "unc-backslash", toolsDir: `\\server\share`, wantErr: true},
			{name: "unc-slash", toolsDir: "//server/share", wantErr: true},
			{name: "dot-git", toolsDir: ".git", wantErr: true},
			{name: "dot-git-upper", toolsDir: ".GIT/hooks", wantErr: true},
			{name: "dot-backlog", toolsDir: ".backlog", wantErr: true},
			{name: "dot-backlog-mixed", toolsDir: ".Backlog/sub", wantErr: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := config.DefaultConfig()
				cfg.ToolsDir = tc.toolsDir
				err := cfg.Validate()
				if tc.wantErr {
					require.ErrorIs(t, err, corerrors.ErrValidation)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("config-load", func(t *testing.T) {
		baseConfigYAML := `artifact_types:
  task:
    prefix: T
    name_format: "{prefix}{NNN}-{title_slug}"
    allowed_children: []
fields:
  status:
    type: enum
    values: [todo, in_progress, done]
    default: todo
max_slug_length: 60
`
		t.Run("accepts-valid-tools-dir", func(t *testing.T) {
			ws := filepath.Join(t.TempDir(), ".backlogit")
			require.NoError(t, os.MkdirAll(ws, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(baseConfigYAML+"tools_dir: \".tools/bin\"\n"), 0o644))

			cfg, err := config.Load(context.Background(), ws)
			require.NoError(t, err)
			assert.Equal(t, ".tools/bin", cfg.ToolsDir)
		})

		invalidYAML := []struct {
			name       string
			storageDir string
			extraYAML  string
		}{
			{name: "explicit-blank", storageDir: ".backlogit", extraYAML: "tools_dir: \"\"\n"},
			{name: "whitespace-only", storageDir: ".backlogit", extraYAML: "tools_dir: \"   \"\n"},
			{name: "explicit-null", storageDir: ".backlogit", extraYAML: "tools_dir: null\n"},
			{name: "empty-scalar", storageDir: ".backlogit", extraYAML: "tools_dir:\n"},
			{name: "non-string-int", storageDir: ".backlogit", extraYAML: "tools_dir: 123\n"},
			{name: "non-string-seq", storageDir: ".backlogit", extraYAML: "tools_dir: [bin]\n"},
			{name: "storage-dir-dot-backlogit", storageDir: ".backlogit", extraYAML: "tools_dir: \".backlogit\"\n"},
			{name: "storage-dir-dot-backlog", storageDir: ".backlog", extraYAML: "tools_dir: \".backlog\"\n"},
		}
		for _, tc := range invalidYAML {
			t.Run(tc.name, func(t *testing.T) {
				ws := filepath.Join(t.TempDir(), tc.storageDir)
				require.NoError(t, os.MkdirAll(ws, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(baseConfigYAML+tc.extraYAML), 0o644))

				_, err := config.Load(context.Background(), ws)
				require.ErrorIs(t, err, corerrors.ErrValidation)
			})
		}
	})
}
