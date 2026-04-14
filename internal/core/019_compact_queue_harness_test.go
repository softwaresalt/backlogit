package core_test

// 019.003-T: Compact projection mode for QueueView (get_queue compact flag).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestCompactView_ReducesItemFields(t *testing.T) {
	// 019.003-T: CompactView maps full Artifact slices to CompactArtifact slices.
	full := &core.QueueView{
		Items: []*models.Artifact{
			{
				ID: "T001", Title: "Task one", Status: models.StatusQueued,
				ArtifactType: "task", Priority: "high",
				Description: "Should be omitted",
			},
			{
				ID: "T002", Title: "Task two", Status: models.StatusActive,
				ArtifactType: "task", Priority: "medium",
				Description: "Should also be omitted",
			},
		},
		TotalCount: 2,
	}

	compact := core.CompactView(full)

	require.NotNil(t, compact, "CompactView must return non-nil result")
	assert.Len(t, compact.Items, 2)
	assert.Equal(t, 2, compact.TotalCount)
	assert.Equal(t, "T001", compact.Items[0].ID)
	assert.Equal(t, "T002", compact.Items[1].ID)
}

func TestCompactView_PreservesGroups(t *testing.T) {
	// 019.003-T: CompactView preserves group structure and maps group items to compact form.
	full := &core.QueueView{
		Items:     []*models.Artifact{{ID: "T001", Title: "Task", Status: models.StatusQueued, ArtifactType: "task"}},
		GroupedBy: "status",
		Groups: []core.QueueGroup{
			{
				Label: "queued",
				Items: []*models.Artifact{
					{ID: "T001", Title: "Task", Status: models.StatusQueued, ArtifactType: "task"},
				},
				Count: 1,
			},
		},
		TotalCount: 1,
	}

	compact := core.CompactView(full)

	require.NotNil(t, compact, "CompactView must return non-nil result")
	assert.Equal(t, "status", compact.GroupedBy)
	require.Len(t, compact.Groups, 1)
	assert.Equal(t, "queued", compact.Groups[0].Label)
	assert.Len(t, compact.Groups[0].Items, 1)
}
