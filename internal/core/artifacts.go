package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/db"
	blerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/hooks"
	"github.com/backlogit/backlogit/internal/models"
)

// Option configures artifact creation.
type Option func(*createOptions)

type createOptions struct {
	ParentID     string
	Sprint       string
	Status       string
	Priority     string
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

// WithPriority sets the artifact priority.
func WithPriority(priority string) Option {
	return func(o *createOptions) { o.Priority = priority }
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
	if err := validateArtifactParent(ctx, ws, artifactType, o.ParentID); err != nil {
		return nil, err
	}

	// Fire pre-create hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ArtifactType: artifactType,
			NewValues:    map[string]any{"title": title, "artifact_type": artifactType},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if o.ParentID != "" {
			hookCtx.NewValues["parent_id"] = o.ParentID
		}
		if o.Status != "" {
			hookCtx.NewValues["status"] = o.Status
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookCreateArtifact, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-create hook: %w", err)
		}
	}

	artifactID := ""
	if ws.Config.QueueLayout != nil {
		if _, levelErr := LevelForType(ws.Config.QueueLayout, artifactType); levelErr == nil {
			nextTypedID, idErr := NextTypedHierarchicalID(
				ctx,
				ws.DB,
				o.ParentID,
				artifactType,
				typeConfig,
				ws.Config.QueueLayout,
			)
			if idErr != nil {
				return nil, fmt.Errorf("get next hierarchical id: %w", idErr)
			}
			artifactID = nextTypedID
		}
	}
	if artifactID == "" {
		nextID, err := NextID(ctx, ws.DB, artifactType, typeConfig)
		if err != nil {
			return nil, fmt.Errorf("get next id: %w", err)
		}
		artifactID = ResolveName(typeConfig, title, nextID, ws.Config.MaxSlugLength)
	}

	status := o.Status
	if status == "" {
		status = "queued"
	}

	now := time.Now()
	artifact := &models.Artifact{
		ID:           artifactID,
		Title:        title,
		Status:       models.ArtifactStatus(status),
		ArtifactType: artifactType,
		ParentID:     o.ParentID,
		Sprint:       o.Sprint,
		Priority:     o.Priority,
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

	backlogitDir := WorkspaceStorageRoot(ws.RootPath)
	var dir string
	if registry, regErr := config.LoadRegistry(backlogitDir); regErr == nil {
		dir = ResolveTargetDir(registry, artifactType, status)
	}
	if ws.Config.QueueLayout != nil {
		if _, levelErr := LevelForType(ws.Config.QueueLayout, artifactType); levelErr == nil {
			hierPath, hierErr := ResolveHierarchicalPath(ws.Config.QueueLayout, o.ParentID, artifactType)
			if hierErr == nil && dir == "" {
				dir = hierPath
			}
			level, _ := LevelForType(ws.Config.QueueLayout, artifactType)
			artifact.Level = level
		}
	}
	if dir == "" {
		dir = "queue"
	}
	dirAbs := filepath.Join(backlogitDir, dir)
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
	content := models.SerializeFrontmatter(fm, artifact.Description)
	fileName := ResolveFileName(typeConfig, artifact.ID, title, ws.Config.MaxSlugLength)
	filePath := filepath.Join(dirAbs, fileName+".md")

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write artifact file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("rename artifact file: %w", err)
	}

	// Upsert to the SQLite index so that NextID and query-based callers see
	// the new artifact immediately without requiring an explicit rehydration.
	if ws.DB != nil {
		if upsertErr := db.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
			// Remove the file we just wrote so we don't leave an orphaned artifact
			// on disk that cannot be found via the DB index.
			os.Remove(filePath)
			return nil, fmt.Errorf("index artifact %s: %w", artifact.ID, upsertErr)
		}
	}

	// Fire post-create hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       artifact.ID,
			ArtifactType: artifactType,
			NewValues: map[string]any{
				"id":     artifact.ID,
				"title":  artifact.Title,
				"status": string(artifact.Status),
			},
			Actor:     "backlogit",
			Workspace: ws.RootPath,
			TopLevel:  true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookCreateArtifact, hookCtx)
	}

	return artifact, nil
}

func validateArtifactParent(ctx context.Context, ws *Workspace, artifactType string, parentID string) error {
	// Enforce hierarchy: level-2+ artifact types require parent_id.
	// Prefer QueueLayout for level lookup; fall back to allowedChildren membership
	// so enforcement applies even when QueueLayout is not configured.
	if parentID == "" && ws.Config != nil {
		if ws.Config.QueueLayout != nil {
			if level, levelErr := LevelForType(ws.Config.QueueLayout, artifactType); levelErr == nil && level >= 2 {
				return fmt.Errorf("artifact type %q requires parent_id: %w", artifactType, blerrors.ErrValidation)
			}
		} else if len(allowedParentTypes(ws, artifactType)) > 0 {
			// No QueueLayout configured: infer child status from allowedChildren membership.
			return fmt.Errorf("artifact type %q requires parent_id: %w", artifactType, blerrors.ErrValidation)
		}
	}
	allowedParents := allowedParentTypes(ws, artifactType)
	if parentID == "" {
		if artifactType == "review" && len(allowedParents) > 0 {
			return fmt.Errorf("artifact type %q requires a parent_id", artifactType)
		}
		return nil
	}
	if len(allowedParents) == 0 {
		return nil
	}

	parentPath, err := FindArtifactPath(ctx, ws, parentID)
	if err != nil {
		return fmt.Errorf("find parent artifact %q: %w", parentID, err)
	}
	parentArtifact, _, err := parseFile(parentPath)
	if err != nil {
		return fmt.Errorf("parse parent artifact %q: %w", parentID, err)
	}

	if _, ok := ws.Config.ArtifactTypes[parentArtifact.ArtifactType]; !ok {
		return fmt.Errorf("parent artifact type %q is not configured", parentArtifact.ArtifactType)
	}
	if _, ok := allowedParents[parentArtifact.ArtifactType]; ok {
		return nil
	}

	return fmt.Errorf("artifact type %q is not allowed under parent type %q", artifactType, parentArtifact.ArtifactType)
}

func allowedParentTypes(ws *Workspace, artifactType string) map[string]struct{} {
	allowed := map[string]struct{}{}
	if ws == nil || ws.Config == nil {
		return allowed
	}
	for parentType, parentCfg := range ws.Config.ArtifactTypes {
		if parentCfg == nil {
			continue
		}
		for _, allowedChild := range parentCfg.AllowedChildren {
			if allowedChild == artifactType {
				allowed[parentType] = struct{}{}
				break
			}
		}
	}
	if artifactType == "bug" {
		switch ws.Config.BugLevel {
		case 2:
			allowed["feature"] = struct{}{}
		case 3:
			allowed["task"] = struct{}{}
		}
	}
	return allowed
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
	previousStatus := artifact.Status
	previousTitle := artifact.Title

	// Fire pre-update hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       id,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"status": string(artifact.Status), "title": previousTitle},
			NewValues:    updates,
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookUpdateArtifact, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-update hook: %w", err)
		}
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
	if v, ok := updates["links"].([]models.ArtifactLink); ok {
		artifact.Links = v
	}
	if v, ok := updates["references"].([]string); ok {
		artifact.References = v
	}
	if v, ok := updates["commit"].(string); ok {
		artifact.Commit = v
	}
	if v, ok := updates["parent_id"].(string); ok {
		artifact.ParentID = v
	}
	if v, ok := updates["custom_fields"].(map[string]any); ok {
		artifact.CustomFields = v
	}
	if v, ok := updates["harness_status"].(string); ok {
		if artifact.CustomFields == nil {
			artifact.CustomFields = map[string]any{}
		}
		artifact.CustomFields["harness_status"] = v
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

	if err := persistArtifact(ctx, ws, artifact, shouldRelocateOnStatusChange(previousStatus, artifact.Status)); err != nil {
		return nil, fmt.Errorf("persist artifact %s: %w", id, err)
	}

	// Fire post-update hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       id,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"status": string(previousStatus), "title": previousTitle},
			NewValues:    updates,
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookUpdateArtifact, hookCtx)
	}

	return artifact, nil
}

// FindArtifactPath locates the Markdown file for an artifact by ID.
// It searches all non-hidden subdirectories of the .backlogit workspace, including
// status-based routing directories such as "archive" and "review".
// artifactSearchDirs returns the set of directories to search for artifact .md files.
// When ws.Config is available, it derives directories from artifact type names and
// registry routing rules, avoiding expensive scans of unrelated repository directories.
// When ws.Config is nil (e.g., in bare test workspaces), it falls back to all non-hidden
// top-level directories under .backlogit.
func artifactSearchDirs(ws *Workspace) ([]string, error) {
	backlogitDir := WorkspaceStorageRoot(ws.RootPath)

	if ws.Config == nil {
		// No config loaded: scan all non-hidden dirs under .backlogit.
		var dirs []string
		entries, err := os.ReadDir(backlogitDir)
		if err != nil {
			return nil, fmt.Errorf("read workspace storage root: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || len(entry.Name()) > 0 && entry.Name()[0] == '.' {
				continue
			}
			dirs = append(dirs, filepath.Join(backlogitDir, entry.Name()))
		}
		return dirs, nil
	}

	seen := make(map[string]bool)
	var dirs []string
	addDir := func(rel string) {
		abs := filepath.Join(backlogitDir, rel)
		if !seen[abs] {
			seen[abs] = true
			dirs = append(dirs, abs)
		}
	}

	// Registry-specified paths cover status-based relocations (e.g., archive, review).
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
	} else {
		addDir("queue")
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
	return "", fmt.Errorf("artifact not found: %s: %w", id, blerrors.ErrNotFound)
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
	if len(artifact.Links) > 0 {
		fm["links"] = artifact.Links
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
	return nil, fmt.Errorf("artifact not found: %s: %w", id, blerrors.ErrNotFound)
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

// AddArtifactLink persists an outgoing semantic link to Markdown first and then the SQLite cache.
func AddArtifactLink(ctx context.Context, ws *Workspace, sourceID, targetID, linkType string) error {
	valid := false
	for _, allowed := range db.ValidLinkTypes {
		if allowed == linkType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: %q", blerrors.ErrInvalidLinkType, linkType)
	}

	source, err := findArtifact(ctx, ws, sourceID)
	if err != nil {
		return fmt.Errorf("find source artifact %s: %w", sourceID, err)
	}
	if _, err := loadArtifact(ctx, ws, targetID); err != nil {
		return fmt.Errorf("load target artifact %s: %w", targetID, err)
	}
	for _, link := range source.Links {
		if link.TargetID == targetID && link.LinkType == linkType {
			return nil
		}
	}

	source.Links = append(source.Links, models.ArtifactLink{
		TargetID: targetID,
		LinkType: linkType,
	})
	source.UpdatedAt = time.Now()
	if err := persistArtifact(ctx, ws, source, false); err != nil {
		return fmt.Errorf("persist source artifact %s: %w", sourceID, err)
	}
	// SQLite cache update is best-effort: the Markdown write above is authoritative.
	// A cache miss here is self-healing on the next rehydration cycle.
	if err := db.AddLink(ctx, ws.DB, sourceID, targetID, linkType); err != nil {
		slog.Warn("link cache update failed; rehydration will recover",
			"op", "add_link", "source", sourceID, "target", targetID, "type", linkType, "error", err)
	}
	return nil
}

// RemoveArtifactLink removes an outgoing semantic link from Markdown first and then the SQLite cache.
func RemoveArtifactLink(ctx context.Context, ws *Workspace, sourceID, targetID, linkType string) error {
	source, err := findArtifact(ctx, ws, sourceID)
	if err != nil {
		return fmt.Errorf("find source artifact %s: %w", sourceID, err)
	}
	filtered := source.Links[:0]
	removed := false
	for _, link := range source.Links {
		if link.TargetID == targetID && link.LinkType == linkType {
			removed = true
			continue
		}
		filtered = append(filtered, link)
	}
	if !removed {
		return nil
	}

	if len(filtered) == 0 {
		source.Links = nil
	} else {
		source.Links = filtered
	}
	source.UpdatedAt = time.Now()
	if err := persistArtifact(ctx, ws, source, false); err != nil {
		return fmt.Errorf("persist source artifact %s: %w", sourceID, err)
	}
	// SQLite cache update is best-effort: the Markdown write above is authoritative.
	// A cache miss here is self-healing on the next rehydration cycle.
	if err := db.RemoveLink(ctx, ws.DB, sourceID, targetID, linkType); err != nil {
		slog.Warn("link cache update failed; rehydration will recover",
			"op", "remove_link", "source", sourceID, "target", targetID, "type", linkType, "error", err)
	}
	return nil
}
