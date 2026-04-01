package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/parser"
)

// TASK-010.01.04: Write Backlog.md migration integration tests
// End-to-end tests for the Backlog.md migration pipeline.

func setupMigrationWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	require.NoError(t, config.WriteMigrationDefaults(backlogitDir))
	return root
}

func TestMigration_EndToEnd_BasicBacklogMd(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	legacyContent := `# Project Backlog

## In Progress
- [ ] Implement authentication
- [ ] Build user dashboard

## Done
- [x] Set up CI pipeline
- [x] Create database schema

## Blocked
- [ ] Deploy to production
`
	legacyFile := filepath.Join(root, "backlog.md")
	require.NoError(t, os.WriteFile(legacyFile, []byte(legacyContent), 0o644))

	// Act
	items, err := parser.Migrate(context.Background(), legacyFile)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 5)

	statusCounts := map[string]int{}
	for _, item := range items {
		statusCounts[item.Status]++
	}
	assert.Equal(t, 2, statusCounts["active"])
	assert.Equal(t, 2, statusCounts["done"])
	assert.Equal(t, 1, statusCounts["blocked"])
}

func TestMigration_EndToEnd_DryRunNoWrites(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	legacyFile := filepath.Join(root, "backlog.md")
	require.NoError(t, os.WriteFile(legacyFile, []byte("## Tasks\n- [ ] Test task\n"), 0o644))

	opts := parser.MigrateOptions{
		DryRun: true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), legacyFile, opts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, report.ItemsMigrated)
	// Verify no artifact files were created in the workspace
	entries, readErr := os.ReadDir(filepath.Join(root, ".backlogit"))
	require.NoError(t, readErr)
	for _, entry := range entries {
		assert.NotEqual(t, "tasks", entry.Name(), "dry run should not create task directories")
	}
}

func TestMigration_EndToEnd_EnhancedParser(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	legacyContent := `# Backlog

## Sprint 1
### Epic: User Management
#### Story: Create User
- [ ] [P0] @alice Implement create user API
- [x] Write unit tests

### Epic: Reporting
- [ ] Build report dashboard
`
	legacyFile := filepath.Join(root, "backlog.md")
	require.NoError(t, os.WriteFile(legacyFile, []byte(legacyContent), 0o644))

	// Act
	items, err := parser.ParseLegacyEnhanced(legacyContent)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "Implement create user API", items[0].Title)
}

func TestMigration_EndToEnd_ErrorRecovery(t *testing.T) {
	// Arrange — source file does not exist
	_, err := parser.Migrate(context.Background(), "/nonexistent/backlog.md")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read legacy file")
}

func TestMigration_EndToEnd_EmptyFile(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	legacyFile := filepath.Join(root, "backlog.md")
	require.NoError(t, os.WriteFile(legacyFile, []byte(""), 0o644))

	// Act
	items, err := parser.Migrate(context.Background(), legacyFile)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestMigration_EndToEnd_MalformedMarkdown(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	legacyFile := filepath.Join(root, "backlog.md")
	content := "This is not a backlog file.\nJust random text.\nNo headings or checklists.\n"
	require.NoError(t, os.WriteFile(legacyFile, []byte(content), 0o644))

	// Act
	items, err := parser.Migrate(context.Background(), legacyFile)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestMigration_EndToEnd_HierarchyPreservation(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)
	content := `## Epic A
- [ ] Task under Epic A

## Epic B
- [ ] Task under Epic B
- [x] Done task under Epic B
`
	legacyFile := filepath.Join(root, "backlog.md")
	require.NoError(t, os.WriteFile(legacyFile, []byte(content), 0o644))

	// Act
	items, err := parser.Migrate(context.Background(), legacyFile)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "Epic A", items[0].ParentTitle)
	assert.Equal(t, "Epic B", items[1].ParentTitle)
	assert.Equal(t, "Epic B", items[2].ParentTitle)
}

func TestMigration_EndToEnd_WithMigrationConfig(t *testing.T) {
	// Arrange
	root := setupMigrationWorkspace(t)

	// Load migration config to verify it's accessible
	backlogitDir := filepath.Join(root, ".backlogit")
	cfg, err := config.LoadMigrationConfig(backlogitDir)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DocumentClasses)
}

// TASK-010.04.04: Write general migration integration tests
// End-to-end tests for the general purpose migration framework.

func TestGeneralMigration_AdapterRegistry(t *testing.T) {
	// Arrange
	parser.ResetRegistry()

	// Act
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))
	names := parser.ListAdapters()

	// Assert
	assert.Contains(t, names, "backlog-md")
}

func TestGeneralMigration_AdapterDetection(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("- [ ] Task\n"), 0o644))

	// Act
	adapter, err := parser.DetectAdapter(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "backlog-md", adapter.Name())
}

func TestGeneralMigration_ClassifierAccuracy(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Create sample documents of each class
	samples := map[string]struct {
		path    string
		content string
		want    parser.DocumentClass
	}{
		"work_item": {
			path:    filepath.Join(dir, "tasks", "fix-bug.md"),
			content: "---\nstatus: todo\npriority: high\n---\n# Fix Bug\n- [ ] Reproduce\n- [ ] Fix\n",
			want:    parser.ClassWorkItem,
		},
		"decision": {
			path:    filepath.Join(dir, "decisions", "adr-001.md"),
			content: "# ADR 001: Use Go\n## Status\nAccepted\n## Context\nNeed a language.\n## Decision\nUse Go.\n",
			want:    parser.ClassDecision,
		},
		"spec": {
			path:    filepath.Join(dir, "specs", "auth.md"),
			content: "# Auth Requirements\n## User Story\nAs a user...\n## Acceptance Criteria\n- Given...\n",
			want:    parser.ClassSpec,
		},
	}

	for _, s := range samples {
		require.NoError(t, os.MkdirAll(filepath.Dir(s.path), 0o755))
		require.NoError(t, os.WriteFile(s.path, []byte(s.content), 0o644))
	}

	c := parser.NewClassifier()

	// Act & Assert
	for name, s := range samples {
		t.Run(name, func(t *testing.T) {
			result, err := c.Classify(s.path)
			require.NoError(t, err)
			assert.Equal(t, s.want, result.Class)
			assert.Greater(t, result.Confidence, 0.5)
		})
	}
}

func TestGeneralMigration_ClassifierLowConfidence(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	ambiguousFile := filepath.Join(dir, "ambiguous.md")
	require.NoError(t, os.WriteFile(ambiguousFile, []byte("Hello world\n"), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(ambiguousFile)

	// Assert
	require.NoError(t, err)
	assert.Less(t, result.Confidence, 0.5, "ambiguous documents should have low confidence")
}

func TestGeneralMigration_EmptyDirectory(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	c := parser.NewClassifier()

	// Act
	results, err := c.ClassifyDir(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestGeneralMigration_NonMarkdownFiles(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte{0x89, 0x50, 0x4E, 0x47}, 0o644))
	c := parser.NewClassifier()

	// Act
	results, err := c.ClassifyDir(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, results, "should skip non-markdown files")
}

func TestGeneralMigration_DeepNestedStructure(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	deepPath := filepath.Join(dir, "a", "b", "c", "d")
	require.NoError(t, os.MkdirAll(deepPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deepPath, "deep.md"), []byte("- [ ] Deep task\n"), 0o644))
	c := parser.NewClassifier()

	// Act
	results, err := c.ClassifyDir(dir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestGeneralMigration_MigrateWithAdapterFlag(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("## Todo\n- [ ] Task A\n- [ ] Task B\n"), 0o644))

	opts := parser.MigrateOptions{
		Adapter: "backlog-md",
		DryRun:  true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, report.ItemsMigrated)
}
