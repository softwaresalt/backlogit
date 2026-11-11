package toolchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"gopkg.in/yaml.v3"
)

func defaultWorkspaceToolchainManifest(rootPath string) *ToolchainManifest {
	m := &ToolchainManifest{
		SchemaVersion: "1.0", ContainmentLevel: Level1WorkspaceLocalPinned,
		OSContainerSandboxDeferred: true, WorkspaceRoot: rootPath,
		Tools: []ToolEntry{}, HostPrerequisites: []HostPrerequisiteEntry{},
	}
	for _, id := range []string{"backlogit", "engram", "graphtor-docs", "context7", "tavily", "github", "bun", "bunx", "graphtor-mcp-shim"} {
		m.Tools = append(m.Tools, ToolEntry{ID: id, Kind: "tool", ContainmentClass: NonConformant,
			Reason: "Installation and pinning have not been verified", WorkspaceBindingArgs: []string{}})
	}
	m.Tools = append(m.Tools, ToolEntry{ID: "agent-host", Kind: "agent", ContainmentClass: BuiltInHost,
		Reason: "Provided by the agent host", WorkspaceBindingArgs: []string{}})
	for _, id := range []string{"pwsh", "bash", "git", "gh", "go", "copilot"} {
		m.HostPrerequisites = append(m.HostPrerequisites, HostPrerequisiteEntry{
			ID: id, Command: id, Purpose: "workspace bootstrap", Reason: "host-provided prerequisite",
		})
	}
	return m
}

// workspace opens a root only after binding it to the supplied directory.
func workspace(rootPath string) (string, *os.Root, error) {
	if strings.TrimSpace(rootPath) == "" {
		return "", nil, fmt.Errorf("empty workspace root: %w", blerrors.ErrValidation)
	}
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return "", nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", nil, fmt.Errorf("inspect workspace root: %w", err)
	}
	link, err := core.IsSymlinkOrReparsePoint(info, abs)
	if err != nil || link || !info.IsDir() {
		return "", nil, fmt.Errorf("invalid workspace root: %w", blerrors.ErrValidation)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return "", nil, fmt.Errorf("open workspace root: %w", err)
	}
	opened, err := r.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		_ = r.Close()
		return "", nil, fmt.Errorf("workspace root changed: %w", blerrors.ErrValidation)
	}
	return abs, r, nil
}

// relativePath refuses traversal and unqualified executable names.
func relativePath(root, path string, executable bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path: %w", blerrors.ErrValidation)
	}
	if executable && !filepath.IsAbs(path) && !strings.ContainsAny(path, `/\`) {
		return "", fmt.Errorf("unqualified executable: %w", blerrors.ErrValidation)
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("path traversal: %w", blerrors.ErrValidation)
		}
	}
	if filepath.IsAbs(path) {
		var err error
		path, err = filepath.Rel(root, path)
		if err != nil {
			return "", fmt.Errorf("relative path: %w", err)
		}
	}
	path = filepath.Clean(path)
	if !filepath.IsLocal(path) || path == "." {
		return "", fmt.Errorf("path outside workspace: %w", blerrors.ErrValidation)
	}
	return path, nil
}

// openLocal walks pinned directory handles and verifies each opened identity.
func openLocal(root *os.Root, absRoot, rel string) (*os.File, error) {
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	var owned []*os.Root
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	path := absRoot
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := current.Lstat(part)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", rel, err)
		}
		link, err := core.IsSymlinkOrReparsePoint(info, path)
		if err != nil {
			return nil, fmt.Errorf("inspect reparse point %s: %w", rel, err)
		}
		if link || info.Mode()&os.ModeIrregular != 0 {
			return nil, fmt.Errorf("redirected path %s: %w", rel, blerrors.ErrValidation)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("not a regular file %s: %w", rel, blerrors.ErrValidation)
			}
			file, err := current.Open(part)
			if err != nil {
				return nil, fmt.Errorf("open %s: %w", rel, err)
			}
			opened, err := file.Stat()
			if err != nil || !os.SameFile(info, opened) {
				_ = file.Close()
				return nil, fmt.Errorf("file identity changed %s: %w", rel, blerrors.ErrValidation)
			}
			return file, nil
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("not a directory %s: %w", rel, blerrors.ErrValidation)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, fmt.Errorf("open directory %s: %w", rel, err)
		}
		owned = append(owned, next)
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			return nil, fmt.Errorf("directory identity changed %s: %w", rel, blerrors.ErrValidation)
		}
		current = next
	}
	return nil, fmt.Errorf("empty path: %w", blerrors.ErrValidation)
}

func loadToolchainManifest(rootPath string) (*ToolchainManifest, error) {
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("load toolchain: %w", err)
	}
	defer func() { _ = root.Close() }()
	// Respect the storage directory precedence without following redirects.
	storage := ".backlogit"
	if _, err := root.Lstat(".backlog"); err == nil {
		storage = ".backlog"
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect storage: %w", err)
	}
	file, err := openLocal(root, abs, filepath.Join(storage, "toolchain.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return defaultWorkspaceToolchainManifest(rootPath), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open toolchain manifest: %w", err)
	}
	defer func() { _ = file.Close() }()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var m ToolchainManifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode toolchain manifest: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("additional YAML document: %w", blerrors.ErrValidation)
	}
	if m.Tools == nil {
		m.Tools = []ToolEntry{}
	}
	if m.HostPrerequisites == nil {
		m.HostPrerequisites = []HostPrerequisiteEntry{}
	}
	for i := range m.Tools {
		if m.Tools[i].WorkspaceBindingArgs == nil {
			m.Tools[i].WorkspaceBindingArgs = []string{}
		}
	}
	return &m, nil
}

func finding(id string, class ToolContainmentClass, status, code, message string) ToolVerificationFinding {
	return ToolVerificationFinding{ToolID: id, ContainmentClass: class, Status: status,
		Code: code, Message: message, Remediation: "Declare and verify a workspace-local pinned tool or an explicit exception."}
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func verifyWorkspaceToolchain(ctx context.Context, rootPath string, m *ToolchainManifest) (*ToolchainVerificationReport, error) {
	if ctx == nil || m == nil {
		return nil, fmt.Errorf("verify toolchain: %w", blerrors.ErrValidation)
	}
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("verify toolchain: %w", err)
	}
	defer func() { _ = root.Close() }()
	declared, err := filepath.Abs(m.WorkspaceRoot)
	if m.WorkspaceRoot == "" || err != nil || !samePath(abs, declared) {
		return nil, fmt.Errorf("manifest workspace mismatch: %w", blerrors.ErrValidation)
	}
	report := &ToolchainVerificationReport{ContainmentLevel: Level1WorkspaceLocalPinned,
		OSContainerSandboxDeferred: true, WorkspaceRoot: abs,
		Tools: []ToolVerificationFinding{}, HostPrerequisites: []ToolVerificationFinding{}}
	if m.SchemaVersion != "1.0" || m.ContainmentLevel != Level1WorkspaceLocalPinned || !m.OSContainerSandboxDeferred {
		return nil, fmt.Errorf("invalid manifest containment declaration: %w", blerrors.ErrValidation)
	}
	seen := make(map[string]bool)
	for _, tool := range m.Tools {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("verify toolchain: %w", err)
		}
		f := finding(tool.ID, tool.ContainmentClass, "declared", "declared", tool.Reason)
		if tool.ID == "" || tool.Kind == "" || seen[tool.ID] {
			f = finding(tool.ID, NonConformant, "violation", "invalid_inventory", "Missing or duplicate tool identity.")
		} else {
			switch tool.ContainmentClass {
			case WorkspaceLocalPinned:
				if err := verifyLocalTool(root, abs, tool); err != nil {
					f = finding(tool.ID, NonConformant, "violation", "local_tool_invalid", err.Error())
				} else {
					f = finding(tool.ID, WorkspaceLocalPinned, "verified", "local_tool_verified", "Workspace-local file and pin verified.")
				}
			case HostPrerequisite, BuiltInHost:
				if tool.Reason == "" {
					f = finding(tool.ID, NonConformant, "violation", "missing_reason", "Host exception requires a reason.")
				}
			case RemoteDeclared:
				if tool.RemotePolicy == "" {
					f = finding(tool.ID, NonConformant, "violation", "missing_remote_policy", "Remote tool requires a policy.")
				}
			case Unsupported, NonConformant:
				f.Status = "violation"
			default:
				f = finding(tool.ID, NonConformant, "violation", "invalid_class", "Unknown containment class.")
			}
		}
		seen[tool.ID] = true
		report.Tools = append(report.Tools, f)
		if f.Status == "violation" {
			report.Violations++
		}
	}
	for _, p := range m.HostPrerequisites {
		f := finding(p.ID, HostPrerequisite, "declared", "host_prerequisite", p.Reason)
		if p.ID == "" || p.Command == "" || p.Purpose == "" || p.Reason == "" || seen[p.ID] {
			f = finding(p.ID, NonConformant, "violation", "invalid_prerequisite", "Invalid host prerequisite declaration.")
			report.Violations++
		}
		seen[p.ID] = true
		report.HostPrerequisites = append(report.HostPrerequisites, f)
	}
	report.Valid = report.Violations == 0
	return report, nil
}

func verifyLocalTool(root *os.Root, abs string, tool ToolEntry) error {
	if tool.PinnedVersion == "" && tool.LockfilePath == "" {
		return fmt.Errorf("missing version or lockfile pin: %w", blerrors.ErrValidation)
	}
	for _, path := range []string{tool.ExecutablePath, tool.EntrypointPath, tool.LockfilePath} {
		if path == "" {
			continue
		}
		rel, err := relativePath(abs, path, path == tool.ExecutablePath)
		if err != nil {
			return err
		}
		file, err := openLocal(root, abs, rel)
		if err != nil {
			return err
		}
		if path == tool.ExecutablePath && tool.SHA256 != "" {
			hash := sha256.New()
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return fmt.Errorf("hash executable: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close executable: %w", closeErr)
			}
			expected, err := hex.DecodeString(tool.SHA256)
			if err != nil || !bytes.Equal(expected, hash.Sum(nil)) {
				return fmt.Errorf("executable SHA-256 mismatch: %w", blerrors.ErrValidation)
			}
		} else if err := file.Close(); err != nil {
			return fmt.Errorf("close local file: %w", err)
		}
	}
	if tool.ExecutablePath == "" {
		return fmt.Errorf("missing executable: %w", blerrors.ErrValidation)
	}
	for i, arg := range tool.WorkspaceBindingArgs {
		if arg == "--cwd" || arg == "COPILOT_HOME" || arg == "ENGRAM_DATA_DIR" {
			if i+1 >= len(tool.WorkspaceBindingArgs) || !bindingWithin(root, abs, tool.WorkspaceBindingArgs[i+1]) {
				return fmt.Errorf("invalid workspace binding: %w", blerrors.ErrValidation)
			}
		} else if strings.HasPrefix(arg, "--cwd=") || strings.HasPrefix(arg, "COPILOT_HOME=") || strings.HasPrefix(arg, "ENGRAM_DATA_DIR=") {
			_, value, _ := strings.Cut(arg, "=")
			if !bindingWithin(root, abs, value) {
				return fmt.Errorf("invalid workspace binding: %w", blerrors.ErrValidation)
			}
		}
	}
	return nil
}

func bindingWithin(handle *os.Root, root, path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) && samePath(path, root) {
		return true
	}
	rel, err := relativePath(root, path, false)
	if err != nil {
		return false
	}
	current := handle
	var owned []*os.Root
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	abs := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		abs = filepath.Join(abs, part)
		info, err := current.Lstat(part)
		if errors.Is(err, os.ErrNotExist) {
			return true
		}
		if err != nil {
			return false
		}
		link, err := core.IsSymlinkOrReparsePoint(info, abs)
		if err != nil || link || info.Mode()&os.ModeIrregular != 0 {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		if !info.IsDir() {
			return false
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return false
		}
		owned = append(owned, next)
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			return false
		}
		current = next
	}
	return true
}

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func validateMCPConfigPinning(rootPath, configPath string, m *ToolchainManifest) ([]ToolVerificationFinding, error) {
	if m == nil {
		return nil, fmt.Errorf("validate MCP config: %w", blerrors.ErrValidation)
	}
	abs, root, err := workspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("validate MCP config: %w", err)
	}
	defer func() { _ = root.Close() }()
	declared, err := filepath.Abs(m.WorkspaceRoot)
	if m.WorkspaceRoot == "" || err != nil || !samePath(abs, declared) {
		return nil, fmt.Errorf("manifest workspace mismatch: %w", blerrors.ErrValidation)
	}
	rel, err := relativePath(abs, configPath, false)
	if err != nil {
		return nil, fmt.Errorf("MCP config path: %w", err)
	}
	file, err := openLocal(root, abs, rel)
	if err != nil {
		return nil, fmt.Errorf("open MCP config: %w", err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read MCP config: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close MCP config: %w", closeErr)
	}
	if err := scanJSON(data); err != nil {
		return nil, fmt.Errorf("validate MCP JSON: %w", err)
	}
	var config struct {
		Servers map[string]mcpServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil || config.Servers == nil {
		return nil, fmt.Errorf("invalid MCP config: %w", blerrors.ErrValidation)
	}
	inventory := make(map[string]ToolEntry, len(m.Tools))
	for _, tool := range m.Tools {
		if tool.ID == "" {
			return nil, fmt.Errorf("empty inventory ID: %w", blerrors.ErrValidation)
		}
		if _, exists := inventory[tool.ID]; exists {
			return nil, fmt.Errorf("duplicate inventory ID: %w", blerrors.ErrValidation)
		}
		inventory[tool.ID] = tool
	}
	names := make([]string, 0, len(config.Servers))
	for name := range config.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	findings := make([]ToolVerificationFinding, 0, len(names))
	for _, name := range names {
		server := config.Servers[name]
		tool, exists := inventory[name]
		if !exists {
			findings = append(findings, finding(name, Unsupported, "violation", "uninventoried_shadow_tool", "MCP server is not inventoried."))
			continue
		}
		command, pathErr := relativePath(abs, server.Command, true)
		expected, expectedErr := relativePath(abs, tool.ExecutablePath, true)
		if tool.ContainmentClass != WorkspaceLocalPinned || pathErr != nil || expectedErr != nil ||
			!samePath(command, expected) || (name == "backlogit" && !samePath(command, filepath.Join("bin", "backlogit.exe"))) {
			findings = append(findings, finding(name, NonConformant, "violation", "mcp_command_invalid", "MCP command does not match a local pinned executable."))
			continue
		}
		if err := verifyLocalTool(root, abs, tool); err != nil {
			findings = append(findings, finding(name, NonConformant, "violation", "local_tool_invalid", err.Error()))
			continue
		}
		bad := false
		for _, key := range []string{"COPILOT_HOME", "ENGRAM_DATA_DIR"} {
			if value, ok := server.Env[key]; ok && !bindingWithin(root, abs, value) {
				bad = true
			}
		}
		for i, arg := range server.Args {
			if arg == "--cwd" && (i+1 >= len(server.Args) || !bindingWithin(root, abs, server.Args[i+1])) {
				bad = true
			}
			if strings.HasPrefix(arg, "--cwd=") && !bindingWithin(root, abs, strings.TrimPrefix(arg, "--cwd=")) {
				bad = true
			}
			if i > 0 && server.Args[i-1] == "--cwd" {
				continue
			}
			script, scriptFlag := strings.CutPrefix(arg, "--script=")
			if scriptFlag || (!strings.HasPrefix(arg, "-") && strings.ContainsAny(arg, `/\`)) {
				if !scriptFlag {
					script = arg
				}
				entry, entryErr := relativePath(abs, tool.EntrypointPath, false)
				actual, actualErr := relativePath(abs, script, false)
				if entryErr != nil || actualErr != nil || !samePath(entry, actual) {
					bad = true
				}
			}
		}
		if bad {
			findings = append(findings, finding(name, NonConformant, "violation", "mcp_binding_invalid", "MCP workspace binding escapes the workspace."))
		} else {
			findings = append(findings, finding(name, WorkspaceLocalPinned, "verified", "mcp_command_verified", "MCP command matches the pinned local tool."))
		}
	}
	return findings, nil
}

func foldJSONKey(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < runes[i] {
				runes[i] = f
			}
		}
	}
	return string(runes)
}

func scanJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var value func(int) error
	value = func(depth int) error {
		if depth > 10000 {
			return fmt.Errorf("JSON nesting too deep: %w", blerrors.ErrValidation)
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delim != '{' && delim != '[' {
			return fmt.Errorf("unexpected JSON delimiter: %w", blerrors.ErrValidation)
		}
		seen := make(map[string]bool)
		for d.More() {
			if delim == '{' {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[foldJSONKey(name)] {
					return fmt.Errorf("duplicate JSON key: %w", blerrors.ErrValidation)
				}
				seen[foldJSONKey(name)] = true
			}
			if err := value(depth + 1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if err := value(0); err != nil {
		return fmt.Errorf("scan JSON: %w", err)
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON data: %w", blerrors.ErrValidation)
	}
	return nil
}
