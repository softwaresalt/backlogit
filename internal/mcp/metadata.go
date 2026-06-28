package mcp

import (
	"context"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func (s *Server) loadMetadataCatalog(ctx context.Context) (*core.MetadataCatalog, error) {
	backlogitDir := s.backlogitDir()
	registry, err := config.LoadRegistry(backlogitDir)
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	migration, err := config.LoadMigrationConfig(backlogitDir)
	if err != nil {
		migration = nil
	}

	var templateInfos []core.TemplateInfo
	if s.templateSvc != nil {
		for _, info := range s.templateSvc.ListTemplates() {
			sections := make([]core.SectionInfo, 0, len(info.Sections))
			for _, section := range info.Sections {
				sections = append(sections, core.SectionInfo{
					Name:     section.Name,
					Required: section.Required,
				})
			}
			templateInfos = append(templateInfos, core.TemplateInfo{
				TypeName:    info.TypeName,
				DisplayName: info.DisplayName,
				Sections:    sections,
			})
		}
	}

	var sqlSchema []db.TableSchema
	if s.Workspace != nil && s.Workspace.DB != nil {
		schema, err := db.IntrospectSchema(ctx, s.Workspace.DB)
		if err != nil {
			logger.Warn("schema introspection failed, catalog will omit sql_schema", "error", err)
		} else {
			sqlSchema = schema
		}
	}

	catalog, err := core.BuildMetadataCatalog(
		s.Workspace,
		templateInfos,
		registry,
		migration,
		nil,
		s.DescribeTools(),
		sqlSchema,
	)
	if err != nil {
		return nil, err
	}

	// Restore CLI/MCP metadata parity: the cli package injects a provider that
	// describes the CLI command tree (internal/mcp cannot import internal/cli
	// because cli imports mcp). When wired, the MCP catalog carries the same CLI
	// command data the CLI metadata path produces.
	if s.CLICommandProvider != nil {
		catalog.CLI = s.CLICommandProvider()
	}
	return catalog, nil
}

// MetadataCatalog builds the unified metadata catalog for this server. It is
// exported so cross-package tests can assert CLI/MCP catalog parity.
func (s *Server) MetadataCatalog(ctx context.Context) (*core.MetadataCatalog, error) {
	return s.loadMetadataCatalog(ctx)
}

func (s *Server) handleGetMetadataCatalog(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	catalog, err := s.loadMetadataCatalog(ctx)
	if err != nil {
		return InternalError(fmt.Sprintf("load metadata catalog: %v", err)), nil
	}
	return toolResultJSON(catalog)
}

func (s *Server) handleExportCommandMap(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	targetPath, _ := request.Params.Arguments["path"].(string)
	if targetPath == "" {
		return ValidationFailed("path is required"), nil
	}
	format, _ := request.Params.Arguments["format"].(string)

	catalog, err := s.loadMetadataCatalog(ctx)
	if err != nil {
		return InternalError(fmt.Sprintf("load metadata catalog: %v", err)), nil
	}

	// Resolve the target against the workspace root, matching the CLI
	// (core.WriteCommandMap(*cwd, ...)). Resolving against .backlogit caused the
	// same workspace-relative target to land under a different root than the CLI
	// and made valid workspace-relative paths fail the containment check.
	writtenPath, err := core.WriteCommandMap(s.RootPath, targetPath, catalog, format)
	if err != nil {
		return InternalError(fmt.Sprintf("export command map: %v", err)), nil
	}
	return toolResultJSON(map[string]string{
		"path":   writtenPath,
		"format": normalizeCommandMapFormat(format),
		"status": "written",
	})
}

func normalizeCommandMapFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "markdown", "md":
		return "markdown"
	default:
		return "json"
	}
}
