package core

import "github.com/backlogit/backlogit/internal/models"

// CompactQueueView is a token-efficient projection of QueueView, replacing full Artifact
// slices with CompactArtifact slices for list and queue responses.
type CompactQueueView struct {
	Items      []models.CompactArtifact `json:"items"`
	TotalCount int                      `json:"total_count"`
	GroupedBy  string                   `json:"grouped_by,omitempty"`
	Groups     []CompactQueueGroup      `json:"groups,omitempty"`
}

// CompactQueueGroup mirrors QueueGroup but with CompactArtifact items.
type CompactQueueGroup struct {
	Label string                   `json:"label"`
	Items []models.CompactArtifact `json:"items"`
	Count int                      `json:"count"`
}

// CompactView converts a QueueView into its compact projection.
func CompactView(v *QueueView) *CompactQueueView {
	if v == nil {
		return nil
	}
	compact := &CompactQueueView{
		TotalCount: v.TotalCount,
		GroupedBy:  v.GroupedBy,
	}
	compact.Items = make([]models.CompactArtifact, 0, len(v.Items))
	for _, a := range v.Items {
		compact.Items = append(compact.Items, a.Compact())
	}
	compact.Groups = make([]CompactQueueGroup, 0, len(v.Groups))
	for _, g := range v.Groups {
		cg := CompactQueueGroup{
			Label: g.Label,
			Count: g.Count,
			Items: make([]models.CompactArtifact, 0, len(g.Items)),
		}
		for _, a := range g.Items {
			cg.Items = append(cg.Items, a.Compact())
		}
		compact.Groups = append(compact.Groups, cg)
	}
	return compact
}
