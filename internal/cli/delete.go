package cli

import (
	"github.com/spf13/cobra"
)

// newDeleteCommand creates the `backlogit delete` command.
func newDeleteCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='delete <id>', --force flag to skip confirmation. Remove artifact's markdown file from disk and delete from SQLite index via db.DeleteItem. Without --force, prompt for confirmation on stderr. Include slog instrumentation per review F4.")
}
