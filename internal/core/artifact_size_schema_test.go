package core

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/models"
)

// U6 (071.003-T): `size` is a logical task field physically stored under
// custom_fields.size. These tests lock the model round-trip and backward-compat.

func TestArtifactSize_CustomFieldRoundTrips(t *testing.T) {
	fm := map[string]any{
		"id":            "500.001-T",
		"title":         "Sized task",
		"status":        "active",
		"artifact_type": "task",
		"priority":      "medium",
		"custom_fields": map[string]any{"size": "M"},
	}
	a, err := models.ArtifactFromFrontmatter(fm, "Body content.\n")
	require.NoError(t, err)
	require.NotNil(t, a.CustomFields)
	assert.Equal(t, "M", a.CustomFields["size"])

	path := filepath.Join(t.TempDir(), "500.001-T.md")
	require.NoError(t, WriteArtifactFile(a, path))

	got, _, err := parseFile(path)
	require.NoError(t, err)
	require.NotNil(t, got.CustomFields)
	assert.Equal(t, "M", got.CustomFields["size"], "size must survive write -> re-parse")
}

func TestArtifactSize_AbsentSizeStillValid(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	hd, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	task := &models.Artifact{
		ID:           "500.002-T",
		Title:        "No size task",
		Status:       "active",
		ArtifactType: "task",
		Priority:     "medium",
	}
	// size is optional with no default: a task without it must still validate.
	assert.NoError(t, ValidateArtifactFields(task, hd))
}

func TestSE1ValidateSizeValueFeatureShipmentHarness(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	hd, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)
	ws := &Workspace{HeaderDef: hd}

	for _, artifactType := range []string{"feature", "shipment"} {
		t.Run(artifactType, func(t *testing.T) {
			err := validateSizeValue(ws, artifactType, "M")
			if err != nil {
				t.Fatalf("TODO: implement SE-1 size seam schema for %s: %v", artifactType, err)
			}
			assert.NoError(t, validateSizeValue(ws, artifactType, "M"))
			assert.Error(t, validateSizeValue(ws, artifactType, "bogus"))
		})
	}
}
