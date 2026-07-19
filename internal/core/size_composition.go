package core

import (
	"context"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/models"
)

// SizeCompositionMember describes one canonical member included in a size rollup.
type SizeCompositionMember struct {
	ID           string `json:"id"`
	ArtifactType string `json:"artifact_type"`
	Size         string `json:"size,omitempty"`
}

// SizeCompositionResult is the computed-on-read size rollup for a feature or shipment.
type SizeCompositionResult struct {
	Histogram      map[string]int          `json:"histogram"`
	Unsized        int                     `json:"unsized"`
	Members        []SizeCompositionMember `json:"members"`
	Skipped        []string                `json:"skipped,omitempty"`
	RulesetVersion *string                 `json:"ruleset_version"`
}

// SizeComposition computes the size histogram for a feature or shipment.
func SizeComposition(_ context.Context, _ *Workspace, _ *models.Artifact) (*SizeCompositionResult, error) {
	return nil, fmt.Errorf("SE-4 SizeComposition: %w", ErrSizeEstimationNotImplemented)
}
