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

	"github.com/softwaresalt/backlogit/internal/core"
)

// 143.009-T (Unit 9): MCP contract tests for the shipped-event reconciliation
// audit parameter, mirroring internal/mcp/doctor_partial_mutations_test.go.
// check_partial_mutations and check_workspace_root_conflict are the verified
// precedent for exposing a read-only doctor check on MCP; CheckGateEvidence is
// NOT a precedent because it is CLI-only.
//
// These scenarios were written and observed failing before the parameter was
// wired: the InputSchema lacked the property and the handler ignored it.

func TestDoctorMCP_CheckShippedEventCompletenessParamRegistered(t *testing.T) {
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

	_, ok := toolDef.InputSchema.Properties["check_shipped_event_completeness"]
	assert.True(t, ok, "backlogit_doctor must advertise check_shipped_event_completeness")
}

func TestHandleDoctor_CheckShippedEventCompletenessReturnsFindings(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	seedMCPShippedEventFixture(t, ws.RootPath)

	result, err := s.handleDoctor(ctx, contractRequest(map[string]any{
		"check_shipped_event_completeness": true,
		"check_orphans":                    false,
		"check_duplicates":                 false,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "doctor should return a report, not an MCP error")

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)

	var report struct {
		Findings []struct {
			Type        string `json:"type"`
			ArtifactID  string `json:"artifact_id"`
			Description string `json:"description"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &report))

	types := make(map[string]string)
	for _, finding := range report.Findings {
		types[finding.Type] = finding.ArtifactID
	}
	assert.Equal(t, "900-S", types[string(core.FindingMissingShippedEvent)],
		"MCP must render the missing_shipped_event finding")
	assert.Equal(t, "901-S", types[string(core.FindingShippedUnarchivedResidue)],
		"MCP must render the shipped_unarchived_residue finding")
}

func TestHandleDoctor_ShippedEventCompletenessIsOffByDefault(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	seedMCPShippedEventFixture(t, ws.RootPath)

	result, err := s.handleDoctor(ctx, contractRequest(map[string]any{
		"check_orphans":    false,
		"check_duplicates": false,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	assert.NotContains(t, text.Text, string(core.FindingMissingShippedEvent))
	assert.NotContains(t, text.Text, string(core.FindingShippedUnarchivedResidue))
}

// seedMCPShippedEventFixture writes the shared fixture used by both the MCP
// contract tests here and the CLI-versus-MCP parity fixture in internal/cli.
func seedMCPShippedEventFixture(t *testing.T, root string) {
	t.Helper()
	queue := filepath.Join(root, ".backlogit", "queue")
	archive := filepath.Join(root, ".backlogit", "archive")
	require.NoError(t, os.MkdirAll(queue, 0o755))
	require.NoError(t, os.MkdirAll(archive, 0o755))

	archivedShipment := `---
id: 900-S
title: Archived shipment without shipped event
artifact_type: shipment
status: archived
archived_status: shipped
archived_from: .backlogit/queue/900-S.md
level: 1
custom_fields:
    items:
        - 900.001-T
---

# Archived shipment without shipped event
`
	require.NoError(t, os.WriteFile(filepath.Join(archive, "900-S.md"), []byte(archivedShipment), 0o644))

	residueShipment := `---
id: 901-S
title: Shipped but unarchived shipment
artifact_type: shipment
status: shipped
level: 1
custom_fields:
    items:
        - 901.001-T
---

# Shipped but unarchived shipment
`
	require.NoError(t, os.WriteFile(filepath.Join(queue, "901-S.md"), []byte(residueShipment), 0o644))

	member := `---
id: 901.001-T
title: Stranded release scope task
artifact_type: task
status: done
parent_id: 901-F
level: 2
---

# Stranded release scope task
`
	require.NoError(t, os.WriteFile(filepath.Join(queue, "901.001-T.md"), []byte(member), 0o644))
}
