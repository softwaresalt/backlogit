package cli

import (
	"github.com/spf13/cobra"
)

// newMoveCommand creates the `backlogit move` command.
func newMoveCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='move <id> --status <new_status>'. Change artifact status via core.UpdateArtifact, then relocate file according to registry.yaml routing rules via core.MoveArtifactFile. Re-sync index after move. Include slog instrumentation per review F4.")
}
