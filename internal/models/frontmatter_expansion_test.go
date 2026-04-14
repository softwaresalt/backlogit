package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TASK-002.01.03: Update frontmatter parser and serializer for new fields.

func TestArtifactFromFrontmatter_NewFields(t *testing.T) {
	// Arrange
	fm := map[string]any{
		"id":            "T001",
		"title":         "Test",
		"status":        "queued",
		"artifact_type": "task",
		"assigned_to":   "alice",
		"owner":         "bob",
		"labels":        []any{"backend", "urgent"},
		"dependencies":  []any{"T002", "T003"},
		"references":    []any{"docs/spec.md"},
		"commit":        "abc123",
	}

	// Act
	artifact, err := models.ArtifactFromFrontmatter(fm, "description")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "alice", artifact.AssignedTo)
	assert.Equal(t, "bob", artifact.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, artifact.Labels)
	assert.Equal(t, []string{"T002", "T003"}, artifact.Dependencies)
	assert.Equal(t, []string{"docs/spec.md"}, artifact.References)
	assert.Equal(t, "abc123", artifact.Commit)
}

func TestArtifactFromFrontmatter_MissingNewFields(t *testing.T) {
	// Arrange — no new fields present
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
	assert.Empty(t, artifact.AssignedTo)
	assert.Nil(t, artifact.Labels)
}

func TestSerializeFrontmatter_NewFieldsRoundTrip(t *testing.T) {
	// Arrange
	fields := map[string]any{
		"id":            "T001",
		"title":         "Test",
		"status":        "queued",
		"artifact_type": "task",
		"assigned_to":   "alice",
		"owner":         "bob",
		"labels":        []string{"backend", "urgent"},
		"dependencies":  []string{"T002"},
		"references":    []string{"docs/spec.md"},
		"commit":        "abc123",
	}

	// Act
	serialized := models.SerializeFrontmatter(fields, "body")
	fm, body, err := models.ParseFrontmatter(serialized)
	require.NoError(t, err)

	artifact, err := models.ArtifactFromFrontmatter(fm, body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "alice", artifact.AssignedTo)
	assert.Equal(t, "bob", artifact.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, artifact.Labels)
	assert.Equal(t, []string{"T002"}, artifact.Dependencies)
	assert.Equal(t, []string{"docs/spec.md"}, artifact.References)
	assert.Equal(t, "abc123", artifact.Commit)
}

func TestArtifactFromFrontmatter_DerivesHierarchyMetadataFromID(t *testing.T) {
	fm := map[string]any{
		"id":            "016.002-T",
		"title":         "Test",
		"status":        "queued",
		"artifact_type": "task",
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, "description")

	require.NoError(t, err)
	assert.Equal(t, 2, artifact.Level)
	assert.Equal(t, "016/016.002", artifact.HierarchyPath)
}

func TestArtifactFromFrontmatter_PreservesExplicitHierarchyMetadata(t *testing.T) {
	fm := map[string]any{
		"id":             "016.002-T",
		"title":          "Test",
		"status":         "queued",
		"artifact_type":  "task",
		"level":          7,
		"hierarchy_path": "custom/path",
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, "description")

	require.NoError(t, err)
	assert.Equal(t, 7, artifact.Level)
	assert.Equal(t, "custom/path", artifact.HierarchyPath)
}
