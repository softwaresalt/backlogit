package cli

import (
	"github.com/spf13/cobra"
)

// newGetCommand creates the `backlogit get` command.
func newGetCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='get <id>', flags for --json and --section. Retrieve artifact via db.GetItem, then read the full Markdown file from disk. Display full content (frontmatter + body) by default. --json emits frontmatter-only JSON. --section extracts a specific section via parser.ParseSections. Include slog instrumentation per review F4.")
}
