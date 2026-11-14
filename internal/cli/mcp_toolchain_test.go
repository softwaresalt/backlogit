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

// TestU11_LauncherInspectionRejectsUnprovenScripts extends the static audit
// contract without executing a shell or accepting comments as evidence.
func TestU11_LauncherInspectionRejectsUnprovenScripts(t *testing.T) {
	for _, scenario := range []string{
		"comment-only", "ps1-reassignment", "sh-reassignment", "ps1-traversal",
		"sh-external", "ps1-nonterminating-check", "sh-nonterminating-check",
		"ps1-unbound-cwd", "sh-unbound-cwd", "ps1-late-state",
		"sh-late-check", "missing-script", "missing-config", "unparsed-script",
		"companion-path-lookup", "state-is-file",
	} {
		t.Run(scenario, func(t *testing.T) {
			root, _ := u11Fixture(t, false)
			name := "start.ps1"
			if strings.HasPrefix(scenario, "sh-") {
				name = "start.sh"
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			require.NoError(t, err)
			script := string(data)
			switch scenario {
			case "comment-only":
				script = "# " + strings.ReplaceAll(script, "\n", "\n# ")
			case "ps1-reassignment":
				script = strings.ReplaceAll(script, "& $backlogitPath", "$backlogitPath = \"backlogit\"\n& $backlogitPath")
			case "sh-reassignment":
				script = strings.ReplaceAll(script, "\"$backlogit_path\" --cwd", "backlogit_path=\"backlogit\"\n\"$backlogit_path\" --cwd")
			case "ps1-traversal":
				script = strings.ReplaceAll(script, `"bin"`, `"../bin"`)
			case "sh-external":
				script = strings.ReplaceAll(script, `"$workspace_root/bin"`, `"/external/bin"`)
			case "ps1-nonterminating-check":
				script = strings.ReplaceAll(script, `throw "Missing`, `Write-Warning "Missing`)
			case "sh-nonterminating-check":
				script = strings.ReplaceAll(script, "; exit 1", "")
			case "ps1-unbound-cwd":
				script = strings.ReplaceAll(script, "--cwd $workspaceRoot ", "")
			case "sh-unbound-cwd":
				script = strings.ReplaceAll(script, `--cwd "$workspace_root" `, "")
			case "ps1-late-state":
				lines := strings.Split(script, "\n")
				var state []string
				var rest []string
				for _, line := range lines {
					if strings.Contains(line, "$env:") {
						state = append(state, line)
					} else {
						rest = append(rest, line)
					}
				}
				script = strings.Join(append(rest, state...), "\n")
			case "sh-late-check":
				name = "start.sh"
				data, err = os.ReadFile(filepath.Join(root, name))
				require.NoError(t, err)
				lines := strings.Split(string(data), "\n")
				var check string
				var rest []string
				for _, line := range lines {
					if strings.HasPrefix(line, "if [") {
						check = line
					} else {
						rest = append(rest, line)
					}
				}
				script = strings.Join(rest, "\n") + "\n" + check + "\n"
			case "missing-script":
				require.NoError(t, os.Remove(filepath.Join(root, name)))
			case "missing-config":
				require.NoError(t, os.Remove(filepath.Join(root, ".mcp.json")))
			case "unparsed-script":
				script += "\nInvoke-Expression $operatorInput\n"
			case "companion-path-lookup":
				script += "\n& engram sync\n"
			case "state-is-file":
				u11Write(t, root, ".copilot", "not a directory\n")
			}
			if scenario != "missing-script" {
				u11Write(t, root, name, script)
			}
			before := u11Snapshot(t, root)
			report, err := toolchain.AuditRepositoryToolchain(context.Background(), root)
			require.NoError(t, err)
			require.NotNil(t, report)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
			assert.Equal(t, before, u11Snapshot(t, root))
			for _, f := range report.Tools {
				if f.Status == "violation" {
					assert.NotEmpty(t, f.Code)
					assert.NotEmpty(t, f.Message)
					assert.NotEmpty(t, f.Remediation)
				}
			}
		})
	}
}

func TestU11_AuditRejectsRedirectionsAndHonorsCancellation(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		root, _ := u11Fixture(t, false)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := toolchain.AuditRepositoryToolchain(ctx, root)
		require.ErrorIs(t, err, context.Canceled)
	})
	t.Run("nil-context", func(t *testing.T) {
		root, _ := u11Fixture(t, false)
		_, err := toolchain.AuditRepositoryToolchain(nil, root) //nolint:staticcheck // Deliberately test rejection of invalid public API input.
		require.ErrorIs(t, err, corerrors.ErrValidation)
	})
	for _, name := range []string{"start.ps1", "start.sh", ".mcp.json", "bin", ".copilot"} {
		t.Run("redirect-"+name, func(t *testing.T) {
			root, _ := u11Fixture(t, false)
			// Both ends stay in the fixture; even internal redirections must not
			// earn a workspace-local pin. Windows junctions use the same primitive.
			target := filepath.Join(root, "redirect-target")
			if name == "bin" || name == ".copilot" {
				require.NoError(t, os.Mkdir(target, 0o755))
			} else {
				data, err := os.ReadFile(filepath.Join(root, name))
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(target, data, 0o600))
			}

			path := filepath.Join(root, name)
			if name == "bin" {
				require.NoError(t, os.Rename(path, path+"-original"))
			} else if name != ".copilot" {
				require.NoError(t, os.Remove(path))
			}
			require.NoError(t, os.Symlink(target, path), "symlink capability is required for this containment gate")
			report, err := toolchain.AuditRepositoryToolchain(context.Background(), root)
			require.NoError(t, err)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
		})
	}
}

func TestU11_LauncherProfileRefusesAmbiguousSyntaxAndNamesCompanionFallback(t *testing.T) {
	for _, scenario := range []struct {
		name string
		file string
		old  string
		new  string
	}{
		{"unquoted-shell-command", "start.sh", `"$backlogit_path" --cwd`, `$backlogit_path --cwd`},
		{"unbalanced-shell-quote", "start.sh", `"$backlogit_path" --cwd`, `"$backlogit_path"" --cwd`},
		{"wrong-assignment-dialect", "start.ps1", `$workspaceRoot =`, `workspaceRoot =`},
		{"mismatched-powershell-quote", "start.ps1", `Join-Path $workspaceRoot "bin"`, `Join-Path $workspaceRoot "bin'`},
		{"spaced-shell-assignment", "start.sh", `workspace_bin_directory=`, `workspace_bin_directory = `},
		{"companion-lookup", "start.ps1", `& $backlogitPath`, "$companion = (Get-Command \"engram\")\n& $backlogitPath"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, _ := u11Fixture(t, false)
			data, err := os.ReadFile(filepath.Join(root, scenario.file))
			require.NoError(t, err)
			u11Write(t, root, scenario.file, strings.ReplaceAll(string(data), scenario.old, scenario.new))
			report, err := toolchain.AuditRepositoryToolchain(context.Background(), root)
			require.NoError(t, err)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
			if scenario.name == "companion-lookup" {
				found := false
				for _, f := range report.Tools {
					if f.ToolID == "engram" && f.Status == "violation" {
						found = true
						assert.Contains(t, f.Remediation, "./bin/engram.exe")
					}
				}
				assert.True(t, found, "the unresolved companion must be named, not hidden behind a generic launcher diagnostic")
			}
		})
	}
}

func TestU11_LauncherRunnerRequiresDeclaredEntrypoint(t *testing.T) {
	setupRunnerFixture := func(t *testing.T, psInvocation string) string {
		t.Helper()
		root, _ := u11Fixture(t, false)
		manifest, err := toolchain.LoadToolchainManifest(root)
		require.NoError(t, err)
		bunBytes := []byte("inert bun binary\n")
		u11Write(t, root, "bin/bun.exe", string(bunBytes))
		u11Write(t, root, "tools/mcp/install.ps1", "# pinned install script\n")
		u11Write(t, root, "tools/mcp/context7.js", "// pinned context7 entrypoint\n")
		sum := sha256.Sum256(bunBytes)
		sha := hex.EncodeToString(sum[:])
		manifest.Tools = append(manifest.Tools,
			toolchain.ToolEntry{
				ID:               "bun",
				Kind:             "tool",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   "tools/mcp/install.ps1",
				PinnedVersion:    "1.4.0",
				SHA256:           sha,
			},
			toolchain.ToolEntry{
				ID:               "context7",
				Kind:             "mcp",
				ContainmentClass: toolchain.WorkspaceLocalPinned,
				ExecutablePath:   "./bin/bun.exe",
				EntrypointPath:   "tools/mcp/context7.js",
				PinnedVersion:    "4.4.4",
				SHA256:           sha,
			},
		)
		require.NoError(t, toolchain.WriteToolchainManifest(root, manifest))
		cfgBytes, err := toolchain.RenderPinnedMCPConfig(root, manifest)
		require.NoError(t, err)
		u11Write(t, root, ".mcp.json", string(cfgBytes))

		data, err := os.ReadFile(filepath.Join(root, "start.ps1"))
		require.NoError(t, err)
		prefix := strings.Join([]string{
			`$bunPath = Join-Path $workspaceBinDirectory "bun.exe"`,
			`$bunEntry = Join-Path $workspaceRoot "tools\mcp\context7.js"`,
			`if (-not (Test-Path -LiteralPath $bunPath -PathType Leaf)) { throw "Missing ./bin/bun.exe" }`,
		}, "\n")
		updated := strings.Replace(string(data), "& $backlogitPath", prefix+"\n"+psInvocation+"\n& $backlogitPath", 1)
		u11Write(t, root, "start.ps1", updated)
		return root
	}

	t.Run("accepts-shared-executable-declared-entrypoint", func(t *testing.T) {
		root := setupRunnerFixture(t, "& $bunPath $bunEntry")
		report, err := toolchain.AuditRepositoryToolchain(t.Context(), root)
		require.NoError(t, err)
		assert.True(t, report.Valid, "declared runner entrypoint should pass audit: %+v", report.Tools)
		assert.Zero(t, report.Violations)
	})

	for _, bad := range []struct {
		name string
		line string
		code string
	}{
		{"rejects-subcommand-bypass", "& $bunPath run evil", "launcher_entrypoint_unverified"},
		{"rejects-eval-flag", "& $bunPath --eval 123", "launcher_entrypoint_unverified"},
		{"rejects-trailing-eval-flag", "& $bunPath $bunEntry --eval 123", "launcher_static_unsupported"},
		{"rejects-bare-runner", "& $bunPath", "launcher_entrypoint_unverified"},
		{"rejects-undeclared-script", "& $bunPath tools/mcp/other.js", "launcher_entrypoint_unverified"},
	} {
		t.Run(bad.name, func(t *testing.T) {
			root := setupRunnerFixture(t, bad.line)
			report, err := toolchain.AuditRepositoryToolchain(t.Context(), root)
			require.NoError(t, err)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
			foundCode := false
			for _, f := range report.Tools {
				if f.Status == "violation" && f.Code == bad.code {
					foundCode = true
				}
			}
			assert.True(t, foundCode, "expected violation code %q in %+v", bad.code, report.Tools)
		})
	}

	t.Run("accepts-posix-shared-executable-declared-entrypoint", func(t *testing.T) {
		root := setupRunnerFixture(t, "& $bunPath $bunEntry")
		data, err := os.ReadFile(filepath.Join(root, "start.sh"))
		require.NoError(t, err)
		prefix := strings.Join([]string{
			`bun_path="$workspace_bin_directory/bun.exe"`,
			`bun_entry="$workspace_root/tools/mcp/context7.js"`,
			`if [ ! -f "$bun_path" ]; then echo "Missing ./bin/bun.exe" >&2; exit 1; fi`,
			`"$bun_path" "$bun_entry"`,
		}, "\n")
		u11Write(t, root, "start.sh", strings.Replace(string(data), `"$backlogit_path" --cwd`, prefix+"\n"+`"$backlogit_path" --cwd`, 1))

		report, err := toolchain.AuditRepositoryToolchain(t.Context(), root)
		require.NoError(t, err)
		assert.True(t, report.Valid, "declared POSIX runner entrypoint should pass audit: %+v", report.Tools)
		assert.Zero(t, report.Violations)
	})

	for _, shBad := range []struct {
		name string
		old  string
		new  string
		code string
	}{
		{
			name: "rejects-unquoted-posix-entrypoint",
			old:  `"$backlogit_path" --cwd`,
			new:  "bun_path=\"$workspace_bin_directory/bun.exe\"\nbun_entry=\"$workspace_root/tools/mcp/context7.js\"\nif [ ! -f \"$bun_path\" ]; then echo \"Missing ./bin/bun.exe\" >&2; exit 1; fi\n\"$bun_path\" $bun_entry\n\"$backlogit_path\" --cwd",
			code: "launcher_entrypoint_unverified",
		},
		{
			name: "rejects-posix-backslash-suffix",
			old:  `workspace_bin_directory="$workspace_root/bin"`,
			new:  `workspace_bin_directory="$workspace_root\bin"`,
			code: "launcher_path_invalid",
		},
		{
			name: "rejects-posix-backslash-entrypoint",
			old:  `"$backlogit_path" --cwd`,
			new:  "bun_path=\"$workspace_bin_directory/bun.exe\"\nif [ ! -f \"$bun_path\" ]; then echo \"Missing ./bin/bun.exe\" >&2; exit 1; fi\n\"$bun_path\" \"tools\\mcp\\context7.js\"\n\"$backlogit_path\" --cwd",
			code: "launcher_entrypoint_unverified",
		},
		{
			name: "rejects-shell-operator-entrypoint",
			old:  `"$backlogit_path" --cwd`,
			new:  "bun_path=\"$workspace_bin_directory/bun.exe\"\nif [ ! -f \"$bun_path\" ]; then echo \"Missing ./bin/bun.exe\" >&2; exit 1; fi\n\"$bun_path\" tools/mcp/context7.js;evil\n\"$backlogit_path\" --cwd",
			code: "launcher_entrypoint_unverified",
		},
		{
			name: "rejects-glob-entrypoint",
			old:  `"$backlogit_path" --cwd`,
			new:  "bun_path=\"$workspace_bin_directory/bun.exe\"\nif [ ! -f \"$bun_path\" ]; then echo \"Missing ./bin/bun.exe\" >&2; exit 1; fi\n\"$bun_path\" tools/mcp/context[7].js\n\"$backlogit_path\" --cwd",
			code: "launcher_entrypoint_unverified",
		},
	} {
		t.Run(shBad.name, func(t *testing.T) {
			root := setupRunnerFixture(t, "& $bunPath $bunEntry")
			data, err := os.ReadFile(filepath.Join(root, "start.sh"))
			require.NoError(t, err)
			updated := strings.Replace(string(data), shBad.old, shBad.new, 1)
			require.NotEqual(t, string(data), updated, "fixture mutation must apply")
			u11Write(t, root, "start.sh", updated)

			report, err := toolchain.AuditRepositoryToolchain(t.Context(), root)
			require.NoError(t, err)
			assert.False(t, report.Valid)
			assert.Positive(t, report.Violations)
			foundCode := false
			for _, f := range report.Tools {
				if f.Status == "violation" && f.Code == shBad.code {
					foundCode = true
				}
			}
			assert.True(t, foundCode, "expected violation code %q in %+v", shBad.code, report.Tools)
		})
	}
}
