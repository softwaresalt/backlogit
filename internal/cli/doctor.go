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
		checkOrphans     bool
		checkDuplicates  bool
		outputFormatFlag string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check workspace integrity",
		Long: `Scan the .backlogit workspace for structural issues such as
orphaned artifacts (child types with no parent) and duplicate IDs
across queue and archive directories.`,
		Example: `  backlogit doctor
  backlogit doctor --check-orphans=false
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
				CheckOrphans:    checkOrphans,
				CheckDuplicates: checkDuplicates,
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
			if len(report.Findings) == 0 {
				fmt.Fprintln(w, "No issues found.")
				return nil
			}
			for _, f := range report.Findings {
				fmt.Fprintf(w, "[%s] %s: %s\n", f.Type, f.ArtifactID, f.Description)
			}
			fmt.Fprintf(w, "\n%d issue(s) found.\n", len(report.Findings))
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOrphans, "check-orphans", true, "check for orphaned child artifacts")
	cmd.Flags().BoolVar(&checkDuplicates, "check-duplicates", true, "check for duplicate IDs across directories")
	cmd.Flags().StringVar(&outputFormatFlag, "format", "text", "output format: text or json")

	return cmd
}
