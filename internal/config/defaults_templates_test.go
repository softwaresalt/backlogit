package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// TASK-002.03.02: Create default templates for the initial artifact types.

func TestWriteDefaults_CreatesTemplatesDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := config.WriteDefaults(dir)

	// Assert
	require.NoError(t, err)
	templatesDir := filepath.Join(dir, "templates")
	assert.DirExists(t, templatesDir)
}

func TestWriteDefaults_CreatesTemplates(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)

	// Assert — default types including shipment
	require.NoError(t, err)
	assert.Len(t, templates, 6)
}

func TestWriteDefaults_AllTemplateTypesPresent(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)
	require.NoError(t, err)

	// Act — collect types
	types := make(map[string]bool)
	for _, tmpl := range templates {
		types[tmpl.ArtifactType] = true
	}

	// Assert — default types including shipment
	expectedTypes := []string{"feature", "deliberation", "task", "review", "subtask", "shipment"}
	for _, typeName := range expectedTypes {
		assert.True(t, types[typeName], "missing template for type: %s", typeName)
	}
}

func TestWriteDefaults_TaskTemplateHasExpectedSections(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)
	require.NoError(t, err)

	// Act
	taskTmpl := config.GetTemplateForType(templates, "task")

	// Assert
	require.NotNil(t, taskTmpl)
	sectionNames := make([]string, 0, len(taskTmpl.Sections))
	for _, s := range taskTmpl.Sections {
		sectionNames = append(sectionNames, s.Name)
	}
	assert.Contains(t, sectionNames, "description")
	assert.Contains(t, sectionNames, "acceptance-criteria")
}

func TestWriteDefaults_FeatureTemplateHasGoals(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)
	require.NoError(t, err)

	// Act
	featureTmpl := config.GetTemplateForType(templates, "feature")

	// Assert
	require.NotNil(t, featureTmpl)
	sectionNames := make([]string, 0, len(featureTmpl.Sections))
	for _, s := range featureTmpl.Sections {
		sectionNames = append(sectionNames, s.Name)
	}
	assert.Contains(t, sectionNames, "description")
	assert.Contains(t, sectionNames, "goals")
}

func TestWriteDefaults_SubtaskTemplateHasImplementationNotes(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)
	require.NoError(t, err)

	// Act
	subtaskTmpl := config.GetTemplateForType(templates, "subtask")

	// Assert
	require.NotNil(t, subtaskTmpl)
	sectionNames := make([]string, 0, len(subtaskTmpl.Sections))
	for _, s := range subtaskTmpl.Sections {
		sectionNames = append(sectionNames, s.Name)
	}
	assert.Contains(t, sectionNames, "description")
	assert.Contains(t, sectionNames, "implementation-notes")
}
