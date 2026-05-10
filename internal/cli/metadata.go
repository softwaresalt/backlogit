package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// NewMetadataCmd creates the metadata command group.
func NewMetadataCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Discover backlogit metadata for agents and tooling",
		Long: `Inspect the unified backlogit metadata catalog that agents need for
programmatic reasoning, including artifact types, field enums, template sections,
registry routing, stash conventions, CLI commands, and MCP tools.`,
	}
	cmd.AddCommand(newMetadataCatalogCommand(cwd))
	cmd.AddCommand(newMetadataExportCommand(cwd))
	return cmd
}

func newMetadataCatalogCommand(cwd *string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Print the unified metadata catalog",
		Example: `  backlogit metadata catalog
  backlogit metadata catalog --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			catalog, err := loadMetadataCatalog(ctx, *cwd)
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if jsonOutput {
				return enc.Encode(catalog)
			}
			return enc.Encode(catalog)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", true, "output as JSON")
	return cmd
}

func newMetadataExportCommand(cwd *string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export-command-map <workspace-relative-path>",
		Short: "Write an agent-readable command map into the workspace",
		Long: `Export a cacheable command map file into a workspace-relative path such as
.github\instructions\backlogit-command-map.instructions.md so agents can reason
over backlogit commands and metadata without re-discovering them each run.`,
		Example: `  backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
  backlogit metadata export-command-map .github\instructions\backlogit-command-map.json --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			catalog, err := loadMetadataCatalog(ctx, *cwd)
			if err != nil {
				return err
			}

			writtenPath, err := core.WriteCommandMap(*cwd, args[0], catalog, format)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote command map to %s\n", writtenPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or json")
	return cmd
}

func loadMetadataCatalog(ctx context.Context, cwd string) (*core.MetadataCatalog, error) {
	ws, err := core.NewWorkspace(ctx, cwd)
	if err != nil {
		return nil, fmt.Errorf("open workspace: %w", err)
	}
	defer ws.Close()

	backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	migration, err := config.LoadMigrationConfig(backlogitDir)
	if err != nil {
		migration = nil
	}

	var sqlSchema []db.TableSchema
	if ws.DB != nil {
		schema, err := db.IntrospectSchema(ctx, ws.DB)
		if err != nil {
			slog.Warn("schema introspection failed, catalog will omit sql_schema", "error", err)
		} else {
			sqlSchema = schema
		}
	}

	metadataRoot := buildMetadataRootCommand(cwd)
	return core.BuildMetadataCatalog(
		ws,
		describeTemplateInfos(ws.Templates),
		registry,
		migration,
		metadataRoot,
		describeMCPTools(ws),
		sqlSchema,
	)
}

func describeTemplateInfos(templates []*config.TemplateConfig) []core.TemplateInfo {
	infos := make([]core.TemplateInfo, 0, len(templates))
	for _, tmpl := range templates {
		sections := make([]core.SectionInfo, 0, len(tmpl.Sections))
		for _, sec := range tmpl.Sections {
			sections = append(sections, core.SectionInfo{
				Name:     sec.Name,
				Required: sec.Required,
			})
		}
		infos = append(infos, core.TemplateInfo{
			TypeName:    tmpl.ArtifactType,
			DisplayName: tmpl.Name,
			Sections:    sections,
		})
	}
	return infos
}

func describeMCPTools(ws *core.Workspace) []core.ToolInfo {
	server := mcpinternal.NewServer(ws)
	return server.DescribeTools()
}

func buildMetadataRootCommand(cwd string) *cobra.Command {
	root := NewRootCommand()
	root.SetArgs(nil)
	propagateMetadataCWD(root, cwd)
	return root
}

func propagateMetadataCWD(cmd *cobra.Command, cwd string) {
	if cmd == nil {
		return
	}
	if flag := cmd.Flags().Lookup("cwd"); flag != nil {
		_ = flag.Value.Set(cwd)
	}
	if flag := cmd.PersistentFlags().Lookup("cwd"); flag != nil {
		_ = flag.Value.Set(cwd)
	}
	for _, sub := range cmd.Commands() {
		propagateMetadataCWD(sub, cwd)
	}
}
