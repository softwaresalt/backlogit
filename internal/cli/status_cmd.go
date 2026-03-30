package cli

import (
	"github.com/spf13/cobra"
)

// newStatusCommand creates the `backlogit status` command.
func newStatusCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='status'. Show workspace summary including artifact counts by type and status, last sync time, workspace path, and template count. Include slog instrumentation per review F4.")
}
