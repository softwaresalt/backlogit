package models_test

// 019.003-T: Compact projection mode for Artifact.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestArtifact_Compact_ReturnsReducedFields(t *testing.T) {
	// 019.003-T: Compact() returns a CompactArtifact with only the summary fields.
	now := time.Now()
	full := &models.Artifact{
		ID:           "019-T",
		Title:        "Implement compact projection",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		ParentID:     "019-F",
		Priority:     "high",
		AssignedTo:   "alice",
		Owner:        "bob",
		Description:  "Long description that should be omitted in compact view",
		Labels:       []string{"backend"},
		Dependencies: []string{"018-T"},
		References:   []string{"docs/spec.md"},
		CustomFields: map[string]any{"foo": "bar"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	compact := full.Compact()

	assert.Equal(t, full.ID, compact.ID, "compact ID must match")
	assert.Equal(t, full.Title, compact.Title, "compact title must match")
	assert.Equal(t, full.Status, compact.Status, "compact status must match")
	assert.Equal(t, full.ArtifactType, compact.ArtifactType, "compact artifact_type must match")
	assert.Equal(t, full.ParentID, compact.ParentID, "compact parent_id must match")
	assert.Equal(t, full.Priority, compact.Priority, "compact priority must match")
	assert.Equal(t, full.AssignedTo, compact.AssignedTo, "compact assigned_to must match")
	assert.Equal(t, full.Owner, compact.Owner, "compact owner must match")
}
