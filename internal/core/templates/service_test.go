package templates_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/core/templates"
	"github.com/softwaresalt/backlogit/internal/db"
)

// TASK-002.05.02 (revised): Application service boundary for template operations.

func setupTemplateWorkspace(t *testing.T) (string, *core.Workspace) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return root, ws
}

func writeTaskTemplate(t *testing.T, templatesDir string) {
	t.Helper()
	content := `---
name: task-template
type: task
sections:
  - name: description
    required: true
  - name: acceptance-criteria
    required: false
  - name: implementation-notes
    required: false
---
# {title}

<!-- BEGIN:description -->
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
<!-- END:acceptance-criteria -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
`
	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "task.md"), []byte(content), 0o644))
}

func writeBugTemplate(t *testing.T, templatesDir string) {
	t.Helper()
	content := `---
name: bug-template
type: bug
sections:
  - name: description
    required: true
  - name: steps-to-reproduce
    required: true
  - name: expected-behavior
    required: false
  - name: actual-behavior
    required: false
---
# {title}

<!-- BEGIN:description -->
<!-- END:description -->

## Steps to Reproduce

<!-- BEGIN:steps-to-reproduce -->
<!-- END:steps-to-reproduce -->

## Expected Behavior

<!-- BEGIN:expected-behavior -->
<!-- END:expected-behavior -->

## Actual Behavior

<!-- BEGIN:actual-behavior -->
<!-- END:actual-behavior -->
`
	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "bug.md"), []byte(content), 0o644))
}

// --- NewService ---

func TestNewService_LoadsTemplates(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTaskTemplate(t, dir)

	// Act
	svc, err := templates.NewService(context.Background(), dir)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, svc)
}

func TestNewService_EmptyDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	svc, err := templates.NewService(context.Background(), dir)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Empty(t, svc.ListTemplates())
}

func TestNewService_InvalidTemplates(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.md"), []byte("---\n:::invalid\n---\n"), 0o644))

	// Act
	_, err := templates.NewService(context.Background(), dir)

	// Assert
	require.Error(t, err)
}

// --- Resolve ---

func TestResolve_FoundType(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTaskTemplate(t, dir)
	svc, err := templates.NewService(context.Background(), dir)
	require.NoError(t, err)

	// Act
	tmpl, err := svc.Resolve("task")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "task", tmpl.ArtifactType)
}

func TestResolve_UnknownType(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTaskTemplate(t, dir)
	svc, err := templates.NewService(context.Background(), dir)
	require.NoError(t, err)

	// Act
	_, err = svc.Resolve("nonexistent")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- ListTemplates ---

func TestListTemplates_ReturnsAllRegistered(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTaskTemplate(t, dir)
	writeBugTemplate(t, dir)
	svc, err := templates.NewService(context.Background(), dir)
	require.NoError(t, err)

	// Act
	infos := svc.ListTemplates()

	// Assert
	assert.Len(t, infos, 2)
	types := make(map[string]bool)
	for _, info := range infos {
		types[info.TypeName] = true
	}
	assert.True(t, types["task"])
	assert.True(t, types["bug"])
}

func TestListTemplates_IncludesSectionMetadata(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeTaskTemplate(t, dir)
	svc, err := templates.NewService(context.Background(), dir)
	require.NoError(t, err)

	// Act
	infos := svc.ListTemplates()

	// Assert
	require.Len(t, infos, 1)
	assert.Len(t, infos[0].Sections, 3)

	sectionNames := make([]string, 0, len(infos[0].Sections))
	for _, s := range infos[0].Sections {
		sectionNames = append(sectionNames, s.Name)
	}
	assert.Contains(t, sectionNames, "description")
	assert.Contains(t, sectionNames, "acceptance-criteria")
	assert.Contains(t, sectionNames, "implementation-notes")
}

func TestListTemplates_EmptyWhenNoTemplates(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	svc, err := templates.NewService(context.Background(), dir)
	require.NoError(t, err)

	// Act
	infos := svc.ListTemplates()

	// Assert
	assert.NotNil(t, infos)
	assert.Empty(t, infos)
}

// --- Create with sections ---

func TestCreate_WithSections(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	sections := map[string]string{
		"description":         "This is the task body",
		"acceptance-criteria": "- [ ] Criteria met",
	}

	feat, err := core.CreateArtifact(context.Background(), ws, "Section feature", "feature")
	require.NoError(t, err)

	// Act
	artifact, err := svc.Create(context.Background(), ws, "Section test", "task", sections, core.WithParent(feat.ID))

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, artifact.ID)
	assert.Equal(t, "Section test", artifact.Title)
	assert.Contains(t, artifact.Description, "# Section test")
	assert.NotContains(t, artifact.Description, "{title}")
}

func TestCreate_RejectsInvalidSectionName(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	sections := map[string]string{
		"nonexistent-section": "content",
	}

	// Act
	_, err = svc.Create(context.Background(), ws, "Bad section", "task", sections)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-section")
}

func TestCreate_WithEmptySections(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	// Act — no sections provided
	feat, err := core.CreateArtifact(context.Background(), ws, "No-sections feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(context.Background(), ws, "No sections", "task", nil, core.WithParent(feat.ID))

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, artifact.ID)
}

// --- Update with sections ---

func TestUpdate_SectionContent(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(context.Background(), ws, "Section-content feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(context.Background(), ws, "Update sections", "task", map[string]string{
		"description": "Original content",
	}, core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(context.Background(), core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	// Act — update only the description section
	updated, err := svc.Update(context.Background(), ws, artifact.ID, map[string]string{
		"description": "Updated content",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, artifact.ID, updated.ID)
}

func TestUpdate_RejectsInvalidSection(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(context.Background(), ws, "Bad-update feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(context.Background(), ws, "Bad update", "task", nil, core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(context.Background(), core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	// Act
	_, err = svc.Update(context.Background(), ws, artifact.ID, map[string]string{
		"no-such-section": "content",
	})

	// Assert
	require.Error(t, err)
}

// --- GetSection ---

func TestGetSection_ExtractsContent(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	expectedContent := "This is the description"
	feat, err := core.CreateArtifact(context.Background(), ws, "GetSection feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(context.Background(), ws, "Get section", "task", map[string]string{
		"description": expectedContent,
	}, core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(context.Background(), core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	// Act
	content, err := svc.GetSection(context.Background(), ws, artifact.ID, "description")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, content, expectedContent)
}

func TestGetSection_NotFound(t *testing.T) {
	// Arrange
	root, ws := setupTemplateWorkspace(t)
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	writeTaskTemplate(t, templatesDir)

	svc, err := templates.NewService(context.Background(), templatesDir)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(context.Background(), ws, "Missing-section feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(context.Background(), ws, "Missing section", "task", nil, core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(context.Background(), core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	// Act
	_, err = svc.GetSection(context.Background(), ws, artifact.ID, "nonexistent")

	// Assert
	require.Error(t, err)
}
