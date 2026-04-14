package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// TASK-010.01.03: Create migration configuration for document class mapping

func TestLoadMigrationConfig_ValidFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `document_classes:
  - name: work_items
    glob_patterns:
      - "tasks/*.md"
      - "bugs/*.md"
    artifact_type: task
    keywords:
      - todo
      - bug
  - name: specs
    glob_patterns:
      - "requirements/*.md"
    artifact_type: story
    keywords:
      - requirement
      - acceptance criteria
default_layout: flat
source_paths:
  - path: tasks/
    class: work_items
  - path: requirements/
    class: specs
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "migration.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.LoadMigrationConfig(ws)

	// Assert
	require.NoError(t, err)
	assert.Len(t, cfg.DocumentClasses, 2)
	assert.Equal(t, "flat", cfg.DefaultLayout)
	assert.Len(t, cfg.SourcePaths, 2)
}

func TestLoadMigrationConfig_MissingFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	_, err := config.LoadMigrationConfig(ws)

	// Assert
	require.Error(t, err)
}

func TestLoadMigrationConfig_InvalidYAML(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ws, "migration.yaml"), []byte("{{invalid"), 0o644))

	// Act
	_, err := config.LoadMigrationConfig(ws)

	// Assert
	require.Error(t, err)
}

func TestLoadMigrationConfig_ValidationFails(t *testing.T) {
	// Arrange — missing required document_classes
	ws := t.TempDir()
	configContent := `default_layout: unknown_layout
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "migration.yaml"), []byte(configContent), 0o644))

	// Act
	_, err := config.LoadMigrationConfig(ws)

	// Assert
	require.Error(t, err)
}

func TestDefaultMigrationConfig_ReturnsValid(t *testing.T) {
	// Act
	cfg := config.DefaultMigrationConfig()

	// Assert
	require.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DocumentClasses)
	assert.NotEmpty(t, cfg.DefaultLayout)
	assert.NoError(t, cfg.Validate())
}

func TestDefaultMigrationConfig_CoversAllClasses(t *testing.T) {
	// Act
	cfg := config.DefaultMigrationConfig()

	// Assert
	classNames := make([]string, 0, len(cfg.DocumentClasses))
	for _, dc := range cfg.DocumentClasses {
		classNames = append(classNames, dc.Name)
	}
	assert.Contains(t, classNames, "work_items")
	assert.Contains(t, classNames, "milestones")
}

func TestWriteMigrationDefaults_CreatesFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	err := config.WriteMigrationDefaults(ws)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(ws, "migration.yaml"))
}

func TestWriteMigrationDefaults_DoesNotOverwrite(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	existingContent := "# custom config\n"
	require.NoError(t, os.WriteFile(filepath.Join(ws, "migration.yaml"), []byte(existingContent), 0o644))

	// Act
	err := config.WriteMigrationDefaults(ws)

	// Assert
	require.NoError(t, err)
	content, readErr := os.ReadFile(filepath.Join(ws, "migration.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, existingContent, string(content), "existing file should not be overwritten")
}

func TestWriteMigrationDefaults_WrittenFileIsLoadable(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	require.NoError(t, config.WriteMigrationDefaults(ws))

	// Act
	cfg, err := config.LoadMigrationConfig(ws)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NoError(t, cfg.Validate())
}

func TestMigrationConfig_ResolveArtifactType(t *testing.T) {
	tests := []struct {
		name      string
		className string
		wantType  string
		wantErr   bool
	}{
		{
			name:      "resolves work_items to task",
			className: "work_items",
			wantType:  "task",
		},
		{
			name:      "resolves specs to story",
			className: "specs",
			wantType:  "story",
		},
		{
			name:      "error on unknown class",
			className: "nonexistent",
			wantErr:   true,
		},
	}

	cfg := &config.MigrationConfig{
		DocumentClasses: []config.DocumentClassConfig{
			{Name: "work_items", GlobPatterns: []string{"tasks/*.md"}, ArtifactType: "task"},
			{Name: "specs", GlobPatterns: []string{"requirements/*.md"}, ArtifactType: "story"},
		},
		DefaultLayout: "flat",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			artType, err := cfg.ResolveArtifactType(tt.className)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantType, artType)
			}
		})
	}
}

func TestMigrationConfig_MatchClass(t *testing.T) {
	// Arrange
	cfg := &config.MigrationConfig{
		DocumentClasses: []config.DocumentClassConfig{
			{Name: "work_items", GlobPatterns: []string{"tasks/*.md"}, ArtifactType: "task"},
			{Name: "specs", GlobPatterns: []string{"requirements/*.md"}, ArtifactType: "story"},
		},
		DefaultLayout: "flat",
	}

	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "matches tasks directory",
			filePath: "tasks/fix-login.md",
			want:     "work_items",
		},
		{
			name:     "matches requirements directory",
			filePath: "requirements/auth.md",
			want:     "specs",
		},
		{
			name:     "no match returns empty",
			filePath: "random/file.md",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := cfg.MatchClass(tt.filePath)

			// Assert
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMigrationConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.MigrationConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.MigrationConfig{
				DocumentClasses: []config.DocumentClassConfig{
					{Name: "items", GlobPatterns: []string{"*.md"}, ArtifactType: "task"},
				},
				DefaultLayout: "flat",
			},
			wantErr: false,
		},
		{
			name: "missing document classes",
			cfg: config.MigrationConfig{
				DefaultLayout: "flat",
			},
			wantErr: true,
		},
		{
			name: "invalid layout",
			cfg: config.MigrationConfig{
				DocumentClasses: []config.DocumentClassConfig{
					{Name: "items", GlobPatterns: []string{"*.md"}, ArtifactType: "task"},
				},
				DefaultLayout: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.cfg.Validate()

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
