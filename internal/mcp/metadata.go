package mcp

import (
	"context"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
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

	return core.BuildMetadataCatalog(
		s.Workspace,
		templateInfos,
		registry,
		migration,
		nil,
		s.DescribeTools(),
	)
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

	writtenPath, err := core.WriteCommandMap(s.backlogitDir(), targetPath, catalog, format)
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
