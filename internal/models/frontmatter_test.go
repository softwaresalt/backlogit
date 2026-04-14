package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	// Arrange
	content := "---\nid: T001\ntitle: Test\n---\n\nBody text"

	// Act
	fm, body, err := models.ParseFrontmatter(content)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "T001", fm["id"])
	assert.Equal(t, "Body text", body)
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	// Arrange
	content := "Just body text"

	// Act
	fm, body, err := models.ParseFrontmatter(content)

	// Assert
	require.NoError(t, err)
	assert.Nil(t, fm)
	assert.Equal(t, "Just body text", body)
}

func TestSerializeFrontmatter_RoundTrip(t *testing.T) {
	// Arrange
	fields := map[string]any{"id": "T001", "title": "Test"}
	body := "Body text"

	// Act
	serialized := models.SerializeFrontmatter(fields, body)

	// Assert
	assert.Contains(t, serialized, "---")
	assert.Contains(t, serialized, "Body text")
}

func TestArtifactFromFrontmatter_Valid(t *testing.T) {
	// Arrange
	fm := map[string]any{
		"id":            "T001",
		"title":         "Test",
		"status":        "queued",
		"artifact_type": "task",
	}

	// Act
	artifact, err := models.ArtifactFromFrontmatter(fm, "description")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "T001", artifact.ID)
	assert.Equal(t, models.StatusQueued, artifact.Status)
}
