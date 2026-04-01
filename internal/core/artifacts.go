package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/models"
)

// Option configures artifact creation.
type Option func(*createOptions)

type createOptions struct {
	ParentID     string
	Sprint       string
	Status       string
	Description  string
	Fields       map[string]any
	AssignedTo   string
	Owner        string
	Labels       []string
	Dependencies []string
	References   []string
	Commit       string
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

// WithAssignedTo sets the assigned user.
func WithAssignedTo(user string) Option {
	return func(o *createOptions) { o.AssignedTo = user }
}

// WithOwner sets the artifact owner.
func WithOwner(owner string) Option {
	return func(o *createOptions) { o.Owner = owner }
}

// WithLabels sets the artifact labels.
func WithLabels(labels []string) Option {
	return func(o *createOptions) { o.Labels = labels }
}

// WithDependencies sets the artifact dependencies.
func WithDependencies(deps []string) Option {
	return func(o *createOptions) { o.Dependencies = deps }
}

// WithReferences sets the artifact references.
func WithReferences(refs []string) Option {
	return func(o *createOptions) { o.References = refs }
}

// WithCommit sets the artifact commit hash.
func WithCommit(commit string) Option {
	return func(o *createOptions) { o.Commit = commit }
}

// CreateArtifact creates a new artifact with atomic file write.
func CreateArtifact(ctx context.Context, ws *Workspace, title string, artifactType string, opts ...Option) (*models.Artifact, error) {
	if ws == nil || ws.Config == nil {
		return nil, fmt.Errorf("workspace config is required")
	}
	o := &createOptions{}
	for _, opt := range opts {
		opt(o)
	}

	typeConfig, ok := ws.Config.ArtifactTypes[artifactType]
	if !ok {
		return nil, fmt.Errorf("unknown artifact type: %s", artifactType)
	}

	nextID, err := NextID(ctx, ws.DB, artifactType)
	if err != nil {
		return nil, fmt.Errorf("get next id: %w", err)
	}

	name := ResolveName(typeConfig, title, nextID, ws.Config.MaxSlugLength)

	status := o.Status
	if status == "" {
		status = "queued"
	}

	now := time.Now()
	artifact := &models.Artifact{
		ID:           name,
		Title:        title,
		Status:       models.ArtifactStatus(status),
		ArtifactType: artifactType,
		ParentID:     o.ParentID,
		Sprint:       o.Sprint,
		Description:  o.Description,
		CustomFields: o.Fields,
		AssignedTo:   o.AssignedTo,
		Owner:        o.Owner,
		Labels:       o.Labels,
		Dependencies: o.Dependencies,
		References:   o.References,
		Commit:       o.Commit,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Apply field defaults and validate against header-def if available.
	if ws.HeaderDef != nil {
		if err := ApplyFieldDefaults(artifact, ws.HeaderDef); err != nil {
			return nil, fmt.Errorf("apply field defaults: %w", err)
		}
		if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
			return nil, fmt.Errorf("validate artifact fields: %w", err)
		}
	}

	if err := artifact.Validate(); err != nil {
		return nil, fmt.Errorf("validate artifact: %w", err)
	}

	// Determine target directory: use hierarchy layout if configured, otherwise flat.
	var dir string
	if ws.Config.QueueLayout != nil {
		if _, levelErr := LevelForType(ws.Config.QueueLayout, artifactType); levelErr == nil {
			hierPath, hierErr := ResolveHierarchicalPath(ws.Config.QueueLayout, o.ParentID, artifactType)
			if hierErr == nil {
				dir = hierPath
				artifact.HierarchyPath = dir
				level, _ := LevelForType(ws.Config.QueueLayout, artifactType)
				artifact.Level = level
			}
		}
	}
	if dir == "" {
		dir = artifactType + "s"
	}
	dirAbs := filepath.Join(ws.RootPath, dir)
	if err := os.MkdirAll(dirAbs, 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	fm := map[string]any{
		"id":            artifact.ID,
		"title":         artifact.Title,
		"status":        string(artifact.Status),
		"artifact_type": artifact.ArtifactType,
		"created_at":    artifact.CreatedAt,
		"updated_at":    artifact.UpdatedAt,
	}
	if artifact.ParentID != "" {
		fm["parent_id"] = artifact.ParentID
	}
	if artifact.Sprint != "" {
		fm["sprint"] = artifact.Sprint
	}
	if artifact.Priority != "" {
		fm["priority"] = artifact.Priority
	}
	if artifact.AssignedTo != "" {
		fm["assigned_to"] = artifact.AssignedTo
	}
	if artifact.Owner != "" {
		fm["owner"] = artifact.Owner
	}
	if len(artifact.Labels) > 0 {
		fm["labels"] = artifact.Labels
	}
	if len(artifact.Dependencies) > 0 {
		fm["dependencies"] = artifact.Dependencies
	}
	if len(artifact.References) > 0 {
		fm["references"] = artifact.References
	}
	if artifact.Commit != "" {
		fm["commit"] = artifact.Commit
	}
	if artifact.CustomFields != nil {
		fm["custom_fields"] = artifact.CustomFields
	}
	if artifact.Level > 0 {
		fm["level"] = artifact.Level
	}
	if artifact.HierarchyPath != "" {
		fm["hierarchy_path"] = artifact.HierarchyPath
	}

	content := models.SerializeFrontmatter(fm, artifact.Description)
	filePath := filepath.Join(dirAbs, name+".md")

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write artifact file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("rename artifact file: %w", err)
	}

	return artifact, nil
}

// UpdateArtifact updates an existing artifact's fields.
func UpdateArtifact(ctx context.Context, ws *Workspace, id string, updates map[string]any) (*models.Artifact, error) {
	if _, hasID := updates["id"]; hasID {
		return nil, fmt.Errorf("field %q is immutable and cannot be changed", "id")
	}

	artifact, err := findArtifact(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("find artifact %s: %w", id, err)
	}

	if v, ok := updates["title"].(string); ok {
		artifact.Title = v
	}
	if v, ok := updates["status"].(string); ok {
		artifact.Status = models.ArtifactStatus(v)
	}
	if v, ok := updates["description"].(string); ok {
		artifact.Description = v
	}
	if v, ok := updates["sprint"].(string); ok {
		artifact.Sprint = v
	}
	if v, ok := updates["priority"].(string); ok {
		artifact.Priority = v
	}
	if v, ok := updates["assigned_to"].(string); ok {
		artifact.AssignedTo = v
	}
	if v, ok := updates["owner"].(string); ok {
		artifact.Owner = v
	}
	if v, ok := updates["labels"].([]string); ok {
		artifact.Labels = v
	}
	if v, ok := updates["dependencies"].([]string); ok {
		artifact.Dependencies = v
	}
	if v, ok := updates["references"].([]string); ok {
		artifact.References = v
	}
	if v, ok := updates["commit"].(string); ok {
		artifact.Commit = v
	}
	if v, ok := updates["custom_fields"].(map[string]any); ok {
		artifact.CustomFields = v
	}
	artifact.UpdatedAt = time.Now()

	// Validate against header-def if available.
	if ws.HeaderDef != nil {
		if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
			return nil, fmt.Errorf("validate artifact fields: %w", err)
		}
	}

	if err := artifact.Validate(); err != nil {
		return nil, fmt.Errorf("validate artifact: %w", err)
	}

	return artifact, nil
}

// FindArtifactPath locates the Markdown file for an artifact by ID.
// It searches all non-hidden subdirectories of the workspace root, including
// status-based routing directories such as "archive" and "review".
// artifactSearchDirs returns the set of directories to search for artifact .md files.
// When ws.Config is available, it derives directories from artifact type names and
// registry routing rules, avoiding expensive scans of source directories like cmd/ or internal/.
// When ws.Config is nil (e.g., in bare test workspaces), it falls back to all non-hidden
// top-level directories under ws.RootPath.
func artifactSearchDirs(ws *Workspace) ([]string, error) {
	if ws.Config == nil {
		entries, err := os.ReadDir(ws.RootPath)
		if err != nil {
			return nil, fmt.Errorf("read workspace root: %w", err)
		}
		var dirs []string
		for _, entry := range entries {
			if !entry.IsDir() || len(entry.Name()) > 0 && entry.Name()[0] == '.' {
				continue
			}
			dirs = append(dirs, filepath.Join(ws.RootPath, entry.Name()))
		}
		return dirs, nil
	}

	seen := make(map[string]bool)
	var dirs []string
	addDir := func(rel string) {
		abs := filepath.Join(ws.RootPath, rel)
		if !seen[abs] {
			seen[abs] = true
			dirs = append(dirs, abs)
		}
	}

	// Artifact type directories are {type}s (e.g., tasks, bugs, stories).
	for artifactType := range ws.Config.ArtifactTypes {
		addDir(artifactType + "s")
	}

	// Registry-specified paths cover status-based relocations (e.g., archive, review).
	backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
	if registry, err := config.LoadRegistry(backlogitDir); err == nil {
		for _, rule := range registry.Directories {
			if rule.Path != "" {
				addDir(rule.Path)
			}
		}
	}

	// Include queue layout root directory if configured.
	if ws.Config != nil && ws.Config.QueueLayout != nil {
		addDir(ws.Config.QueueLayout.RootDir)
	}

	return dirs, nil
}

// Returns the absolute file path or an error if not found.
func FindArtifactPath(_ context.Context, ws *Workspace, id string) (string, error) {
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return "", err
	}

	for _, dirPath := range searchDirs {
		if _, statErr := os.Stat(dirPath); os.IsNotExist(statErr) {
			continue
		}
		var found string
		walkErr := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			a, _, parseErr := parseFile(path)
			if parseErr != nil {
				return nil
			}
			if a.ID == id {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if walkErr != nil {
			return "", fmt.Errorf("walk %s: %w", dirPath, walkErr)
		}
		if found != "" {
			return found, nil
		}
	}
	return "", fmt.Errorf("artifact not found: %s", id)
}

// WriteArtifactFile atomically writes an artifact to the given file path.
func WriteArtifactFile(artifact *models.Artifact, filePath string) error {
	fm := map[string]any{
		"id":            artifact.ID,
		"title":         artifact.Title,
		"status":        string(artifact.Status),
		"artifact_type": artifact.ArtifactType,
		"created_at":    artifact.CreatedAt,
		"updated_at":    artifact.UpdatedAt,
	}
	if artifact.ParentID != "" {
		fm["parent_id"] = artifact.ParentID
	}
	if artifact.Sprint != "" {
		fm["sprint"] = artifact.Sprint
	}
	if artifact.Priority != "" {
		fm["priority"] = artifact.Priority
	}
	if artifact.AssignedTo != "" {
		fm["assigned_to"] = artifact.AssignedTo
	}
	if artifact.Owner != "" {
		fm["owner"] = artifact.Owner
	}
	if len(artifact.Labels) > 0 {
		fm["labels"] = artifact.Labels
	}
	if len(artifact.Dependencies) > 0 {
		fm["dependencies"] = artifact.Dependencies
	}
	if len(artifact.References) > 0 {
		fm["references"] = artifact.References
	}
	if artifact.Commit != "" {
		fm["commit"] = artifact.Commit
	}
	if artifact.CustomFields != nil {
		fm["custom_fields"] = artifact.CustomFields
	}

	content := models.SerializeFrontmatter(fm, artifact.Description)
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("rename artifact file: %w", err)
	}
	return nil
}

func findArtifact(_ context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return nil, err
	}

	for _, dirPath := range searchDirs {
		if _, statErr := os.Stat(dirPath); os.IsNotExist(statErr) {
			continue
		}
		var found *models.Artifact
		walkErr := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			a, _, parseErr := parseFile(path)
			if parseErr != nil {
				return nil
			}
			if a.ID == id {
				found = a
				return filepath.SkipAll
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
		if found != nil {
			return found, nil
		}
	}
	return nil, fmt.Errorf("artifact not found: %s", id)
}

func parseFile(path string) (*models.Artifact, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, "", err
	}
	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, "", err
	}
	return artifact, body, nil
}
