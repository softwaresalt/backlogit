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
	"net/url"
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
	storage := ""
	for _, candidate := range []string{".backlog", ".backlogit"} {
		if _, err := root.Lstat(candidate); err == nil {
			if storage != "" {
				return nil, fmt.Errorf("ambiguous workspace storage: %w", blerrors.ErrValidation)
			}
			storage = candidate
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect storage: %w", err)
		}
	}
	if storage == "" {
		return defaultWorkspaceToolchainManifest(rootPath), nil
	}
	file, err := openLocal(root, abs, filepath.Join(storage, "toolchain.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return defaultWorkspaceToolchainManifest(rootPath), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open toolchain manifest: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read toolchain manifest: %w", err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse toolchain manifest: %w", err)
	}
	if err := validateYAMLKeys(&node); err != nil {
		return nil, fmt.Errorf("validate toolchain manifest: %w", err)
	}
	if err := validateManifestFields(&node); err != nil {
		return nil, fmt.Errorf("validate toolchain manifest: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var m ToolchainManifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode toolchain manifest: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("additional YAML document: %w", blerrors.ErrValidation)
	}
	declared, err := filepath.Abs(m.WorkspaceRoot)
	if m.SchemaVersion != "1.0" || m.ContainmentLevel != Level1WorkspaceLocalPinned ||
		!m.OSContainerSandboxDeferred || strings.TrimSpace(m.WorkspaceRoot) == "" ||
		err != nil || !samePath(abs, declared) || m.Tools == nil || m.HostPrerequisites == nil {
		return nil, fmt.Errorf("invalid manifest required fields: %w", blerrors.ErrValidation)
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

func validateYAMLKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := foldJSONKey(node.Content[i].Value)
			if seen[key] {
				return fmt.Errorf("duplicate YAML key: %w", blerrors.ErrValidation)
			}
			seen[key] = true
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func validateManifestFields(document *yaml.Node) error {
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("expected manifest mapping: %w", blerrors.ErrValidation)
	}
	fields := func(node *yaml.Node, required ...string) error {
		if node.Kind != yaml.MappingNode {
			return fmt.Errorf("expected mapping: %w", blerrors.ErrValidation)
		}
		found := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			found[node.Content[i].Value] = true
		}
		for _, key := range required {
			if !found[key] {
				return fmt.Errorf("missing required field %s: %w", key, blerrors.ErrValidation)
			}
		}
		return nil
	}
	mapping := document.Content[0]
	if err := fields(mapping, "schema_version", "containment_level", "os_container_sandbox_deferred", "workspace_root", "tools", "host_prerequisites"); err != nil {
		return err
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		name, value := mapping.Content[i].Value, mapping.Content[i+1]
		if name != "tools" && name != "host_prerequisites" {
			continue
		}
		if value.Kind != yaml.SequenceNode {
			return fmt.Errorf("expected %s sequence: %w", name, blerrors.ErrValidation)
		}
		for _, item := range value.Content {
			var err error
			if name == "tools" {
				err = fields(item, "id", "kind", "containment_class")
			} else {
				err = fields(item, "id", "command", "purpose", "required", "reason")
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
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
		if strings.TrimSpace(tool.ID) == "" || strings.TrimSpace(tool.Kind) == "" || seen[tool.ID] {
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
				if strings.TrimSpace(tool.Reason) == "" {
					f = finding(tool.ID, NonConformant, "violation", "missing_reason", "Host exception requires a reason.")
				}
			case RemoteDeclared:
				if strings.TrimSpace(tool.RemotePolicy) == "" {
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
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Command) == "" || strings.TrimSpace(p.Purpose) == "" || strings.TrimSpace(p.Reason) == "" || seen[p.ID] {
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
	if strings.TrimSpace(tool.PinnedVersion) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
		return fmt.Errorf("missing version, binary digest, or lockfile pin: %w", blerrors.ErrValidation)
	}
	if strings.TrimSpace(tool.SHA256) == "" && strings.TrimSpace(tool.LockfilePath) == "" {
		return fmt.Errorf("missing binary digest or lockfile provenance: %w", blerrors.ErrValidation)
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
		if path == tool.EntrypointPath && strings.TrimSpace(tool.EntrypointPath) != "" {
			data, readErr := io.ReadAll(file)
			if readErr != nil {
				_ = file.Close()
				return fmt.Errorf("read entrypoint: %w", readErr)
			}
			if len(bytes.TrimSpace(data)) == 0 {
				_ = file.Close()
				return fmt.Errorf("empty entrypoint: %w", blerrors.ErrValidation)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				_ = file.Close()
				return fmt.Errorf("rewind entrypoint: %w", err)
			}
		}
		if path == tool.LockfilePath && strings.TrimSpace(tool.LockfilePath) != "" {
			data, readErr := io.ReadAll(file)
			if readErr != nil {
				_ = file.Close()
				return fmt.Errorf("read lockfile: %w", readErr)
			}
			version := strings.TrimSpace(tool.PinnedVersion)
			if len(bytes.TrimSpace(data)) == 0 ||
				!strings.Contains(string(data), strings.TrimSpace(tool.ID)) ||
				(version != "" && !strings.Contains(string(data), version)) {
				_ = file.Close()
				return fmt.Errorf("invalid lockfile provenance: %w", blerrors.ErrValidation)
			}
			if path == tool.ExecutablePath && tool.SHA256 != "" {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					_ = file.Close()
					return fmt.Errorf("rewind executable: %w", err)
				}
			}
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
		} else {
			if err := file.Close(); err != nil {
				return fmt.Errorf("close local file: %w", err)
			}
		}
	}
	if tool.ExecutablePath == "" {
		return fmt.Errorf("missing executable: %w", blerrors.ErrValidation)
	}
	if _, err := parseBindings(root, abs, tool.WorkspaceBindingArgs); err != nil {
		return err
	}
	return nil
}

func parseBindings(root *os.Root, abs string, args []string) (map[string]string, error) {
	bindings := make(map[string]string)
	for i := 0; i < len(args); i++ {
		key, value, hasValue := strings.Cut(args[i], "=")
		key = strings.TrimSpace(key)
		if key != "--cwd" {
			key = strings.ToUpper(strings.TrimSpace(key))
		}
		if key != "--cwd" && key != "COPILOT_HOME" && key != "ENGRAM_DATA_DIR" {
			return nil, fmt.Errorf("unknown workspace binding: %w", blerrors.ErrValidation)
		}
		if !hasValue {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("missing workspace binding: %w", blerrors.ErrValidation)
			}
			value = args[i]
		}
		if _, exists := bindings[key]; exists || !bindingWithin(root, abs, value) {
			return nil, fmt.Errorf("invalid workspace binding: %w", blerrors.ErrValidation)
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(abs, value)
		}
		bindings[key] = filepath.Clean(value)
	}
	return bindings, nil
}

func bindingWithin(handle *os.Root, root, path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) && samePath(path, root) {
		return true
	}
	if path == "." {
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
	Type    string            `json:"type"`
	URL     string            `json:"url"`
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
		switch tool.ContainmentClass {
		case RemoteDeclared:
			remoteAddress := strings.TrimSpace(server.URL)
			remoteURL, urlErr := url.Parse(remoteAddress)
			if strings.TrimSpace(tool.RemotePolicy) == "" || strings.TrimSpace(server.Command) != "" || len(server.Args) != 0 || len(server.Env) != 0 ||
				strings.EqualFold(strings.TrimSpace(server.Type), "stdio") || (remoteAddress != "" &&
				(urlErr != nil || remoteURL.Host == "" || (!strings.EqualFold(remoteURL.Scheme, "http") && !strings.EqualFold(remoteURL.Scheme, "https")))) {
				findings = append(findings, finding(name, NonConformant, "violation", "invalid_remote_server", "Remote server requires a policy and no local execution configuration."))
			} else {
				findings = append(findings, finding(name, RemoteDeclared, "declared", "remote_declared", tool.RemotePolicy))
			}
			continue
		case HostPrerequisite, BuiltInHost:
			if strings.TrimSpace(tool.Reason) == "" {
				findings = append(findings, finding(name, NonConformant, "violation", "missing_reason", "Host-provided server requires a reason."))
			} else if strings.TrimSpace(server.Command) != "" || strings.TrimSpace(server.URL) != "" || strings.TrimSpace(server.Type) != "" || len(server.Args) != 0 || len(server.Env) != 0 {
				findings = append(findings, finding(name, NonConformant, "violation", "invalid_host_server", "Host-provided server cannot specify local execution configuration."))
			} else {
				findings = append(findings, finding(name, tool.ContainmentClass, "declared", string(tool.ContainmentClass), tool.Reason))
			}
			continue
		}
		command, pathErr := relativePath(abs, server.Command, true)
		expected, expectedErr := relativePath(abs, tool.ExecutablePath, true)
		serverType := strings.TrimSpace(server.Type)
		if tool.ContainmentClass != WorkspaceLocalPinned || strings.TrimSpace(server.URL) != "" || (serverType != "" && !strings.EqualFold(serverType, "stdio")) || pathErr != nil || expectedErr != nil ||
			!samePath(command, expected) || (name == "backlogit" && !samePath(command, filepath.Join("bin", "backlogit.exe"))) {
			findings = append(findings, finding(name, NonConformant, "violation", "mcp_command_invalid", "MCP command does not match a local pinned executable."))
			continue
		}
		if err := verifyLocalTool(root, abs, tool); err != nil {
			findings = append(findings, finding(name, NonConformant, "violation", "local_tool_invalid", err.Error()))
			continue
		}
		bad := false
		expectedBindings, _ := parseBindings(root, abs, tool.WorkspaceBindingArgs)
		actualBindings := make(map[string]string)
		for key, value := range server.Env {
			key = strings.ToUpper(strings.TrimSpace(key))
			canonical := ""
			switch {
			case strings.EqualFold(key, "COPILOT_HOME"):
				canonical = "COPILOT_HOME"
			case strings.EqualFold(key, "ENGRAM_DATA_DIR"):
				canonical = "ENGRAM_DATA_DIR"
			}
			if canonical != "" {
				within := bindingWithin(root, abs, value)
				if _, exists := actualBindings[canonical]; exists || !within {
					bad = true
				}
				if !filepath.IsAbs(value) {
					value = filepath.Join(abs, value)
				}
				actualBindings[canonical] = filepath.Clean(value)
			} else {
				bad = true
			}
		}
		entryFound := false
		entryCount := 0
		hasEntry := strings.TrimSpace(tool.EntrypointPath) != ""
		for i, arg := range server.Args {
			if i > 0 && server.Args[i-1] == "--cwd" {
				continue
			}
			flag, _, hasValue := strings.Cut(arg, "=")
			lowerFlag := strings.ToLower(flag)
			if strings.HasPrefix(lowerFlag, "-") && !strings.HasPrefix(lowerFlag, "--") &&
				(strings.HasPrefix(lowerFlag, "-e") || strings.HasPrefix(lowerFlag, "-c") ||
					strings.HasPrefix(lowerFlag, "-m") || strings.HasPrefix(lowerFlag, "-p") || strings.HasPrefix(lowerFlag, "-r")) {
				bad = true
				continue
			}
			switch lowerFlag {
			case "-e", "-c", "-m", "-p", "-r", "--eval", "--print", "--import", "--require", "--loader", "-command", "-encodedcommand", "-file":
				bad = true
				continue
			}
			if hasValue && strings.HasPrefix(flag, "-") && flag != "--cwd" && !(hasEntry && flag == "--script") {
				bad = true
				continue
			}
			if arg == "--cwd" || strings.HasPrefix(arg, "--cwd=") {
				value := strings.TrimPrefix(arg, "--cwd=")
				if arg == "--cwd" {
					if i+1 >= len(server.Args) {
						bad = true
						continue
					}
					value = server.Args[i+1]
				}
				if _, exists := actualBindings["--cwd"]; exists || !bindingWithin(root, abs, value) {
					bad = true
				}
				if !filepath.IsAbs(value) {
					value = filepath.Join(abs, value)
				}
				actualBindings["--cwd"] = filepath.Clean(value)
				continue
			}
			value := arg
			positional := !strings.HasPrefix(arg, "-")
			scriptFlag := strings.HasPrefix(arg, "--script=")
			if _, after, ok := strings.Cut(arg, "="); ok && strings.HasPrefix(arg, "-") {
				value = after
			}
			pathLike := strings.ContainsAny(value, `/\`) || filepath.IsAbs(value)
			if !hasEntry {
				ext := strings.ToLower(filepath.Ext(value))
				switch ext {
				case ".js", ".cjs", ".mjs", ".ts", ".py", ".ps1", ".sh", ".exe", ".cmd", ".bat":
					bad = true
				}
				if pathLike || strings.Contains(value, "..") {
					bad = true
				}
			} else if positional || scriptFlag || pathLike {
				entry, entryErr := relativePath(abs, tool.EntrypointPath, false)
				actual, actualErr := relativePath(abs, value, false)
				if entryErr != nil || actualErr != nil || !samePath(entry, actual) {
					bad = true
				} else if positional || scriptFlag {
					entryFound = true
				}
				if positional || scriptFlag {
					entryCount++
				}
			}
		}
		if hasEntry && (!entryFound || entryCount != 1) {
			bad = true
		}
		for key, expected := range expectedBindings {
			actual, exists := actualBindings[key]
			if !exists || !samePath(actual, expected) {
				bad = true
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
