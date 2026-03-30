package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/backlogit/backlogit/internal/models"
)

func TestSprintValidate_Valid(t *testing.T) {
	// Arrange
	s := models.Sprint{
		ID:   "SPRINT-01",
		Goal: "Ship MVP",
	}

	// Act
	err := s.Validate()

	// Assert
	assert.NoError(t, err)
}

func TestSprintValidate_MissingGoal(t *testing.T) {
	// Arrange
	s := models.Sprint{
		ID: "SPRINT-01",
	}

	// Act
	err := s.Validate()

	// Assert
	assert.Error(t, err)
}
