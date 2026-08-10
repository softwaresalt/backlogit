package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestDoctorMCP_CheckPartialMutationsParamRegistered(t *testing.T) {
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

	_, ok := toolDef.InputSchema.Properties["check_partial_mutations"]
	assert.True(t, ok, "backlogit_doctor must advertise check_partial_mutations")
}

func TestHandleDoctor_CheckPartialMutationsReturnsFindings(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Commit feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Commit task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	_, err = ws.DB.ExecContext(ctx,
		`INSERT INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
		task.ID, "abc123def", "feat: track commit", "test@example.com",
	)
	require.NoError(t, err)

	result, err := s.handleDoctor(ctx, contractRequest(map[string]any{
		"check_partial_mutations": true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "doctor should return a report, not an MCP error")

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)

	var report struct {
		Findings []struct {
			Type       string `json:"type"`
			ArtifactID string `json:"artifact_id"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &report))

	found := false
	for _, finding := range report.Findings {
		if finding.Type == string(core.FindingPartialCommitAssociation) && finding.ArtifactID == task.ID {
			found = true
		}
	}
	assert.True(t, found, "doctor MCP must surface partial mutation findings when enabled")
}
