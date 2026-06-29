package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

func newDoctorCommand(cwd *string) *cobra.Command {
	var (
		checkOrphans      bool
		checkDuplicates   bool
		checkArchivedFrom bool
		fixOrphans        bool
		fixArchivedFrom   bool
		fixMalformed      bool
		outputFormatFlag  string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check workspace integrity",
		Long: `Scan the .backlogit workspace for structural issues such as
orphaned artifacts (child types with no parent) and duplicate IDs
across queue and archive directories.

Use --fix-orphans to archive orphaned artifacts automatically.
Use --fix-archived-from to repair legacy self-referential archived_from
records (rewrites them to their canonical queue restore path). Use
--fix-malformed to clear malformed archived_from records that have no restore
target. Both are destructive, CLI-only migrations: not exposed on the MCP doctor tool.`,
		Example: `  backlogit doctor
  backlogit doctor --check-orphans=false
  backlogit doctor --fix-orphans
  backlogit doctor --fix-archived-from
  backlogit doctor --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if outputFormatFlag != "text" && outputFormatFlag != "json" {
				return fmt.Errorf("unsupported format %q (expected text or json)", outputFormatFlag)
			}

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			report, err := core.Doctor(ctx, ws, &core.DoctorOptions{
				CheckOrphans:      checkOrphans,
				CheckDuplicates:   checkDuplicates,
				CheckArchivedFrom: checkArchivedFrom,
				FixOrphans:        fixOrphans,
				FixArchivedFrom:   fixArchivedFrom,
				FixMalformed:      fixMalformed,
			})
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			w := cmd.OutOrStdout()
			if outputFormatFlag == "json" {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			// Text output.
			if len(report.Findings) == 0 && len(report.FixActions) == 0 {
				fmt.Fprintln(w, "No issues found.")
				return nil
			}
			for _, f := range report.Findings {
				fmt.Fprintf(w, "[%s] %s: %s\n", f.Type, f.ArtifactID, f.Description)
			}
			for _, a := range report.FixActions {
				fmt.Fprintf(w, "[fix:%s] %s: %s\n", a.Type, a.ArtifactID, a.Detail)
			}
			fmt.Fprintf(w, "\n%d issue(s) found, %d fix(es) applied.\n", len(report.Findings), len(report.FixActions))
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOrphans, "check-orphans", true, "check for orphaned child artifacts")
	cmd.Flags().BoolVar(&checkDuplicates, "check-duplicates", true, "check for duplicate IDs across directories")
	cmd.Flags().BoolVar(&checkArchivedFrom, "check-archived-from", true, "check archive records for self-referential/malformed archived_from fields")
	cmd.Flags().BoolVar(&fixOrphans, "fix-orphans", false, "archive orphaned artifacts instead of just reporting them")
	cmd.Flags().BoolVar(&fixArchivedFrom, "fix-archived-from", false, "repair legacy self-referential archived_from records (destructive, CLI-only)")
	cmd.Flags().BoolVar(&fixMalformed, "fix-malformed", false, "clear malformed archived_from records with no restore target (destructive, CLI-only; requires --check-archived-from)")
	cmd.Flags().StringVar(&outputFormatFlag, "format", "text", "output format: text or json")

	return cmd
}
