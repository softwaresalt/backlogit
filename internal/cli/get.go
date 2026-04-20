package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/parser"
)

// newGetCommand creates the `backlogit get` command.
func newGetCommand(cwd *string) *cobra.Command {
	var (
		jsonOutput bool
		format     string
		section    string
	)

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve an artifact by ID",
		Long: `Retrieve a single artifact and print either a human-readable detail view,
JSON output, or a specific named body section.`,
		Example: `  backlogit get 001-F
  backlogit get 001-F --format json
  backlogit get 001-F --section description`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			filePath, err := core.FindArtifactPath(ctx, ws, id)
			if err != nil {
				return err
			}

			raw, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read artifact file: %w", err)
			}

			fm, body, err := models.ParseFrontmatter(string(raw))
			if err != nil {
				return fmt.Errorf("parse artifact: %w", err)
			}

			if section != "" {
				sections, parseErr := parser.ParseSections(body)
				if parseErr != nil {
					return parseErr
				}
				content, ok := sections[section]
				if !ok {
					return fmt.Errorf("section %q not found in artifact %s", section, id)
				}
				fmt.Fprintln(cmd.OutOrStdout(), content)
				return nil
			}

			useJSON := jsonOutput || format == "json"
			if !useJSON && format != "" && format != "table" {
				return fmt.Errorf("--format %q is not supported on 'get': allowed values are table, json", format)
			}

			if useJSON {
				detail := buildDetailMap(ctx, ws, fm, body, id)
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(detail)
			}

			return printDetailView(cmd, fm, body, ctx, ws, id)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output frontmatter as JSON")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, json")
	cmd.Flags().StringVar(&section, "section", "", "extract a named section from the body")
	return cmd
}

// buildDetailMap constructs a combined map of frontmatter, body, dependencies, and commits.
func buildDetailMap(ctx context.Context, ws *core.Workspace, fm map[string]any, body, id string) map[string]any {
	detail := make(map[string]any, len(fm)+3)
	for k, v := range fm {
		detail[k] = v
	}
	if body != "" {
		detail["body"] = strings.TrimSpace(body)
	}

	deps, err := db.GetDependencies(ctx, ws.DB, id)
	if err == nil && len(deps) > 0 {
		detail["dependencies_detail"] = deps
	}

	commits, err := core.GetCommitLinks(ctx, ws.DB, id)
	if err == nil && len(commits) > 0 {
		detail["commit_links"] = commits
	}
	return detail
}

// printDetailView writes a human-readable detail view to the command output.
func printDetailView(cmd *cobra.Command, fm map[string]any, body string, ctx context.Context, ws *core.Workspace, id string) error {
	out := cmd.OutOrStdout()
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Frontmatter fields in a clear format
	orderedKeys := []string{"id", "title", "status", "artifact_type", "priority", "parent_id", "sprint", "assigned_to", "owner"}
	for _, key := range orderedKeys {
		if v, ok := fm[key]; ok && v != nil && fmt.Sprint(v) != "" {
			fmt.Fprintf(w, "%s:\t%v\n", key, v)
		}
	}
	// Remaining fields not in the ordered set
	for key, val := range fm {
		if val == nil || fmt.Sprint(val) == "" {
			continue
		}
		if isInSlice(key, orderedKeys) {
			continue
		}
		fmt.Fprintf(w, "%s:\t%v\n", key, val)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// Description body
	trimmed := strings.TrimSpace(body)
	if trimmed != "" {
		fmt.Fprintf(out, "\n--- Description ---\n%s\n", trimmed)
	}

	// Dependencies
	deps, err := db.GetDependencies(ctx, ws.DB, id)
	if err == nil && len(deps) > 0 {
		fmt.Fprintln(out, "\n--- Dependencies ---")
		for _, d := range deps {
			fmt.Fprintf(out, "  %s → %s (%s)\n", d.ItemID, d.DependsOn, d.DepType)
		}
	}

	// Commits
	commits, err := core.GetCommitLinks(ctx, ws.DB, id)
	if err == nil && len(commits) > 0 {
		fmt.Fprintln(out, "\n--- Commits ---")
		for _, c := range commits {
			fmt.Fprintf(out, "  %s  %s  (%s)\n", c.CommitSHA, c.Message, c.Author)
		}
	}
	return nil
}

func isInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
