package models

import "time"

// Sprint represents a sprint container with goal and date boundaries.
type Sprint struct {
	ID          string    `json:"id" yaml:"id" validate:"required"`
	Goal        string    `json:"goal" yaml:"goal" validate:"required"`
	StartDate   time.Time `json:"start_date,omitempty" yaml:"start_date,omitempty"`
	EndDate     time.Time `json:"end_date,omitempty" yaml:"end_date,omitempty"`
	ArtifactIDs []string  `json:"artifact_ids,omitempty" yaml:"artifact_ids,omitempty"`
}

// Validate checks sprint required fields.
//
// Worker: Implement sprint validation.
func (s Sprint) Validate() error {
	panic("not implemented: Worker: Implement sprint validation")
}
