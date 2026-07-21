package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// artifactColumns defines the standard output columns for artifact list and queue views.
var artifactColumns = []format.Column{
	{Key: "id", Header: "ID"},
	{Key: "title", Header: "TITLE"},
	{Key: "status", Header: "STATUS"},
	{Key: "type", Header: "TYPE"},
	{Key: "priority", Header: "PRIORITY"},
	{Key: "size", Header: "SIZE"},
	{Key: "composition", Header: "COMPOSITION"},
}

// sizeBucketOrder is the canonical display order for the size histogram in the
// human-readable composition summary. Any bucket outside this set is appended in
// deterministic (sorted) order so the summary never depends on map iteration.
var sizeBucketOrder = []string{"XS", "S", "M", "L", "XL"}

// formatCompositionSummary renders a size-composition rollup as a compact,
// deterministic one-line summary (e.g. "L:1 M:1 unsized:2") for human table and
// tile surfaces. It returns an empty string for a nil result or an empty rollup.
func formatCompositionSummary(c *core.SizeCompositionResult) string {
	if c == nil {
		return ""
	}
	var parts []string
	seen := make(map[string]bool, len(c.Histogram))
	for _, bucket := range sizeBucketOrder {
		if n := c.Histogram[bucket]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", bucket, n))
			seen[bucket] = true
		}
	}
	// Append any non-canonical buckets in sorted order for determinism.
	extra := make([]string, 0, len(c.Histogram))
	for bucket, n := range c.Histogram {
		if n > 0 && !seen[bucket] {
			extra = append(extra, bucket)
		}
	}
	sort.Strings(extra)
	for _, bucket := range extra {
		parts = append(parts, fmt.Sprintf("%s:%d", bucket, c.Histogram[bucket]))
	}
	if c.Unsized > 0 {
		parts = append(parts, fmt.Sprintf("unsized:%d", c.Unsized))
	}
	return strings.Join(parts, " ")
}

// artifactsToRows converts a slice of artifacts to the row maps consumed by format.Renderer.
// It projects the stored per-artifact size and, for aggregate types
// (feature/shipment), a computed-on-read composition summary — keeping the human
// table/tile surfaces at parity with the JSON read surfaces (114-F). Composition
// derivation is read-only and warn-skips on error so a rollup failure never
// aborts the listing.
func artifactsToRows(ctx context.Context, ws *core.Workspace, artifacts []*models.Artifact) []map[string]any {
	rows := make([]map[string]any, len(artifacts))
	for i, a := range artifacts {
		size, _ := a.CustomFields["size"].(string)
		composition := ""
		if core.IsSizeCompositionAggregate(a.ArtifactType) {
			if result, err := core.SizeComposition(ctx, ws, a); err != nil {
				slog.WarnContext(ctx, "list: skipping size composition summary", "artifact_id", a.ID, "error", err)
			} else {
				composition = formatCompositionSummary(result)
			}
		}
		rows[i] = map[string]any{
			"id":          a.ID,
			"title":       a.Title,
			"status":      string(a.Status),
			"type":        a.ArtifactType,
			"priority":    a.Priority,
			"size":        size,
			"composition": composition,
		}
	}
	return rows
}

// isTerminal reports whether w is a terminal-connected file descriptor.
// It returns false when w is not an *os.File or when the file descriptor
// is not attached to a terminal (e.g., piped output, test buffers).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// newRenderer returns a format.Renderer for the given format.
// w is the destination writer used for TTY detection when the tile renderer
// is selected; bold output is enabled only when w is a real terminal.
func newRenderer(f format.Format, w io.Writer) format.Renderer {
	if f == format.FormatTile {
		return format.NewTileRenderer(isTerminal(w))
	}
	return format.NewTableRenderer()
}

// validateFormat returns an error when f is not one of the allowed formats.
func validateFormat(f format.Format, allowed ...format.Format) error {
	for _, a := range allowed {
		if f == a {
			return nil
		}
	}
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	return fmt.Errorf("--format %q is not supported: allowed values are %s", f, strings.Join(names, ", "))
}

// newListCommand creates the `backlogit list` command.
func newListCommand(cwd *string) *cobra.Command {
	var (
		filterType       string
		filterStatus     string
		filterPriority   string
		filterAssignedTo string
		filterOwner      string
		filterSprint     string
		groupBy          string
		jsonOutput       bool
		formatOutput     string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts in the workspace",
		Long: `List artifacts from the backlogit index with optional filters.

Use this command for quick operator views, grouped summaries, or JSON output
that can be piped into other tooling.`,
		Example: `  backlogit list
  backlogit list --status active --type task
  backlogit list --group-by status
  backlogit list --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			artifacts, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{
				Type:       filterType,
				Status:     filterStatus,
				Priority:   filterPriority,
				AssignedTo: filterAssignedTo,
				Owner:      filterOwner,
				Sprint:     filterSprint,
			})
			if err != nil {
				return fmt.Errorf("query items: %w", err)
			}

			// --json is a legacy alias for --format json.
			effectiveFormat := format.Format(formatOutput)
			if jsonOutput {
				effectiveFormat = format.FormatJSON
			} else if err := validateFormat(effectiveFormat, format.FormatTable, format.FormatJSON, format.FormatTile); err != nil {
				return err
			}

			if effectiveFormat == format.FormatJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(artifacts)
			}

			if groupBy != "" {
				items := make([]ListItem, len(artifacts))
				for i, a := range artifacts {
					items[i] = ListItem{
						ID:       a.ID,
						Title:    a.Title,
						Status:   string(a.Status),
						Type:     a.ArtifactType,
						ParentID: a.ParentID,
						Priority: a.Priority,
					}
				}
				fmt.Fprint(cmd.OutOrStdout(), FormatGroupedView(items, groupBy))
				return nil
			}

			return newRenderer(effectiveFormat, cmd.OutOrStdout()).Render(cmd.OutOrStdout(), artifactColumns, artifactsToRows(ctx, ws, artifacts))
		},
	}

	cmd.Flags().StringVar(&filterType, "type", "", "filter by artifact type")
	cmd.Flags().StringVar(&filterStatus, "status", "", "filter by status")
	cmd.Flags().StringVar(&filterPriority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&filterAssignedTo, "assigned-to", "", "filter by assignee")
	cmd.Flags().StringVar(&filterOwner, "owner", "", "filter by owner")
	cmd.Flags().StringVar(&filterSprint, "sprint", "", "filter by sprint ID")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "group output by field (status, type, priority)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON array")
	cmd.Flags().StringVar(&formatOutput, "format", "table", "output format: table, json, tile")
	return cmd
}
