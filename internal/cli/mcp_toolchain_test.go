package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/toolchain"
)

// These JSON fixtures deliberately do not depend on the future CLI output wrapper.
type u11StatusJSON struct {
	Running       *bool                                  `json:"running"`
	WorkspaceRoot string                                 `json:"workspace_root"`
	LockPath      string                                 `json:"lock_path"`
	Toolchain     *toolchain.ToolchainVerificationReport `json:"toolchain"`
}

type u11MCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
}

type u11MCPConfig struct {
	Servers map[string]u11MCPServer `json:"mcpServers"`
}

func u11Write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func u11WriteConfig(t *testing.T, root string, config u11MCPConfig) {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	u11Write(t, root, ".mcp.json", string(data))
}

// u11Fixture uses inert bytes, never runnable tools, and no ambient PATH fallback.
// The invocation's contained TEMP/TMP makes t.TempDir workspace-local.
func u11Fixture(t *testing.T, violations bool) (string, u11MCPConfig) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "empty-path")
	require.NoError(t, os.Mkdir(path, 0o755))
	t.Setenv("PATH", path)
	t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
	const binary = "U11 inert binary fixture: must not be executed\n"
	u11Write(t, root, "bin/backlogit.exe", binary)
	digest := sha256.Sum256([]byte(binary))
	manifest := toolchain.ToolchainManifest{
		SchemaVersion: "1.0", WorkspaceRoot: root,
		ContainmentLevel:           toolchain.Level1WorkspaceLocalPinned,
		OSContainerSandboxDeferred: true,
		Tools: []toolchain.ToolEntry{
			{ID: "backlogit", Kind: "mcp", ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath: "./bin/backlogit.exe", PinnedVersion: "1.0.0",
				SHA256: hex.EncodeToString(digest[:]), WorkspaceBindingArgs: []string{"--cwd", root}},
			{ID: "github", Kind: "mcp", ContainmentClass: toolchain.RemoteDeclared,
				RemotePolicy: "approved fixture endpoint; inspection only"},
			{ID: "agent-host", Kind: "agent", ContainmentClass: toolchain.BuiltInHost,
				Reason: "provided by agent host, not a local executable"},
		},
		HostPrerequisites: make([]toolchain.HostPrerequisiteEntry, 0, 3),
	}
	for _, id := range []string{"pwsh", "sh", "git"} {
		manifest.HostPrerequisites = append(manifest.HostPrerequisites, toolchain.HostPrerequisiteEntry{
			ID: id, Command: id, Purpose: "workspace bootstrap", Reason: "explicit host prerequisite",
		})
	}
	config := u11MCPConfig{Servers: map[string]u11MCPServer{
		"backlogit": {Type: "stdio", Command: "./bin/backlogit.exe", Args: []string{"mcp", "--cwd", root}},
		"github":    {Type: "http", URL: "https://github.invalid/mcp"},
	}}
	if violations {
		manifest.Tools = append(manifest.Tools,
			toolchain.ToolEntry{ID: "engram", Kind: "mcp", ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath: "./bin/engram.exe", PinnedVersion: "1.0.0", SHA256: hex.EncodeToString(digest[:])},
			toolchain.ToolEntry{ID: "graphtor-docs", Kind: "mcp", ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath: "./bin/kbdb.exe", PinnedVersion: "1.0.0", SHA256: hex.EncodeToString(digest[:])},
			toolchain.ToolEntry{ID: "context7", Kind: "mcp", ContainmentClass: toolchain.Unsupported,
				Reason: "no supported local pin in fixture"},
		)
		config.Servers["engram"] = u11MCPServer{Type: "stdio", Command: "engram"}
		config.Servers["graphtor-docs"] = u11MCPServer{Type: "stdio", Command: "./bin/kbdb.exe"}
		config.Servers["context7"] = u11MCPServer{Type: "stdio", Command: "bunx", Args: []string{"context7"}}
	}
	data, err := yaml.Marshal(manifest)
	require.NoError(t, err)
	u11Write(t, root, ".backlogit/config.yaml", "{}\n")
	u11Write(t, root, ".backlogit/toolchain.yaml", string(data))
	u11WriteConfig(t, root, config)
	u11Write(t, root, "start.ps1", `# operator-owned fixture; preserve byte-for-byte
$workspaceRoot = $PSScriptRoot
$workspaceBinDirectory = Join-Path $workspaceRoot "bin"
$backlogitPath = Join-Path $workspaceBinDirectory "backlogit.exe"
if (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw "Missing ./bin/backlogit.exe" }
$env:COPILOT_HOME = Join-Path $workspaceRoot ".copilot"
$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot ".engram"
& $backlogitPath --cwd $workspaceRoot mcp status
`)
	u11Write(t, root, "start.sh", `#!/bin/sh
# operator-owned fixture; preserve byte-for-byte
workspace_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace_bin_directory="$workspace_root/bin"
backlogit_path="$workspace_bin_directory/backlogit.exe"
if [ ! -f "$backlogit_path" ]; then echo "Missing ./bin/backlogit.exe" >&2; exit 1; fi
export COPILOT_HOME="$workspace_root/.copilot"
export ENGRAM_DATA_DIR="$workspace_root/.engram"
"$backlogit_path" --cwd "$workspace_root" mcp status
`)
	return root, config
}

func u11Snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	require.NoError(t, err)
	return files
}

func u11Execute(t *testing.T, root string, flags ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	var out, diagnostics bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostics)
	cmd.SetArgs(append([]string{"--cwd", root, "mcp", "status"}, flags...))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func u11Decode(t *testing.T, root, output string) *toolchain.ToolchainVerificationReport {
	t.Helper()
	var status u11StatusJSON
	require.NoError(t, json.Unmarshal([]byte(output), &status), "status must write structured JSON, not diagnostics")
	require.NotNil(t, status.Running, "existing daemon status must be preserved")
	assert.False(t, *status.Running, "inspection must not start a daemon")
	assert.Equal(t, filepath.Clean(root), filepath.Clean(status.WorkspaceRoot))
	assert.NotEmpty(t, status.LockPath)
	require.NotNil(t, status.Toolchain, "TODO: implement toolchain output composition")
	assert.Equal(t, filepath.Clean(root), filepath.Clean(status.Toolchain.WorkspaceRoot))
	assert.Equal(t, toolchain.Level1WorkspaceLocalPinned, status.Toolchain.ContainmentLevel)
	assert.True(t, status.Toolchain.OSContainerSandboxDeferred, "Level 1 pinning is not an OS sandbox")
	return status.Toolchain
}

func u11RequireClass(t *testing.T, report *toolchain.ToolchainVerificationReport, id string, class toolchain.ToolContainmentClass) {
	t.Helper()
	for _, findings := range [][]toolchain.ToolVerificationFinding{report.Tools, report.HostPrerequisites} {
		for _, finding := range findings {
			if finding.ToolID == id && finding.ContainmentClass == class {
				return
			}
		}
	}
	t.Errorf("tool %q must have an honest %q finding", id, class)
}

// TestU11_MCPStatusToolchainFlagReportsPinnedAndNonConformantCompanionTools covers SC11/SC13.
func TestU11_MCPStatusToolchainFlagReportsPinnedAndNonConformantCompanionTools(t *testing.T) {
	for _, entrypoint := range []string{"audit", "cli"} {
		t.Run(entrypoint, func(t *testing.T) {
			root, _ := u11Fixture(t, true)
			before := u11Snapshot(t, root)
			var report *toolchain.ToolchainVerificationReport
			if entrypoint == "audit" {
				var err error
				report, err = toolchain.AuditRepositoryToolchain(context.Background(), root)
				assert.Equal(t, before, u11Snapshot(t, root), "auditor must be strictly read-only")
				require.NoError(t, err, "TODO: implement AuditRepositoryToolchain")
			} else {
				output, err := u11Execute(t, root, "--toolchain", "--format", "json")
				require.NoError(t, err, "TODO: implement --toolchain; non-strict violations are report data")
				report = u11Decode(t, root, output)
				for path, contents := range before {
					data, readErr := os.ReadFile(path)
					require.NoError(t, readErr)
					assert.Equal(t, contents, string(data), "inspection changed %s", filepath.Base(path))
				}
			}
			require.NotNil(t, report)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
			assert.Equal(t, toolchain.Level1WorkspaceLocalPinned, report.ContainmentLevel)
			assert.True(t, report.OSContainerSandboxDeferred)
			u11RequireClass(t, report, "backlogit", toolchain.WorkspaceLocalPinned)
			for _, id := range []string{"pwsh", "sh", "git"} {
				u11RequireClass(t, report, id, toolchain.HostPrerequisite)
			}
			u11RequireClass(t, report, "github", toolchain.RemoteDeclared)
			u11RequireClass(t, report, "agent-host", toolchain.BuiltInHost)
			u11RequireClass(t, report, "engram", toolchain.NonConformant)
			u11RequireClass(t, report, "graphtor-docs", toolchain.NonConformant)
			u11RequireClass(t, report, "context7", toolchain.Unsupported)
			for _, finding := range report.Tools {
				switch finding.ToolID {
				case "engram", "graphtor-docs", "context7", "github", "agent-host":
					assert.NotEqual(t, toolchain.WorkspaceLocalPinned, finding.ContainmentClass,
						"missing, remote, unsupported, or built-in tools must not be claimed locally pinned")
				}
			}
		})
	}
}

// TestU11_VerifyLauncherScriptsChecksStartPs1AndMcpJsonBacklogitPinning covers R12/SC5/SC8/SC12.
func TestU11_VerifyLauncherScriptsChecksStartPs1AndMcpJsonBacklogitPinning(t *testing.T) {
	for _, scenario := range []string{
		"pinned", "missing-binary", "mcp-bare-command", "ps1-path-fallback",
		"sh-path-fallback", "ps1-missing-check", "sh-missing-check", "ps1-unscoped-state", "sh-unscoped-state",
	} {
		t.Run(scenario, func(t *testing.T) {
			root, config := u11Fixture(t, false)
			remediation := "./bin/backlogit.exe"
			switch scenario {
			case "missing-binary":
				require.NoError(t, os.Remove(filepath.Join(root, "bin", "backlogit.exe")))
			case "mcp-bare-command":
				server := config.Servers["backlogit"]
				server.Command = "backlogit"
				config.Servers["backlogit"] = server
				u11WriteConfig(t, root, config)
			case "ps1-path-fallback":
				u11Write(t, root, "start.ps1", "& backlogit --cwd $PSScriptRoot mcp status\n")
			case "sh-path-fallback":
				u11Write(t, root, "start.sh", "#!/bin/sh\nbacklogit_path=$(command -v backlogit)\n\"$backlogit_path\" mcp status\n")
			case "ps1-missing-check", "sh-missing-check", "ps1-unscoped-state", "sh-unscoped-state":
				name := "start.ps1"
				if strings.HasPrefix(scenario, "sh-") {
					name = "start.sh"
				}
				data, err := os.ReadFile(filepath.Join(root, name))
				require.NoError(t, err)
				lines := strings.Split(string(data), "\n")
				kept := make([]string, 0, len(lines))
				for _, line := range lines {
					if strings.HasSuffix(scenario, "missing-check") &&
						(strings.Contains(line, "Test-Path") || strings.Contains(line, "[ ! -f")) {
						continue
					}
					if strings.HasSuffix(scenario, "unscoped-state") &&
						(strings.Contains(line, "COPILOT_HOME") || strings.Contains(line, "ENGRAM_DATA_DIR")) {
						continue
					}
					kept = append(kept, line)
				}
				u11Write(t, root, name, strings.Join(kept, "\n"))
				if strings.HasSuffix(scenario, "unscoped-state") {
					remediation = ""
				}
			}
			before := u11Snapshot(t, root)
			findings, err := toolchain.VerifyLauncherScripts(root)
			assert.Equal(t, before, u11Snapshot(t, root), "launcher inspection must not repair operator files")
			require.NoError(t, err, "TODO: implement VerifyLauncherScripts")
			require.NotEmpty(t, findings, "launcher inspection must account for the configured tools")
			var violations []toolchain.ToolVerificationFinding
			for _, finding := range findings {
				if finding.ContainmentClass == toolchain.NonConformant || finding.ContainmentClass == toolchain.Unsupported {
					violations = append(violations, finding)
				}
			}
			if scenario == "pinned" {
				assert.Empty(t, violations, "local pin and fail-closed scoped scripts must be accepted")
				u11RequireClass(t, &toolchain.ToolchainVerificationReport{Tools: findings},
					"backlogit", toolchain.WorkspaceLocalPinned)
				return
			}
			require.NotEmpty(t, violations, "unsafe launcher must produce an explicit finding")
			var fixes strings.Builder
			for _, finding := range violations {
				assert.NotEmpty(t, finding.Code)
				assert.NotEmpty(t, finding.Message)
				assert.NotEmpty(t, finding.Remediation)
				fixes.WriteString(strings.ReplaceAll(finding.Remediation, `\`, "/"))
				fixes.WriteByte('\n')
			}
			if remediation != "" {
				assert.Contains(t, fixes.String(), remediation, "remediation must name the exact workspace-local binary")
			} else {
				assert.Contains(t, fixes.String(), "COPILOT_HOME")
				assert.Contains(t, fixes.String(), "ENGRAM_DATA_DIR")
			}
		})
	}
}

// TestU11_MCPStatusToolchainStrictExitsNonZeroOnViolations covers output-before-error and implicit auditing.
func TestU11_MCPStatusToolchainStrictExitsNonZeroOnViolations(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		violations bool
		implicit   bool
		format     string
	}{
		{name: "violations-json", violations: true, format: "json"},
		{name: "violations-text", violations: true, format: "text"},
		{name: "implicit-toolchain", violations: true, implicit: true, format: "json"},
		{name: "conformant-json", format: "json"},
		{name: "conformant-text", format: "text"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, _ := u11Fixture(t, scenario.violations)
			flags := []string{"--strict", "--format", scenario.format}
			if !scenario.implicit {
				flags = append(flags, "--toolchain")
			}
			output, err := u11Execute(t, root, flags...)
			if scenario.violations {
				assert.ErrorIs(t, err, corerrors.ErrValidation, "TODO: implement strict wrapped validation error")
				assert.NotEqual(t, corerrors.ErrValidation, err, "strict error must include wrapping context")
			} else {
				assert.NoError(t, err, "TODO: implement strict success for a conformant workspace")
			}
			require.NotEmpty(t, output, "TODO: write the report to cmd.OutOrStdout before returning strict error")
			if scenario.format == "json" {
				report := u11Decode(t, root, output)
				assert.Equal(t, !scenario.violations, report.Valid)
				if scenario.violations {
					assert.Positive(t, report.Violations)
					u11RequireClass(t, report, "engram", toolchain.NonConformant)
				} else {
					assert.Zero(t, report.Violations)
				}
			} else {
				assert.Contains(t, strings.ToLower(output), "daemon")
				assert.Contains(t, strings.ToLower(output), "toolchain")
				if scenario.violations {
					assert.Contains(t, output, "engram")
				}
			}
		})
	}
}
