package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestValidateArtifactFields_RequiredFieldMissing(t *testing.T) {
	// Arrange
	headerDef := testHeaderDef()
	artifact := &models.Artifact{
		ID:           "T001",
		Title:        "Test task",
		ArtifactType: "task",
		Status:       models.StatusQueued,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		// status field required but set via Status — severity required for bug but not task
	}

	// Act
	err := core.ValidateArtifactFields(artifact, headerDef)

	// Assert — task should pass (status is set, priority is optional)
	require.NoError(t, err)
}

func TestValidateArtifactFields_BugMissingSeverity(t *testing.T) {
	// Arrange
	headerDef := testHeaderDef()
	artifact := &models.Artifact{
		ID:           "B001",
		Title:        "Test bug",
		ArtifactType: "bug",
		Status:       models.StatusQueued,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		// severity is required for bug type but not set
	}

	// Act
	err := core.ValidateArtifactFields(artifact, headerDef)

	// Assert — should error because severity is required for bug
	require.Error(t, err)
	assert.Contains(t, err.Error(), "severity")
}

func TestApplyFieldDefaults_SetsOptionalDefaults(t *testing.T) {
	// Arrange
	headerDef := testHeaderDef()
	artifact := &models.Artifact{
		ID:           "T002",
		Title:        "Task without priority",
		ArtifactType: "task",
		Status:       models.StatusQueued,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		// priority is optional with default "medium" — should be applied
	}

	// Act
	err := core.ApplyFieldDefaults(artifact, headerDef)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "medium", artifact.Priority)
}

func TestValidateArtifactFields_BackwardCompat(t *testing.T) {
	// Arrange — existing artifact without new required fields should pass
	headerDef := testHeaderDef()
	artifact := &models.Artifact{
		ID:           "T003",
		Title:        "Old task",
		ArtifactType: "task",
		Status:       models.StatusDone,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Act
	err := core.ValidateArtifactFields(artifact, headerDef)

	// Assert — should pass for backward compatibility
	require.NoError(t, err)
}
