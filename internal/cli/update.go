package cli

import (
	"github.com/spf13/cobra"
)

// newUpdateCommand creates the `backlogit update` command.
func newUpdateCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='update <id>', flags for --title, --status, --priority, and section flags from templates. Update frontmatter fields via core.UpdateArtifact. Section flags update individual sections via parser.WriteSection. Enforce ID immutability by rejecting --id flag. Support stdin for multi-line section content (flag value '-'). Re-sync SQLite index after update. Include slog instrumentation per review F4.")
}
