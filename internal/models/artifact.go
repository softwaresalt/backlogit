package models

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ArtifactStatus represents the lifecycle state of a backlogit artifact.
type ArtifactStatus string

const (
	StatusQueued   ArtifactStatus = "queued"
	StatusActive   ArtifactStatus = "active"
	StatusBlocked  ArtifactStatus = "blocked"
	StatusReview   ArtifactStatus = "review"
	StatusDone     ArtifactStatus = "done"
	StatusAccepted ArtifactStatus = "accepted"
	StatusRejected ArtifactStatus = "rejected"
	StatusArchived ArtifactStatus = "archived"
)

// Artifact holds the current state of a backlogit work item.
type Artifact struct {
	ID           string         `json:"id" yaml:"id" validate:"required"`
	Title        string         `json:"title" yaml:"title" validate:"required,max=200"`
	Status       ArtifactStatus `json:"status" yaml:"status" validate:"required,oneof=queued active blocked review done accepted rejected archived"`
	ArtifactType string         `json:"artifact_type" yaml:"artifact_type" validate:"required"`
	ParentID     string         `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Sprint       string         `json:"sprint,omitempty" yaml:"sprint,omitempty"`
	Priority     string         `json:"priority,omitempty" yaml:"priority,omitempty"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
	AssignedTo   string         `json:"assigned_to,omitempty" yaml:"assigned_to,omitempty"`
	Owner        string         `json:"owner,omitempty" yaml:"owner,omitempty"`
	Labels       []string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	References   []string       `json:"references,omitempty" yaml:"references,omitempty"`
	Commit       string         `json:"commit,omitempty" yaml:"commit,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	CreatedAt    time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" yaml:"updated_at"`
	Level         int    `json:"level,omitempty" yaml:"level,omitempty"`
	HierarchyPath string `json:"hierarchy_path,omitempty" yaml:"hierarchy_path,omitempty"`
}

// Validate checks all struct tags and returns a descriptive error on failure.
func (a Artifact) Validate() error {
	return validate.Struct(a)
}
