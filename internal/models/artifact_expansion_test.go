package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TASK-002.01.01: Expand Artifact struct with queue-specified fields.

func TestArtifactExpansion_NewFieldsPresent(t *testing.T) {
	// Arrange
	a := models.Artifact{
		ID:           "T001",
		Title:        "Test task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		AssignedTo:   "alice",
		Owner:        "bob",
		Labels:       []string{"backend", "urgent"},
		Dependencies: []string{"T002", "T003"},
		References:   []string{"docs/spec.md"},
		Commit:       "abc123",
	}

	// Act
	err := a.Validate()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "alice", a.AssignedTo)
	assert.Equal(t, "bob", a.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, a.Labels)
	assert.Equal(t, []string{"T002", "T003"}, a.Dependencies)
	assert.Equal(t, []string{"docs/spec.md"}, a.References)
	assert.Equal(t, "abc123", a.Commit)
}

func TestArtifactExpansion_EmptyOptionalFields(t *testing.T) {
	// Arrange — new fields omitted, should still validate
	a := models.Artifact{
		ID:           "T001",
		Title:        "Test task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
	}

	// Act
	err := a.Validate()

	// Assert
	require.NoError(t, err)
	assert.Empty(t, a.AssignedTo)
	assert.Empty(t, a.Owner)
	assert.Nil(t, a.Labels)
	assert.Nil(t, a.Dependencies)
	assert.Nil(t, a.References)
	assert.Empty(t, a.Commit)
}

func TestArtifactExpansion_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		assignedTo   string
		owner        string
		labels       []string
		dependencies []string
		references   []string
		commit       string
		wantErr      bool
	}{
		{
			name:         "all new fields populated",
			assignedTo:   "alice",
			owner:        "bob",
			labels:       []string{"frontend"},
			dependencies: []string{"T010"},
			references:   []string{"README.md"},
			commit:       "def456",
		},
		{
			name:   "only labels populated",
			labels: []string{"bug", "p0"},
		},
		{
			name:         "only dependencies populated",
			dependencies: []string{"T020", "T021", "T022"},
		},
		{
			name:       "only assigned_to populated",
			assignedTo: "charlie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			a := models.Artifact{
				ID:           "T001",
				Title:        "Test",
				Status:       models.StatusQueued,
				ArtifactType: "task",
				AssignedTo:   tt.assignedTo,
				Owner:        tt.owner,
				Labels:       tt.labels,
				Dependencies: tt.dependencies,
				References:   tt.references,
				Commit:       tt.commit,
			}

			// Act
			err := a.Validate()

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
