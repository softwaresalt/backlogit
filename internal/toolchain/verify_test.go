package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// u9Workspace keeps both workspaces inside the repository while making them siblings.
func u9Workspace(t *testing.T) (string, string) {
	t.Helper()
	base, err := os.MkdirTemp(".", "u9-fixture-")
	require.NoError(t, err)
	base, err = filepath.Abs(base)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(base)) })
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(a, 0o755))
	require.NoError(t, os.MkdirAll(b, 0o755))
	return a, b
}

func u9Write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func u9Manifest(root string) *ToolchainManifest {
	sum := sha256.Sum256([]byte("pinned binary"))
	return &ToolchainManifest{
		SchemaVersion: "1.0", ContainmentLevel: Level1WorkspaceLocalPinned,
		OSContainerSandboxDeferred: true, WorkspaceRoot: root,
		Tools: []ToolEntry{
			{ID: "backlogit", Kind: "mcp", ContainmentClass: WorkspaceLocalPinned,
				ExecutablePath: "./bin/backlogit.exe", PinnedVersion: "1.0.0",
				SHA256: hex.EncodeToString(sum[:]), WorkspaceBindingArgs: []string{"--cwd", root}},
			{ID: "remote", Kind: "mcp", ContainmentClass: RemoteDeclared, RemotePolicy: "approved"},
			{ID: "host-built-in", Kind: "agent", ContainmentClass: BuiltInHost, Reason: "provided by host"},
		},
		HostPrerequisites: []HostPrerequisiteEntry{
			{ID: "shell", Command: "shell", Purpose: "bootstrap", Reason: "host shell", Required: false},
		},
	}
}

func u9PinnedWorkspace(t *testing.T, root string) *ToolchainManifest {
	t.Helper()
	u9Write(t, root, "bin/backlogit.exe", "pinned binary")
	u9Write(t, root, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./bin/backlogit.exe","args":["mcp","--cwd",%q]}}}`, root))
	return u9Manifest(root)
}

func u9RequireNotStub(t *testing.T, err error) {
	t.Helper()
	require.False(t, errors.Is(err, corerrors.ErrNotImplemented), "TODO: implement toolchain verification: %v", err)
}

func u9RequireRejected(t *testing.T, report *ToolchainVerificationReport, err error) {
	t.Helper()
	u9RequireNotStub(t, err)
	if err != nil {
		return
	}
	require.NotNil(t, report)
	require.False(t, report.Valid, "unsafe workspace must not be reported conformant")
	require.Positive(t, report.Violations)
}

func TestU9_VerifyWorkspaceToolchainAcceptsPinnedLocalToolsAndReportsPrereqsAndRemote(t *testing.T) {
	data, err := os.ReadFile("../../schemas/workspace-toolchain.schema.json")
	require.NoError(t, err)
	var schema struct {
		Schema     string `json:"$schema"`
		Properties struct {
			ContainmentLevel struct {
				Enum []string `json:"enum"`
			} `json:"containment_level"`
		} `json:"properties"`
		Defs struct {
			Tool struct {
				Properties struct {
					ContainmentClass struct {
						Enum []string `json:"enum"`
					} `json:"containment_class"`
				} `json:"properties"`
			} `json:"tool"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(data, &schema))
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema.Schema)
	require.ElementsMatch(t, []string{"level-1-workspace-local-pinned"}, schema.Properties.ContainmentLevel.Enum)
	require.ElementsMatch(t, []string{"workspace-local-pinned", "host-prerequisite", "remote-declared", "built-in-host", "unsupported", "non-conformant"}, schema.Defs.Tool.Properties.ContainmentClass.Enum)
	require.Len(t, schema.Defs.Tool.Properties.ContainmentClass.Enum, 6)

	root, _ := u9Workspace(t)
	manifest := u9PinnedWorkspace(t, root)
	report, err := VerifyWorkspaceToolchain(context.Background(), root, manifest)
	require.NoError(t, err, "TODO: implement conformant workspace verification")
	require.NotNil(t, report)
	require.True(t, report.Valid)
	require.Zero(t, report.Violations)
	require.Equal(t, Level1WorkspaceLocalPinned, report.ContainmentLevel)
	require.True(t, report.OSContainerSandboxDeferred)
	require.NotNil(t, report.Tools)
	require.NotNil(t, report.HostPrerequisites)
	classes := make(map[string]ToolContainmentClass)
	for _, finding := range append(report.Tools, report.HostPrerequisites...) {
		classes[finding.ToolID] = finding.ContainmentClass
	}
	require.Equal(t, WorkspaceLocalPinned, classes["backlogit"])
	require.Equal(t, RemoteDeclared, classes["remote"])
	require.Equal(t, BuiltInHost, classes["host-built-in"])
	require.Equal(t, HostPrerequisite, classes["shell"])
	findings, err := ValidateMCPConfigPinning(root, ".mcp.json", manifest)
	require.NoError(t, err)
	for _, finding := range findings {
		require.NotEqual(t, NonConformant, finding.ContainmentClass)
		require.NotEqual(t, Unsupported, finding.ContainmentClass)
	}
}

func TestU9_VerifyWorkspaceToolchainRejectsGlobalPATHShadowToolsAndCrossWorkspacePaths(t *testing.T) {
	for _, scenario := range []string{"missing-binary", "bare-command", "cross-workspace-binary", "traversal", "wrong-sha256", "missing-pin"} {
		t.Run(scenario, func(t *testing.T) {
			root, other := u9Workspace(t)
			manifest := u9PinnedWorkspace(t, root)
			u9Write(t, other, "bin/backlogit.exe", "pinned binary")
			switch scenario {
			case "missing-binary":
				require.NoError(t, os.Remove(filepath.Join(root, "bin", "backlogit.exe")))
				t.Setenv("PATH", filepath.Join(other, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
			case "bare-command":
				manifest.Tools[0].ExecutablePath = "backlogit"
			case "cross-workspace-binary":
				manifest.Tools[0].ExecutablePath = filepath.Join(other, "bin", "backlogit.exe")
			case "traversal":
				manifest.Tools[0].ExecutablePath = "../b/bin/backlogit.exe"
			case "wrong-sha256":
				manifest.Tools[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
			case "missing-pin":
				manifest.Tools[0].PinnedVersion = ""
				manifest.Tools[0].SHA256 = ""
			}
			report, err := VerifyWorkspaceToolchain(context.Background(), root, manifest)
			u9RequireRejected(t, report, err)
		})
	}

	for _, scenario := range []string{"shadow", "bare-command", "cross-workspace-command", "cross-workspace-config"} {
		t.Run(scenario, func(t *testing.T) {
			root, other := u9Workspace(t)
			manifest := u9PinnedWorkspace(t, root)
			configPath := ".mcp.json"
			switch scenario {
			case "shadow":
				u9Write(t, root, configPath, `{"mcpServers":{"backlogit":{"command":"./bin/backlogit.exe"},"shadow":{"command":"./bin/shadow.exe"}}}`)
			case "bare-command":
				u9Write(t, root, configPath, `{"mcpServers":{"backlogit":{"command":"backlogit"}}}`)
			case "cross-workspace-command":
				data, err := json.Marshal(map[string]map[string]map[string]string{"mcpServers": {"backlogit": {"command": filepath.Join(other, "bin", "backlogit.exe")}}})
				require.NoError(t, err)
				u9Write(t, root, configPath, string(data))
			case "cross-workspace-config":
				u9Write(t, other, ".mcp.json", `{"mcpServers":{"backlogit":{"command":"./bin/backlogit.exe"}}}`)
				configPath = filepath.Join(other, ".mcp.json")
			}
			findings, err := ValidateMCPConfigPinning(root, configPath, manifest)
			u9RequireNotStub(t, err)
			if scenario == "shadow" {
				found := false
				for _, finding := range findings {
					found = found || (finding.ToolID == "shadow" && finding.Code == "uninventoried_shadow_tool")
				}
				require.True(t, found, "shadow tool must have its specified finding code")
				return
			}
			if err == nil {
				require.NotEmpty(t, findings, "unsafe MCP configuration must be refused")
				found := false
				for _, finding := range findings {
					found = found || finding.ContainmentClass == NonConformant || finding.ContainmentClass == Unsupported
				}
				require.True(t, found, "unsafe configuration needs an explicit violation finding")
			}
		})
	}
}

func TestU9_ValidateMCPConfigPinningRejectsCrossWorkspaceLauncherScripts(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		absolute bool
		flag     bool
	}{
		{name: "relative-positional"},
		{name: "absolute-positional", absolute: true},
		{name: "relative-script-flag", flag: true},
		{name: "absolute-script-flag", absolute: true, flag: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, other := u9Workspace(t)
			manifest := u9PinnedWorkspace(t, root)
			manifest.Tools[0].ID = "launcher"
			manifest.Tools[0].EntrypointPath = "./scripts/server.js"
			u9Write(t, root, "scripts/server.js", "local launcher script")
			u9Write(t, other, "scripts/server.js", "other workspace launcher script")

			writeConfig := func(script string) {
				t.Helper()
				if scenario.flag {
					script = "--script=" + script
				}
				type server struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				}
				config := struct {
					Servers map[string]server `json:"mcpServers"`
				}{
					Servers: map[string]server{
						"launcher": {Command: "./bin/backlogit.exe", Args: []string{script, "--cwd", root}},
					},
				}
				data, err := json.Marshal(config)
				require.NoError(t, err)
				u9Write(t, root, ".mcp.json", string(data))
			}

			writeConfig("./scripts/server.js")
			findings, err := ValidateMCPConfigPinning(root, ".mcp.json", manifest)
			require.NoError(t, err)
			require.Len(t, findings, 1)
			require.Equal(t, WorkspaceLocalPinned, findings[0].ContainmentClass)

			script := "../b/scripts/server.js"
			if scenario.absolute {
				script = filepath.Join(other, "scripts", "server.js")
			}
			writeConfig(script)
			findings, err = ValidateMCPConfigPinning(root, ".mcp.json", manifest)
			u9RequireNotStub(t, err)
			if err != nil {
				return
			}
			require.Len(t, findings, 1)
			require.Equal(t, NonConformant, findings[0].ContainmentClass,
				"cross-workspace launcher script must be rejected")
		})
	}
}

func TestU9_VerifyWorkspaceToolchainRejectsSymlinkAndReparseEscapes(t *testing.T) {
	// Junctions exercise Windows reparse points without requiring symlink privilege.
	kind := "symlink"
	if runtime.GOOS == "windows" {
		kind = "junction"
	}
	for _, location := range []string{"bin", "tools/mcp"} {
		t.Run(kind+"/"+location, func(t *testing.T) {
			root, other := u9Workspace(t)
			u9Write(t, other, "backlogit.exe", "pinned binary")
			link := filepath.Join(root, filepath.FromSlash(location))
			require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
			if kind == "junction" {
				output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, other).CombinedOutput()
				require.NoError(t, err, "creating junction: %s", output)
			} else {
				require.NoError(t, os.Symlink(other, link), "creating symlink")
			}
			info, err := os.Lstat(link)
			require.NoError(t, err)
			if kind == "junction" {
				require.NotZero(t, info.Mode()&(os.ModeIrregular|os.ModeSymlink), "junction must be a reparse point")
			} else {
				require.NotZero(t, info.Mode()&os.ModeSymlink, "fixture must be a symlink")
			}
			linkTarget, err := os.Stat(link)
			require.NoError(t, err)
			otherTarget, err := os.Stat(other)
			require.NoError(t, err)
			require.True(t, os.SameFile(linkTarget, otherTarget), "link must resolve outside the workspace")
			manifest := u9Manifest(root)
			manifest.Tools[0].ExecutablePath = "./" + location + "/backlogit.exe"
			report, err := VerifyWorkspaceToolchain(context.Background(), root, manifest)
			u9RequireRejected(t, report, err)
		})
	}
}

func TestU9_VerifyWorkspaceToolchainReportsUnsupportedAndNonConformant(t *testing.T) {
	root, _ := u9Workspace(t)
	manifest := u9PinnedWorkspace(t, root)
	manifest.Tools = append(manifest.Tools,
		ToolEntry{ID: "unsupported", Kind: "agent", ContainmentClass: Unsupported, Reason: "not supported"},
		ToolEntry{ID: "untrusted", Kind: "agent", ContainmentClass: NonConformant, Reason: "not pinned"},
	)
	report, err := VerifyWorkspaceToolchain(context.Background(), root, manifest)
	require.NoError(t, err, "TODO: implement honest unsupported and non-conformant reporting")
	require.NotNil(t, report)
	require.False(t, report.Valid)
	require.Positive(t, report.Violations)
	classes := make(map[string]ToolContainmentClass)
	for _, finding := range report.Tools {
		classes[finding.ToolID] = finding.ContainmentClass
	}
	require.Equal(t, Unsupported, classes["unsupported"])
	require.Equal(t, NonConformant, classes["untrusted"])
}

func TestU9_LoadToolchainManifestRejectsDuplicateYAMLKeys(t *testing.T) {
	root, _ := u9Workspace(t)
	quotedRoot, err := json.Marshal(root)
	require.NoError(t, err)
	valid := "schema_version: '1.0'\ncontainment_level: level-1-workspace-local-pinned\nos_container_sandbox_deferred: true\nworkspace_root: " + string(quotedRoot) + "\ntools: []\nhost_prerequisites: []\n"
	u9Write(t, root, ".backlogit/toolchain.yaml", valid)
	loaded, err := LoadToolchainManifest(root)
	require.NoError(t, err, "TODO: implement valid manifest loading")
	require.NotNil(t, loaded)
	require.Equal(t, "1.0", loaded.SchemaVersion)
	u9Write(t, root, ".backlogit/toolchain.yaml", valid+"schema_version: '1.0'\n")
	manifest, err := LoadToolchainManifest(root)
	u9RequireNotStub(t, err)
	require.Error(t, err, "duplicate YAML keys must be rejected")
	require.Nil(t, manifest)
}

func TestU9_ValidateMCPConfigPinningRejectsDuplicateJSONKeys(t *testing.T) {
	root, _ := u9Workspace(t)
	manifest := u9PinnedWorkspace(t, root)
	_, err := ValidateMCPConfigPinning(root, ".mcp.json", manifest)
	require.NoError(t, err, "TODO: implement valid MCP config loading")
	u9Write(t, root, ".mcp.json", `{"mcpServers":{"backlogit":{"command":"./bin/backlogit.exe","command":"backlogit"}}}`)
	_, err = ValidateMCPConfigPinning(root, ".mcp.json", manifest)
	u9RequireNotStub(t, err)
	require.Error(t, err, "duplicate JSON keys must be rejected")
}

func TestU9_DefaultWorkspaceToolchainManifestInventoriesTools(t *testing.T) {
	root, _ := u9Workspace(t)
	manifest := DefaultWorkspaceToolchainManifest(root)
	require.NotNil(t, manifest)
	require.NotNil(t, manifest.Tools)
	require.NotNil(t, manifest.HostPrerequisites)
	require.Equal(t, Level1WorkspaceLocalPinned, manifest.ContainmentLevel)
	require.True(t, manifest.OSContainerSandboxDeferred)
	ids := make(map[string]bool)
	for _, tool := range manifest.Tools {
		ids[tool.ID] = true
	}
	for _, id := range []string{"backlogit", "engram", "graphtor-docs", "context7", "tavily", "github", "bun", "bunx"} {
		require.True(t, ids[id], "TODO: implement default inventory for %s", id)
	}
	prerequisites := make(map[string]bool)
	for _, prerequisite := range manifest.HostPrerequisites {
		prerequisites[prerequisite.ID] = true
	}
	for _, id := range []string{"pwsh", "bash", "git", "gh", "go", "copilot"} {
		require.True(t, prerequisites[id], "missing declared host prerequisite %s", id)
	}
	loaded, err := LoadToolchainManifest(root)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.NotEmpty(t, loaded.Tools, "absent manifest must synthesize default inventory")
}

func TestU9_ReviewFixMCPArgumentsAndBindings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		env      map[string]string
		bindings []string
		valid    bool
	}{
		{name: "entrypoint-present", args: []string{"./scripts/server.js"}, valid: true},
		{name: "entrypoint-absent", args: []string{"serve"}},
		{name: "wrong-entrypoint", args: []string{"./scripts/other.js"}},
		{name: "embedded-escape", args: []string{"./scripts/server.js", "--config=../outside"}},
		{name: "traversal-flag", args: []string{"./scripts/server.js", "--config=../a/config"}},
		{name: "required-binding-missing", args: []string{"./scripts/server.js"}, bindings: []string{"--cwd", "."}},
		{name: "unknown-binding", args: []string{"./scripts/server.js"}, bindings: []string{"--unknown", "."}},
		{name: "dangling-binding", args: []string{"./scripts/server.js"}, bindings: []string{"--cwd"}},
		{name: "escaping-binding", args: []string{"./scripts/server.js"}, bindings: []string{"--cwd", "../outside"}},
		{name: "matching-binding", args: []string{"./scripts/server.js", "--cwd", "."}, bindings: []string{"--cwd", "."}, valid: true},
		{name: "casefold-env-escape", args: []string{"./scripts/server.js"}, env: map[string]string{"engram_data_dir": "../outside"}},
		{name: "casefold-env-binding", args: []string{"./scripts/server.js"}, env: map[string]string{"engram_data_dir": "."}, bindings: []string{"ENGRAM_DATA_DIR=."}, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := u9Workspace(t)
			m := u9PinnedWorkspace(t, root)
			m.Tools[0].ID = "launcher"
			m.Tools[0].EntrypointPath = "./scripts/server.js"
			m.Tools[0].WorkspaceBindingArgs = tc.bindings
			u9Write(t, root, "scripts/server.js", "script")
			data, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"launcher": map[string]any{"command": "./bin/backlogit.exe", "args": tc.args, "env": tc.env}}})
			require.NoError(t, err)
			u9Write(t, root, ".mcp.json", string(data))
			findings, err := ValidateMCPConfigPinning(root, ".mcp.json", m)
			if err != nil {
				require.False(t, tc.valid, "%v", err)
				return
			}
			require.Len(t, findings, 1)
			require.Equal(t, tc.valid, findings[0].Status != "violation", "%+v", findings[0])
		})
	}
}

func TestU9_ReviewFixManifestStorageAndYAML(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "case-fold-top", body: "schema_version: '1.0'\nSchema_Version: '1.0'\n"},
		{name: "case-fold-nested", body: "tools:\n  - id: one\n    ID: two\n"},
		{name: "case-fold-nested-map", body: "tools:\n  - workspace_binding_args: [{foo: x, FOO: y}]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := u9Workspace(t)
			u9Write(t, root, ".backlogit/toolchain.yaml", tc.body)
			_, err := LoadToolchainManifest(root)
			require.Error(t, err)
		})
	}
	t.Run("conflicting-roots", func(t *testing.T) {
		root, _ := u9Workspace(t)
		require.NoError(t, os.Mkdir(filepath.Join(root, ".backlog"), 0o755))
		require.NoError(t, os.Mkdir(filepath.Join(root, ".backlogit"), 0o755))
		_, err := LoadToolchainManifest(root)
		require.Error(t, err)
	})
	t.Run("legacy-root-is-selected", func(t *testing.T) {
		root, _ := u9Workspace(t)
		quotedRoot, err := json.Marshal(root)
		require.NoError(t, err)
		u9Write(t, root, ".backlogit/toolchain.yaml", "schema_version: '1.0'\ncontainment_level: level-1-workspace-local-pinned\nos_container_sandbox_deferred: true\nworkspace_root: "+string(quotedRoot)+"\ntools: []\nhost_prerequisites: []\n")
		loaded, err := LoadToolchainManifest(root)
		require.NoError(t, err)
		require.Equal(t, "1.0", loaded.SchemaVersion)
	})
	t.Run("missing-prerequisite-required", func(t *testing.T) {
		root, _ := u9Workspace(t)
		quotedRoot, err := json.Marshal(root)
		require.NoError(t, err)
		u9Write(t, root, ".backlogit/toolchain.yaml", "schema_version: '1.0'\ncontainment_level: level-1-workspace-local-pinned\nos_container_sandbox_deferred: true\nworkspace_root: "+string(quotedRoot)+"\ntools: []\nhost_prerequisites:\n  - id: git\n    command: git\n    purpose: bootstrap\n    reason: host tool\n")
		_, err = LoadToolchainManifest(root)
		require.Error(t, err)
	})
}

func TestU9_ReviewFixRequiredFieldsAndLockfile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ToolchainManifest)
	}{
		{name: "whitespace-id", mutate: func(m *ToolchainManifest) { m.Tools[0].ID = " " }},
		{name: "whitespace-kind", mutate: func(m *ToolchainManifest) { m.Tools[0].Kind = " " }},
		{name: "whitespace-remote-policy", mutate: func(m *ToolchainManifest) { m.Tools[1].RemotePolicy = " " }},
		{name: "whitespace-host-reason", mutate: func(m *ToolchainManifest) { m.Tools[2].Reason = " " }},
		{name: "whitespace-prerequisite", mutate: func(m *ToolchainManifest) { m.HostPrerequisites[0].Purpose = " " }},
		{name: "empty-lockfile", mutate: func(m *ToolchainManifest) { m.Tools[0].PinnedVersion = ""; m.Tools[0].LockfilePath = "tool.lock" }},
		{name: "unrelated-lockfile", mutate: func(m *ToolchainManifest) { m.Tools[0].LockfilePath = "tool.lock" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := u9Workspace(t)
			m := u9PinnedWorkspace(t, root)
			lockfile := " \n"
			if tc.name == "unrelated-lockfile" {
				lockfile = "unrelated package and version\n"
			}
			u9Write(t, root, "tool.lock", lockfile)
			tc.mutate(m)
			report, err := VerifyWorkspaceToolchain(context.Background(), root, m)
			u9RequireRejected(t, report, err)
		})
	}
}

func TestU9_LockfilePinProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, version, lockPath, content string
		sameExecutable                   bool
		valid                            bool
	}{
		{name: "missing-pin", version: "  "},
		{name: "version-only", version: "1.0.0", valid: true},
		{name: "lockfile-only", lockPath: "tool.lock", content: "backlogit\n", valid: true},
		{name: "trimmed-pins", version: " 1.0.0 ", lockPath: "tool.lock", content: "backlogit 1.0.0\n", valid: true},
		{name: "empty-lockfile", lockPath: "tool.lock", content: " \n"},
		{name: "missing-id", version: "1.0.0", lockPath: "tool.lock", content: "other 1.0.0"},
		{name: "missing-version", version: "1.0.0", lockPath: "tool.lock", content: "backlogit 2.0.0"},
		{name: "executable-is-lockfile", version: "1.0.0", lockPath: "./bin/backlogit.exe", content: "backlogit 1.0.0", sameExecutable: true, valid: true},
		{name: "executable-lockfile-missing-id", version: "1.0.0", lockPath: "./bin/backlogit.exe", content: "other 1.0.0", sameExecutable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootPath, _ := u9Workspace(t)
			m := u9PinnedWorkspace(t, rootPath)
			tool := m.Tools[0]
			tool.PinnedVersion = tc.version
			tool.LockfilePath = tc.lockPath
			if tc.lockPath != "" {
				u9Write(t, rootPath, tc.lockPath, tc.content)
			}
			if tc.sameExecutable {
				sum := sha256.Sum256([]byte(tc.content))
				tool.SHA256 = hex.EncodeToString(sum[:])
			}
			root, err := os.OpenRoot(rootPath)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, root.Close()) })
			err = verifyLocalTool(root, rootPath, tool)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, corerrors.ErrValidation)
			}
		})
	}
}
