package integration_test

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/toolchain"
)

const u12Protocol = "backlogit-mcp-ipc/v1"

func u12Write(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func u12SHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func u12FullManifest(t *testing.T, root string) *toolchain.ToolchainManifest {
	t.Helper()
	const (
		backlogitBin = "U12 inert backlogit binary fixture\n"
		engramBin    = "U12 inert engram binary fixture\n"
		kbdbBin      = "U12 inert kbdb binary fixture\n"
		bunBin       = "U12 inert bun binary fixture\n"
		bunxBin      = "U12 inert bunx binary fixture\n"
		githubBin    = "U12 inert github-mcp-server binary fixture\n"
		shimScript   = "// U12 inert graphtor-mcp-shim fixture\n"
		context7JS   = "// U12 inert context7-mcp entrypoint fixture\n"
		tavilyJS     = "// U12 inert tavily-mcp entrypoint fixture\n"
		installPS1   = "# U12 inert tools/mcp/install.ps1 fixture\n"
		bunLock      = "context7 4.4.4 dist/index.js\ntavily 0.2.22 build/index.js\n"
	)

	if _, statErr := os.Stat(filepath.Join(root, ".backlogit", "config.yaml")); errors.Is(statErr, os.ErrNotExist) {
		u12Write(t, root, ".backlogit/config.yaml", "{}\n")
	}
	u12Write(t, root, "bin/backlogit.exe", backlogitBin)
	u12Write(t, root, "bin/engram.exe", engramBin)
	u12Write(t, root, "bin/kbdb.exe", kbdbBin)
	u12Write(t, root, "bin/bun.exe", bunBin)
	u12Write(t, root, "bin/bunx.exe", bunxBin)
	u12Write(t, root, "bin/github-mcp-server.exe", githubBin)
	u12Write(t, root, ".copilot/graphtor-mcp-shim.cjs", shimScript)
	u12Write(t, root, "tools/mcp/install.ps1", installPS1)
	u12Write(t, root, "tools/mcp/bun.lock", bunLock)
	u12Write(t, root, "tools/mcp/node_modules/@upstash/context7-mcp/dist/index.js", context7JS)
	u12Write(t, root, "tools/mcp/node_modules/tavily-mcp/build/index.js", tavilyJS)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".copilot"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".engram"), 0o755))

	u12Write(t, root, "start.ps1", `# operator-owned fixture
$workspaceRoot = $PSScriptRoot
$workspaceBinDirectory = Join-Path $workspaceRoot "bin"
$backlogitPath = Join-Path $workspaceBinDirectory "backlogit.exe"
if (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw "Missing ./bin/backlogit.exe" }
$env:COPILOT_HOME = Join-Path $workspaceRoot ".copilot"
$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot ".engram"
& $backlogitPath --cwd $workspaceRoot mcp status
`)
	u12Write(t, root, "start.sh", `#!/bin/sh
workspace_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace_bin_directory="$workspace_root/bin"
backlogit_path="$workspace_bin_directory/backlogit.exe"
if [ ! -f "$backlogit_path" ]; then echo "Missing ./bin/backlogit.exe" >&2; exit 1; fi
export COPILOT_HOME="$workspace_root/.copilot"
export ENGRAM_DATA_DIR="$workspace_root/.engram"
"$backlogit_path" --cwd "$workspace_root" mcp status
`)

	return &toolchain.ToolchainManifest{
		SchemaVersion:              "1.0",
		ContainmentLevel:           toolchain.Level1WorkspaceLocalPinned,
		OSContainerSandboxDeferred: true,
		WorkspaceRoot:              root,
		Tools: []toolchain.ToolEntry{
			{
				ID:                   "backlogit",
				Kind:                 "mcp",
				ContainmentClass:     toolchain.WorkspaceLocalPinned,
				ExecutablePath:       "./bin/backlogit.exe",
				PinnedVersion:        "1.0.0",
				SHA256:               u12SHA256(backlogitBin),
				WorkspaceBindingArgs: []string{"--cwd", root},
			},
			{
				ID:                   "engram",
				Kind:                 "mcp",
				ContainmentClass:     toolchain.WorkspaceLocalPinned,
				ExecutablePath:       "./bin/engram.exe",
				PinnedVersion:        "0.3.0-rc.1",
				SHA256:               u12SHA256(engramBin),
				WorkspaceBindingArgs: []string{"ENGRAM_DATA_DIR=.engram"},
			},
			{
				ID:               "graphtor-docs",
				Kind:             "mcp",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/kbdb.exe",
				PinnedVersion:    "0.3.1",
				SHA256:           u12SHA256(kbdbBin),
			},
			{
				ID:               "context7",
				Kind:             "mcp",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   "tools/mcp/node_modules/@upstash/context7-mcp/dist/index.js",
				PinnedVersion:    "4.4.4",
				SHA256:           u12SHA256(bunBin),
				LockfilePath:     "tools/mcp/bun.lock",
			},
			{
				ID:               "tavily",
				Kind:             "mcp",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   "tools/mcp/node_modules/tavily-mcp/build/index.js",
				PinnedVersion:    "0.2.22",
				SHA256:           u12SHA256(bunBin),
				LockfilePath:     "tools/mcp/bun.lock",
			},
			{
				ID:               "github",
				Kind:             "mcp",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/github-mcp-server.exe",
				PinnedVersion:    "1.5.0",
				SHA256:           u12SHA256(githubBin),
			},
			{
				ID:               "bun",
				Kind:             "tool",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   "./tools/mcp/install.ps1",
				PinnedVersion:    "1.4.0",
				SHA256:           u12SHA256(bunBin),
			},
			{
				ID:               "bunx",
				Kind:             "tool",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bunx.exe",
				EntrypointPath:   "./tools/mcp/install.ps1",
				PinnedVersion:    "1.4.0",
				SHA256:           u12SHA256(bunxBin),
			},
			{
				ID:               "graphtor-mcp-shim",
				Kind:             "tool",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   ".copilot/graphtor-mcp-shim.cjs",
				PinnedVersion:    "1.4.0",
				SHA256:           u12SHA256(bunBin),
			},
			{
				ID:               "agent-host",
				Kind:             "agent",
				ContainmentClass: toolchain.BuiltInHost,
				Reason:           "Provided by the agent host, not a local executable",
			},
		},
		HostPrerequisites: []toolchain.HostPrerequisiteEntry{
			{ID: "pwsh", Command: "pwsh", Purpose: "workspace bootstrap", Required: true, Reason: "host PowerShell runtime"},
			{ID: "bash", Command: "bash", Purpose: "workspace bootstrap", Required: false, Reason: "host POSIX shell runtime"},
			{ID: "git", Command: "git", Purpose: "version control", Required: true, Reason: "host git CLI"},
			{ID: "gh", Command: "gh", Purpose: "PR automation", Required: false, Reason: "host GitHub CLI"},
			{ID: "go", Command: "go", Purpose: "Go toolchain", Required: true, Reason: "host Go compiler"},
			{ID: "copilot", Command: "copilot", Purpose: "agent host", Required: false, Reason: "host Copilot CLI"},
		},
	}
}

func u12InitBacklogitWorkspace(t *testing.T, root string) {
	t.Helper()
	cmd := cli.NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--cwd", root, "init"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.NoError(t, os.Rename(filepath.Join(root, ".backlog"), filepath.Join(root, ".backlogit")))
}

type u12DaemonHandle struct {
	root   string
	status cli.MCPDaemonStatus
	cancel context.CancelFunc
	done   chan error
}

func u12StartDaemon(t *testing.T, root string) *u12DaemonHandle {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- cli.RunMCPDaemon(ctx, root, false, time.Minute)
	}()
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		// Best-effort shutdown in cleanup; assertion below verifies daemon exit.
		_, _ = cli.StopMCPDaemon(stopCtx, root)
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("daemon for %s exited with error: %v", root, err)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("daemon for %s did not exit cleanly", root)
		}
	})

	var status cli.MCPDaemonStatus
	require.Eventually(t, func() bool {
		st, err := cli.InspectMCPDaemonStatus(root)
		if err != nil || !st.Running || st.Endpoint == "" {
			return false
		}
		status = st
		return true
	}, 10*time.Second, 25*time.Millisecond, "daemon for %s did not become ready", root)

	return &u12DaemonHandle{root: root, status: status, cancel: cancel, done: done}
}

type u12IPCResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func u12ConnectSession(t *testing.T, d *u12DaemonHandle) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := cli.DialMCPDaemonIPC(t.Context(), d.status.IPCTransport, d.status.Endpoint, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Best-effort connection close on test teardown.
		_ = conn.Close()
	})

	header := fmt.Sprintf("{\"protocol_version\":%q,\"command\":\"session\",\"workspace_root\":%q,\"storage_root\":%q,\"version\":%q}\n",
		u12Protocol, d.status.WorkspaceRoot, d.status.StorageRoot, d.status.Version)
	_, err = io.WriteString(conn, header)
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"u12-test","version":"1.0"}}}` + "\n"
	_, err = io.WriteString(conn, initReq)
	require.NoError(t, err)

	resp := u12ReadResponse(t, conn, reader, 1)
	require.Empty(t, resp.Error, "initialize must succeed: %s", string(resp.Error))

	_, err = io.WriteString(conn, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n")
	require.NoError(t, err)
	return conn, reader
}

func u12ReadResponse(t *testing.T, conn net.Conn, reader *bufio.Reader, id int) u12IPCResponse {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	defer func() {
		// Clear read deadline after reading the target response.
		_ = conn.SetReadDeadline(time.Time{})
	}()
	for {
		line, err := reader.ReadBytes('\n')
		require.NoError(t, err)
		var resp u12IPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID == id {
			return resp
		}
	}
}

func u12CallTool(t *testing.T, conn net.Conn, reader *bufio.Reader, id int, name string, args map[string]any) u12IPCResponse {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	_, err = conn.Write(append(payload, '\n'))
	require.NoError(t, err)
	return u12ReadResponse(t, conn, reader, id)
}

// TestU12_WriteToolchainManifestAndRenderPinnedMCPConfigProducesZeroViolations verifies R19-R23 and SC11-SC13.
func TestU12_WriteToolchainManifestAndRenderPinnedMCPConfigProducesZeroViolations(t *testing.T) {
	repoRoot := testRepoRoot(t)
	committedPath := filepath.Join(repoRoot, ".backlogit", "toolchain.yaml")
	_, statErr := os.Stat(committedPath)
	require.NoError(t, statErr, "canonical tracked .backlogit/toolchain.yaml must exist")

	t.Chdir(repoRoot)
	committed, err := toolchain.LoadToolchainManifest(repoRoot)
	require.NoError(t, err, "committed .backlogit/toolchain.yaml must load cleanly")
	assert.Equal(t, "1.0", committed.SchemaVersion)
	assert.Equal(t, toolchain.Level1WorkspaceLocalPinned, committed.ContainmentLevel)
	assert.True(t, committed.OSContainerSandboxDeferred)

	seenTools := make(map[string]toolchain.ToolContainmentClass)
	for _, entry := range committed.Tools {
		seenTools[entry.ID] = entry.ContainmentClass
	}
	for _, requiredID := range []string{
		"backlogit", "engram", "graphtor-docs", "context7", "tavily",
		"github", "bun", "bunx", "graphtor-mcp-shim", "agent-host",
	} {
		_, ok := seenTools[requiredID]
		assert.True(t, ok, "committed manifest must account for %q", requiredID)
	}
	assert.Equal(t, toolchain.BuiltInHost, seenTools["agent-host"])

	seenPrereqs := make(map[string]bool)
	for _, prereq := range committed.HostPrerequisites {
		seenPrereqs[prereq.ID] = true
	}
	for _, requiredPrereq := range []string{"pwsh", "bash", "git", "gh", "go", "copilot"} {
		assert.True(t, seenPrereqs[requiredPrereq], "committed manifest must declare host prerequisite %q", requiredPrereq)
	}

	fixtureRoot := t.TempDir()
	t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
	manifest := u12FullManifest(t, fixtureRoot)
	manifest.Tools = append(manifest.Tools, toolchain.ToolEntry{
		ID:               "remote-api",
		Kind:             "mcp",
		ContainmentClass: toolchain.RemoteDeclared,
		RemotePolicy:     "approved remote service endpoint; zero local execution",
	})

	err = toolchain.WriteToolchainManifest(fixtureRoot, manifest)
	require.NoError(t, err, "WriteToolchainManifest must succeed for valid manifest")

	loaded, err := toolchain.LoadToolchainManifest(fixtureRoot)
	require.NoError(t, err)
	assert.Equal(t, len(manifest.Tools), len(loaded.Tools))

	rendered, err := toolchain.RenderPinnedMCPConfig(fixtureRoot, manifest)
	require.NoError(t, err, "RenderPinnedMCPConfig must succeed for valid manifest")
	var mcpCfg struct {
		Servers map[string]struct {
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			URL     string            `json:"url"`
		} `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal(rendered, &mcpCfg))
	require.Contains(t, mcpCfg.Servers, "backlogit")
	assert.Equal(t, "./bin/backlogit.exe", mcpCfg.Servers["backlogit"].Command)
	for name, srv := range mcpCfg.Servers {
		if srv.Command != "" {
			assert.True(t, strings.HasPrefix(srv.Command, "./bin/"),
				"server %s command %q must be pinned under ./bin/", name, srv.Command)
		}
	}

	u12Write(t, fixtureRoot, ".mcp.json", string(rendered))

	report, err := toolchain.AuditRepositoryToolchain(t.Context(), fixtureRoot)
	require.NoError(t, err)
	assert.True(t, report.Valid, "provisioned workspace must report zero violations: %+v", report.Tools)
	assert.Zero(t, report.Violations)
	assert.Equal(t, toolchain.Level1WorkspaceLocalPinned, report.ContainmentLevel)
	assert.True(t, report.OSContainerSandboxDeferred)
}

// TestU12_MultiWorkspaceAllToolsIsolationAndCrossAttachRefusal verifies SC7, SC10, and SC14.
func TestU12_MultiWorkspaceAllToolsIsolationAndCrossAttachRefusal(t *testing.T) {
	t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
	wsA := t.TempDir()
	wsB := t.TempDir()

	u12InitBacklogitWorkspace(t, wsA)
	u12InitBacklogitWorkspace(t, wsB)

	manifestA := u12FullManifest(t, wsA)
	manifestB := u12FullManifest(t, wsB)
	require.NoError(t, toolchain.WriteToolchainManifest(wsA, manifestA), "WriteToolchainManifest must succeed for Workspace A")
	require.NoError(t, toolchain.WriteToolchainManifest(wsB, manifestB), "WriteToolchainManifest must succeed for Workspace B")

	cfgA, err := toolchain.RenderPinnedMCPConfig(wsA, manifestA)
	require.NoError(t, err, "RenderPinnedMCPConfig must succeed for Workspace A")
	cfgB, err := toolchain.RenderPinnedMCPConfig(wsB, manifestB)
	require.NoError(t, err, "RenderPinnedMCPConfig must succeed for Workspace B")
	u12Write(t, wsA, ".mcp.json", string(cfgA))
	u12Write(t, wsB, ".mcp.json", string(cfgB))

	reportA, err := toolchain.AuditRepositoryToolchain(t.Context(), wsA)
	require.NoError(t, err)
	assert.True(t, reportA.Valid)
	reportB, err := toolchain.AuditRepositoryToolchain(t.Context(), wsB)
	require.NoError(t, err)
	assert.True(t, reportB.Valid)

	dA := u12StartDaemon(t, wsA)
	dB := u12StartDaemon(t, wsB)
	assert.NotEqual(t, dA.status.Endpoint, dB.status.Endpoint, "workspaces must have distinct IPC endpoints")
	assert.NotEqual(t, dA.status.LockPath, dB.status.LockPath, "workspaces must have distinct lock files")

	connA, readerA := u12ConnectSession(t, dA)
	connB, readerB := u12ConnectSession(t, dB)

	createResp := u12CallTool(t, connA, readerA, 2, "backlogit_create_item", map[string]any{
		"title":         "Workspace A Isolated Task",
		"artifact_type": "task",
	})
	require.Empty(t, createResp.Error, "create in Workspace A must succeed: %s", string(createResp.Error))

	listB := u12CallTool(t, connB, readerB, 2, "backlogit_list_items", map[string]any{})
	require.Empty(t, listB.Error, "list in Workspace B must succeed: %s", string(listB.Error))
	assert.NotContains(t, string(listB.Result), "Workspace A Isolated Task", "Workspace B must never observe Workspace A backlog state")

	// Cross-workspace daemon attachment refusal.
	rawInfoA, err := os.ReadFile(filepath.Join(wsA, ".backlogit", "runtime", "mcp-daemon.json"))
	require.NoError(t, err)
	var infoA cli.MCPDaemonInfo
	require.NoError(t, json.Unmarshal(rawInfoA, &infoA))

	crossErr := cli.VerifyMCPDaemonAttachment(infoA, wsB, filepath.Join(wsB, ".backlogit"), infoA.Version, infoA.ProtocolVersion)
	require.ErrorIs(t, crossErr, corerrors.ErrDaemonWorkspaceMismatch, "attaching Workspace A daemon to Workspace B must be refused")

	// Cross-workspace toolchain manifest/config refusal.
	crossManifest := u12FullManifest(t, wsA)
	crossManifest.Tools[0].ExecutablePath = filepath.Join(wsB, "bin", "backlogit.exe")
	_, crossRenderErr := toolchain.RenderPinnedMCPConfig(wsA, crossManifest)
	require.Error(t, crossRenderErr, "cross-workspace executable path must be refused in RenderPinnedMCPConfig")

	crossWriteErr := toolchain.WriteToolchainManifest(wsA, crossManifest)
	require.Error(t, crossWriteErr, "cross-workspace executable path must be refused in WriteToolchainManifest")
}

// TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment verifies SC8, SC9, SC12, and SC14.
func TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment(t *testing.T) {
	t.Run("missing_local_binary_refuses_global_path_fallback", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
		u12InitBacklogitWorkspace(t, root)

		fakePathDir := t.TempDir()
		binName := "backlogit"
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		u12Write(t, fakePathDir, binName, "not a real binary\n")
		t.Setenv("PATH", fakePathDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err := cli.PrewarmMCPDaemon(t.Context(), root, false, time.Minute)
		require.ErrorIs(t, err, corerrors.ErrDaemonBinaryMissing, "missing workspace bin/backlogit must fail closed without PATH fallback")
	})

	t.Run("write_and_render_refuse_traversal_external_and_symlink_escapes", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")

		manifest := u12FullManifest(t, root)
		require.NoError(t, toolchain.WriteToolchainManifest(root, manifest), "WriteToolchainManifest must succeed for valid workspace")

		for _, badCase := range []struct {
			name   string
			mutate func(m *toolchain.ToolchainManifest)
		}{
			{
				name: "traversal-executable",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.Tools[0].ExecutablePath = "../outside/bin/backlogit.exe"
				},
			},
			{
				name: "external-executable",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.Tools[0].ExecutablePath = filepath.Join(outside, "bin", "backlogit.exe")
				},
			},
			{
				name: "traversal-entrypoint",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.Tools[3].EntrypointPath = "../../outside/index.js"
				},
			},
			{
				name: "hyphen-entrypoint-flag-injection",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.Tools[3].EntrypointPath = "-e"
				},
			},
			{
				name: "remote-id-url-injection",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.Tools = append(m.Tools, toolchain.ToolEntry{
						ID:               "169.254.169.254/latest/#",
						Kind:             "mcp",
						ContainmentClass: toolchain.RemoteDeclared,
						RemotePolicy:     "injected",
					})
				},
			},
			{
				name: "workspace-root-mismatch",
				mutate: func(m *toolchain.ToolchainManifest) {
					m.WorkspaceRoot = outside
				},
			},
		} {
			t.Run(badCase.name, func(t *testing.T) {
				m := u12FullManifest(t, root)
				badCase.mutate(m)
				assert.ErrorIs(t, toolchain.WriteToolchainManifest(root, m), corerrors.ErrValidation)
				_, err := toolchain.RenderPinnedMCPConfig(root, m)
				assert.ErrorIs(t, err, corerrors.ErrValidation)
			})
		}

		// Symlink / junction escape on .backlogit/toolchain.yaml target.
		target := filepath.Join(outside, "escaped-toolchain.yaml")
		require.NoError(t, os.WriteFile(target, []byte("escaped\n"), 0o600))
		yamlPath := filepath.Join(root, ".backlogit", "toolchain.yaml")
		require.NoError(t, os.Remove(yamlPath))
		require.NoError(t, os.Symlink(target, yamlPath), "symlink capability is required for containment test")

		err := toolchain.WriteToolchainManifest(root, manifest)
		assert.ErrorIs(t, err, corerrors.ErrValidation, "WriteToolchainManifest must refuse symlinked toolchain.yaml")
	})

	t.Run("mcp_tool_rejects_path_traversal_and_external_reference", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
		u12InitBacklogitWorkspace(t, root)

		d := u12StartDaemon(t, root)
		conn, reader := u12ConnectSession(t, d)

		escapedPath := filepath.Join(outside, "escape.md")
		resp := u12CallTool(t, conn, reader, 2, "backlogit_export_command_map", map[string]any{
			"path": "../outside/escape.md",
		})
		if len(resp.Error) == 0 {
			var toolRes struct {
				IsError bool `json:"isError"`
			}
			require.NoError(t, json.Unmarshal(resp.Result, &toolRes))
			assert.True(t, toolRes.IsError, "relative path traversal via MCP tool must be rejected")
		}

		absResp := u12CallTool(t, conn, reader, 3, "backlogit_export_command_map", map[string]any{
			"path": escapedPath,
		})
		if len(absResp.Error) == 0 {
			var toolRes struct {
				IsError bool `json:"isError"`
			}
			require.NoError(t, json.Unmarshal(absResp.Result, &toolRes))
			assert.True(t, toolRes.IsError, "external absolute path via MCP tool must be rejected")
		}
		_, statErr := os.Stat(escapedPath)
		assert.ErrorIs(t, statErr, os.ErrNotExist, "MCP tool must never write outside workspace root")
	})
}
