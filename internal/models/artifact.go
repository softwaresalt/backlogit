package models

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ArtifactStatus represents the lifecycle state of a backlogit artifact.
type ArtifactStatus string

const (
	StatusQueued    ArtifactStatus = "queued"
	StatusActive    ArtifactStatus = "active"
	StatusBlocked   ArtifactStatus = "blocked"
	StatusReview    ArtifactStatus = "review"
	StatusDone      ArtifactStatus = "done"
	StatusAccepted  ArtifactStatus = "accepted"
	StatusRejected  ArtifactStatus = "rejected"
	StatusArchived  ArtifactStatus = "archived"
	StatusShipped   ArtifactStatus = "shipped"
	StatusAbandoned ArtifactStatus = "abandoned"
)

// ArtifactLink represents a durable outgoing semantic link stored in artifact frontmatter.
type ArtifactLink struct {
	TargetID string `json:"target_id" yaml:"target_id"`
	LinkType string `json:"link_type" yaml:"link_type"`
}

// DependencyEdge represents a durable outgoing dependency edge stored in artifact
// frontmatter. The canonical YAML shape supports two entry forms:
//
//   - A bare string (e.g. "T001") means {ID: "T001", Type: "blocks"}.
//   - An object (e.g. {id: T002, type: relates_to}) carries an explicit type.
//
// Accepted Type values: blocks, relates_to, parent_of.
// An empty Type is treated as "blocks" everywhere in the pipeline.
type DependencyEdge struct {
	ID   string `json:"id" yaml:"id"`
	Type string `json:"type" yaml:"type"`
}

// Artifact holds the current state of a backlogit work item.
type Artifact struct {
	ID           string           `json:"id" yaml:"id" validate:"required"`
	Title        string           `json:"title" yaml:"title" validate:"required,max=200"`
	Status       ArtifactStatus   `json:"status" yaml:"status" validate:"required,oneof=queued active blocked review done accepted rejected archived shipped abandoned"`
	ArtifactType string           `json:"artifact_type" yaml:"artifact_type" validate:"required"`
	ParentID     string           `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Sprint       string           `json:"sprint,omitempty" yaml:"sprint,omitempty"`
	Priority     string           `json:"priority,omitempty" yaml:"priority,omitempty"`
	Description  string           `json:"description,omitempty" yaml:"description,omitempty"`
	AssignedTo   string           `json:"assigned_to,omitempty" yaml:"assigned_to,omitempty"`
	Owner        string           `json:"owner,omitempty" yaml:"owner,omitempty"`
	Labels       []string         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Dependencies []DependencyEdge `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Links        []ArtifactLink   `json:"links,omitempty" yaml:"links,omitempty"`
	References   []string         `json:"references,omitempty" yaml:"references,omitempty"`
	Commit       string           `json:"commit,omitempty" yaml:"commit,omitempty"`
	// ArchivedFrom and ArchivedStatus carry archive provenance so an archived
	// item survives the typed update round-trip (they are written raw by
	// ArchiveItem and would otherwise be dropped, leaving the record
	// non-invertible). WriteArtifactFile emits them only while the item is
	// archived; UnarchiveItem clears them via the raw frontmatter path.
	ArchivedFrom   string         `json:"archived_from,omitempty" yaml:"archived_from,omitempty"`
	ArchivedStatus string         `json:"archived_status,omitempty" yaml:"archived_status,omitempty"`
	CustomFields   map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	CreatedAt      time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" yaml:"updated_at"`
	Level          int            `json:"level,omitempty" yaml:"level,omitempty"`
	HierarchyPath  string         `json:"hierarchy_path,omitempty" yaml:"hierarchy_path,omitempty"`
}

// Validate checks all struct tags and returns a descriptive error on failure.
func (a Artifact) Validate() error {
	return validate.Struct(a)
}

// ToFrontmatterMap builds the canonical frontmatter map for an artifact. It is
// the single source of truth for which modeled fields are serialized and under
// what conditions, so every write path (create and update) shares one builder
// and no path can emit a field another path silently drops. Required keys are
// always present; optional keys are emitted only when non-empty. Archive
// provenance is status-gated to preserve the invariant "archive provenance <=>
// archived status": the keys are emitted only while the item is archived, which
// keeps them across an update round-trip on an archived item and omits stale
// keys on any non-archived item.
//
// Dependencies: if all edges carry the default type (blocks or empty), the
// list is serialized as bare strings for backward compatibility. If any edge
// carries a non-default type (relates_to, parent_of), all edges are serialized
// as typed objects {id: <id>, type: <type>} so the type survives rehydration.
func (a *Artifact) ToFrontmatterMap() map[string]any {
	fm := map[string]any{
		"id":            a.ID,
		"title":         a.Title,
		"status":        string(a.Status),
		"artifact_type": a.ArtifactType,
		"created_at":    a.CreatedAt,
		"updated_at":    a.UpdatedAt,
	}
	if a.ParentID != "" {
		fm["parent_id"] = a.ParentID
	}
	if a.Sprint != "" {
		fm["sprint"] = a.Sprint
	}
	if a.Priority != "" {
		fm["priority"] = a.Priority
	}
	if a.AssignedTo != "" {
		fm["assigned_to"] = a.AssignedTo
	}
	if a.Owner != "" {
		fm["owner"] = a.Owner
	}
	if len(a.Labels) > 0 {
		fm["labels"] = a.Labels
	}
	if len(a.Dependencies) > 0 {
		fm["dependencies"] = serializeDependencies(a.Dependencies)
	}
	if len(a.Links) > 0 {
		fm["links"] = a.Links
	}
	if len(a.References) > 0 {
		fm["references"] = a.References
	}
	if a.Commit != "" {
		fm["commit"] = a.Commit
	}
	if a.Status == StatusArchived {
		if a.ArchivedFrom != "" {
			fm["archived_from"] = a.ArchivedFrom
		}
		if a.ArchivedStatus != "" {
			fm["archived_status"] = a.ArchivedStatus
		}
	}
	if a.CustomFields != nil {
		fm["custom_fields"] = a.CustomFields
	}
	return fm
}

// serializeDependencies converts a []DependencyEdge to the YAML-friendly value
// that ToFrontmatterMap emits for the "dependencies" key.
//
// If every edge carries the default type (blocks or empty string), the result
// is a []string of bare IDs for byte-identical round-trips with the legacy
// format. When at least one edge carries a non-default type (relates_to,
// parent_of), all edges are emitted as typed objects {id, type} so dep_type
// survives a Rehydrate cycle.
func serializeDependencies(edges []DependencyEdge) any {
	hasNonDefault := false
	for _, e := range edges {
		if e.Type != "" && e.Type != "blocks" {
			hasNonDefault = true
			break
		}
	}
	if !hasNonDefault {
		// Legacy format: bare string list.
		ids := make([]string, len(edges))
		for i, e := range edges {
			ids[i] = e.ID
		}
		return ids
	}
	// Typed format: all edges as objects.
	objs := make([]map[string]string, len(edges))
	for i, e := range edges {
		t := e.Type
		if t == "" {
			t = "blocks"
		}
		objs[i] = map[string]string{"id": e.ID, "type": t}
	}
	return objs
}
