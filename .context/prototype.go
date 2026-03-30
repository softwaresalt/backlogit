package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"modernc.org/sqlite"
)

// BacklogitConfig holds workspace configuration loaded from YAML files.
type BacklogitConfig struct {
	WorkspaceDir string
	DotDir       string
	Config       map[string]any
	Registry     map[string]any
}

// NewBacklogitConfig loads configuration from the workspace directory.
func NewBacklogitConfig(workspaceDir string) (*BacklogitConfig, error) {
	dotDir := filepath.Join(workspaceDir, ".backlogit")
	cfg := &BacklogitConfig{
		WorkspaceDir: workspaceDir,
		DotDir:       dotDir,
	}

	config, err := loadYAML(filepath.Join(dotDir, "config.yaml"))
	if err != nil {
		slog.Warn("config.yaml not found, using defaults", "error", err)
		config = make(map[string]any)
	}
	cfg.Config = config

	registry, err := loadYAML(filepath.Join(dotDir, "registry.yaml"))
	if err != nil {
		slog.Warn("registry.yaml not found, using defaults", "error", err)
		registry = make(map[string]any)
	}
	cfg.Registry = registry

	return cfg, nil
}

// GetValidStatuses returns the allowed status values from config.
func (c *BacklogitConfig) GetValidStatuses() []string {
	fields, ok := c.Config["fields"].(map[string]any)
	if !ok {
		return nil
	}
	status, ok := fields["status"].(map[string]any)
	if !ok {
		return nil
	}
	values, ok := status["values"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func loadYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if result == nil {
		return make(map[string]any), nil
	}
	return result, nil
}

// BacklogitManager handles workspace operations.
type BacklogitManager struct {
	Config *BacklogitConfig
}

// NewBacklogitManager creates a manager and ensures workspace directories exist.
func NewBacklogitManager(config *BacklogitConfig) (*BacklogitManager, error) {
	mgr := &BacklogitManager{Config: config}
	if err := mgr.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to ensure directories: %w", err)
	}
	return mgr, nil
}

func (m *BacklogitManager) ensureDirectories() error {
	if err := os.MkdirAll(m.Config.DotDir, 0o755); err != nil {
		return err
	}

	dirs, ok := m.Config.Registry["directories"].([]any)
	if !ok {
		return nil
	}
	for _, d := range dirs {
		dirCfg, ok := d.(map[string]any)
		if !ok {
			continue
		}
		path, ok := dirCfg["path"].(string)
		if !ok || path == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(m.Config.DotDir, path), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// CreateTask creates a new task artifact in the workspace.
func (m *BacklogitManager) CreateTask(title, description, status string) (string, error) {
	validStatuses := m.Config.GetValidStatuses()
	if status != "" && len(validStatuses) > 0 {
		valid := false
		for _, s := range validStatuses {
			if s == status {
				valid = true
				break
			}
		}
		if !valid {
			return "", fmt.Errorf("invalid status: %s (allowed: %v)", status, validStatuses)
		}
	}

	if status == "" {
		fields, ok := m.Config.Config["fields"].(map[string]any)
		if ok {
			statusCfg, ok := fields["status"].(map[string]any)
			if ok {
				if def, ok := statusCfg["default"].(string); ok {
					status = def
				}
			}
		}
		if status == "" {
			status = "todo"
		}
	}

	targetDirName := "backlog"
	dirs, ok := m.Config.Registry["directories"].([]any)
	if ok {
		for _, d := range dirs {
			dirCfg, ok := d.(map[string]any)
			if !ok {
				continue
			}
			cond, ok := dirCfg["condition"].(map[string]any)
			if !ok {
				continue
			}
			statuses, ok := cond["status"].([]any)
			if !ok {
				continue
			}
			for _, s := range statuses {
				if s == status {
					if p, ok := dirCfg["path"].(string); ok {
						targetDirName = p
					}
					break
				}
			}
		}
	}

	targetDir := filepath.Join(m.Config.DotDir, targetDirName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	// Count existing .md files for ID generation
	entries, _ := filepath.Glob(filepath.Join(m.Config.DotDir, "**", "*.md"))
	taskID := fmt.Sprintf("TASK-%03d", len(entries)+1)
	filePath := filepath.Join(targetDir, taskID+".md")

	frontmatter := map[string]string{
		"id":     taskID,
		"title":  title,
		"status": status,
	}

	fmData, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(fmData), description)
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write task file: %w", err)
	}

	return taskID, nil
}

// MCP Server Implementation

var appManager *BacklogitManager

func registerTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("create_task",
			mcp.WithDescription("Create a new task in the backlogit system."),
			mcp.WithString("title", mcp.Required(), mcp.Description("Task title")),
			mcp.WithString("description", mcp.Required(), mcp.Description("Task description")),
			mcp.WithString("status", mcp.Description("Task status")),
		),
		handleCreateTask,
	)

	s.AddTool(
		mcp.NewTool("get_schema",
			mcp.WithDescription("Get the valid fields and statuses for tasks from config.yaml"),
		),
		handleGetSchema,
	)
}

func handleCreateTask(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	title, _ := request.RequireString("title")
	description, _ := request.RequireString("description")
	status := request.GetString("status", "")

	taskID, err := appManager.CreateTask(title, description, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Task %s created successfully.", taskID)), nil
}

func handleGetSchema(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(appManager.Config.Config, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// CLI Entry Point

func main() {
	var cwd string

	rootCmd := &cobra.Command{
		Use:   "backlogit",
		Short: "Backlogit: AI-Native Task Management",
	}
	rootCmd.PersistentFlags().StringVar(&cwd, "cwd", "", "Workspace directory")

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a .backlogit workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := cwd
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			dotDir := filepath.Join(dir, ".backlogit")
			if err := os.MkdirAll(dotDir, 0o755); err != nil {
				return err
			}
			fmt.Printf("Initialized empty Backlogit workspace in %s/.backlogit\n", dir)
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new task",
		RunE: func(cmd *cobra.Command, args []string) error {
			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			if title == "" || desc == "" {
				return fmt.Errorf("--title and --desc are required for create command")
			}

			dir := cwd
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			config, err := NewBacklogitConfig(dir)
			if err != nil {
				return err
			}
			mgr, err := NewBacklogitManager(config)
			if err != nil {
				return err
			}

			taskID, err := mgr.CreateTask(title, desc, "")
			if err != nil {
				return err
			}
			fmt.Printf("Created %s\n", taskID)
			return nil
		},
	}
	createCmd.Flags().String("title", "", "Task title")
	createCmd.Flags().String("desc", "", "Task description")

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := cwd
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			config, err := NewBacklogitConfig(dir)
			if err != nil {
				return err
			}
			mgr, err := NewBacklogitManager(config)
			if err != nil {
				return err
			}
			appManager = mgr

			s := server.NewMCPServer("backlogit", "0.1.0")
			registerTools(s)

			slog.Info("starting backlogit MCP server on stdio")
			return server.ServeStdio(s)
		},
	}

	rootCmd.AddCommand(initCmd, createCmd, mcpCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Placeholder for database connection helper.
var _ *sql.DB
