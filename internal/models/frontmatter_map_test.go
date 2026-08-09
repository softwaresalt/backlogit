package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToFrontmatterMap_EmitsModeledFields pins the shared serializer contract:
// every modeled frontmatter field is emitted by ToFrontmatterMap so the create
// path and WriteArtifactFile can share one map builder and never diverge
// (stash 12B5649E).
func TestToFrontmatterMap_EmitsModeledFields(t *testing.T) {
	now := NowUTC()
	a := &Artifact{
		ID:           "500-T",
		Title:        "Round trip",
		Status:       StatusQueued,
		ArtifactType: "task",
		ParentID:     "500-F",
		Sprint:       "sprint-1",
		Priority:     "high",
		Description:  "body text",
		AssignedTo:   "agent",
		Owner:        "owner",
		Labels:       []string{"a", "b"},
		Dependencies: []DependencyEdge{{ID: "499-T", Type: "blocks"}},
		Links:        []ArtifactLink{{TargetID: "498-T", LinkType: "related_to"}},
		References:   []string{"docs/x.md"},
		Commit:       "abc1234",
		CustomFields: map[string]any{"k": "v"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	fm := a.ToFrontmatterMap()

	assert.Equal(t, "500-T", fm["id"])
	assert.Equal(t, "Round trip", fm["title"])
	assert.Equal(t, "queued", fm["status"])
	assert.Equal(t, "task", fm["artifact_type"])
	assert.Equal(t, now, fm["created_at"])
	assert.Equal(t, now, fm["updated_at"])
	assert.Equal(t, "500-F", fm["parent_id"])
	assert.Equal(t, "sprint-1", fm["sprint"])
	assert.Equal(t, "high", fm["priority"])
	assert.Equal(t, "agent", fm["assigned_to"])
	assert.Equal(t, "owner", fm["owner"])
	assert.Equal(t, []string{"a", "b"}, fm["labels"])
	assert.Equal(t, []string{"499-T"}, fm["dependencies"])
	assert.Contains(t, fm, "links")
	assert.Equal(t, []string{"docs/x.md"}, fm["references"])
	assert.Equal(t, "abc1234", fm["commit"])
	assert.Equal(t, map[string]any{"k": "v"}, fm["custom_fields"])

	// Description is the body, not a frontmatter key.
	assert.NotContains(t, fm, "description")
}

// TestToFrontmatterMap_OmitsEmptyOptionalFields proves optional fields are
// omitted when empty so the shared builder never emits stale zero-value keys.
func TestToFrontmatterMap_OmitsEmptyOptionalFields(t *testing.T) {
	now := NowUTC()
	a := &Artifact{
		ID:           "501-T",
		Title:        "Minimal",
		Status:       StatusQueued,
		ArtifactType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	fm := a.ToFrontmatterMap()

	for _, key := range []string{
		"parent_id", "sprint", "priority", "assigned_to", "owner",
		"labels", "dependencies", "links", "references", "commit",
		"custom_fields", "archived_from", "archived_status",
	} {
		assert.NotContains(t, fm, key, "empty optional field %q must be omitted", key)
	}
}

// TestToFrontmatterMap_ArchiveProvenanceStatusGated proves the shared builder
// keeps the invariant "archive provenance <=> archived status": provenance keys
// are emitted only while the artifact is archived, matching WriteArtifactFile's
// prior status-gated behavior.
func TestToFrontmatterMap_ArchiveProvenanceStatusGated(t *testing.T) {
	now := NowUTC()
	base := Artifact{
		ID:             "502-T",
		Title:          "Provenance",
		ArtifactType:   "task",
		ArchivedFrom:   "queue/502-T.md",
		ArchivedStatus: "done",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	t.Run("emits provenance when archived", func(t *testing.T) {
		a := base
		a.Status = StatusArchived
		fm := a.ToFrontmatterMap()
		assert.Equal(t, "queue/502-T.md", fm["archived_from"])
		assert.Equal(t, "done", fm["archived_status"])
	})

	t.Run("omits provenance when not archived", func(t *testing.T) {
		a := base
		a.Status = StatusQueued
		fm := a.ToFrontmatterMap()
		assert.NotContains(t, fm, "archived_from")
		assert.NotContains(t, fm, "archived_status")
	})
}
