package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
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

	// canonicalCache, when set via WithCanonicalCache, shares a single canonical
	// scan across a bulk-create batch instead of scanning once per create.
	canonicalCache *CanonicalCache
}

var createArtifactPostCreateFailureHook func(context.Context, *Workspace, *models.Artifact) error

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

// requireHeaderDef fails closed when the workspace header-def schema is not
// loaded. The create/update write paths gate required-field validation and
// default application on the header-def; an absent (nil) schema is a
// system/config precondition fault, so the write must refuse rather than
// silently skip validation and succeed — the same fail-open shape closed for
// the doctor --target path in 072-S. It is wrapped in blerrors.ErrConfig (NOT
// blerrors.ErrValidation): a missing workspace schema is not a user-correctable
// field error. ErrConfig is absent from domainError's validation case, so the
// MCP layer still surfaces it as `internal` (500), never `validation_failed`
// (422) — while giving callers/tests a positive errors.Is seam instead of a
// brittle message-substring match.
//
// This check is load-bearing at the call sites: it MUST run before
// ApplyFieldDefaults / ValidateArtifactFields, both of which call
// headerDef.ResolveFieldSchema, which dereferences its (now nil) receiver with
// no nil-guard and would nil-panic otherwise.
func requireHeaderDef(ws *Workspace) error {
	if ws.HeaderDef == nil {
		return fmt.Errorf("header definition not loaded; cannot validate artifact fields: %w", blerrors.ErrConfig)
	}
	return nil
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
	if err := rejectUnprovenancedReservedSize(o.Fields); err != nil {
		return nil, err
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

	// 066.002-T: Single pre-write chokepoint guarding canonical ID uniqueness.
	// The DB-backed allocators (NextID / NextTypedHierarchicalID) derive the next
	// ID from the SQLite index, which can lag the filesystem (e.g. a stale-index
	// window where an archived ordinal is no longer visible to the allocator).
	// Scan the full canonical artifactSearchDirs set once and fail loud if the
	// resolved ID already exists on disk, rather than letting a later write
	// silently overwrite a distinct artifact that shares the ID/filename.
	//
	// 070.001-T: bulk callers (migrate import loop, priority harvest) pass a
	// CanonicalCache via WithCanonicalCache so this O(files) scan runs once per
	// batch instead of once per create (avoiding the O(N^2) blowup on a large
	// backlog). Each successful create records its ID back into the cache below,
	// so within-batch collisions are still detected without a re-scan. Single
	// interactive creates pass no cache and scan per call, exactly as before.
	var canonical map[string][]artifactRef
	if o.canonicalCache != nil {
		// Seed a zero-value (externally constructed) cache on first use so an
		// unseeded refs map cannot bypass the uniqueness guard; a cache built via
		// NewCanonicalCache is already seeded and this is a no-op.
		if err := o.canonicalCache.ensureSeeded(ws); err != nil {
			return nil, fmt.Errorf("create artifact %q: canonical uniqueness scan: %w", artifactID, err)
		}
		canonical = o.canonicalCache.refs
	} else {
		scanned, scanErr := scanCanonicalArtifactsFn(ws)
		if scanErr != nil {
			return nil, fmt.Errorf("create artifact %q: canonical uniqueness scan: %w", artifactID, scanErr)
		}
		canonical = scanned
	}
	if existing := canonical[artifactID]; len(existing) > 0 {
		return nil, fmt.Errorf("create artifact %q: %w", artifactID, blerrors.ErrIDCollision)
	}

	status := o.Status
	if status == "" {
		status = "queued"
	}

	now := models.NowUTC()
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

	// A nil header-def means the workspace schema is absent, so required-field
	// validation and default application cannot be performed. Fail closed rather
	// than silently skip them and persist an unvalidated artifact. This check MUST
	// precede ApplyFieldDefaults/ValidateArtifactFields: both call
	// headerDef.ResolveFieldSchema, which dereferences the (now nil) receiver with
	// no nil-guard, so removing the old `if != nil` guard without failing closed
	// first would nil-pointer panic. The ordering is a load-bearing invariant.
	if err := requireHeaderDef(ws); err != nil {
		return nil, err
	}
	if err := ApplyFieldDefaults(artifact, ws.HeaderDef); err != nil {
		return nil, fmt.Errorf("apply field defaults: %w", err)
	}
	if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
		return nil, fmt.Errorf("validate artifact fields: %w", err)
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

	// 066.002-T (review hardening): defense-in-depth for the corruption case the
	// canonical scan above cannot see. scanCanonicalArtifacts keys on parsed
	// frontmatter IDs, so a file already sitting at this exact destination path
	// that is unparseable -- or whose frontmatter ID does not match its filename
	// -- would be skipped by that scan and then silently clobbered by the rename
	// below. Stat the concrete destination and fail loud if it is already
	// occupied, so a create never overwrites an existing canonical file.
	if _, statErr := os.Stat(filePath); statErr == nil {
		return nil, fmt.Errorf("create artifact %q: destination %q already exists: %w", artifactID, fileName+".md", blerrors.ErrIDCollision)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("create artifact %q: stat destination %q: %w", artifactID, fileName+".md", statErr)
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write artifact file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("rename artifact file: %w", err)
	}

	// 070.001-T: when this create is part of a batch, register the freshly
	// written ID into the shared cache so later creates in the same batch detect
	// a collision against it without re-scanning the filesystem.
	if o.canonicalCache != nil {
		o.canonicalCache.record(artifactID, filePath)
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
	if createArtifactPostCreateFailureHook != nil {
		if postCreateErr := createArtifactPostCreateFailureHook(ctx, ws, artifact); postCreateErr != nil {
			if rollbackErr := rollbackCreatedArtifactAfterPostCreateFailure(ctx, ws, artifact, filePath); rollbackErr != nil {
				return nil, fmt.Errorf("rollback artifact %s after post-create failure: %w", artifact.ID, rollbackErr)
			}
			return nil, fmt.Errorf("post-create size/provenance for artifact %s: %w", artifact.ID, postCreateErr)
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

// rollbackCreatedArtifactAfterPostCreateFailure removes the freshly-created
// artifact's Markdown file and SQLite index row so a post-create failure leaves
// no half-created artifact behind and the sequence ID is freed for a retry
// (108-F F6). It returns the first cleanup error, if any.
func rollbackCreatedArtifactAfterPostCreateFailure(ctx context.Context, ws *Workspace, artifact *models.Artifact, filePath string) error {
	var firstErr error
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		firstErr = fmt.Errorf("remove artifact file %s: %w", artifact.ID, err)
	}
	if ws.DB != nil {
		if err := db.DeleteItemCascade(ctx, ws.DB, artifact.ID); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete index row %s: %w", artifact.ID, err)
		}
	}
	return firstErr
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

// UpdateArtifact updates an existing artifact's fields. It is preserved for all
// existing callers and delegates to UpdateArtifactWithGate with no transition
// options, so the pre-task-completion gate (082-F) engages transparently for
// task/subtask completions whenever a workspace has a gate broker wired.
func UpdateArtifact(ctx context.Context, ws *Workspace, id string, updates map[string]any) (*models.Artifact, error) {
	artifact, _, err := UpdateArtifactWithGate(ctx, ws, id, updates, TransitionOptions{})
	return artifact, err
}

// updateArtifactUngated performs the field-apply/validate/persist/hook-fire body
// of an artifact update WITHOUT the pre-task-completion gate. It is the ungated
// core used both by non-gated transitions and, under the task lock, by the gated
// completion path once the gate decision resolves to a durable write.
func updateArtifactUngated(ctx context.Context, ws *Workspace, id string, updates map[string]any) (*models.Artifact, error) {
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
		artifact.CustomFields = mergePreserveReservedSizingKeys(artifact.CustomFields, v)
	}
	if v, ok := updates["harness_status"].(string); ok {
		if artifact.CustomFields == nil {
			artifact.CustomFields = map[string]any{}
		}
		artifact.CustomFields["harness_status"] = v
	}
	artifact.UpdatedAt = models.NowUTC()
	clearStaleBlockedReason(artifact, previousStatus)

	// Fail closed when the workspace schema is absent (see requireHeaderDef). This
	// check MUST precede ValidateArtifactFields, which dereferences the header-def
	// via ResolveFieldSchema with no nil-guard.
	if err := requireHeaderDef(ws); err != nil {
		return nil, err
	}
	if err := ValidateArtifactFields(artifact, ws.HeaderDef); err != nil {
		return nil, fmt.Errorf("validate artifact fields: %w", err)
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
			if guardErr := ensureArtifactLookupContained(ws, path); guardErr != nil {
				return guardErr
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

func ensureArtifactLookupContained(ws *Workspace, path string) error {
	absTarget, inScope, err := confineToStorageRoot(ws, path)
	if err != nil {
		return fmt.Errorf("resolve artifact path containment: %w", err)
	}
	if !inScope {
		return fmt.Errorf("artifact path %q resolves outside the workspace storage root: %w", absTarget, blerrors.ErrValidation)
	}
	return nil
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
	// Archive provenance is emitted only while the item is archived, keeping the
	// invariant "archive provenance <=> archived status". This preserves the
	// keys across an update round-trip on an archived item and omits stale keys
	// on any non-archived item.
	if artifact.Status == models.StatusArchived {
		if artifact.ArchivedFrom != "" {
			fm["archived_from"] = artifact.ArchivedFrom
		}
		if artifact.ArchivedStatus != "" {
			fm["archived_status"] = artifact.ArchivedStatus
		}
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
	source.UpdatedAt = models.NowUTC()
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
	source.UpdatedAt = models.NowUTC()
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

// GetLinks returns the outgoing semantic links for the given item ID, optionally
// filtered to a single linkType when it is non-empty. The result is normalized to
// a non-nil slice so every caller inherits the never-null guarantee (Rule 3):
// both the CLI `link list` command and the MCP handleGetLinks handler render an
// empty result as [] rather than null. Errors from the underlying db layer are
// wrapped with %w so callers can use errors.Is/As at their boundary.
func GetLinks(ctx context.Context, ws *Workspace, id, linkType string) ([]db.LinkEdge, error) {
	var (
		edges []db.LinkEdge
		err   error
	)
	if linkType != "" {
		edges, err = db.GetLinksByType(ctx, ws.DB, id, linkType)
	} else {
		edges, err = db.GetLinks(ctx, ws.DB, id)
	}
	if err != nil {
		return nil, fmt.Errorf("get links for %s: %w", id, err)
	}
	if edges == nil {
		edges = []db.LinkEdge{}
	}
	return edges, nil
}
