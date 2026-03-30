package cli

import (
	"github.com/spf13/cobra"
)

// newListCommand creates the `backlogit list` command.
func newListCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='list', flags for --type, --status, --assigned-to, --sprint, --json. Query SQLite index via db.QueryItems with filters. Format output as table (columns: ID, Title, Status, Type, Priority) or JSON array. Include slog instrumentation per review F4.")
}
