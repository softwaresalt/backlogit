package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/stash"
)

// MetadataCatalog is the unified agent-facing metadata view of a workspace.
type MetadataCatalog struct {
	Workspace     MetadataWorkspaceInfo `json:"workspace"`
	ArtifactTypes []WITMetadata         `json:"artifact_types"`
	Registry      []MetadataRoute       `json:"registry"`
	Templates     []TemplateInfo        `json:"templates"`
	Migration     *MetadataMigration    `json:"migration,omitempty"`
	Stash         MetadataStashInfo     `json:"stash"`
	CLI           []CommandInfo         `json:"cli"`
	MCPTools      []ToolInfo            `json:"mcp_tools"`
}

// MetadataWorkspaceInfo describes key workspace paths and layout.
type MetadataWorkspaceInfo struct {
	RootPath      string `json:"root_path"`
	StorageRoot   string `json:"storage_root"`
	QueuePath     string `json:"queue_path"`
	ArchivePath   string `json:"archive_path"`
	LogsPath      string `json:"logs_path"`
	DatabasePath  string `json:"database_path"`
	StashPath     string `json:"stash_path"`
	TemplatesPath string `json:"templates_path"`
	QueueRootDir  string `json:"queue_root_dir"`
}

// MetadataRoute describes a single registry routing rule.
type MetadataRoute struct {
	Path   string   `json:"path"`
	Status []string `json:"status,omitempty"`
	Types  []string `json:"types,omitempty"`
}

// MetadataMigration summarizes configured migration behavior.
type MetadataMigration struct {
	DefaultLayout   string                       `json:"default_layout"`
	DocumentClasses []config.DocumentClassConfig `json:"document_classes"`
	SourcePaths     []config.SourcePathConfig    `json:"source_paths"`
}

// MetadataStashInfo describes stash conventions and supported kinds.
type MetadataStashInfo struct {
	Path                string   `json:"path"`
	SupportedKinds      []string `json:"supported_kinds"`
	SupportedPriorities []string `json:"supported_priorities"`
	DefaultPriority     string   `json:"default_priority"`
}

// TemplateInfo describes a registered template and its section metadata.
type TemplateInfo struct {
	TypeName    string        `json:"type_name"`
	DisplayName string        `json:"display_name"`
	Sections    []SectionInfo `json:"sections"`
}

// SectionInfo describes a single section within a template.
type SectionInfo struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// CommandInfo describes a CLI command for agent discovery and caching.
type CommandInfo struct {
	Command     string   `json:"command"`
	Use         string   `json:"use"`
	Short       string   `json:"short,omitempty"`
	Long        string   `json:"long,omitempty"`
	Example     string   `json:"example,omitempty"`
	Subcommands []string `json:"subcommands,omitempty"`
}

// ToolInfo describes an MCP tool for agent discovery and caching.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// BuildMetadataCatalog assembles a unified workspace metadata catalog for agents.
func BuildMetadataCatalog(
	ws *Workspace,
	templateInfos []TemplateInfo,
	registry *config.RegistryConfig,
	migration *config.MigrationConfig,
	cliRoot *cobra.Command,
	mcpTools []ToolInfo,
) (*MetadataCatalog, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if ws.HeaderDef == nil {
		return nil, fmt.Errorf("header-def metadata is required")
	}
	if ws.Config == nil || ws.Config.QueueLayout == nil {
		return nil, fmt.Errorf("queue layout metadata is required")
	}

	artifactTypes, err := ListTypes(ws.HeaderDef, ws.Templates, ws.Config.QueueLayout)
	if err != nil {
		return nil, fmt.Errorf("list artifact types: %w", err)
	}
	enrichArtifactTypes(artifactTypes, ws.Config)

	catalog := &MetadataCatalog{
		Workspace: MetadataWorkspaceInfo{
			RootPath:      ws.RootPath,
			StorageRoot:   WorkspaceStorageRoot(ws.RootPath),
			QueuePath:     filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue"),
			ArchivePath:   filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive"),
			LogsPath:      WorkspaceLogsRoot(ws.RootPath),
			DatabasePath:  filepath.Join(WorkspaceStorageRoot(ws.RootPath), "backlogit.db"),
			StashPath:     filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue", ".stash.md"),
			TemplatesPath: filepath.Join(WorkspaceStorageRoot(ws.RootPath), "templates"),
			QueueRootDir:  ws.Config.QueueLayout.RootDir,
		},
		ArtifactTypes: artifactTypes,
		Registry:      registryRoutes(registry),
		Templates:     templateInfos,
		Stash: MetadataStashInfo{
			Path:                filepath.Join(WorkspaceStorageRoot(ws.RootPath), "queue", ".stash.md"),
			SupportedKinds:      stash.AllowedKinds(),
			SupportedPriorities: stash.AllowedPriorities(),
			DefaultPriority:     stash.DefaultPriority,
		},
		CLI:      DescribeCLICommands(cliRoot),
		MCPTools: sortToolInfos(mcpTools),
	}

	if migration != nil {
		catalog.Migration = &MetadataMigration{
			DefaultLayout:   migration.DefaultLayout,
			DocumentClasses: migration.DocumentClasses,
			SourcePaths:     migration.SourcePaths,
		}
	}

	return catalog, nil
}

// DescribeCLICommands flattens a cobra command tree into agent-readable command info.
func DescribeCLICommands(root *cobra.Command) []CommandInfo {
	if root == nil {
		return nil
	}

	var result []CommandInfo
	var walk func(cmd *cobra.Command, parents []string)
	walk = func(cmd *cobra.Command, parents []string) {
		if cmd == nil || cmd.Hidden {
			return
		}

		current := append([]string{}, parents...)
		if cmd.Name() != "" {
			current = append(current, cmd.Name())
		}

		info := CommandInfo{
			Command: strings.Join(current, " "),
			Use:     cmd.Use,
			Short:   strings.TrimSpace(cmd.Short),
			Long:    strings.TrimSpace(cmd.Long),
			Example: strings.TrimSpace(cmd.Example),
		}
		for _, sub := range cmd.Commands() {
			if sub.Hidden {
				continue
			}
			info.Subcommands = append(info.Subcommands, sub.Name())
		}
		sort.Strings(info.Subcommands)
		result = append(result, info)

		for _, sub := range cmd.Commands() {
			walk(sub, current)
		}
	}

	walk(root, nil)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Command < result[j].Command
	})
	return result
}

// RenderCommandMapMarkdown renders the unified command map as Markdown for agent caching.
func RenderCommandMapMarkdown(catalog *MetadataCatalog) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: backlogit command map\n")
	b.WriteString("description: Agent-readable backlogit metadata and command reference\n")
	b.WriteString("---\n\n")
	b.WriteString("## Workspace\n\n")
	fmt.Fprintf(&b, "* Storage root: `%s`\n", catalog.Workspace.StorageRoot)
	fmt.Fprintf(&b, "* Queue path: `%s`\n", catalog.Workspace.QueuePath)
	fmt.Fprintf(&b, "* Archive path: `%s`\n", catalog.Workspace.ArchivePath)
	fmt.Fprintf(&b, "* Logs path: `%s`\n", catalog.Workspace.LogsPath)
	fmt.Fprintf(&b, "* Stash path: `%s`\n", catalog.Workspace.StashPath)

	b.WriteString("\n## Artifact Types\n\n")
	for _, artifactType := range catalog.ArtifactTypes {
		fmt.Fprintf(&b, "### `%s`\n\n", artifactType.TypeName)
		if artifactType.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", artifactType.Description)
		}
		fmt.Fprintf(&b, "* Prefix: `%s`\n", artifactType.Prefix)
		fmt.Fprintf(&b, "* Hierarchy level: `%d`\n", artifactType.HierarchyLevel)
		fmt.Fprintf(&b, "* ID format: `%s`\n", artifactType.IDFormat)
		if len(artifactType.AllowedChildren) > 0 {
			fmt.Fprintf(&b, "* Allowed children: `%s`\n", strings.Join(artifactType.AllowedChildren, "`, `"))
		}
		if len(artifactType.Fields) > 0 {
			b.WriteString("* Fields:\n")
			fieldNames := make([]string, 0, len(artifactType.Fields))
			for name := range artifactType.Fields {
				fieldNames = append(fieldNames, name)
			}
			sort.Strings(fieldNames)
			for _, name := range fieldNames {
				field := artifactType.Fields[name]
				required := "optional"
				if field.Required {
					required = "required"
				}
				line := fmt.Sprintf("  * `%s` (%s, %s)", name, field.Type, required)
				if len(field.Values) > 0 {
					line += fmt.Sprintf(" values: `%s`", strings.Join(field.Values, "`, `"))
				}
				if field.Default != "" {
					line += fmt.Sprintf(" default: `%s`", field.Default)
				}
				b.WriteString(line + "\n")
			}
		}
		if len(artifactType.Sections) > 0 {
			b.WriteString("* Template sections:\n")
			for _, section := range artifactType.Sections {
				required := "optional"
				if section.Required {
					required = "required"
				}
				line := fmt.Sprintf("  * `%s` (%s)", section.Name, required)
				if section.Description != "" {
					line += ": " + section.Description
				}
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n## Stash\n\n")
	fmt.Fprintf(&b, "* Path: `%s`\n", catalog.Stash.Path)
	fmt.Fprintf(&b, "* Supported kinds: `%s`\n", strings.Join(catalog.Stash.SupportedKinds, "`, `"))
	fmt.Fprintf(&b, "\n* Supported priorities: `%s`\n", strings.Join(catalog.Stash.SupportedPriorities, "`, `"))
	fmt.Fprintf(&b, "\n* Default priority: `%s`\n", catalog.Stash.DefaultPriority)

	b.WriteString("\n## CLI Commands\n\n")
	for _, cmd := range catalog.CLI {
		fmt.Fprintf(&b, "### `%s`\n\n", cmd.Command)
		if cmd.Short != "" {
			b.WriteString(cmd.Short + "\n\n")
		}
		if cmd.Example != "" {
			b.WriteString("```text\n")
			b.WriteString(cmd.Example + "\n")
			b.WriteString("```\n\n")
		}
	}

	b.WriteString("## MCP Tools\n\n")
	for _, tool := range catalog.MCPTools {
		fmt.Fprintf(&b, "* `%s`", tool.Name)
		if tool.Description != "" {
			fmt.Fprintf(&b, ": %s", tool.Description)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// WriteCommandMap writes a catalog snapshot to a workspace-relative path.
func WriteCommandMap(workspaceRoot, targetPath string, catalog *MetadataCatalog, format string) (string, error) {
	if catalog == nil {
		return "", fmt.Errorf("catalog is required")
	}
	if targetPath == "" {
		return "", fmt.Errorf("target path is required")
	}

	resolvedPath, err := SafeResolve(workspaceRoot, targetPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return "", fmt.Errorf("create command map directory: %w", err)
	}

	var content []byte
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "markdown", "md":
		content = []byte(RenderCommandMapMarkdown(catalog))
	case "json":
		content, err = json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal command map JSON: %w", err)
		}
		content = append(content, '\n')
	default:
		return "", fmt.Errorf("unsupported command map format %q", format)
	}

	tmpPath := resolvedPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write command map: %w", err)
	}
	if err := os.Rename(tmpPath, resolvedPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename command map: %w", err)
	}
	return resolvedPath, nil
}

func enrichArtifactTypes(artifactTypes []WITMetadata, cfg *config.WorkspaceConfig) {
	if cfg == nil {
		return
	}
	for i := range artifactTypes {
		typeCfg, ok := cfg.ArtifactTypes[artifactTypes[i].TypeName]
		if !ok || typeCfg == nil {
			continue
		}
		artifactTypes[i].AllowedChildren = append([]string{}, typeCfg.AllowedChildren...)
		if artifactTypes[i].Prefix == "" {
			artifactTypes[i].Prefix = typeCfg.Prefix
		}
	}
}

func registryRoutes(registry *config.RegistryConfig) []MetadataRoute {
	if registry == nil {
		return nil
	}
	routes := make([]MetadataRoute, 0, len(registry.Directories))
	for _, rule := range registry.Directories {
		routes = append(routes, MetadataRoute{
			Path:   rule.Path,
			Status: append([]string{}, rule.Condition.Status...),
			Types:  append([]string{}, rule.Condition.Type...),
		})
	}
	return routes
}

func sortToolInfos(tools []ToolInfo) []ToolInfo {
	result := append([]ToolInfo(nil), tools...)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
