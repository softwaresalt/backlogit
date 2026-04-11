package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/models"
)

// TestArtifactLink_Struct verifies that ArtifactLink can be constructed and its
// fields are accessible as specified.
func TestArtifactLink_Struct(t *testing.T) {
	link := models.ArtifactLink{
		TargetID: "002-F",
		LinkType: "related_to",
	}

	assert.Equal(t, "002-F", link.TargetID)
	assert.Equal(t, "related_to", link.LinkType)
}

// TestArtifactFromFrontmatter_WithoutLinks confirms backward compatibility:
// an artifact serialized without a links key deserializes with a nil Links
// slice, not an error.
func TestArtifactFromFrontmatter_WithoutLinks(t *testing.T) {
	fields := map[string]any{
		"id":            "001-F",
		"title":         "Feature without links",
		"status":        "queued",
		"artifact_type": "feature",
	}

	artifact, err := models.ArtifactFromFrontmatter(fields, "body")
	require.NoError(t, err)
	assert.Nil(t, artifact.Links)
}

func TestArtifactFromFrontmatter_LinksField_RoundTrip(t *testing.T) {
	fields := map[string]any{
		"id":            "001-F",
		"title":         "Feature",
		"status":        "queued",
		"artifact_type": "feature",
		"links": []map[string]any{
			{"target_id": "002-F", "link_type": "related_to"},
			{"target_id": "003-F", "link_type": "informs"},
		},
	}

	artifact, err := models.ArtifactFromFrontmatter(fields, "body")
	require.NoError(t, err)
	assert.Equal(t, []models.ArtifactLink{
		{TargetID: "002-F", LinkType: "related_to"},
		{TargetID: "003-F", LinkType: "informs"},
	}, artifact.Links)
}

func TestSerializeFrontmatter_LinksField_EmptyOmitted(t *testing.T) {
	fields := map[string]any{
		"id":            "001-F",
		"title":         "Feature",
		"status":        "queued",
		"artifact_type": "feature",
	}

	serialized := models.SerializeFrontmatter(fields, "body")
	assert.NotContains(t, serialized, "links:")
}

func TestSerializeFrontmatter_LinksField_RoundTrip(t *testing.T) {
	fields := map[string]any{
		"id":            "001-F",
		"title":         "Feature",
		"status":        "queued",
		"artifact_type": "feature",
		"links": []models.ArtifactLink{
			{TargetID: "002-F", LinkType: "related_to"},
			{TargetID: "003-F", LinkType: "supersedes"},
			{TargetID: "004-F", LinkType: "spike_ref"},
		},
	}

	serialized := models.SerializeFrontmatter(fields, "body")
	fm, body, err := models.ParseFrontmatter(serialized)
	require.NoError(t, err)

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	require.NoError(t, err)
	assert.Equal(t, []models.ArtifactLink{
		{TargetID: "002-F", LinkType: "related_to"},
		{TargetID: "003-F", LinkType: "supersedes"},
		{TargetID: "004-F", LinkType: "spike_ref"},
	}, artifact.Links)
}
