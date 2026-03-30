package models

import "time"

// ArtifactStatus represents the lifecycle state of a backlogit artifact.
type ArtifactStatus string

const (
	StatusTodo       ArtifactStatus = "todo"
	StatusInProgress ArtifactStatus = "in_progress"
	StatusBlocked    ArtifactStatus = "blocked"
	StatusReview     ArtifactStatus = "review"
	StatusDone       ArtifactStatus = "done"
	StatusAccepted   ArtifactStatus = "accepted"
	StatusRejected   ArtifactStatus = "rejected"
)

// Artifact holds the current state of a backlogit work item.
type Artifact struct {
	ID           string         `json:"id" yaml:"id" validate:"required"`
	Title        string         `json:"title" yaml:"title" validate:"required,max=200"`
	Status       ArtifactStatus `json:"status" yaml:"status" validate:"required,oneof=todo in_progress blocked review done accepted rejected"`
	ArtifactType string         `json:"artifact_type" yaml:"artifact_type" validate:"required"`
	ParentID     string         `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Sprint       string         `json:"sprint,omitempty" yaml:"sprint,omitempty"`
	Priority     string         `json:"priority,omitempty" yaml:"priority,omitempty"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	CreatedAt    time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" yaml:"updated_at"`
}

// Validate checks all struct tags and returns a descriptive error on failure.
//
// Worker: Implement struct validation using go-playground/validator.
func (a Artifact) Validate() error {
	panic("not implemented: Worker: Implement artifact struct validation")
}
