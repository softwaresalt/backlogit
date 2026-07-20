package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 109.004-T / 096-S ground-truth guard: these tests codify the architectural
// premise behind the size-estimation decision to store a task `size` under
// custom_fields.size rather than a top-level docline map.
//
// The premise, proven here empirically, is that the GENERIC artifact codec
// (ParseFrontmatter -> ArtifactFromFrontmatter -> WriteArtifactFile) DROPS an
// unmodeled top-level `docline` key because models.Artifact carries no docline
// field, while custom_fields (a recognized carrier) and the mdfront-backed
// SetArtifactSize seam PRESERVE their contents. If a future change adds a
// docline carrier to models.Artifact (or otherwise alters codec behavior),
// these guards fail and force the spike decision to be revisited.

// doclineCarrierTaskFile handcrafts an artifact whose frontmatter carries BOTH a
// top-level docline map and custom_fields.size, so the preserve/drop assertions
// are non-vacuous.
const doclineCarrierTaskFile = "---\n" +
	"artifact_type: task\n" +
	"created_at: 2026-07-18T00:00:00.000Z\n" +
	"custom_fields:\n" +
	"    size: M\n" +
	"docline:\n" +
	"    backlogit:\n" +
	"        size: L\n" +
	"id: 910.001-T\n" +
	"parent_id: 910-F\n" +
	"priority: high\n" +
	"status: active\n" +
	"title: Docline carrier task\n" +
	"updated_at: 2026-07-18T00:00:00.000Z\n" +
	"---\n" +
	"\n" +
	"# Heading\n" +
	"\n" +
	"Body paragraph.\n"

// TestGenericArtifactCodec_DropsTopLevelDocline proves the generic artifact
// codec drops an unmodeled top-level `docline` map while custom_fields survives.
func TestGenericArtifactCodec_DropsTopLevelDocline(t *testing.T) {
	fmIn, body, err := models.ParseFrontmatter(doclineCarrierTaskFile)
	require.NoError(t, err)
	// Precondition: the raw parse retains the top-level docline map, so a later
	// absence is a genuine drop, not a parse-time omission.
	require.Contains(t, fmIn, "docline", "raw ParseFrontmatter must retain top-level docline")

	artifact, err := models.ArtifactFromFrontmatter(fmIn, body)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "910.001-T.md")
	require.NoError(t, core.WriteArtifactFile(artifact, path))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fmOut, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)

	// The struct round-trip has no docline carrier: the key is dropped on write.
	assert.NotContains(t, fmOut, "docline",
		"generic codec must drop unmodeled top-level docline (no models.Artifact carrier)")

	// custom_fields is a recognized carrier and survives the round-trip.
	cf, ok := fmOut["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must survive the generic codec round-trip")
	assert.Equal(t, "M", cf["size"], "custom_fields.size must survive the generic codec")
}

// TestSetArtifactSize_PreservesTopLevelDocline proves the mdfront-backed size
// seam preserves an unmodeled top-level `docline` map while still landing the
// mutation in custom_fields.size.
func TestSetArtifactSize_PreservesTopLevelDocline(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path := filepath.Join(backlogitDir, "queue", "910.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(doclineCarrierTaskFile), 0o644))

	_, err = core.SetArtifactSize(ctx, ws, "910.001-T", "S")
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md, err := mdfront.Decode(raw)
	require.NoError(t, err)

	// The top-level docline map is preserved untouched by the mdfront seam.
	docline, ok := md.Frontmatter["docline"].(map[string]any)
	require.True(t, ok, "mdfront seam must preserve the top-level docline map")
	backlogit, ok := docline["backlogit"].(map[string]any)
	require.True(t, ok, "docline.backlogit subtree must be preserved")
	assert.Equal(t, "L", backlogit["size"], "docline.backlogit.size must be untouched by the size seam")

	// The mutation lands in custom_fields.size, distinct from the docline value.
	cf, ok := md.Frontmatter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must be present after the size mutation")
	assert.Equal(t, "S", cf["size"], "SetArtifactSize must write custom_fields.size")
}

// TestUpdateArtifact_DropsTopLevelDocline_PreservesCustomFields proves the same
// drop/preserve behavior through the ORDINARY generic mutation path
// (core.UpdateArtifact -> findArtifact -> persistArtifact/WriteArtifactFile),
// not just the codec functions in isolation. An update that touches neither
// docline nor custom_fields still drops an unmodeled top-level docline map while
// preserving custom_fields.size. If UpdateArtifact/persist ever switched to a
// docline-preserving writer, this guard fails and forces the spike premise to be
// revisited — the codec-only test above would not catch that regression.
func TestUpdateArtifact_DropsTopLevelDocline_PreservesCustomFields(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path := filepath.Join(backlogitDir, "queue", "910.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(doclineCarrierTaskFile), 0o644))

	// An ordinary field update: no docline key, no custom_fields key.
	updated, err := core.UpdateArtifact(ctx, ws, "910.001-T", map[string]any{"title": "Updated title"})
	require.NoError(t, err)
	assert.Equal(t, "Updated title", updated.Title)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fmOut, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)

	// The ordinary mutation path drops the unmodeled top-level docline map.
	assert.NotContains(t, fmOut, "docline",
		"UpdateArtifact must drop unmodeled top-level docline (no models.Artifact carrier)")

	// custom_fields.size survives the ordinary mutation path untouched.
	cf, ok := fmOut["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must survive the ordinary UpdateArtifact path")
	assert.Equal(t, "M", cf["size"], "custom_fields.size must survive UpdateArtifact")
}

func TestSE2TaskSizingCustomFieldsRoundTrip(t *testing.T) {
	// Size estimation is task-only: the task artifact is the sole supported sizing
	// carrier. These cases verify the generic codec round-trips a task's sizing
	// custom_fields (size + provenance) and drops the unmodeled docline map.
	tests := []struct {
		name     string
		id       string
		parentID string
		status   string
	}{
		{name: "active task", id: "920.001-T", parentID: "920-F", status: "active"},
		{name: "queued task", id: "921.001-T", parentID: "921-F", status: "queued"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := sizingCarrierArtifactFile(tt.id, tt.parentID, tt.status)
			fmIn, body, err := models.ParseFrontmatter(input)
			require.NoError(t, err)
			require.Contains(t, fmIn, "docline")

			artifact, err := models.ArtifactFromFrontmatter(fmIn, body)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), tt.id+".md")
			require.NoError(t, core.WriteArtifactFile(artifact, path))

			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			fmOut, _, err := models.ParseFrontmatter(string(raw))
			require.NoError(t, err)

			assert.NotContains(t, fmOut, "docline")
			assertSizingCustomFields(t, fmOut)
		})
	}
}

func TestSE2UpdateArtifactPreservesTaskSizingCustomFields(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		parentID string
		status   string
		update   map[string]any
	}{
		{name: "active task", id: "922.001-T", parentID: "922-F", status: "active", update: map[string]any{"title": "Updated task"}},
		{name: "queued task", id: "923.001-T", parentID: "923-F", status: "queued", update: map[string]any{"title": "Updated task"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			backlogitDir := filepath.Join(root, ".backlogit")
			require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
			require.NoError(t, config.WriteDefaults(backlogitDir))

			ws, err := core.NewWorkspace(ctx, root)
			require.NoError(t, err)
			t.Cleanup(func() { _ = ws.Close() })

			path := filepath.Join(backlogitDir, "queue", tt.id+".md")
			require.NoError(t, os.WriteFile(path, []byte(sizingCarrierArtifactFile(tt.id, tt.parentID, tt.status)), 0o644))

			_, err = core.UpdateArtifact(ctx, ws, tt.id, tt.update)
			require.NoError(t, err)

			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			fmOut, _, err := models.ParseFrontmatter(string(raw))
			require.NoError(t, err)

			assert.NotContains(t, fmOut, "docline")
			assertSizingCustomFields(t, fmOut)
		})
	}
}

func sizingCarrierArtifactFile(id, parentID, status string) string {
	return "---\n" +
		"artifact_type: task\n" +
		"created_at: 2026-07-18T00:00:00.000Z\n" +
		"custom_fields:\n" +
		"    size: M\n" +
		"    size_source: agent\n" +
		"    size_ruleset_version: ruleset-alpha\n" +
		"docline:\n" +
		"    backlogit:\n" +
		"        size: L\n" +
		"id: " + id + "\n" +
		"parent_id: " + parentID + "\n" +
		"priority: medium\n" +
		"status: " + status + "\n" +
		"title: Sizing carrier task\n" +
		"updated_at: 2026-07-18T00:00:00.000Z\n" +
		"---\n\nBody paragraph.\n"
}

func assertSizingCustomFields(t *testing.T, fm map[string]any) {
	t.Helper()
	cf, ok := fm["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must survive")
	assert.Equal(t, "M", cf["size"])
	assert.Equal(t, "agent", cf["size_source"])
	assert.Equal(t, "ruleset-alpha", cf["size_ruleset_version"])
}
