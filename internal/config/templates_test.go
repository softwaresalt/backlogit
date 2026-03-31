package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.03.01: Implement template schema and loader (revised: no flag field).

func TestLoadTemplates_ValidTemplate(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `---
name: task-template
type: task
sections:
  - name: description
    required: true
  - name: acceptance-criteria
    required: false
---

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
<!-- END:acceptance-criteria -->
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task.md"), []byte(content), 0o644))

	// Act
	templates, err := config.LoadTemplates(dir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "task", templates[0].ArtifactType)
	assert.Len(t, templates[0].Sections, 2)
}

func TestLoadTemplates_EmptyDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	templates, err := config.LoadTemplates(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLoadTemplates_DuplicateSectionNames(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `---
name: bad-template
type: task
sections:
  - name: description
    required: true
  - name: description
    required: false
---

<!-- BEGIN:description -->
<!-- END:description -->
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.md"), []byte(content), 0o644))

	// Act
	_, err := config.LoadTemplates(dir)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoadTemplates_MismatchedTags(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `---
name: bad-tags
type: task
sections:
  - name: description
    required: true
---

<!-- BEGIN:description -->
No END tag here
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mismatched.md"), []byte(content), 0o644))

	// Act
	_, err := config.LoadTemplates(dir)

	// Assert
	require.Error(t, err)
}

func TestGetTemplateForType_Found(t *testing.T) {
	// Arrange
	templates := []*config.TemplateConfig{
		{Name: "task-tmpl", ArtifactType: "task", Sections: []config.SectionDef{{Name: "desc", Required: true}}},
		{Name: "bug-tmpl", ArtifactType: "bug", Sections: []config.SectionDef{{Name: "steps", Required: true}}},
	}

	// Act
	result := config.GetTemplateForType(templates, "bug")

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "bug", result.ArtifactType)
}

func TestGetTemplateForType_NotFound(t *testing.T) {
	// Arrange
	templates := []*config.TemplateConfig{
		{Name: "task-tmpl", ArtifactType: "task", Sections: []config.SectionDef{{Name: "desc", Required: true}}},
	}

	// Act
	result := config.GetTemplateForType(templates, "epic")

	// Assert
	assert.Nil(t, result)
}
