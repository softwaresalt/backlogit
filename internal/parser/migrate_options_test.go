package parser_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/parser"
)

// TASK-010.01.02: Add migration CLI enhancements (dry-run, progress, validation)
// Tests for MigrateWithOptions and FormatReport.

func TestMigrateWithOptions_DryRun(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	content := "## Tasks\n- [ ] Task one\n- [x] Task two\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))

	opts := parser.MigrateOptions{
		DryRun: true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Greater(t, report.ItemsMigrated, 0, "dry run should count items that would be migrated")
	assert.Empty(t, report.Errors)
}

func TestMigrateWithOptions_Validate(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	content := "## Tasks\n- [ ] Valid task\n- [ ] \n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))

	opts := parser.MigrateOptions{
		Validate: true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, report)
}

func TestMigrateWithOptions_WithAdapter(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	content := "## In Progress\n- [ ] Task A\n- [x] Task B\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))

	opts := parser.MigrateOptions{
		Adapter: "backlog-md",
		DryRun:  true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 2, report.ItemsMigrated)
}

func TestMigrateWithOptions_AutoDetect(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	content := "## Tasks\n- [ ] Auto-detected task\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))

	opts := parser.MigrateOptions{
		DryRun: true,
	}

	// Act
	report, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, report.ItemsMigrated)
}

func TestMigrateWithOptions_ErrorRecovery(t *testing.T) {
	// Arrange — file does not exist
	opts := parser.MigrateOptions{DryRun: true}

	// Act
	_, err := parser.MigrateWithOptions(context.Background(), "/nonexistent/path.md", opts)

	// Assert
	require.Error(t, err)
}

func TestMigrateWithOptions_InvalidAdapter(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("- [ ] task\n"), 0o644))

	opts := parser.MigrateOptions{
		Adapter: "nonexistent-adapter",
		DryRun:  true,
	}

	// Act
	_, err := parser.MigrateWithOptions(context.Background(), mdFile, opts)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-adapter")
}

func TestFormatReport_Text(t *testing.T) {
	// Arrange
	report := &parser.MigrationReport{
		ItemsMigrated: 5,
		ItemsSkipped:  2,
		ItemsFailed:   1,
		Errors:        []string{"item 3 failed: invalid title"},
	}

	// Act
	output, err := parser.FormatReport(report, "text")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "5")
	assert.Contains(t, output, "2")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "invalid title")
}

func TestFormatReport_JSON(t *testing.T) {
	// Arrange
	report := &parser.MigrationReport{
		ItemsMigrated: 3,
		ItemsSkipped:  0,
		ItemsFailed:   0,
	}

	// Act
	output, err := parser.FormatReport(report, "json")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, `"ItemsMigrated"`)
	assert.Contains(t, output, "3")
}

func TestFormatReport_DefaultIsText(t *testing.T) {
	// Arrange
	report := &parser.MigrationReport{
		ItemsMigrated: 1,
	}

	// Act
	output, err := parser.FormatReport(report, "")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestMigrationReport_Fields(t *testing.T) {
	// Arrange & Act
	report := parser.MigrationReport{
		ItemsMigrated: 10,
		ItemsSkipped:  3,
		ItemsFailed:   2,
		Errors:        []string{"err1", "err2"},
		Items: []parser.MigrationItem{
			{Title: "Test", Status: "queued"},
		},
	}

	// Assert
	assert.Equal(t, 10, report.ItemsMigrated)
	assert.Equal(t, 3, report.ItemsSkipped)
	assert.Equal(t, 2, report.ItemsFailed)
	assert.Len(t, report.Errors, 2)
	assert.Len(t, report.Items, 1)
}
