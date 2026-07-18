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
// premise behind the size-extension spike decision to store feature/shipment
// `size` under custom_fields.size rather than a top-level docline map.
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
