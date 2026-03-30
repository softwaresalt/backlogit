package cli

import (
	"github.com/spf13/cobra"
)

// newSearchCommand creates the `backlogit search` command.
func newSearchCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='search <query>', --limit flag (default 20). Full-text search via db.SearchItems with FTS5. Display results in table format with relevance ordering. Include slog instrumentation per review F4.")
}
