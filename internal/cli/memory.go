package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

// newMemoryCmd returns the `memory` command group. It is the CLI fallback for
// the MCP save_memory tool and writes .backlogit/memories.json at the resolved
// workspace root, matching the MCP handler's path resolution.
func newMemoryCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Persist keyed agent session memory",
		Long: `Save keyed session memory summaries.

This is the CLI fallback for the MCP save_memory tool. It writes to
.backlogit/memories.json at the resolved workspace root, matching the MCP
handler's path resolution so a save invoked from a subdirectory targets the
correct workspace.`,
	}
	cmd.AddCommand(newMemorySaveCmd(cwd))
	return cmd
}

func newMemorySaveCmd(cwd *string) *cobra.Command {
	var key, summary string
	cmd := &cobra.Command{
		Use:     "save",
		Short:   "Save a keyed memory summary to .backlogit/memories.json",
		Example: `  backlogit memory save --key session-079 --summary "shipped U1-U8"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			if key == "" {
				return fmt.Errorf("key is required")
			}
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			// Resolve the memories path from the workspace root (not raw --cwd) so a
			// save invoked from a subdirectory writes to the correct .backlogit,
			// matching the MCP handleSaveMemory path resolution exactly.
			memoriesPath := filepath.Join(ws.RootPath, ".backlogit", "memories.json")
			if err := events.SaveMemory(ctx, memoriesPath, key, summary); err != nil {
				return fmt.Errorf("save memory: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]bool{"ok": true})
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "memory key")
	cmd.Flags().StringVar(&summary, "summary", "", "memory summary")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}
