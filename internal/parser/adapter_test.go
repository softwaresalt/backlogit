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

// TASK-010.04.01: Design pluggable migration adapter interface
// Tests for MigrationAdapter interface, adapter registry, and BacklogMdAdapter.

func TestRegisterAdapter_StoresAdapter(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	adapter := &parser.BacklogMdAdapter{}

	// Act
	err := parser.RegisterAdapter(adapter)

	// Assert
	require.NoError(t, err)
	names := parser.ListAdapters()
	assert.Contains(t, names, "backlog-md")
}

func TestRegisterAdapter_RejectsDuplicate(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	adapter := &parser.BacklogMdAdapter{}
	require.NoError(t, parser.RegisterAdapter(adapter))

	// Act
	err := parser.RegisterAdapter(adapter)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backlog-md")
}

func TestGetAdapter_ReturnsRegistered(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	// Act
	adapter, err := parser.GetAdapter("backlog-md")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "backlog-md", adapter.Name())
}

func TestGetAdapter_ErrorOnMissing(t *testing.T) {
	// Arrange
	parser.ResetRegistry()

	// Act
	_, err := parser.GetAdapter("nonexistent")

	// Assert
	require.Error(t, err)
}

func TestListAdapters_ReturnsSorted(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))

	// Act
	names := parser.ListAdapters()

	// Assert
	assert.Equal(t, []string{"backlog-md"}, names)
}

func TestDetectAdapter_FindsMatch(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("# Backlog\n- [ ] Task one\n- [x] Task two\n"), 0o644))

	// Act
	adapter, err := parser.DetectAdapter(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "backlog-md", adapter.Name())
}

func TestDetectAdapter_ErrorOnNoMatch(t *testing.T) {
	// Arrange
	parser.ResetRegistry()
	require.NoError(t, parser.RegisterAdapter(&parser.BacklogMdAdapter{}))
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "readme.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("# Just a readme\nNo checklists here.\n"), 0o644))

	// Act
	_, err := parser.DetectAdapter(mdFile)

	// Assert
	require.Error(t, err)
}

func TestBacklogMdAdapter_Name(t *testing.T) {
	// Arrange
	adapter := &parser.BacklogMdAdapter{}

	// Act
	name := adapter.Name()

	// Assert
	assert.Equal(t, "backlog-md", name)
}

func TestBacklogMdAdapter_DetectTrue(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("## Tasks\n- [ ] First task\n- [x] Done task\n"), 0o644))
	adapter := &parser.BacklogMdAdapter{}

	// Act
	result := adapter.Detect(mdFile)

	// Assert
	assert.True(t, result)
}

func TestBacklogMdAdapter_DetectFalse(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "notes.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("# Meeting Notes\nJust plain text.\n"), 0o644))
	adapter := &parser.BacklogMdAdapter{}

	// Act
	result := adapter.Detect(mdFile)

	// Assert
	assert.False(t, result)
}

func TestBacklogMdAdapter_Parse(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "backlog.md")
	content := "# Backlog\n## In Progress\n- [ ] Task one\n- [x] Task two\n## Done\n- [x] Task three\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))
	adapter := &parser.BacklogMdAdapter{}

	// Act
	items, err := adapter.Parse(context.Background(), mdFile)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "Task one", items[0].Title)
	assert.Equal(t, "active", items[0].Status)
}

func TestBacklogMdAdapter_ParseStructuredDotBacklogRoot(t *testing.T) {
	// Arrange
	root := t.TempDir()
	sourceDir := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.yml"), []byte("project_name: Test\ndefault_status: To Do\nstatuses: [\"To Do\", \"In Progress\", \"Done\"]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "tasks", "task-001 - Example-task.md"), []byte(`---
id: TASK-001
title: Example task
status: To Do
parent_task_id:
priority: medium
---

## Description

Imported body
`), 0o644))
	adapter := &parser.BacklogMdAdapter{}

	// Act
	items, err := adapter.Parse(context.Background(), sourceDir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "TASK-001", items[0].SourceID)
	assert.Equal(t, "Example task", items[0].Title)
	assert.Equal(t, filepath.Join(sourceDir, "tasks", "task-001 - Example-task.md"), items[0].SourcePath)
	assert.Equal(t, "queued", items[0].Status)
}

func TestMigrationItem_Fields(t *testing.T) {
	// Arrange & Act
	item := parser.MigrationItem{
		Title:      "Test task",
		Body:       "Description here",
		Status:     "queued",
		SourceType: parser.ClassWorkItem,
		ParentRef:  "Parent Section",
		Depth:      2,
	}

	// Assert
	assert.Equal(t, "Test task", item.Title)
	assert.Equal(t, parser.ClassWorkItem, item.SourceType)
	assert.Equal(t, 2, item.Depth)
}

// TASK-010.04.02: Implement file classification engine for markdown document types

func TestNewClassifier(t *testing.T) {
	// Act
	c := parser.NewClassifier()

	// Assert
	assert.NotNil(t, c)
}

func TestClassifier_ClassifyWorkItem(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "task.md")
	content := "---\nstatus: todo\npriority: high\ntype: task\n---\n# Fix login bug\n- [ ] Reproduce\n- [ ] Fix\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, parser.ClassWorkItem, result.Class)
	assert.Greater(t, result.Confidence, 0.5)
}

func TestClassifier_ClassifyDecision(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "decisions", "adr-001.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(mdFile), 0o755))
	content := "# ADR 001: Use SQLite for caching\n## Status\nAccepted\n## Context\nWe need a query engine.\n## Decision\nUse SQLite.\n## Consequences\nSimple deployment.\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, parser.ClassDecision, result.Class)
	assert.Greater(t, result.Confidence, 0.5)
}

func TestClassifier_ClassifySpec(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "specs", "auth-requirements.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(mdFile), 0o755))
	content := "# Authentication Requirements\n## User Story\nAs a user I want to log in.\n## Acceptance Criteria\n- Given valid credentials\n- When I submit\n- Then I am authenticated\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, parser.ClassSpec, result.Class)
	assert.Greater(t, result.Confidence, 0.5)
}

func TestClassifier_ClassifyPlan(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "plans", "migration-plan.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(mdFile), 0o755))
	content := "# Migration Plan\n## Implementation Units\n### Unit 1: Schema\n- Modify database\n### Unit 2: API\n- Update endpoints\n## Timeline\nSprint 3\n"
	require.NoError(t, os.WriteFile(mdFile, []byte(content), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, parser.ClassPlan, result.Class)
	assert.Greater(t, result.Confidence, 0.5)
}

func TestClassifier_ClassifyUnknown(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "random.md")
	require.NoError(t, os.WriteFile(mdFile, []byte("Hello world\n"), 0o644))
	c := parser.NewClassifier()

	// Act
	result, err := c.Classify(mdFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, parser.ClassUnknown, result.Class)
	assert.Less(t, result.Confidence, 0.5)
}

func TestClassifier_ClassifyDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task.md"), []byte("- [ ] Todo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not markdown"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Readme\n"), 0o644))
	c := parser.NewClassifier()

	// Act
	results, err := c.ClassifyDir(dir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2, "should classify only .md files")
}

func TestClassifier_ClassifyDir_SkipsNonMarkdown(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main"), 0o644))
	c := parser.NewClassifier()

	// Act
	results, err := c.ClassifyDir(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDocumentClassConstants(t *testing.T) {
	// Assert document class enum values are stable
	assert.Equal(t, parser.DocumentClass("spec"), parser.ClassSpec)
	assert.Equal(t, parser.DocumentClass("plan"), parser.ClassPlan)
	assert.Equal(t, parser.DocumentClass("work_item"), parser.ClassWorkItem)
	assert.Equal(t, parser.DocumentClass("decision"), parser.ClassDecision)
	assert.Equal(t, parser.DocumentClass("note"), parser.ClassNote)
	assert.Equal(t, parser.DocumentClass("unknown"), parser.ClassUnknown)
}
