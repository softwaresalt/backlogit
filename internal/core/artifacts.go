package core

import (
	"context"

	"github.com/backlogit/backlogit/internal/models"
)

// Option configures artifact creation.
type Option func(*createOptions)

type createOptions struct {
	ParentID    string
	Sprint      string
	Status      string
	Description string
	Fields      map[string]any
}

// WithParent sets the parent artifact ID.
func WithParent(id string) Option {
	return func(o *createOptions) { o.ParentID = id }
}

// WithSprint sets the sprint ID.
func WithSprint(id string) Option {
	return func(o *createOptions) { o.Sprint = id }
}

// WithStatus sets the initial status.
func WithStatus(status string) Option {
	return func(o *createOptions) { o.Status = status }
}

// WithDescription sets the artifact description.
func WithDescription(desc string) Option {
	return func(o *createOptions) { o.Description = desc }
}

// WithFields sets custom fields.
func WithFields(fields map[string]any) Option {
	return func(o *createOptions) { o.Fields = fields }
}

// CreateArtifact creates a new artifact with hierarchy enforcement and atomic writes.
//
// Worker: Implement artifact creation with naming, routing, and file writing.
func CreateArtifact(ctx context.Context, ws *Workspace, title string, artifactType string, opts ...Option) (*models.Artifact, error) {
	panic("not implemented: Worker: Implement artifact creation with hierarchy enforcement")
}

// UpdateArtifact updates an existing artifact's fields and moves it if status changed.
//
// Worker: Implement artifact update with re-validation and atomic writes.
func UpdateArtifact(ctx context.Context, ws *Workspace, id string, updates map[string]any) (*models.Artifact, error) {
	panic("not implemented: Worker: Implement artifact update")
}
