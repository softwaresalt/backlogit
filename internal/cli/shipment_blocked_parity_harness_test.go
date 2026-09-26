package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

func TestUR8_BlockAndUnblockSurfaceContractsAreIsomorphic(t *testing.T) {
	ws := setupShipmentMCPWorkspace(t)
	server := mcpinternal.NewServer(ws)

	blockCmd := ur8ShipmentSubcommand("block")
	unblockCmd := ur8ShipmentSubcommand("unblock")
	blockTool, hasBlockTool := ur8ToolDef(server, "backlogit_block_shipment")
	unblockTool, hasUnblockTool := ur8ToolDef(server, "backlogit_unblock_shipment")

	blockReady := assert.NotNil(t, blockCmd,
		"R8 RED: CLI shipment block command must exist and delegate to core.BlockShipment")
	blockReady = assert.True(t, hasBlockTool,
		"R8 RED: MCP backlogit_block_shipment tool must exist and delegate to core.BlockShipment") && blockReady
	unblockReady := assert.NotNil(t, unblockCmd,
		"R8 RED: CLI shipment unblock command must exist and delegate to core.UnblockShipment")
	unblockReady = assert.True(t, hasUnblockTool,
		"R8 RED: MCP backlogit_unblock_shipment tool must exist and delegate to core.UnblockShipment") && unblockReady
	if !blockReady || !unblockReady {
		return
	}

	assert.Equal(t,
		[]string{"by", "reason", "resume-checkpoint"},
		ur8FlagNames(blockCmd),
		"R8 RED: CLI block flags must map 1:1 to BlockOptions without surface-only defaults",
	)
	assert.Equal(t,
		[]string{"by", "id", "reason", "resume_checkpoint_ref"},
		ur8ToolPropertyNames(blockTool),
		"R8 RED: MCP block parameters must map 1:1 to BlockOptions",
	)
	assert.Equal(t,
		[]string{"by", "confirm", "to"},
		ur8FlagNames(unblockCmd),
		"R8 RED: CLI unblock flags must map 1:1 to UnblockOptions without surface-only defaults",
	)
	assert.Equal(t,
		[]string{"by", "confirm", "id", "target"},
		ur8ToolPropertyNames(unblockTool),
		"R8 RED: MCP unblock parameters must map 1:1 to UnblockOptions",
	)
}

func TestUR8_BlockAndUnblockProduceIsomorphicBehavior(t *testing.T) {
	probeWS := setupShipmentMCPWorkspace(t)
	probeServer := mcpinternal.NewServer(probeWS)
	blockCmd := ur8ShipmentSubcommand("block")
	unblockCmd := ur8ShipmentSubcommand("unblock")
	_, hasBlockTool := ur8ToolDef(probeServer, "backlogit_block_shipment")
	_, hasUnblockTool := ur8ToolDef(probeServer, "backlogit_unblock_shipment")
	ready := assert.NotNil(t, blockCmd, "R8 RED: CLI shipment block command is missing")
	ready = assert.NotNil(t, unblockCmd, "R8 RED: CLI shipment unblock command is missing") && ready
	ready = assert.True(t, hasBlockTool, "R8 RED: MCP backlogit_block_shipment tool is missing") && ready
	ready = assert.True(t, hasUnblockTool, "R8 RED: MCP backlogit_unblock_shipment tool is missing") && ready
	if !ready {
		return
	}

	cliRoot, cliShipmentID := ur8ActiveShipmentRoot(t)
	mcpRoot, mcpShipmentID := ur8ActiveShipmentRoot(t)
	require.Equal(t, cliShipmentID, mcpShipmentID, "fixtures must allocate isomorphic IDs")

	cliBlockRaw, cliBlockErr := ur8RunCLI(
		cliRoot,
		"shipment", "block", cliShipmentID,
		"--reason", "waiting for prerequisite",
		"--by", "wave-7",
		"--resume-checkpoint", "checkpoint-7.json",
	)
	mcpWS, err := core.NewWorkspace(context.Background(), mcpRoot)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mcpWS.Close() })
	mcpServer := mcpinternal.NewServer(mcpWS)
	mcpBlockRaw, mcpBlockErr := ur8InvokeTool(t, mcpServer, "backlogit_block_shipment", map[string]any{
		"id":                    mcpShipmentID,
		"reason":                "waiting for prerequisite",
		"by":                    "wave-7",
		"resume_checkpoint_ref": "checkpoint-7.json",
	})

	require.NoError(t, cliBlockErr)
	require.NoError(t, mcpBlockErr)
	cliBlock := ur8Object(t, cliBlockRaw)
	mcpBlock := ur8Object(t, mcpBlockRaw)
	assert.Equal(t, ur8LifecycleView(cliBlock), ur8LifecycleView(mcpBlock),
		"R8 RED: identical block inputs must produce isomorphic CLI and MCP results")

	cliUnblockRaw, cliUnblockErr := ur8RunCLI(
		cliRoot,
		"shipment", "unblock", cliShipmentID,
		"--to", "active",
		"--confirm",
		"--by", "wave-7",
	)
	mcpUnblockRaw, mcpUnblockErr := ur8InvokeTool(t, mcpServer, "backlogit_unblock_shipment", map[string]any{
		"id":      mcpShipmentID,
		"target":  "active",
		"confirm": true,
		"by":      "wave-7",
	})

	require.NoError(t, cliUnblockErr)
	require.NoError(t, mcpUnblockErr)
	cliUnblock := ur8Object(t, cliUnblockRaw)
	mcpUnblock := ur8Object(t, mcpUnblockRaw)
	assert.Equal(t, ur8LifecycleView(cliUnblock), ur8LifecycleView(mcpUnblock),
		"R8 RED: identical unblock inputs must produce isomorphic CLI and MCP results")
}

func TestUR8_BlockedStatusHasCLIAndMCPReadListParity(t *testing.T) {
	probeWS := setupShipmentMCPWorkspace(t)
	probeServer := mcpinternal.NewServer(probeWS)
	blockCmd := ur8ShipmentSubcommand("block")
	_, hasBlockTool := ur8ToolDef(probeServer, "backlogit_block_shipment")
	ready := assert.NotNil(t, blockCmd, "R8 RED: CLI shipment block command is missing")
	ready = assert.True(t, hasBlockTool, "R8 RED: MCP backlogit_block_shipment tool is missing") && ready
	if !ready {
		return
	}

	root, shipmentID := ur8ActiveShipmentRoot(t)
	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	_, err = core.BlockShipment(context.Background(), ws, shipmentID, core.BlockOptions{
		Reason:              "read parity",
		BlockedBy:           "wave-7",
		ResumeCheckpointRef: "checkpoint-read.json",
	})
	require.NoError(t, err)

	cliGetRaw, err := ur8RunCLI(root, "shipment", "get", shipmentID)
	require.NoError(t, err)
	cliGet := ur8Object(t, cliGetRaw)
	cliListRaw, err := ur8RunCLI(root, "shipment", "list", "--status", "blocked", "--format", "json")
	require.NoError(t, err)
	cliList := ur8ObjectList(t, cliListRaw)

	server := mcpinternal.NewServer(ws)
	mcpGetRaw, err := ur8InvokeTool(t, server, "backlogit_get_shipment", map[string]any{"id": shipmentID})
	require.NoError(t, err)
	mcpGet := ur8Object(t, mcpGetRaw)
	mcpListRaw, err := ur8InvokeTool(t, server, "backlogit_list_shipments", map[string]any{"status": "blocked"})
	require.NoError(t, err)
	mcpList := ur8ObjectList(t, mcpListRaw)

	assert.Equal(t, "blocked", cliGet["status"],
		"R8 RED: CLI get must surface blocked status")
	assert.Equal(t, cliGet["status"], mcpGet["status"],
		"R8 RED: CLI and MCP get must surface the same blocked status")
	assert.Equal(t, ur8IDs(cliList), ur8IDs(mcpList),
		"R8 RED: CLI and MCP blocked-list membership must be isomorphic")
	assert.Contains(t, ur8IDs(cliList), shipmentID,
		"R8 RED: blocked shipment must appear in blocked list")

	cliActiveRaw, err := ur8RunCLI(root, "shipment", "list", "--status", "active", "--format", "json")
	require.NoError(t, err)
	mcpActiveRaw, err := ur8InvokeTool(t, server, "backlogit_list_shipments", map[string]any{"status": "active"})
	require.NoError(t, err)
	assert.NotContains(t, ur8IDs(ur8ObjectList(t, cliActiveRaw)), shipmentID,
		"R8 RED: blocked shipment must be excluded from CLI active selection")
	assert.NotContains(t, ur8IDs(ur8ObjectList(t, mcpActiveRaw)), shipmentID,
		"R8 RED: blocked shipment must be excluded from MCP active selection")
}

func ur8ShipmentSubcommand(name string) *cobra.Command {
	for _, command := range NewShipmentCmd().Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}

func ur8ToolDef(server *mcpinternal.Server, name string) (mcplib.Tool, bool) {
	for _, tool := range server.ToolDefs() {
		if tool.Name == name {
			return tool, true
		}
	}
	return mcplib.Tool{}, false
}

func ur8FlagNames(command *cobra.Command) []string {
	names := make([]string, 0)
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)
	})
	sort.Strings(names)
	return names
}

func ur8ToolPropertyNames(tool mcplib.Tool) []string {
	names := make([]string, 0, len(tool.InputSchema.Properties))
	for name := range tool.InputSchema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ur8ActiveShipmentRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	feature, err := core.CreateArtifact(context.Background(), ws, "R8 parity feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(context.Background(), ws, "R8 parity member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(context.Background(), ws, "R8 parity shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(context.Background(), ws, shipment.ID)
	require.NoError(t, err)
	require.NoError(t, ws.Close())
	return root, shipment.ID
}

func ur8RunCLI(root string, args ...string) ([]byte, error) {
	command := NewRootCommand()
	stdout := new(bytes.Buffer)
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))
	command.SetArgs(append([]string{"--cwd", root}, args...))
	if err := command.Execute(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

func ur8InvokeTool(t *testing.T, server *mcpinternal.Server, name string, args map[string]any) ([]byte, error) {
	t.Helper()
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = args
	result, err := server.InvokeTool(context.Background(), name, request)
	if err != nil {
		return nil, err
	}
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "MCP response must be text content")
	if result.IsError {
		return []byte(text.Text), assert.AnError
	}
	return []byte(text.Text), nil
}

func ur8Object(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload
}

func ur8LifecycleView(payload map[string]any) map[string]any {
	fields := map[string]any{}
	for _, key := range []string{"id", "status", "blocked_reason", "blocked_by", "resume_checkpoint_ref"} {
		if value, ok := payload[key]; ok {
			fields[key] = value
		}
	}
	if custom, ok := payload["custom_fields"].(map[string]any); ok {
		for _, key := range []string{"blocked_reason", "blocked_by", "resume_checkpoint_ref"} {
			if value, exists := custom[key]; exists {
				fields[key] = value
			}
		}
	}
	return fields
}

func ur8ObjectList(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var result []map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	return result
}

func ur8IDs(items []map[string]any) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id, ok := item["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
