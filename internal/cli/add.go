package cli

import (
	"github.com/spf13/cobra"
)

// newAddCommand creates the `backlogit add` command.
func newAddCommand(cwd *string) *cobra.Command {
	panic("not implemented: Worker: Create cobra.Command with Use='add', flags for --type, --title, and section flags from templates. Open workspace via core.NewWorkspace, resolve type from header-def.yaml, load template, create artifact via core.CreateArtifact with functional options, populate sections from flags or stdin (multi-line with '-' value), write artifact file with template structure. Include slog instrumentation at Info (entry/exit) and Debug (intermediate) levels. Register via root.AddCommand(newAddCommand(&cwd)).")
}
