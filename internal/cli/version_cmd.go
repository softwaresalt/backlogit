package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/version"
)

// versionInfo holds the structured version payload for JSON output.
type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// newVersionCommand creates the `backlogit version` subcommand.
// It prints version, commit, build date, and Go runtime version.
// --format=json emits a structured JSON object.
func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, build date, and Go runtime information",
		Example: `  backlogit version
  backlogit version --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}
			info := versionInfo{
				Version:   version.Version,
				Commit:    version.Commit,
				BuildDate: version.BuildDate,
				GoVersion: runtime.Version(),
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			buildDate := info.BuildDate
			if buildDate == "" {
				buildDate = "unknown"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "version    %s\ncommit     %s\nbuild date %s\ngo version %s\n",
				info.Version, info.Commit, buildDate, info.GoVersion)
			return nil
		},
	}
	cmd.Flags().String("format", "", "output format: json (default: human)")
	return cmd
}
