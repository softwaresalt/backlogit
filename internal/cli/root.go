package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

// NewRootCommand creates the backlogit CLI root command.
func NewRootCommand() *cobra.Command {
	var cwd string
	root := &cobra.Command{
		Use:          "backlogit",
		Short:        "Backlogit — AI-native agile workspace",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&cwd, "cwd", ".", "workspace directory")

	root.AddCommand(newInitCommand(&cwd))
	root.AddCommand(newSyncCommand(&cwd))
	root.AddCommand(newMCPCommand(&cwd))

	return root
}

func newInitCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new backlogit workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := *cwd
			if len(args) > 0 {
				root = args[0]
			}
			dir := filepath.Join(root, ".backlogit")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create workspace dir: %w", err)
			}
			if err := config.WriteDefaults(dir); err != nil {
				return fmt.Errorf("write defaults: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized backlogit workspace at %s\n", dir)
			return nil
		},
	}
}

func newSyncCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Rehydrate the SQLite index from Markdown source files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			count, err := db.Rehydrate(ctx, ws.RootPath, ws.DB)
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Indexed %d artifacts\n", count)
			return nil
		},
	}
}

func newMCPCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the backlogit MCP stdio server",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			s := mcpinternal.NewServer(ws)
			return mcpinternal.RunStdio(s)
		},
	}
}
