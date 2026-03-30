package cli

import (
	"github.com/spf13/cobra"
)

// newQueryCommand creates the `backlogit query` command.
func newQueryCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='query \"<sql>\"'. Execute read-only SQL via db.ExecuteGatedQuery. Display results as formatted table. Reject non-SELECT statements with descriptive error. Include slog instrumentation per review F4.")
}
