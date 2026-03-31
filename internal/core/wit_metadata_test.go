package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
)

func testHeaderDef() *config.HeaderDefConfig {
	return &config.HeaderDefConfig{
		Defaults: config.SystemDefaults{
			ID:          config.FieldDef{Type: "string", Immutable: true},
			CreatedDate: config.FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: config.FieldDef{Type: "datetime", Immutable: false},
		},
		Types: map[string]*config.TypeDefConfig{
			"task": {
				Prefix:   "T",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*config.FieldDef{
					"status":   {Type: "enum", Values: []string{"queued", "active", "done"}, Default: "queued"},
					"priority": {Type: "enum", Values: []string{"low", "medium", "high"}, Optional: true, Default: "medium"},
				},
			},
			"bug": {
				Prefix:   "B",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*config.FieldDef{
					"status":   {Type: "enum", Values: []string{"queued", "active", "done"}, Default: "queued"},
					"severity": {Type: "enum", Values: []string{"low", "medium", "high", "critical"}},
				},
			},
		},
	}
}

func testTemplates() []*config.TemplateConfig {
	return []*config.TemplateConfig{
		{
			Name:         "task-template",
			ArtifactType: "task",
			Sections:     []config.SectionDef{{Name: "description", Required: true}},
		},
		{
			Name:         "bug-template",
			ArtifactType: "bug",
			Sections:     []config.SectionDef{{Name: "description", Required: true}, {Name: "steps-to-reproduce"}},
		},
	}
}

func TestDescribeType_ReturnsTaskMetadata(t *testing.T) {
	// Arrange
	layout := defaultQueueLayout()

	// Act
	meta, err := core.DescribeType("task", testHeaderDef(), testTemplates(), layout)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "task", meta.TypeName)
	assert.NotEmpty(t, meta.Fields)
	assert.Contains(t, meta.Fields, "status")
}

func TestDescribeType_UnknownTypeErrors(t *testing.T) {
	// Arrange
	layout := defaultQueueLayout()

	// Act
	_, err := core.DescribeType("nonexistent", testHeaderDef(), testTemplates(), layout)

	// Assert
	require.Error(t, err)
}

func TestListTypes_ReturnsAllTypes(t *testing.T) {
	// Arrange
	layout := defaultQueueLayout()

	// Act
	types, err := core.ListTypes(testHeaderDef(), testTemplates(), layout)

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(types), 2, "should return at least task and bug types")
}
