package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/parser"
)

func newMigrateCommand(cwd *string) *cobra.Command {
	var dryRun, rollback, detect, validate, workspaceDir bool
	var source, adapter, format string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate backlog data between supported formats and layouts",
		Long: `Migrate backlog data either from supported source adapters such as backlog-md
or from older internal workspace layouts.

Use --source with --adapter backlog-md for source imports. Use --dry-run and
--validate before writing imported artifacts. Use --rollback only for internal
layout migrations, not source imports. Use --workspace-dir to rename a legacy
.backlogit workspace directory to the new .backlog default.`,
		Example: `  backlogit migrate --source .\.backlog --adapter backlog-md --dry-run
  backlogit migrate --source .\.backlog --adapter backlog-md --validate
  backlogit migrate --source .\.backlog --adapter backlog-md
  backlogit migrate --workspace-dir --dry-run
  backlogit migrate --workspace-dir
  backlogit migrate --dry-run
  backlogit migrate --rollback`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if workspaceDir {
				if source != "" || adapter != "" || detect || validate || rollback {
					return fmt.Errorf("--workspace-dir cannot be combined with source-import or rollback flags")
				}

				result, err := core.MigrateWorkspaceDir(*cwd, core.MigrateWorkspaceDirOptions{DryRun: dryRun})
				if err != nil {
					return fmt.Errorf("migrate workspace dir: %w", err)
				}

				switch {
				case result.AlreadyDone:
					fmt.Fprintf(cmd.OutOrStdout(), "Workspace directory already uses %s\n", filepath.Base(result.Destination))
				case len(result.Files) == 0:
					fmt.Fprintln(cmd.OutOrStdout(), "No legacy .backlogit workspace directory found")
				case result.DryRun:
					fmt.Fprintf(cmd.OutOrStdout(), "Dry run: workspace directory would move from %s to %s\n", result.Source, result.Destination)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "Migrated workspace directory from %s to %s\n", result.Source, result.Destination)
				}
				return nil
			}

			if source != "" || adapter != "" || detect || validate {
				if rollback {
					return fmt.Errorf("--rollback cannot be combined with source-import flags")
				}
				return runSourceMigration(cmd, *cwd, source, adapter, dryRun, detect, validate, format)
			}

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if rollback {
				if err := core.RollbackMigration(ws); err != nil {
					return fmt.Errorf("rollback: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Migration rolled back")
				return nil
			}

			report, err := core.MigrateFlatToHierarchical(ws, dryRun)
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %d files would move, %d skipped\n",
					report.FilesMoved, report.FilesSkipped)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Migrated %d files, %d skipped\n",
					report.FilesMoved, report.FilesSkipped)
				count, rehydErr := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
				if rehydErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: rehydration failed: %v\n", rehydErr)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Rehydrated %d artifacts\n", count)
				}
			}

			for _, e := range report.Errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", e)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without moving files")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "reverse a previous migration")
	cmd.Flags().BoolVar(&workspaceDir, "workspace-dir", false, "rename a legacy .backlogit workspace directory to .backlog")
	cmd.Flags().StringVar(&source, "source", "", "path to source workspace or file to import")
	cmd.Flags().StringVar(&adapter, "adapter", "", "migration adapter to use (for example: backlog-md)")
	cmd.Flags().BoolVar(&detect, "detect", false, "detect the adapter for the source and print it")
	cmd.Flags().BoolVar(&validate, "validate", false, "validate the source import without writing artifacts")
	cmd.Flags().StringVar(&format, "format", "text", "report format: text or json")
	return cmd
}

func runSourceMigration(cmd *cobra.Command, cwd string, source string, adapter string, dryRun bool, detect bool, validate bool, format string) error {
	ctx := context.Background()
	sourcePath, err := resolveImportSourcePath(cwd, source)
	if err != nil {
		return err
	}

	if detect {
		detected, err := parser.DetectAdapter(sourcePath)
		if err != nil {
			return fmt.Errorf("detect adapter: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), detected.Name())
		return nil
	}

	ws, err := core.NewWorkspace(ctx, cwd)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}
	defer ws.Close()

	report, err := parser.MigrateWithOptions(ctx, sourcePath, parser.MigrateOptions{
		DryRun:   dryRun,
		Validate: validate,
		Adapter:  adapter,
		Format:   format,
	})
	if err != nil {
		return fmt.Errorf("parse source migration: %w", err)
	}

	if migrationCfg, cfgErr := config.LoadMigrationConfig(core.WorkspaceStorageRoot(cwd)); cfgErr == nil {
		applyMigrationConfig(sourcePath, report.Items, migrationCfg)
	}
	stampMigrationProvenance(report.Items)

	applyValidation(ws, report)

	formatted, err := parser.FormatReport(report, format)
	if err != nil {
		return fmt.Errorf("format migration report: %w", err)
	}

	if dryRun || validate {
		fmt.Fprint(cmd.OutOrStdout(), formatted)
		if validate && report.ItemsFailed > 0 {
			return fmt.Errorf("migration validation failed")
		}
		return nil
	}

	imported, err := importMigrationItems(ctx, ws, report.Items)
	if err != nil {
		return err
	}

	if imported.Errors != nil {
		for _, importErr := range imported.Errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", importErr)
		}
	}

	count, rehydErr := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	if rehydErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: rehydration failed: %v\n", rehydErr)
	}

	fmt.Fprint(cmd.OutOrStdout(), formatted)
	fmt.Fprintf(cmd.OutOrStdout(), "Imported %d artifacts, skipped %d, failed %d\n", imported.Imported, imported.Skipped, imported.Failed)
	if rehydErr == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Rehydrated %d artifacts\n", count)
	}
	return nil
}

func resolveImportSourcePath(cwd string, source string) (string, error) {
	if source != "" {
		if filepath.IsAbs(source) {
			return source, nil
		}
		return filepath.Join(cwd, source), nil
	}

	for _, candidate := range []string{"backlog", ".backlog", "Backlog.md", "backlog.md"} {
		resolved := filepath.Join(cwd, candidate)
		if _, err := filepath.Abs(resolved); err == nil {
			if _, statErr := os.Stat(resolved); statErr == nil {
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("could not determine source path; pass --source explicitly")
}

type migrationImportResult struct {
	Imported int
	Skipped  int
	Failed   int
	Errors   []string
}

// importedArtifactRef records a previously imported artifact's minted ID along
// with whether it is already archived. The archived flag lets the import loop
// resume an interrupted archived-source import (create succeeded but ArchiveItem
// failed on a prior run) instead of skipping it permanently.
type importedArtifactRef struct {
	id       string
	archived bool
}

type existingImportIndex map[string]importedArtifactRef

func applyValidation(ws *core.Workspace, report *parser.MigrationReport) {
	for _, item := range report.Items {
		if strings.TrimSpace(item.Title) == "" {
			report.ItemsFailed++
			report.Errors = append(report.Errors, fmt.Sprintf("source %s: missing title", item.SourcePath))
			continue
		}

		targetType := item.ArtifactType
		if targetType == "" {
			targetType = "task"
		}
		if _, ok := ws.Config.ArtifactTypes[targetType]; ok {
			continue
		}
		if _, fallback := ws.Config.ArtifactTypes["task"]; fallback {
			continue
		}

		report.ItemsFailed++
		report.Errors = append(report.Errors, fmt.Sprintf("source %s: unsupported target artifact type %q", item.SourcePath, targetType))
	}
}

func importMigrationItems(ctx context.Context, ws *core.Workspace, items []parser.MigrationItem) (*migrationImportResult, error) {
	result := &migrationImportResult{}
	idMap, err := buildExistingImportIndex(ctx, ws)
	if err != nil {
		return nil, err
	}

	sorted := append([]parser.MigrationItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Depth == sorted[j].Depth {
			return sorted[i].SourceID < sorted[j].SourceID
		}
		return sorted[i].Depth < sorted[j].Depth
	})

	// 070.001-T: scan the canonical artifact set once for the whole import batch
	// and share it across every CreateArtifact call, instead of re-walking
	// queue+archive on each imported item (O(files) per create -> O(N^2) on a
	// large backlog). Each create records its minted ID back into the cache, so
	// within-batch ID collisions are still detected without a re-scan.
	var canonicalCache *core.CanonicalCache
	if len(sorted) > 0 {
		canonicalCache, err = core.NewCanonicalCache(ws)
		if err != nil {
			return nil, fmt.Errorf("prepare import canonical cache: %w", err)
		}
	}

	for _, item := range sorted {
		targetType := item.ArtifactType
		if targetType == "" {
			targetType = "task"
		}

		fields := cloneMigrationFields(item.Fields)
		if _, ok := ws.Config.ArtifactTypes[targetType]; !ok {
			if _, fallback := ws.Config.ArtifactTypes["task"]; fallback {
				fields["backlog_md_target_type"] = targetType
				targetType = "task"
			} else {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("skip %s: unsupported target type %q", item.SourcePath, item.ArtifactType))
				continue
			}
		}

		// Archived source items cannot be created directly: a born-archived
		// artifact has no provenance and is non-invertible, which CreateArtifact
		// and the WriteArtifactFile boundary now reject. Import archived items in
		// a restorable terminal status ("done") and then route them through
		// ArchiveItem below, which stamps archived_from/archived_status from the
		// raw frontmatter and moves the file into archive/.
		archivedImport := item.Status == "archived"
		createStatus := item.Status
		if archivedImport {
			createStatus = "done"
		}

		opts := []core.Option{
			core.WithStatus(createStatus),
			core.WithDescription(item.Body),
			core.WithFields(fields),
		}
		if canonicalCache != nil {
			opts = append(opts, core.WithCanonicalCache(canonicalCache))
		}
		if item.AssignedTo != "" {
			opts = append(opts, core.WithAssignedTo(item.AssignedTo))
		}
		if item.Priority != "" {
			fields["backlog_md_priority"] = item.Priority
		}
		if len(item.Tags) > 0 {
			opts = append(opts, core.WithLabels(item.Tags))
		}
		if len(item.References) > 0 {
			opts = append(opts, core.WithReferences(item.References))
		}
		if item.SprintGroup != "" {
			opts = append(opts, core.WithSprint(item.SprintGroup))
		}
		if item.ParentRef != "" {
			if mappedParent, ok := idMap[legacyImportIdentity("", item.ParentRef)]; ok {
				opts = append(opts, core.WithParent(mappedParent.id))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("parent %q for %s not imported yet; creating without parent link", item.ParentRef, item.SourcePath))
			}
		}

		identity := importIdentity(item)
		if existing, ok := idMap[identity]; ok {
			if item.SourceID != "" {
				idMap[legacyImportIdentity("", item.SourceID)] = existing
			}
			// Resume an interrupted archived-source import: a prior run created
			// the artifact (durably writing backlog_md_source_path) but its
			// ArchiveItem step failed, so buildExistingImportIndex now finds a
			// non-archived artifact for an archived source. Skipping here would
			// leave it permanently unarchived; instead re-run ArchiveItem so the
			// import becomes idempotent and retryable after a transient failure.
			if archivedImport && !existing.archived {
				// The index can lag the filesystem: a prior run may have archived
				// the file on disk (moving it into archive/ and stamping
				// provenance) yet crashed before persisting status="archived" to
				// the DB. Re-running ArchiveItem on an already-archived file would
				// derive oldStatus=="archived" and overwrite archived_status,
				// corrupting the restore target. Inspect the on-disk location and
				// treat an already-archived file as complete; the trailing
				// Rehydrate reconciles the stale index.
				onDiskPath, pathErr := core.FindArtifactPath(ctx, ws, existing.id)
				if pathErr != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("locate imported item %s: %v", existing.id, pathErr))
					continue
				}
				if isArchivedArtifactPath(ws, onDiskPath) {
					result.Skipped++
					continue
				}
				if _, archErr := core.ArchiveItem(ctx, ws.DB, ws, existing.id); archErr != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("archive imported item %s: %v", existing.id, archErr))
					continue
				}
				idMap[identity] = importedArtifactRef{id: existing.id, archived: true}
				if item.SourceID != "" {
					idMap[legacyImportIdentity("", item.SourceID)] = importedArtifactRef{id: existing.id, archived: true}
				}
				result.Imported++
				continue
			}
			result.Skipped++
			continue
		}

		artifact, err := core.CreateArtifact(ctx, ws, item.Title, targetType, opts...)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("create artifact for %s: %v", item.SourcePath, err))
			continue
		}
		// Skip status-directed relocation for archived imports: the item was
		// created in a restorable status and ArchiveItem (below) performs the
		// final move into archive/ while stamping provenance, so relocating here
		// first would only add a redundant move and a non-canonical archived_from.
		if !archivedImport {
			if relocatedPath, relocateErr := core.RelocateArtifactFile(ctx, ws, artifact.ArtifactType, artifact.ID, string(artifact.Status)); relocateErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("relocate artifact %s: %v", artifact.ID, relocateErr))
			} else if relocateErr == nil && relocatedPath != "" {
				if writeErr := core.WriteArtifactFile(artifact, relocatedPath); writeErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("rewrite relocated artifact %s: %v", artifact.ID, writeErr))
				}
			}
		}
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("index artifact %s: %v", artifact.ID, err))
			continue
		}
		// Archive imported archived items through ArchiveItem so archive
		// provenance (archived_from/archived_status) is stamped and the record
		// remains invertible via UnarchiveItem.
		if archivedImport {
			if _, archErr := core.ArchiveItem(ctx, ws.DB, ws, artifact.ID); archErr != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("archive imported item %s: %v", artifact.ID, archErr))
				continue
			}
		}

		if item.SourceID != "" {
			idMap[legacyImportIdentity("", item.SourceID)] = importedArtifactRef{id: artifact.ID, archived: archivedImport}
		}
		idMap[identity] = importedArtifactRef{id: artifact.ID, archived: archivedImport}
		result.Imported++
	}

	for _, item := range sorted {
		if len(item.Dependencies) == 0 || item.SourceID == "" {
			continue
		}
		newRef, ok := idMap[legacyImportIdentity("", item.SourceID)]
		if !ok {
			continue
		}
		newID := newRef.id

		mappedDeps := make([]models.DependencyEdge, 0, len(item.Dependencies))
		for _, dep := range item.Dependencies {
			if mapped, ok := idMap[legacyImportIdentity("", dep)]; ok {
				mappedDeps = append(mappedDeps, models.DependencyEdge{ID: mapped.id, Type: "blocks"})
			}
		}
		if len(mappedDeps) == 0 {
			continue
		}

		artifact, err := core.UpdateArtifact(ctx, ws, newID, map[string]any{"dependencies": mappedDeps})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update dependencies for %s: %v", newID, err))
			continue
		}
		artifactPath, err := core.FindArtifactPath(ctx, ws, newID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("find artifact path for %s: %v", newID, err))
			continue
		}
		if err := core.WriteArtifactFile(artifact, artifactPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write artifact file for %s: %v", newID, err))
			continue
		}
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("index dependencies for %s: %v", newID, err))
		}
	}

	return result, nil
}

func buildExistingImportIndex(ctx context.Context, ws *core.Workspace) (existingImportIndex, error) {
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{IncludeArchived: true})
	if err != nil {
		return nil, fmt.Errorf("query existing imported artifacts: %w", err)
	}

	index := make(existingImportIndex)
	for _, item := range items {
		if item == nil || item.CustomFields == nil {
			continue
		}

		sourcePath, _ := item.CustomFields["backlog_md_source_path"].(string)
		sourceID, _ := item.CustomFields["backlog_md_id"].(string)
		if sourcePath == "" {
			continue
		}

		index[legacyImportIdentity(sourcePath, sourceID)] = importedArtifactRef{id: item.ID, archived: string(item.Status) == "archived"}
		if sourceID != "" {
			index[legacyImportIdentity("", sourceID)] = importedArtifactRef{id: item.ID, archived: string(item.Status) == "archived"}
		}
	}
	return index, nil
}

func stampMigrationProvenance(items []parser.MigrationItem) {
	for i := range items {
		if items[i].Fields == nil {
			items[i].Fields = map[string]any{}
		}
		items[i].Fields["backlog_md_source_path"] = filepath.ToSlash(items[i].SourcePath)
		if items[i].SourceID != "" {
			items[i].Fields["backlog_md_id"] = items[i].SourceID
		}
	}
}

func importIdentity(item parser.MigrationItem) string {
	return legacyImportIdentity(filepath.ToSlash(item.SourcePath), item.SourceID)
}

// isArchivedArtifactPath reports whether path resolves inside the workspace
// archive directory (.backlogit/archive). It is used to detect an artifact that
// is already archived on disk even when the SQLite index still reports it as
// non-archived, so an interrupted-archive resume does not re-archive a
// completed file and corrupt its archived_status provenance.
func isArchivedArtifactPath(ws *core.Workspace, path string) bool {
	archiveDir := filepath.Clean(filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "archive"))
	cleaned := filepath.Clean(path)
	if cleaned == archiveDir {
		return false
	}
	rel, err := filepath.Rel(archiveDir, cleaned)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func legacyImportIdentity(sourcePath, sourceID string) string {
	return filepath.ToSlash(sourcePath) + "::" + sourceID
}

func cloneMigrationFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}

func applyMigrationConfig(sourcePath string, items []parser.MigrationItem, cfg *config.MigrationConfig) {
	if cfg == nil {
		return
	}

	structuredRoot, _ := resolveStructuredSourceRoot(sourcePath)
	for i := range items {
		if items[i].SourceID != "" {
			continue
		}

		rel := filepath.Base(items[i].SourcePath)
		if structuredRoot != "" {
			if computed, err := filepath.Rel(structuredRoot, items[i].SourcePath); err == nil {
				rel = filepath.ToSlash(computed)
			}
		} else if dir, ok := items[i].Metadata["source_dir"]; ok && dir != "" {
			rel = filepath.ToSlash(filepath.Join(dir, filepath.Base(items[i].SourcePath)))
		}

		className := cfg.MatchClass(rel)
		if className == "" {
			continue
		}
		artifactType, err := cfg.ResolveArtifactType(className)
		if err == nil && artifactType != "" {
			items[i].ArtifactType = artifactType
		}
	}
}

func resolveStructuredSourceRoot(sourcePath string) (string, bool) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return sourcePath, true
	}
	return filepath.Dir(sourcePath), false
}
