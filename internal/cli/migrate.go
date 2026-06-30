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
	"github.com/softwaresalt/backlogit/internal/parser"
)

func newMigrateCommand(cwd *string) *cobra.Command {
	var dryRun, rollback, detect, validate bool
	var source, adapter, format string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate backlog data between supported formats and layouts",
		Long: `Migrate backlog data either from supported source adapters such as backlog-md
or from older internal workspace layouts.

Use --source with --adapter backlog-md for source imports. Use --dry-run and
--validate before writing imported artifacts. Use --rollback only for internal
layout migrations, not source imports.`,
		Example: `  backlogit migrate --source .\.backlog --adapter backlog-md --dry-run
  backlogit migrate --source .\.backlog --adapter backlog-md --validate
  backlogit migrate --source .\.backlog --adapter backlog-md
  backlogit migrate --dry-run
  backlogit migrate --rollback`,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

	if migrationCfg, cfgErr := config.LoadMigrationConfig(filepath.Join(cwd, ".backlogit")); cfgErr == nil {
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

type existingImportIndex map[string]string

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

		opts := []core.Option{
			core.WithStatus(item.Status),
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
				opts = append(opts, core.WithParent(mappedParent))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("parent %q for %s not imported yet; creating without parent link", item.ParentRef, item.SourcePath))
			}
		}

		identity := importIdentity(item)
		if existingID, ok := idMap[identity]; ok {
			if item.SourceID != "" {
				idMap[legacyImportIdentity("", item.SourceID)] = existingID
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
		if relocatedPath, relocateErr := core.RelocateArtifactFile(ctx, ws, artifact.ArtifactType, artifact.ID, string(artifact.Status)); relocateErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("relocate artifact %s: %v", artifact.ID, relocateErr))
		} else if relocateErr == nil && relocatedPath != "" {
			if writeErr := core.WriteArtifactFile(artifact, relocatedPath); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("rewrite relocated artifact %s: %v", artifact.ID, writeErr))
			}
		}
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("index artifact %s: %v", artifact.ID, err))
			continue
		}

		if item.SourceID != "" {
			idMap[legacyImportIdentity("", item.SourceID)] = artifact.ID
		}
		idMap[identity] = artifact.ID
		result.Imported++
	}

	for _, item := range sorted {
		if len(item.Dependencies) == 0 || item.SourceID == "" {
			continue
		}
		newID, ok := idMap[legacyImportIdentity("", item.SourceID)]
		if !ok {
			continue
		}

		mappedDeps := make([]string, 0, len(item.Dependencies))
		for _, dep := range item.Dependencies {
			if mapped, ok := idMap[legacyImportIdentity("", dep)]; ok {
				mappedDeps = append(mappedDeps, mapped)
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

		index[legacyImportIdentity(sourcePath, sourceID)] = item.ID
		if sourceID != "" {
			index[legacyImportIdentity("", sourceID)] = item.ID
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
