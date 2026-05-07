package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// newManifestCommand creates the `backlogit manifest` command.
func newManifestCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "manifest",
		Short: "Print a JSON-RPC manifest of all backlogit MCP tool definitions",
		Long: `Print the manifest of all backlogit MCP tools as a JSON object.

The output format is compatible with the MCP tools/list response:

  {"tools": [{"name": "...", "description": "...", "inputSchema": {...}}, ...]}

Tools are sorted alphabetically by name to match MCP tools/list ordering.
This allows agents to discover the full backlogit tool surface through the CLI
in the same format they receive during MCP server initialization. Combine with
--jsonrpc to receive a JSON-RPC 2.0 response envelope.`,
		Example: `  backlogit manifest
  backlogit manifest | jq '.tools[].name'
  backlogit --jsonrpc manifest`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := mcpinternal.NewServerForRoot(*cwd)
			defs := s.ToolDefs()

			sort.Slice(defs, func(i, j int) bool {
				return defs[i].Name < defs[j].Name
			})

			out := map[string]any{
				"tools": defs,
			}
			b, err := json.Marshal(out)
			if err != nil {
				return fmt.Errorf("manifest: marshal tool definitions: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
			return err
		},
	}
}
