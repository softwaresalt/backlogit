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
	{Key: "complexity", Header: "COMPLEXITY"},
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

// batchCompositions computes the size_composition rollups for every aggregate
// (feature/shipment) in artifacts exactly once, removing the per-row N+1 that a
// per-artifact rollup would otherwise incur on the human list/queue table, tile,
// and grouped renders (117-F / A6A1B47E). On a batch failure it warns and returns
// nil so the human surfaces degrade to size-only rows rather than aborting.
func batchCompositions(ctx context.Context, ws *core.Workspace, artifacts []*models.Artifact) map[string]*core.SizeCompositionResult {
	comps, err := core.SizeCompositions(ctx, ws, artifacts)
	if err != nil {
		slog.WarnContext(ctx, "list: size compositions batch failed; rows left without composition", "error", err)
		return nil
	}
	return comps
}

// artifactSizeAndComposition returns the stored per-artifact size and, for
// aggregate types (feature/shipment), the computed-on-read one-line composition
// summary projected from the precomputed batch map. It centralizes the read-only
// projection shared by the ungrouped table/tile rows and the grouped human view
// so the human list surfaces cannot drift on composition parity (114-F), and it
// reads from the shared batch (built once via batchCompositions) so those renders
// incur no per-row N+1 (117-F / A6A1B47E). A missing or nil entry yields an empty
// composition, matching the warn-skip degradation of the batch build.
func artifactSizeAndComposition(a *models.Artifact, comps map[string]*core.SizeCompositionResult) (size, composition string) {
	size, _ = a.CustomFields["size"].(string)
	if core.IsSizeCompositionAggregate(a.ArtifactType) {
		if result, ok := comps[a.ID]; ok && result != nil {
			composition = formatCompositionSummary(result)
		}
	}
	return size, composition
}

func artifactComplexity(a *models.Artifact) string {
	complexity, _ := a.CustomFields["complexity"].(string)
	return complexity
}

// artifactsToRows converts a slice of artifacts to the row maps consumed by format.Renderer.
// It projects the stored per-artifact size and, for aggregate types
// (feature/shipment), a computed-on-read composition summary — keeping the human
// table/tile surfaces at parity with the JSON read surfaces (114-F). The rollups
// for every aggregate are computed once via batchCompositions so the render incurs
// no per-row N+1, and derivation is read-only and warn-skips on error so a rollup
// failure never aborts the listing (117-F / A6A1B47E).
func artifactsToRows(ctx context.Context, ws *core.Workspace, artifacts []*models.Artifact) []map[string]any {
	comps := batchCompositions(ctx, ws, artifacts)
	rows := make([]map[string]any, len(artifacts))
	for i, a := range artifacts {
		size, composition := artifactSizeAndComposition(a, comps)
		rows[i] = map[string]any{
			"id":          a.ID,
			"title":       a.Title,
			"status":      string(a.Status),
			"type":        a.ArtifactType,
			"priority":    a.Priority,
			"size":        size,
			"complexity":  artifactComplexity(a),
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
		filterComplexity string
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

			if filterComplexity != "" {
				if err := core.ValidateComplexityValue(ws, "task", filterComplexity); err != nil {
					return err
				}
			}

			artifacts, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{
				Type:       filterType,
				Status:     filterStatus,
				Priority:   filterPriority,
				Complexity: filterComplexity,
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
				// Route through the shared core shaper so `list --json` attaches
				// the computed-on-read size_composition rollup to aggregate rows
				// at parity with the MCP list_items surface (117-F / 60336CC0).
				return enc.Encode(core.ListWithSizeComposition(ctx, ws, artifacts))
			}

			if groupBy != "" {
				comps := batchCompositions(ctx, ws, artifacts)
				items := make([]ListItem, len(artifacts))
				for i, a := range artifacts {
					size, composition := artifactSizeAndComposition(a, comps)
					items[i] = ListItem{
						ID:          a.ID,
						Title:       a.Title,
						Status:      string(a.Status),
						Type:        a.ArtifactType,
						ParentID:    a.ParentID,
						Priority:    a.Priority,
						Size:        size,
						Complexity:  artifactComplexity(a),
						Composition: composition,
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
	cmd.Flags().StringVar(&filterComplexity, "complexity", "", "filter by implementation complexity (trivial, low, medium, high)")
	cmd.Flags().StringVar(&filterAssignedTo, "assigned-to", "", "filter by assignee")
	cmd.Flags().StringVar(&filterOwner, "owner", "", "filter by owner")
	cmd.Flags().StringVar(&filterSprint, "sprint", "", "filter by sprint ID")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "group output by field (status, type, priority)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON array")
	cmd.Flags().StringVar(&formatOutput, "format", "table", "output format: table, json, tile")
	return cmd
}
