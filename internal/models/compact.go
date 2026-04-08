package models

// CompactArtifact is a reduced projection of an Artifact for token-efficient agent consumption.
// It omits description, timestamps, labels, dependencies, references, and custom fields.
type CompactArtifact struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Status       ArtifactStatus `json:"status"`
	ArtifactType string         `json:"artifact_type"`
	ParentID     string         `json:"parent_id,omitempty"`
	Priority     string         `json:"priority,omitempty"`
	AssignedTo   string         `json:"assigned_to,omitempty"`
	Owner        string         `json:"owner,omitempty"`
}

// Compact returns a reduced projection of the artifact suitable for list views.
func (a *Artifact) Compact() CompactArtifact {
	return CompactArtifact{
		ID:           a.ID,
		Title:        a.Title,
		Status:       a.Status,
		ArtifactType: a.ArtifactType,
		ParentID:     a.ParentID,
		Priority:     a.Priority,
		AssignedTo:   a.AssignedTo,
		Owner:        a.Owner,
	}
}
