package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.03.02: Create default templates for 3 initial artifact types (revision-3).

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

func TestWriteDefaults_Creates3Templates(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)

	// Assert — revision-3: 3 initial types (task, bug, epic)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
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

	// Assert — revision-3: initial 3 types only
	expectedTypes := []string{"task", "bug", "epic"}
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

func TestWriteDefaults_BugTemplateHasReproSteps(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	templatesDir := filepath.Join(dir, "templates")
	templates, err := config.LoadTemplates(templatesDir)
	require.NoError(t, err)

	// Act
	bugTmpl := config.GetTemplateForType(templates, "bug")

	// Assert
	require.NotNil(t, bugTmpl)
	sectionNames := make([]string, 0, len(bugTmpl.Sections))
	for _, s := range bugTmpl.Sections {
		sectionNames = append(sectionNames, s.Name)
	}
	assert.Contains(t, sectionNames, "steps-to-reproduce")
	assert.Contains(t, sectionNames, "expected-behavior")
	assert.Contains(t, sectionNames, "actual-behavior")
}
