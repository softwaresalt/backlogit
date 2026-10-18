package contract_test

// checkpoint_create_opt_in_test.go — 171.006-T: MCP surface for the
// migration-only allow_legacy_import opt-in on backlogit_create_checkpoint.
// Handlers are dispatched through the REGISTERED tool (callToolForTest) with a
// separate t.TempDir() workspace per surface.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
)

const (
	// inWindowLegacyDump is schema-less but otherwise V1-valid.
	inWindowLegacyDump = `{"agent":"ship","session_id":"legacy-optin","phase":"build"}`
	// outOfWindowLegacyDump lacks session_id, so it can never be upgraded.
	outOfWindowLegacyDump = `{"agent":"ship","phase":"build"}`
)

func checkpointFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".backlogit", "checkpoints"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func errorBody(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.True(t, result.IsError)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))
	return body
}

func TestU6_MCPCreateWithoutOptInRejectsSchemaless(t *testing.T) {
	s, root := setupRealMCPServerWithRoot(t)
	result, err := callToolForTest(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump": inWindowLegacyDump,
	})
	require.NoError(t, err)
	body := errorBody(t, result)
	assert.Equal(t, "validation_failed", body["error"])
	assert.Equal(t, "checkpoint_schema_rejected", body["code"])
	assert.Equal(t, true, body["retry_with_opt_in"])
	assert.Equal(t, "allow_legacy_import", body["remediation"])
	assert.Empty(t, checkpointFiles(t, root), "rejected create must write no file")
}

func TestU6_MCPCreateExplicitFalseIsStrict(t *testing.T) {
	s, root := setupRealMCPServerWithRoot(t)
	result, err := callToolForTest(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump":          inWindowLegacyDump,
		"allow_legacy_import": false,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Empty(t, checkpointFiles(t, root))
}

func TestU6_MCPCreateWithOptInImportsReadableV1(t *testing.T) {
	s, _ := setupRealMCPServerWithRoot(t)
	data := callToolAndParseJSON(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump":          inWindowLegacyDump,
		"allow_legacy_import": true,
	})
	path, _ := data["path"].(string)
	require.NotEmpty(t, path)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	cp, err := events.ParseCheckpoint(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, cp.SchemaVersion)
	assert.Equal(t, "legacy-optin", cp.SessionID)
}

func TestU6_MCPCreateOptInOutOfWindowStillUnsupported(t *testing.T) {
	s, root := setupRealMCPServerWithRoot(t)
	result, err := callToolForTest(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump":          outOfWindowLegacyDump,
		"allow_legacy_import": true,
	})
	require.NoError(t, err)
	body := errorBody(t, result)
	assert.Equal(t, "checkpoint_schema_unsupported", body["code"])
	assert.NotEqual(t, true, body["retry_with_opt_in"])
	assert.Empty(t, checkpointFiles(t, root))
}

func TestU6_ToolMetadataAdvertisesOptIn(t *testing.T) {
	s := setupRealMCPServer(t)
	ctx := context.Background()
	c, err := client.NewInProcessClient(s.MCPServer())
	require.NoError(t, err)
	require.NoError(t, c.Start(ctx))
	defer c.Close() //nolint:errcheck
	initReq := mcplib.InitializeRequest{}
	initReq.Params.ClientInfo = mcplib.Implementation{Name: "test", Version: "0.0.1"}
	initReq.Params.ProtocolVersion = mcplib.LATEST_PROTOCOL_VERSION
	_, err = c.Initialize(ctx, initReq)
	require.NoError(t, err)
	list, err := c.ListTools(ctx, mcplib.ListToolsRequest{})
	require.NoError(t, err)

	var found *mcplib.Tool
	for i := range list.Tools {
		if list.Tools[i].Name == "backlogit_create_checkpoint" {
			found = &list.Tools[i]
		}
	}
	require.NotNil(t, found)
	prop, ok := found.InputSchema.Properties["allow_legacy_import"].(map[string]any)
	require.True(t, ok, "allow_legacy_import must be advertised")
	assert.Equal(t, "boolean", prop["type"])
	assert.NotContains(t, found.InputSchema.Required, "allow_legacy_import")
	assert.NotContains(t, found.Description, "written verbatim")
	assert.Contains(t, found.Description, "allow_legacy_import")
}

func TestU6_RegistryListsOptInParamButKeepsCLIStrict(t *testing.T) {
	raw, err := os.ReadFile(findRegistry(t))
	require.NoError(t, err)
	var reg map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &reg))
	op := reg["operations"].(map[string]any)["create_checkpoint"].(map[string]any)
	params, _ := op["params"].(map[string]any)
	assert.Contains(t, params, "allow_legacy_import")
	assert.Contains(t, params, "state_dump")
	cmd, _ := op["cli_command"].(string)
	assert.False(t, strings.Contains(cmd, "--allow-legacy-import"),
		"base cli_command must stay strict-by-default: %q", cmd)
}

// TestU6_OptInParityAcrossSurfaces dispatches the flag only in the explicit
// opt-in scenario and compares observable results across CLI and MCP using a
// separate workspace per surface; default (no opt-in) parity is on error class.
func TestU6_OptInParityAcrossSurfaces(t *testing.T) {
	newCLIRoot := func() string {
		root := t.TempDir()
		bd := filepath.Join(root, ".backlogit")
		require.NoError(t, os.MkdirAll(bd, 0o755))
		require.NoError(t, config.WriteDefaults(bd))
		return root
	}
	runCLI := func(root string, args ...string) (string, error) {
		cmd := cli.NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append([]string{"--cwd", root}, args...))
		err := cmd.Execute()
		return out.String(), err
	}

	t.Run("default rejects on both", func(t *testing.T) {
		cliRoot := newCLIRoot()
		_, err := runCLI(cliRoot, "checkpoint", "create", "--state-dump", inWindowLegacyDump)
		require.Error(t, err)
		s, mcpRoot := setupRealMCPServerWithRoot(t)
		res, mErr := callToolForTest(t, s, "backlogit_create_checkpoint", map[string]any{"state_dump": inWindowLegacyDump})
		require.NoError(t, mErr)
		require.True(t, res.IsError)
		assert.Empty(t, checkpointFiles(t, cliRoot))
		assert.Empty(t, checkpointFiles(t, mcpRoot))
	})

	t.Run("explicit opt-in imports on both", func(t *testing.T) {
		cliRoot := newCLIRoot()
		out, err := runCLI(cliRoot, "checkpoint", "create", "--state-dump", inWindowLegacyDump, "--allow-legacy-import")
		require.NoError(t, err)
		var cliResp map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &cliResp))
		cliRaw, err := os.ReadFile(cliResp["path"].(string))
		require.NoError(t, err)
		cliCP, err := events.ParseCheckpoint(cliRaw)
		require.NoError(t, err)

		s, _ := setupRealMCPServerWithRoot(t)
		data := callToolAndParseJSON(t, s, "backlogit_create_checkpoint", map[string]any{
			"state_dump": inWindowLegacyDump, "allow_legacy_import": true,
		})
		mcpRaw, err := os.ReadFile(data["path"].(string))
		require.NoError(t, err)
		mcpCP, err := events.ParseCheckpoint(mcpRaw)
		require.NoError(t, err)

		assert.Equal(t, cliCP.Agent, mcpCP.Agent)
		assert.Equal(t, cliCP.SessionID, mcpCP.SessionID)
		assert.Equal(t, cliCP.Phase, mcpCP.Phase)
		assert.Equal(t, cliCP.Status, mcpCP.Status)
		assert.Equal(t, 1, mcpCP.SchemaVersion)
	})
}
