package toolchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

var replaceToolchainManifestFile = func(storageHandle *os.Root, storageDirAbs, tmpRel, dstRel string) error {
	return atomicfile.ReplaceTempFileInRoot(storageHandle, storageDirAbs, tmpRel, dstRel, true)
}

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

// VerifyProposedLauncherScripts validates proposed start.ps1 and start.sh
// launcher bytes in-memory against a proposed ToolchainManifest without
// reloading toolchain.yaml or launcher files from disk.
func VerifyProposedLauncherScripts(rootPath string, m *ToolchainManifest, startPS1, startSH []byte) ([]ToolVerificationFinding, error) {
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("verify proposed launcher scripts: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := validateManifestStructure(root, abs, m, false); err != nil {
		return nil, err
	}
	manifestCopy := *m
	manifestCopy.WorkspaceRoot = abs
	var findings []ToolVerificationFinding
	for _, script := range []struct {
		name  string
		bytes []byte
	}{
		{name: "start.ps1", bytes: startPS1},
		{name: "start.sh", bytes: startSH},
	} {
		if script.bytes == nil {
			continue
		}
		if info, statErr := root.Lstat(script.name); statErr == nil {
			link, linkErr := core.IsSymlinkOrReparsePoint(info, filepath.Join(abs, script.name))
			if linkErr != nil || link || !info.Mode().IsRegular() || info.Mode()&os.ModeIrregular != 0 {
				findings = append(findings, launcherFinding(&manifestCopy, script.name, "launcher_file_invalid",
					script.name+": existing launcher file is redirected or not a regular file."))
				continue
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect %s: %w", script.name, statErr)
		}
		findings = append(findings, inspectLauncher(root, abs, script.name, string(script.bytes), &manifestCopy, true)...)
	}
	return findings, nil
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

func normalizedManifestWorkspaceRoot(m *ToolchainManifest) string {
	if m == nil || strings.TrimSpace(m.WorkspaceRoot) == "" {
		return ""
	}
	if abs, err := filepath.Abs(m.WorkspaceRoot); err == nil {
		return abs
	}
	return m.WorkspaceRoot
}

func isReservedToolsDirSegment(seg string) bool {
	return strings.EqualFold(seg, ".git") ||
		strings.EqualFold(seg, ".backlog") ||
		strings.EqualFold(seg, ".backlogit") ||
		strings.EqualFold(seg, ".gitignore") ||
		strings.EqualFold(seg, ".mcp.json") ||
		strings.EqualFold(seg, "start.ps1") ||
		strings.EqualFold(seg, "start.sh")
}

func effectiveManifestToolsDir(m *ToolchainManifest) string {
	workspaceRoot := normalizedManifestWorkspaceRoot(m)
	if m != nil && m.ToolsDir != "" && m.ToolsDir == strings.TrimSpace(m.ToolsDir) &&
		!strings.ContainsRune(m.ToolsDir, '\x00') && !filepath.IsAbs(m.ToolsDir) && filepath.VolumeName(m.ToolsDir) == "" {
		if relDir, err := relativePath(workspaceRoot, m.ToolsDir, false); err == nil && relDir != "." {
			firstSeg, _, _ := strings.Cut(strings.ReplaceAll(filepath.ToSlash(relDir), "\\", "/"), "/")
			if !isReservedToolsDirSegment(firstSeg) {
				return filepath.ToSlash(relDir)
			}
		}
	}
	if m != nil {
		toolDir := func(execPath string) (string, bool) {
			rel, err := relativePath(workspaceRoot, execPath, true)
			if err != nil {
				return "", false
			}
			dir := filepath.Dir(rel)
			if dir == "." || dir == ".." {
				return "", false
			}
			firstSeg, _, _ := strings.Cut(strings.ReplaceAll(filepath.ToSlash(dir), "\\", "/"), "/")
			if isReservedToolsDirSegment(firstSeg) {
				return "", false
			}
			return filepath.ToSlash(dir), true
		}
		for _, tool := range m.Tools {
			if isBacklogitTool(workspaceRoot, tool) {
				if dir, ok := toolDir(tool.ExecutablePath); ok {
					return dir
				}
			}
		}
		for _, tool := range m.Tools {
			if tool.ContainmentClass == WorkspaceLocalPinned {
				if dir, ok := toolDir(tool.ExecutablePath); ok {
					return dir
				}
			}
		}
	}
	return ".bin"
}

func backlogitRemediationPath(m *ToolchainManifest) string {
	return "./" + effectiveManifestToolsDir(m) + "/backlogit.exe"
}

func launcherFinding(m *ToolchainManifest, id, code, message string) ToolVerificationFinding {
	f := finding(id, NonConformant, "violation", code, message)
	f.Remediation = "Use " + backlogitRemediationPath(m) + " with --cwd bound to the workspace, a terminating missing-file check before invocation, and workspace-scoped COPILOT_HOME and ENGRAM_DATA_DIR."
	return f
}

func toolRemediation(f ToolVerificationFinding, m *ToolchainManifest) ToolVerificationFinding {
	if f.Status != "violation" {
		return f
	}
	toolsDir := effectiveManifestToolsDir(m)
	prefix := "./" + toolsDir + "/"
	path := ""
	if m != nil {
		workspaceRoot := normalizedManifestWorkspaceRoot(m)
		for _, tool := range m.Tools {
			if strings.EqualFold(tool.ID, f.ToolID) {
				rel, err := relativePath(workspaceRoot, tool.ExecutablePath, true)
				if err == nil {
					relSlash := filepath.ToSlash(rel)
					if strings.HasPrefix(relSlash, toolsDir+"/") ||
						(runtime.GOOS == "windows" && strings.HasPrefix(strings.ToLower(relSlash), strings.ToLower(toolsDir)+"/")) {
						path = "./" + relSlash
					}
				}
				break
			}
		}
	}
	if path == "" {
		switch strings.ToLower(f.ToolID) {
		case "backlogit", "engram", "bun", "bunx":
			path = prefix + strings.ToLower(f.ToolID) + ".exe"
		case "github", "github-mcp-server":
			path = prefix + "github-mcp-server.exe"
		case "graphtor-docs", "kbdb":
			path = prefix + "kbdb.exe"
		case "context7", "tavily":
			path = prefix + "bun.exe with a pinned workspace-local MCP entrypoint and lockfile"
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
	defer func() {
		// Read-only launcher root cleanup.
		_ = root.Close()
	}()
	findings, err := ValidateMCPConfigPinning(abs, ".mcp.json", m)
	if err != nil {
		findings = []ToolVerificationFinding{launcherFinding(m, ".mcp.json", "mcp_config_invalid",
			"MCP configuration is missing, redirected, unreadable or invalid; restore a workspace-pinned configuration.")}
	}
	backlogitIDs := make(map[string]bool)
	if m != nil {
		for _, tool := range m.Tools {
			if isBacklogitTool(abs, tool) {
				backlogitIDs[strings.ToLower(strings.TrimSpace(tool.ID))] = true
			}
		}
	}
	hasBacklogit := false
	for i, f := range findings {
		id := strings.ToLower(strings.TrimSpace(f.ToolID))
		if id == "backlogit" || backlogitIDs[id] {
			hasBacklogit = true
		}
		findings[i] = toolRemediation(f, m)
	}
	if !hasBacklogit {
		findings = append(findings, launcherFinding(m, "backlogit", "mcp_backlogit_missing", ".mcp.json must declare the pinned backlogit server."))
	}
	for _, name := range []string{"start.ps1", "start.sh"} {
		file, err := openLocal(root, abs, name)
		if err != nil {
			findings = append(findings, launcherFinding(m, name, "launcher_file_invalid", name+" is missing, redirected or not a readable regular file."))
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
		findings = append(findings, inspectLauncher(root, abs, name, string(data), m, false)...)
	}
	return findings, nil
}

// launcherProfile is a conservative recognizer, not a shell interpreter.
// Only root-anchored assignments, terminating guards, state exports and
// direct local invocations can establish positive evidence. Unknown control
// flow, interpolation or command substitution always prevents verification.
type launcherProfile struct {
	root            *os.Root
	abs             string
	name            string
	ps              bool
	allowPreInstall bool
	vars            map[string]string
	state           map[string]string
	guards          map[string]bool
	manifest        *ToolchainManifest
	findings        []ToolVerificationFinding
	backlogit       bool
}

func inspectLauncher(root *os.Root, abs, name, data string, m *ToolchainManifest, allowPreInstall bool) []ToolVerificationFinding {
	p := launcherProfile{root: root, abs: abs, name: name, ps: name == "start.ps1", allowPreInstall: allowPreInstall,
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
		p.fail("launcher_backlogit_unverified", "No verified, guarded "+backlogitRemediationPath(m)+" invocation with workspace --cwd binding.")
	}
	if len(p.findings) == 0 {
		return []ToolVerificationFinding{finding("backlogit", WorkspaceLocalPinned, "verified", "launcher_verified",
			name+": static local invocation, fail-closed guard and workspace state verified; no runtime sandbox asserted.")}
	}
	return p.findings
}

func verifyPreInstallBacklogitTool(root *os.Root, abs string, m *ToolchainManifest, tool ToolEntry) error {
	if !validBacklogitToolInManifest(abs, m, tool) {
		return fmt.Errorf("backlogit must use the workspace-local pinned binary in the declared tools_dir: %w", corerrors.ErrValidation)
	}
	if !bindingWithin(root, abs, tool.ExecutablePath) {
		return fmt.Errorf("backlogit executable path outside or redirected from workspace: %w", corerrors.ErrValidation)
	}
	if strings.TrimSpace(tool.PinnedVersion) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
		return fmt.Errorf("missing version, binary digest, or lockfile pin: %w", corerrors.ErrValidation)
	}
	if strings.TrimSpace(tool.SHA256) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
		return fmt.Errorf("missing binary digest or lockfile provenance: %w", corerrors.ErrValidation)
	}
	bindings, err := parseBindings(root, abs, tool.WorkspaceBindingArgs)
	if err != nil {
		return err
	}
	if bindings["--cwd"] == "" {
		return fmt.Errorf("backlogit requires cwd binding: %w", corerrors.ErrValidation)
	}
	if strings.TrimSpace(tool.LockfilePath) != "" {
		relLock, relErr := relativePath(abs, tool.LockfilePath, false)
		if relErr != nil {
			return relErr
		}
		file, openErr := openLocal(root, abs, relLock)
		if openErr != nil {
			return openErr
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return fmt.Errorf("read lockfile: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close lockfile: %w", closeErr)
		}
		version := strings.TrimSpace(tool.PinnedVersion)
		if len(strings.TrimSpace(string(data))) == 0 ||
			!strings.Contains(string(data), strings.TrimSpace(tool.ID)) ||
			(version != "" && !strings.Contains(string(data), version)) {
			return fmt.Errorf("invalid lockfile provenance: %w", corerrors.ErrValidation)
		}
	}
	return nil
}

func (p *launcherProfile) fail(code, message string) {
	p.findings = append(p.findings, launcherFinding(p.manifest, p.name, code, p.name+": "+message))
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
			slashChild := strings.ReplaceAll(parts[3], "\\", "/")
			child, err := relativePath(p.abs, filepath.FromSlash(slashChild), false)
			if base != "" && err == nil && !filepath.IsAbs(parts[3]) && !strings.HasPrefix(slashChild, "/") {
				value := filepath.Join(base, child)
				if bindingWithin(p.root, p.abs, value) {
					return value
				}
			}
			return ""
		}
	}
	quoted := false
	if strings.Contains(expression, `"`) {
		if !strings.HasPrefix(expression, `"`) || !strings.HasSuffix(expression, `"`) || strings.Count(expression, `"`) != 2 {
			return ""
		}
		expression = expression[1 : len(expression)-1]
		quoted = true
	}
	variablePattern := `^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))([/\\][A-Za-z0-9_./\\-]+)?$`
	if quoted {
		variablePattern = `^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))([/\\][A-Za-z0-9_./\\\[\] #!-]+)?$`
	}
	variable := regexp.MustCompile(variablePattern)
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
	if strings.Contains(parts[3], "\\") || !strings.HasPrefix(parts[3], "/") {
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
		f := launcherFinding(p.manifest, strings.Trim(args[0], `"'`), "launcher_command_invalid", p.name+": command is not a statically resolved local executable.")
		p.findings = append(p.findings, toolRemediation(f, p.manifest))
		return
	}
	var candidates []*ToolEntry
	for i := range p.manifest.Tools {
		entry := &p.manifest.Tools[i]
		rel, err := relativePath(p.abs, entry.ExecutablePath, true)
		if err == nil && samePath(filepath.Join(p.abs, rel), path) {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		p.unsupported()
		return
	}
	argStart := 1
	var tool *ToolEntry
	if isRunner(path) {
		if len(args) < 2 {
			p.fail("launcher_entrypoint_unverified", p.name+": runner invocation requires a declared workspace-local entrypoint.")
			return
		}
		entryArg := p.entrypointArg(args[1])
		if entryArg == "" {
			p.fail("launcher_entrypoint_unverified", p.name+": runner invocation does not match a declared workspace-local entrypoint.")
			return
		}
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate.EntrypointPath) == "" {
				continue
			}
			relEntry, err := relativePath(p.abs, candidate.EntrypointPath, false)
			if err == nil && samePath(filepath.Join(p.abs, relEntry), entryArg) {
				tool = candidate
				break
			}
		}
		if tool == nil {
			p.fail("launcher_entrypoint_unverified", p.name+": runner invocation does not match any declared workspace-local entrypoint.")
			return
		}
		argStart = 2
	} else {
		tool = candidates[0]
	}
	if tool.ContainmentClass != WorkspaceLocalPinned {
		p.findings = append(p.findings, toolRemediation(launcherFinding(p.manifest, tool.ID, "launcher_class_invalid",
			p.name+": local execution contradicts the declared containment class."), p.manifest))
		return
	}
	if err := verifyLocalTool(p.root, p.abs, *tool); err != nil {
		allowPreInstall := false
		if p.allowPreInstall && isBacklogitTool(p.abs, *tool) && errors.Is(err, os.ErrNotExist) {
			if relExec, relErr := relativePath(p.abs, tool.ExecutablePath, true); relErr == nil {
				if f, openErr := openLocal(p.root, p.abs, relExec); errors.Is(openErr, os.ErrNotExist) {
					if preErr := verifyPreInstallBacklogitTool(p.root, p.abs, p.manifest, *tool); preErr == nil {
						allowPreInstall = true
					}
				} else if f != nil {
					_ = f.Close()
				}
			}
		}
		if !allowPreInstall {
			p.findings = append(p.findings, toolRemediation(launcherFinding(p.manifest, tool.ID, "launcher_tool_invalid",
				p.name+": local tool pin could not be verified."), p.manifest))
			return
		}
	}
	if !p.guards[path] || len(p.state) != 2 {
		p.fail("launcher_not_fail_closed", "Local invocation needs a preceding terminating file check and scoped COPILOT_HOME and ENGRAM_DATA_DIR.")
		return
	}
	cwd := ""
	runnerMode := isRunner(path)
	for i := argStart; i < len(args); i++ {
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
		} else if (runnerMode && strings.HasPrefix(args[i], "-")) || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(args[i]) || args[i] == "--cwd" {
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
	if isBacklogitTool(p.abs, *tool) {
		if !samePath(cwd, p.abs) {
			p.fail("launcher_cwd_invalid", "Backlogit must explicitly bind --cwd to the anchored workspace.")
			return
		}
		p.backlogit = true
	}
}

var literalEntrypointPattern = regexp.MustCompile(`^[A-Za-z0-9_./\\@-]+$`)

func (p launcherProfile) entrypointArg(token string) string {
	if !p.ps && strings.HasPrefix(strings.TrimSpace(token), "$") {
		return ""
	}
	if resolved := p.path(token); resolved != "" {
		return resolved
	}
	trimmed := strings.TrimSpace(token)
	if len(trimmed) >= 2 && ((strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`)) ||
		(strings.HasPrefix(trimmed, `'`) && strings.HasSuffix(trimmed, `'`))) {
		inner := trimmed[1 : len(trimmed)-1]
		if strings.ContainsAny(inner, `"'`) {
			return ""
		}
		trimmed = inner
	} else if strings.ContainsAny(trimmed, `"'`) {
		return ""
	}
	if trimmed == "" || strings.HasPrefix(trimmed, "-") || !literalEntrypointPattern.MatchString(trimmed) {
		return ""
	}
	if !p.ps && strings.Contains(trimmed, "\\") {
		return ""
	}
	slashTrimmed := trimmed
	if p.ps {
		slashTrimmed = strings.ReplaceAll(trimmed, "\\", "/")
	}
	if strings.HasPrefix(slashTrimmed, "/") {
		return ""
	}
	rel, err := relativePath(p.abs, filepath.FromSlash(slashTrimmed), false)
	if err != nil {
		return ""
	}
	candidate := filepath.Join(p.abs, rel)
	if !bindingWithin(p.root, p.abs, candidate) {
		return ""
	}
	return candidate
}

func (p *launcherProfile) unsupported() {
	f := launcherFinding(p.manifest, p.name, "launcher_static_unsupported",
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
			p.findings = append(p.findings, toolRemediation(launcherFinding(p.manifest, id, "launcher_path_lookup",
				p.name+": ambient PATH resolution cannot establish a workspace-local pin."), p.manifest))
		}
	}
}

var (
	remoteToolIDPattern      = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
	toolchainManifestTempSeq atomic.Uint64
)

func wrapProvisioningValidation(msg string, err error) error {
	if errors.Is(err, corerrors.ErrValidation) {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return fmt.Errorf("%s: %w (%w)", msg, corerrors.ErrValidation, err)
}

func validateManifestForProvisioning(root *os.Root, abs string, manifest *ToolchainManifest) error {
	return validateManifestStructure(root, abs, manifest, true)
}

func validateManifestStructure(root *os.Root, abs string, manifest *ToolchainManifest, requireNonEmpty bool) error {
	if root == nil || manifest == nil {
		return fmt.Errorf("validate toolchain manifest: %w", corerrors.ErrValidation)
	}
	if manifest.SchemaVersion != "1.0" ||
		manifest.ContainmentLevel != Level1WorkspaceLocalPinned ||
		!manifest.OSContainerSandboxDeferred ||
		manifest.Tools == nil ||
		(requireNonEmpty && (len(manifest.Tools) == 0 || manifest.HostPrerequisites == nil || len(manifest.HostPrerequisites) == 0)) {
		return fmt.Errorf("invalid manifest required fields: %w", corerrors.ErrValidation)
	}
	if strings.TrimSpace(manifest.WorkspaceRoot) == "" {
		return fmt.Errorf("empty manifest workspace root: %w", corerrors.ErrValidation)
	}
	declared, err := resolveManifestWorkspaceRoot(abs, manifest.WorkspaceRoot)
	if err != nil || !samePath(abs, declared) {
		return fmt.Errorf("manifest workspace mismatch: %w", corerrors.ErrValidation)
	}
	if manifest.ToolsDir != "" {
		if _, err := validManifestToolsDir(root, abs, manifest.ToolsDir); err != nil {
			return fmt.Errorf("invalid manifest tools_dir: %w", err)
		}
	}

	hasBacklogitMCP := false
	seen := make(map[string]bool, len(manifest.Tools)+len(manifest.HostPrerequisites))
	for _, tool := range manifest.Tools {
		trimmedID := strings.TrimSpace(tool.ID)
		id := strings.ToLower(trimmedID)
		if id == "" || tool.ID != trimmedID || strings.TrimSpace(tool.Kind) == "" || seen[id] {
			return fmt.Errorf("invalid or duplicate tool identity %q: %w", tool.ID, corerrors.ErrValidation)
		}
		seen[id] = true

		if !validBacklogitToolInManifest(abs, manifest, tool) {
			return fmt.Errorf("backlogit must use the workspace-local pinned binary: %w", corerrors.ErrValidation)
		}
		isBacklogit := isBacklogitTool(abs, tool)
		if isBacklogit && strings.EqualFold(strings.TrimSpace(tool.Kind), "mcp") {
			hasBacklogitMCP = true
		}

		switch tool.ContainmentClass {
		case WorkspaceLocalPinned:
			if strings.TrimSpace(tool.ExecutablePath) == "" {
				return fmt.Errorf("tool %q missing executable path: %w", tool.ID, corerrors.ErrValidation)
			}
			if _, err := relativePath(abs, tool.ExecutablePath, true); err != nil {
				return wrapProvisioningValidation(fmt.Sprintf("tool %q invalid executable path", tool.ID), err)
			}
			if !bindingWithin(root, abs, tool.ExecutablePath) {
				return fmt.Errorf("tool %q executable path redirected or outside workspace: %w", tool.ID, corerrors.ErrValidation)
			}
			if strings.TrimSpace(tool.PinnedVersion) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
				return fmt.Errorf("tool %q missing pinned_version or lockfile_path: %w", tool.ID, corerrors.ErrValidation)
			}
			if strings.TrimSpace(tool.SHA256) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
				return fmt.Errorf("tool %q missing sha256 or lockfile_path: %w", tool.ID, corerrors.ErrValidation)
			}
			if isRunner(tool.ExecutablePath) && strings.TrimSpace(tool.EntrypointPath) == "" {
				return fmt.Errorf("runner tool %q requires entrypoint path: %w", tool.ID, corerrors.ErrValidation)
			}
			if isBacklogit && strings.TrimSpace(tool.EntrypointPath) != "" {
				return fmt.Errorf("backlogit tool must not specify entrypoint_path: %w", corerrors.ErrValidation)
			}
			if tool.EntrypointPath != "" {
				if strings.TrimSpace(tool.EntrypointPath) == "" || tool.EntrypointPath != strings.TrimSpace(tool.EntrypointPath) {
					return fmt.Errorf("tool %q invalid whitespace in entrypoint path: %w", tool.ID, corerrors.ErrValidation)
				}
				relEntry, err := relativePath(abs, tool.EntrypointPath, false)
				if err != nil {
					return wrapProvisioningValidation(fmt.Sprintf("tool %q invalid entrypoint path", tool.ID), err)
				}
				if strings.HasPrefix(filepath.ToSlash(relEntry), "-") || strings.HasPrefix(filepath.Base(relEntry), "-") {
					return fmt.Errorf("tool %q entrypoint path must not start with '-': %w", tool.ID, corerrors.ErrValidation)
				}
				if !bindingWithin(root, abs, tool.EntrypointPath) {
					return fmt.Errorf("tool %q entrypoint path redirected or outside workspace: %w", tool.ID, corerrors.ErrValidation)
				}
			}
			if tool.LockfilePath != "" {
				if strings.TrimSpace(tool.LockfilePath) == "" {
					return fmt.Errorf("tool %q blank lockfile path: %w", tool.ID, corerrors.ErrValidation)
				}
				if _, err := relativePath(abs, tool.LockfilePath, false); err != nil {
					return wrapProvisioningValidation(fmt.Sprintf("tool %q invalid lockfile path", tool.ID), err)
				}
				if !bindingWithin(root, abs, tool.LockfilePath) {
					return fmt.Errorf("tool %q lockfile path redirected or outside workspace: %w", tool.ID, corerrors.ErrValidation)
				}
			}
			bindings, err := parseBindings(root, abs, tool.WorkspaceBindingArgs)
			if err != nil {
				return wrapProvisioningValidation(fmt.Sprintf("tool %q invalid workspace bindings", tool.ID), err)
			}
			for _, envKey := range []string{"COPILOT_HOME", "ENGRAM_DATA_DIR"} {
				if boundPath, ok := bindings[envKey]; ok {
					if samePath(boundPath, abs) {
						return fmt.Errorf("tool %q %s binding must not equal workspace root: %w", tool.ID, envKey, corerrors.ErrValidation)
					}
					if relBound, relErr := relativePath(abs, boundPath, false); relErr == nil {
						if info, statErr := root.Lstat(relBound); statErr == nil && !info.IsDir() {
							return fmt.Errorf("tool %q %s binding must not target a non-directory: %w", tool.ID, envKey, corerrors.ErrValidation)
						}
					}
				}
			}
			if isBacklogit && bindings["--cwd"] == "" {
				return fmt.Errorf("backlogit requires --cwd workspace binding: %w", corerrors.ErrValidation)
			}
		case RemoteDeclared:
			if !remoteToolIDPattern.MatchString(tool.ID) {
				return fmt.Errorf("remote tool %q must use DNS-label safe characters: %w", tool.ID, corerrors.ErrValidation)
			}
			if strings.TrimSpace(tool.RemotePolicy) == "" {
				return fmt.Errorf("remote tool %q requires remote_policy: %w", tool.ID, corerrors.ErrValidation)
			}
		case HostPrerequisite, BuiltInHost:
			if strings.TrimSpace(tool.Reason) == "" {
				return fmt.Errorf("host tool %q requires reason: %w", tool.ID, corerrors.ErrValidation)
			}
		case Unsupported, NonConformant:
			return fmt.Errorf("tool %q has disallowed containment class %q: %w", tool.ID, tool.ContainmentClass, corerrors.ErrValidation)
		default:
			return fmt.Errorf("tool %q has unknown containment class %q: %w", tool.ID, tool.ContainmentClass, corerrors.ErrValidation)
		}
	}

	if !hasBacklogitMCP {
		return fmt.Errorf("manifest must declare a workspace-local-pinned backlogit MCP entry: %w", corerrors.ErrValidation)
	}

	for _, prereq := range manifest.HostPrerequisites {
		trimmedID := strings.TrimSpace(prereq.ID)
		id := strings.ToLower(trimmedID)
		if id == "" ||
			prereq.ID != trimmedID ||
			strings.TrimSpace(prereq.Command) == "" ||
			strings.TrimSpace(prereq.Purpose) == "" ||
			strings.TrimSpace(prereq.Reason) == "" ||
			strings.EqualFold(id, "backlogit") ||
			strings.HasPrefix(strings.ToLower(filepath.Base(strings.TrimSpace(prereq.Command))), "backlogit") ||
			seen[id] {
			return fmt.Errorf("invalid host prerequisite %q: %w", prereq.ID, corerrors.ErrValidation)
		}
		seen[id] = true
	}
	return nil
}

func resolveProvisioningStorageDir(root *os.Root) (string, error) {
	if root == nil {
		return "", fmt.Errorf("missing workspace root handle: %w", corerrors.ErrValidation)
	}
	if override, hasOverride := os.LookupEnv("BACKLOGIT_WORKSPACE_DIR"); hasOverride && override != "" {
		switch override {
		case ".backlog", ".backlogit":
			if _, statErr := root.Lstat(override); errors.Is(statErr, os.ErrNotExist) {
				if err := root.Mkdir(override, 0o755); err != nil {
					return "", fmt.Errorf("create %s directory: %w", override, err)
				}
			} else if statErr != nil {
				return "", fmt.Errorf("inspect %s directory: %w", override, statErr)
			}
			return override, nil
		default:
			return "", fmt.Errorf("invalid BACKLOGIT_WORKSPACE_DIR %q: %w", override, corerrors.ErrValidation)
		}
	}

	_, backlogErr := root.Lstat(".backlog")
	if backlogErr != nil && !errors.Is(backlogErr, os.ErrNotExist) {
		return "", fmt.Errorf("inspect .backlog directory: %w", backlogErr)
	}
	_, backlogitErr := root.Lstat(".backlogit")
	if backlogitErr != nil && !errors.Is(backlogitErr, os.ErrNotExist) {
		return "", fmt.Errorf("inspect .backlogit directory: %w", backlogitErr)
	}
	hasBacklog := backlogErr == nil
	hasBacklogit := backlogitErr == nil
	if hasBacklog && hasBacklogit {
		return "", fmt.Errorf("ambiguous workspace storage: %w", corerrors.ErrValidation)
	}

	if hasBacklog {
		return ".backlog", nil
	}
	if hasBacklogit {
		return ".backlogit", nil
	}
	if err := root.Mkdir(".backlogit", 0o755); err != nil {
		return "", fmt.Errorf("create .backlogit directory: %w", err)
	}
	return ".backlogit", nil
}

// WriteToolchainManifest validates and writes the workspace toolchain manifest.
func WriteToolchainManifest(rootPath string, manifest *ToolchainManifest) error {
	abs, root, err := workspace(rootPath)
	if err != nil {
		return fmt.Errorf("write toolchain manifest: %w", err)
	}
	rootClosed := false
	defer func() {
		if !rootClosed {
			// Cleanup on early error return.
			_ = root.Close()
		}
	}()

	if err := validateManifestForProvisioning(root, abs, manifest); err != nil {
		return err
	}

	storageDir, err := resolveProvisioningStorageDir(root)
	if err != nil {
		return err
	}

	storageInfo, err := root.Lstat(storageDir)
	if err != nil {
		return fmt.Errorf("inspect storage directory %s: %w", storageDir, err)
	}
	link, err := core.IsSymlinkOrReparsePoint(storageInfo, filepath.Join(abs, storageDir))
	if err != nil || link || !storageInfo.IsDir() {
		return fmt.Errorf("invalid storage directory %s: %w", storageDir, corerrors.ErrValidation)
	}

	storageHandle, err := root.OpenRoot(storageDir)
	if err != nil {
		return fmt.Errorf("open storage directory %s: %w", storageDir, err)
	}
	storageClosed := false
	defer func() {
		if !storageClosed {
			// Cleanup on early error return.
			_ = storageHandle.Close()
		}
	}()

	openedStorage, err := storageHandle.Stat(".")
	if err != nil || !os.SameFile(storageInfo, openedStorage) {
		return fmt.Errorf("storage directory changed %s: %w", storageDir, corerrors.ErrValidation)
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal toolchain manifest: %w", err)
	}

	existingInfo, err := storageHandle.Lstat("toolchain.yaml")
	if err == nil {
		existingLink, linkErr := core.IsSymlinkOrReparsePoint(existingInfo, filepath.Join(abs, storageDir, "toolchain.yaml"))
		if linkErr != nil || existingLink || !existingInfo.Mode().IsRegular() {
			return fmt.Errorf("existing toolchain.yaml is redirected or not a regular file: %w", corerrors.ErrValidation)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect toolchain.yaml: %w", err)
	}

	tmpName := fmt.Sprintf("toolchain.yaml.tmp.%d.%d", os.Getpid(), toolchainManifestTempSeq.Add(1))
	file, err := storageHandle.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temp toolchain.yaml: %w", err)
	}
	fileClosed := false
	tmpRemoved := false
	defer func() {
		if !fileClosed {
			// Cleanup on early error return.
			_ = file.Close()
		}
		if !tmpRemoved {
			// Remove orphaned temp file on early error return.
			_ = storageHandle.Remove(tmpName)
		}
	}()

	openedStat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat opened temp toolchain.yaml: %w", err)
	}
	lstatAfterOpen, err := storageHandle.Lstat(tmpName)
	if err != nil {
		return fmt.Errorf("lstat temp toolchain.yaml after open: %w", err)
	}
	if !os.SameFile(openedStat, lstatAfterOpen) {
		return fmt.Errorf("temp toolchain.yaml identity changed after open: %w", corerrors.ErrValidation)
	}

	written := 0
	for written < len(data) {
		n, writeErr := file.Write(data[written:])
		if writeErr != nil {
			return fmt.Errorf("write temp toolchain.yaml: %w", writeErr)
		}
		if n == 0 {
			return fmt.Errorf("short write to temp toolchain.yaml: %w", io.ErrShortWrite)
		}
		written += n
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temp toolchain.yaml: %w", err)
	}
	fileClosed = true
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp toolchain.yaml: %w", err)
	}

	preInfo, err := storageHandle.Lstat("toolchain.yaml")
	if err == nil {
		preLink, linkErr := core.IsSymlinkOrReparsePoint(preInfo, filepath.Join(abs, storageDir, "toolchain.yaml"))
		if linkErr != nil || preLink || !preInfo.Mode().IsRegular() {
			return fmt.Errorf("existing toolchain.yaml is redirected or not a regular file: %w", corerrors.ErrValidation)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("re-inspect toolchain.yaml before replacement: %w", err)
	}

	storageDirAbs := filepath.Join(abs, storageDir)
	if err := replaceToolchainManifestFile(storageHandle, storageDirAbs, tmpName, "toolchain.yaml"); err != nil {
		return fmt.Errorf("replace toolchain.yaml: %w", err)
	}
	tmpRemoved = true

	postInfo, err := storageHandle.Lstat("toolchain.yaml")
	if err != nil {
		return fmt.Errorf("verify replaced toolchain.yaml: %w", err)
	}
	postLink, linkErr := core.IsSymlinkOrReparsePoint(postInfo, filepath.Join(abs, storageDir, "toolchain.yaml"))
	if linkErr != nil || postLink || !postInfo.Mode().IsRegular() {
		return fmt.Errorf("replaced toolchain.yaml is redirected or not a regular file: %w", corerrors.ErrValidation)
	}

	storageClosed = true
	if err := storageHandle.Close(); err != nil {
		return fmt.Errorf("close storage directory %s: %w", storageDir, err)
	}
	rootClosed = true
	if err := root.Close(); err != nil {
		return fmt.Errorf("close workspace root: %w", err)
	}
	return nil
}

type pinnedMCPServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

type pinnedMCPConfig struct {
	MCPServers map[string]pinnedMCPServer `json:"mcpServers"`
}

// RenderPinnedMCPConfig renders the workspace's pinned MCP configuration.
func RenderPinnedMCPConfig(rootPath string, manifest *ToolchainManifest) ([]byte, error) {
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("render pinned MCP config: %w", err)
	}
	rootClosed := false
	defer func() {
		if !rootClosed {
			// Cleanup on early error return.
			_ = root.Close()
		}
	}()

	if err := validateManifestForProvisioning(root, abs, manifest); err != nil {
		return nil, err
	}

	servers := make(map[string]pinnedMCPServer)
	for _, tool := range manifest.Tools {
		if !strings.EqualFold(strings.TrimSpace(tool.Kind), "mcp") {
			continue
		}
		switch tool.ContainmentClass {
		case RemoteDeclared:
			servers[tool.ID] = pinnedMCPServer{
				Type: "http",
				URL:  "https://" + strings.ToLower(tool.ID) + ".invalid/mcp",
			}
		case HostPrerequisite, BuiltInHost:
			continue
		case WorkspaceLocalPinned:
			relExec, err := relativePath(abs, tool.ExecutablePath, true)
			if err != nil {
				return nil, wrapProvisioningValidation(fmt.Sprintf("tool %q invalid executable path", tool.ID), err)
			}
			srv := pinnedMCPServer{
				Type:    "stdio",
				Command: "./" + filepath.ToSlash(relExec),
			}
			switch {
			case isBacklogitTool(abs, tool):
				srv.Args = []string{"mcp", "--cwd", abs}
			case strings.TrimSpace(tool.EntrypointPath) != "":
				relEntry, err := relativePath(abs, tool.EntrypointPath, false)
				if err != nil {
					return nil, wrapProvisioningValidation(fmt.Sprintf("tool %q invalid entrypoint path", tool.ID), err)
				}
				srv.Args = []string{filepath.ToSlash(relEntry)}
			case strings.EqualFold(tool.ID, "engram"):
				srv.Args = []string{"shim"}
			case strings.EqualFold(tool.ID, "graphtor-docs") || strings.EqualFold(tool.ID, "kbdb"):
				srv.Args = []string{"serve"}
			case strings.EqualFold(tool.ID, "github") || strings.EqualFold(tool.ID, "github-mcp-server"):
				srv.Args = []string{"stdio"}
			}
			bindings, err := parseBindings(root, abs, tool.WorkspaceBindingArgs)
			if err != nil {
				return nil, wrapProvisioningValidation(fmt.Sprintf("tool %q invalid workspace bindings", tool.ID), err)
			}
			if cwdVal, hasCWD := bindings["--cwd"]; hasCWD && !isBacklogitTool(abs, tool) {
				srv.Args = append(srv.Args, "--cwd", cwdVal)
			}
			env := make(map[string]string)
			for _, key := range []string{"COPILOT_HOME", "ENGRAM_DATA_DIR"} {
				if val, ok := bindings[key]; ok && val != "" {
					env[key] = val
				}
			}
			if len(env) > 0 {
				srv.Env = env
			}
			servers[tool.ID] = srv
		}
	}

	rendered, err := json.MarshalIndent(pinnedMCPConfig{MCPServers: servers}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal pinned MCP config: %w", err)
	}
	rootClosed = true
	if err := root.Close(); err != nil {
		return nil, fmt.Errorf("close workspace root: %w", err)
	}
	return append(rendered, '\n'), nil
}
