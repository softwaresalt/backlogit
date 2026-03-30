package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/backlogit/backlogit/internal/models"
)

func TestArtifactValidate_ValidArtifact(t *testing.T) {
	// Arrange
	a := models.Artifact{
		ID:           "T001",
		Title:        "Test task",
		Status:       models.StatusTodo,
		ArtifactType: "task",
	}

	// Act
	err := a.Validate()

	// Assert
	assert.NoError(t, err)
}

func TestArtifactValidate_MissingID(t *testing.T) {
	// Arrange
	a := models.Artifact{
		Title:        "Test task",
		Status:       models.StatusTodo,
		ArtifactType: "task",
	}

	// Act
	err := a.Validate()

	// Assert
	assert.Error(t, err)
}

func TestArtifactValidate_InvalidStatus(t *testing.T) {
	// Arrange
	a := models.Artifact{
		ID:           "T001",
		Title:        "Test",
		Status:       "invalid_status",
		ArtifactType: "task",
	}

	// Act
	err := a.Validate()

	// Assert
	assert.Error(t, err)
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, models.ArtifactStatus("todo"), models.StatusTodo)
	assert.Equal(t, models.ArtifactStatus("in_progress"), models.StatusInProgress)
	assert.Equal(t, models.ArtifactStatus("blocked"), models.StatusBlocked)
	assert.Equal(t, models.ArtifactStatus("review"), models.StatusReview)
	assert.Equal(t, models.ArtifactStatus("done"), models.StatusDone)
}
