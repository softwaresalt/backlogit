package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// VerifyLauncherScripts statically inspects configuration and startup scripts.
// It never executes a tool or repairs operator-owned files. Shell constructs
// outside the supported straight-line profile are reported as unsupported,
// rather than inferred safe from matching text in comments or dead branches.
func VerifyLauncherScripts(rootPath string) ([]ToolVerificationFinding, error) {
	m, err := LoadToolchainManifest(rootPath)
	if err != nil {
		return nil, fmt.Errorf("verify launcher scripts: %w", err)
	}
	return verifyLaunchers(rootPath, m)
}

// AuditRepositoryToolchain combines inventory, MCP and launcher evidence.
// Level 1 is static workspace-local pinning, not an OS/container sandbox.
func AuditRepositoryToolchain(ctx context.Context, rootPath string) (*ToolchainVerificationReport, error) {
	if ctx == nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", corerrors.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", err)
	}
	m, err := LoadToolchainManifest(rootPath)
	if err != nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", err)
	}
	report, err := VerifyWorkspaceToolchain(ctx, rootPath, m)
	if err != nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", err)
	}
	for i, f := range report.Tools {
		report.Tools[i] = toolRemediation(f, m)
	}
	findings, err := verifyLaunchers(rootPath, m)
	if err != nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("audit repository toolchain: %w", err)
	}
	// Keep one outcome per identity. A successful declaration must never hide
	// contradictory execution/configuration evidence, nor vice versa.
	for _, f := range findings {
		merged := false
		for i, old := range report.Tools {
			if strings.EqualFold(old.ToolID, f.ToolID) {
				if old.Status != "violation" {
					report.Tools[i] = f
				}
				merged = true
				break
			}
		}
		if !merged {
			report.Tools = append(report.Tools, f)
		}
	}
	report.Violations = 0
	for _, group := range [][]ToolVerificationFinding{report.Tools, report.HostPrerequisites} {
		for _, f := range group {
			if f.Status == "violation" {
				report.Violations++
			}
		}
	}
	report.Valid = report.Violations == 0
	return report, nil
}

func launcherFinding(id, code, message string) ToolVerificationFinding {
	f := finding(id, NonConformant, "violation", code, message)
	f.Remediation = "Use ./bin/backlogit.exe with --cwd bound to the workspace, a terminating missing-file check before invocation, and workspace-scoped COPILOT_HOME and ENGRAM_DATA_DIR."
	return f
}

func toolRemediation(f ToolVerificationFinding, m *ToolchainManifest) ToolVerificationFinding {
	if f.Status != "violation" {
		return f
	}
	path := ""
	for _, tool := range m.Tools {
		if strings.EqualFold(tool.ID, f.ToolID) {
			rel, err := relativePath(m.WorkspaceRoot, tool.ExecutablePath, true)
			if err == nil && strings.HasPrefix(filepath.ToSlash(rel), "bin/") {
				path = "./" + filepath.ToSlash(rel)
			}
			break
		}
	}
	if path == "" {
		switch strings.ToLower(f.ToolID) {
		case "backlogit", "engram", "bun", "bunx":
			path = "./bin/" + strings.ToLower(f.ToolID) + ".exe"
		case "github", "github-mcp-server":
			path = "./bin/github-mcp-server.exe"
		case "graphtor-docs", "kbdb":
			path = "./bin/kbdb.exe"
		case "context7", "tavily":
			path = "./bin/bun.exe with a pinned workspace-local MCP entrypoint and lockfile"
		}
	}
	if path != "" {
		f.Remediation = "Configure and verify " + path + "; refuse ambient PATH fallback and bind state/--cwd to this workspace."
	}
	return f
}

func verifyLaunchers(rootPath string, m *ToolchainManifest) ([]ToolVerificationFinding, error) {
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("verify launcher scripts: %w", err)
	}
	defer func() { _ = root.Close() }()
	findings, err := ValidateMCPConfigPinning(abs, ".mcp.json", m)
	if err != nil {
		findings = []ToolVerificationFinding{launcherFinding(".mcp.json", "mcp_config_invalid",
			"MCP configuration is missing, redirected, unreadable or invalid; restore a workspace-pinned configuration.")}
	}
	hasBacklogit := false
	for i, f := range findings {
		hasBacklogit = hasBacklogit || strings.EqualFold(f.ToolID, "backlogit")
		findings[i] = toolRemediation(f, m)
	}
	if !hasBacklogit {
		findings = append(findings, launcherFinding("backlogit", "mcp_backlogit_missing", ".mcp.json must declare the pinned backlogit server."))
	}
	for _, name := range []string{"start.ps1", "start.sh"} {
		file, err := openLocal(root, abs, name)
		if err != nil {
			findings = append(findings, launcherFinding(name, "launcher_file_invalid", name+" is missing, redirected or not a readable regular file."))
			continue
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read launcher %s: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close launcher %s: %w", name, closeErr)
		}
		findings = append(findings, inspectLauncher(root, abs, name, string(data), m)...)
	}
	return findings, nil
}

// launcherProfile is a conservative recognizer, not a shell interpreter.
// Only root-anchored assignments, terminating guards, state exports and
// direct local invocations can establish positive evidence. Unknown control
// flow, interpolation or command substitution always prevents verification.
type launcherProfile struct {
	root      *os.Root
	abs       string
	name      string
	ps        bool
	vars      map[string]string
	state     map[string]string
	guards    map[string]bool
	manifest  *ToolchainManifest
	findings  []ToolVerificationFinding
	backlogit bool
}

func inspectLauncher(root *os.Root, abs, name, data string, m *ToolchainManifest) []ToolVerificationFinding {
	p := launcherProfile{root: root, abs: abs, name: name, ps: name == "start.ps1",
		vars: make(map[string]string), state: make(map[string]string), guards: make(map[string]bool), manifest: m}
	if p.ps {
		p.vars["psscriptroot"] = abs
	}
	assignment := regexp.MustCompile(`^(\$[A-Za-z_][A-Za-z0-9_:]*|(?:export )?[A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p.inspectLookups(line)
		if p.guard(line) {
			continue
		}
		if parts := assignment.FindStringSubmatch(line); parts != nil {
			if !p.ps && line != parts[1]+"="+parts[2] {
				p.unsupported()
				continue
			}
			p.assign(parts[1], parts[2])
			continue
		}
		p.invoke(line)
	}
	if !p.backlogit {
		p.fail("launcher_backlogit_unverified", "No verified, guarded ./bin/backlogit.exe invocation with workspace --cwd binding.")
	}
	if len(p.findings) == 0 {
		return []ToolVerificationFinding{finding("backlogit", WorkspaceLocalPinned, "verified", "launcher_verified",
			name+": static local invocation, fail-closed guard and workspace state verified; no runtime sandbox asserted.")}
	}
	return p.findings
}

func (p *launcherProfile) fail(code, message string) {
	p.findings = append(p.findings, launcherFinding(p.name, code, p.name+": "+message))
}

func (p launcherProfile) key(s string) string {
	s = strings.TrimPrefix(s, "$")
	if p.ps {
		return strings.ToLower(s)
	}
	return s
}

func (p *launcherProfile) assign(name, expression string) {
	if p.ps != strings.HasPrefix(name, "$") {
		p.unsupported()
		return
	}
	exported := strings.HasPrefix(name, "export ")
	name = strings.TrimPrefix(name, "export ")
	key := p.key(name)
	// Reassignments make earlier guards/bindings stale. Refuse them rather
	// than attempting dataflow through arbitrary shell programs.
	if _, exists := p.vars[key]; exists {
		p.fail("launcher_reassignment", "Reassigned path variable cannot establish static pinning.")
		delete(p.vars, key)
		return
	}
	value := p.path(expression)
	if !p.ps && (expression == `$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)` ||
		expression == `"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`) {
		value = p.abs
	}
	if value == "" {
		p.fail("launcher_path_invalid", "Unresolved, global, external or traversing path assignment; local pinning cannot be proven.")
		return
	}
	p.vars[key] = value
	stateKey := name
	if p.ps {
		stateKey = strings.ToUpper(strings.TrimPrefix(key, "env:"))
		exported = strings.HasPrefix(key, "env:")
	}
	if exported && (stateKey == "COPILOT_HOME" || stateKey == "ENGRAM_DATA_DIR") {
		if !bindingWithin(p.root, p.abs, value) || samePath(value, p.abs) || !p.directory(value) {
			p.fail("launcher_state_invalid", "COPILOT_HOME and ENGRAM_DATA_DIR must be contained workspace state directories, not files or redirected paths.")
			return
		}
		p.state[stateKey] = value
	}
}

// directory checks existing state directories without creating missing ones.
func (p launcherProfile) directory(path string) bool {
	rel, err := relativePath(p.abs, path, false)
	if err != nil {
		return false
	}
	info, err := p.root.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	return err == nil && info.IsDir()
}

func (p launcherProfile) path(expression string) string {
	expression = strings.TrimSpace(expression)
	if p.ps {
		join := regexp.MustCompile(`(?i)^Join-Path\s+(\$[A-Za-z_][A-Za-z0-9_]*)\s+(["'])([^"'$` + "`" + `]+)(["'])$`)
		if parts := join.FindStringSubmatch(expression); parts != nil {
			if parts[2] != parts[4] {
				return ""
			}
			base := p.vars[p.key(parts[1])]
			child, err := relativePath(p.abs, filepath.FromSlash(parts[3]), false)
			if base != "" && err == nil && !filepath.IsAbs(parts[3]) {
				value := filepath.Join(base, child)
				if bindingWithin(p.root, p.abs, value) {
					return value
				}
			}
			return ""
		}
	}
	if strings.Contains(expression, `"`) {
		if !strings.HasPrefix(expression, `"`) || !strings.HasSuffix(expression, `"`) || strings.Count(expression, `"`) != 2 {
			return ""
		}
		expression = expression[1 : len(expression)-1]
	}
	variable := regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))([/\\][A-Za-z0-9_./\\-]+)?$`)
	parts := variable.FindStringSubmatch(expression)
	if parts == nil {
		return ""
	}
	key := parts[1] + parts[2]
	base := p.vars[p.key(key)]
	if base == "" {
		return ""
	}
	if parts[3] == "" {
		return base
	}
	// PowerShell does not interpolate a suffix on an unquoted variable.
	if p.ps {
		return ""
	}
	child, err := relativePath(p.abs, filepath.FromSlash(parts[3][1:]), false)
	if err != nil {
		return ""
	}
	value := filepath.Join(base, child)
	if !bindingWithin(p.root, p.abs, value) {
		return ""
	}
	return value
}

func (p *launcherProfile) guard(line string) bool {
	pattern := `^if \[ ! -f "(\$[A-Za-z_][A-Za-z0-9_]*)" \]; then echo "[^"$` + "`" + `]*" >&2; exit 1; fi$`
	if p.ps {
		pattern = `(?i)^if \(-not \(Test-Path -LiteralPath (\$[A-Za-z_][A-Za-z0-9_]*) -PathType Leaf\)\) \{ throw "[^"$` + "`" + `]*" \}$`
	}
	parts := regexp.MustCompile(pattern).FindStringSubmatch(line)
	if parts == nil {
		return false
	}
	path := p.path(parts[1])
	if path == "" {
		p.fail("launcher_guard_invalid", "Missing-file guard does not resolve to a workspace-local path.")
	} else {
		p.guards[path] = true
	}
	return true
}

func (p *launcherProfile) invoke(line string) {
	if p.ps {
		if !strings.HasPrefix(line, "& ") {
			p.unsupported()
			return
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "& "))
	}
	args := strings.Fields(line)
	if len(args) == 0 {
		return
	}
	if !p.ps && strings.HasPrefix(args[0], "$") {
		p.unsupported() // Unquoted expansion can split/glob a workspace path.
		return
	}
	path := p.path(args[0])
	if path == "" {
		f := launcherFinding(strings.Trim(args[0], `"'`), "launcher_command_invalid", p.name+": command is not a statically resolved local executable.")
		p.findings = append(p.findings, toolRemediation(f, p.manifest))
		return
	}
	var tool *ToolEntry
	for i := range p.manifest.Tools {
		entry := &p.manifest.Tools[i]
		rel, err := relativePath(p.abs, entry.ExecutablePath, true)
		if err == nil && samePath(filepath.Join(p.abs, rel), path) {
			tool = entry
			break
		}
	}
	if tool == nil {
		p.unsupported()
		return
	}
	if tool.ContainmentClass != WorkspaceLocalPinned {
		p.findings = append(p.findings, toolRemediation(launcherFinding(tool.ID, "launcher_class_invalid",
			p.name+": local execution contradicts the declared containment class."), p.manifest))
		return
	}
	if err := verifyLocalTool(p.root, p.abs, *tool); err != nil {
		p.findings = append(p.findings, toolRemediation(launcherFinding(tool.ID, "launcher_tool_invalid",
			p.name+": local tool pin could not be verified."), p.manifest))
		return
	}
	if !p.guards[path] || len(p.state) != 2 {
		p.fail("launcher_not_fail_closed", "Local invocation needs a preceding terminating file check and scoped COPILOT_HOME and ENGRAM_DATA_DIR.")
		return
	}
	cwd := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--cwd" && i+1 < len(args) && cwd == "" {
			i++
			if !p.ps && !strings.HasPrefix(args[i], `"`) {
				p.unsupported()
				return
			}
			cwd = p.path(args[i])
			if cwd == "" {
				p.fail("launcher_cwd_invalid", "Invocation --cwd is not the anchored workspace.")
				return
			}
		} else if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(args[i]) || args[i] == "--cwd" {
			p.unsupported()
			return
		}
	}
	bindings, err := parseBindings(p.root, p.abs, tool.WorkspaceBindingArgs)
	if err != nil {
		p.fail("launcher_binding_invalid", "Tool workspace binding declaration is invalid.")
		return
	}
	for key, expected := range bindings {
		actual := p.state[key]
		if key == "--cwd" {
			actual = cwd
		}
		if actual == "" || !samePath(actual, expected) {
			p.fail("launcher_binding_invalid", "Invocation does not match the declared --cwd, COPILOT_HOME or ENGRAM_DATA_DIR binding.")
			return
		}
	}
	if strings.EqualFold(tool.ID, "backlogit") {
		if !samePath(cwd, p.abs) {
			p.fail("launcher_cwd_invalid", "Backlogit must explicitly bind --cwd to the anchored workspace.")
			return
		}
		p.backlogit = true
	}
}

func (p *launcherProfile) unsupported() {
	f := launcherFinding(p.name, "launcher_static_unsupported",
		p.name+": shell syntax/control flow is outside the static inspection profile; pinning is not proven.")
	f.ContainmentClass = Unsupported
	p.findings = append(p.findings, f)
}

func (p *launcherProfile) inspectLookups(line string) {
	lookup := regexp.MustCompile(`(?i)(?:command\s+-v|Get-Command)\s+["']?([A-Za-z0-9_.-]+)`)
	for _, parts := range lookup.FindAllStringSubmatch(line, -1) {
		name := strings.TrimSuffix(strings.ToLower(parts[1]), ".exe")
		id := ""
		for _, entry := range p.manifest.Tools {
			executable := strings.TrimSuffix(strings.ToLower(filepath.Base(entry.ExecutablePath)), ".exe")
			if strings.EqualFold(name, entry.ID) || name == executable {
				id = entry.ID
				break
			}
		}
		if id == "" {
			switch name {
			case "backlogit", "engram", "bun", "bunx", "kbdb", "graphtor-docs", "context7", "tavily", "github-mcp-server":
				id = name
			}
		}
		if id != "" {
			p.findings = append(p.findings, toolRemediation(launcherFinding(id, "launcher_path_lookup",
				p.name+": ambient PATH resolution cannot establish a workspace-local pin."), p.manifest))
		}
	}
}

// WriteToolchainManifest writes the workspace toolchain manifest.
func WriteToolchainManifest(rootPath string, manifest *ToolchainManifest) error {
	return fmt.Errorf("write toolchain manifest: %w", corerrors.ErrNotImplemented)
}

// RenderPinnedMCPConfig renders the workspace's pinned MCP configuration.
func RenderPinnedMCPConfig(rootPath string, manifest *ToolchainManifest) ([]byte, error) {
	return nil, fmt.Errorf("render pinned MCP config: %w", corerrors.ErrNotImplemented)
}
