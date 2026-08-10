package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestDoctorMCP_CheckWorkspaceRootConflictParamRegistered(t *testing.T) {
	s, _ := setupBugFixServer(t)

	var toolDef *mcplib.Tool
	for _, def := range s.ToolDefs() {
		def := def
		if def.Name == "backlogit_doctor" {
			toolDef = &def
			break
		}
	}
	require.NotNil(t, toolDef)

	_, ok := toolDef.InputSchema.Properties["check_workspace_root_conflict"]
	assert.True(t, ok, "backlogit_doctor must advertise check_workspace_root_conflict")
}

func TestHandleDoctor_CheckWorkspaceRootConflictWithoutWorkspaceInit(t *testing.T) {
	root := t.TempDir()
	for _, candidate := range []string{".backlog", ".backlogit"} {
		dir := filepath.Join(root, candidate)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, config.WriteDefaults(dir))
	}

	s := NewServerForRoot(root)
	result, err := s.handleDoctor(context.Background(), contractRequest(map[string]any{
		"check_workspace_root_conflict": true,
		"check_orphans":                 false,
		"check_duplicates":              false,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "workspace root conflict check should return a report")

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)

	var report struct {
		Findings []struct {
			Type string `json:"type"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &report))
	require.Len(t, report.Findings, 1)
	assert.Equal(t, "workspace_root_conflict", report.Findings[0].Type)
}
