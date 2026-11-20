package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/toolchain"
	"github.com/softwaresalt/backlogit/internal/version"
)

// MCPDaemonInfo is the sidecar record describing the MCP daemon that holds the
// workspace lock. ProcessStartTime and ExecutablePath bind the record to the
// holder's OS process instance. StartedAt is serialized as RFC 3339.
type MCPDaemonInfo struct {
	PID              int       `json:"pid"`
	ProcessStartTime int64     `json:"process_start_time"`
	ExecutablePath   string    `json:"executable_path"`
	StartedAt        time.Time `json:"started_at"`
	WorkspaceRoot    string    `json:"workspace_root"`
	StorageRoot      string    `json:"storage_root"`
	Version          string    `json:"version"`
	LockPath         string    `json:"lock_path"`
	IPCTransport     string    `json:"ipc_transport,omitempty"`
	Endpoint         string    `json:"endpoint,omitempty"`
	ActiveSessions   int       `json:"active_sessions"`
	ProtocolVersion  string    `json:"protocol_version,omitempty"`
}

// MCPDaemonStatus reports whether an MCP daemon is running for a workspace.
type MCPDaemonStatus struct {
	Running         bool      `json:"running"`
	PID             int       `json:"pid"`
	StartedAt       time.Time `json:"started_at"`
	WorkspaceRoot   string    `json:"workspace_root"`
	StorageRoot     string    `json:"storage_root"`
	Version         string    `json:"version"`
	LockPath        string    `json:"lock_path"`
	IPCTransport    string    `json:"ipc_transport,omitempty"`
	Endpoint        string    `json:"endpoint,omitempty"`
	ActiveSessions  int       `json:"active_sessions"`
	ProtocolVersion string    `json:"protocol_version,omitempty"`
}

// VerifyMCPDaemonAttachment checks identity and compatibility without changing
// the running daemon or interrupting its peer sessions.
func VerifyMCPDaemonAttachment(info MCPDaemonInfo, workspaceRoot, storageRoot, expectedVersion, expectedProtocolVersion string) error {
	if !sameDaemonPath(info.WorkspaceRoot, workspaceRoot) || !sameDaemonPath(info.StorageRoot, storageRoot) {
		return fmt.Errorf("daemon pid %d belongs to workspace %s (storage %s): %w",
			info.PID, info.WorkspaceRoot, info.StorageRoot, corerrors.ErrDaemonWorkspaceMismatch)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve attachment executable: %w", err)
	}
	// In-process hosts use their own image; library callers may instead attach
	// to the workspace-pinned CLI image, never an arbitrary PATH executable.
	if !sameDaemonPath(info.ExecutablePath, exe) {
		pinned, pinnedErr := pinnedMCPDaemonBinary(workspaceRoot)
		if pinnedErr == nil {
			exe = pinned
		}
	}
	if !sameDaemonPath(info.ExecutablePath, exe) || info.Version != expectedVersion ||
		info.ProtocolVersion != expectedProtocolVersion {
		return fmt.Errorf("daemon pid %d has version %s, protocol %s, binary %s; expected %s, %s, %s; active_sessions=%d; run `backlogit mcp stop` or `backlogit mcp --replace`: %w",
			info.PID, info.Version, info.ProtocolVersion, info.ExecutablePath, expectedVersion,
			expectedProtocolVersion, exe, info.ActiveSessions, corerrors.ErrDaemonVersionMismatch)
	}
	return nil
}

// RunMCPDaemon hosts independent MCP streams until stop, idle expiry, or
// cancellation, then joins sessions and closes their workspace before unlocking.
func RunMCPDaemon(ctx context.Context, rootPath string, replace bool, idleTimeout time.Duration) (result error) {
	if ctx == nil || rootPath == "" || idleTimeout < 0 {
		return mcpDaemonValidationError("daemon requires a context, workspace, and nonnegative idle timeout")
	}
	if idleTimeout == 0 {
		idleTimeout = 15 * time.Minute
	}
	release, err := AcquireMCPDaemonLock(ctx, rootPath, replace)
	if err != nil {
		return fmt.Errorf("acquire daemon ownership: %w", err)
	}
	defer func() { result = errors.Join(result, release()) }()
	_, _, runtimeDir, runtimeRoot, err := openMCPDaemonRuntimeRoot(rootPath)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, runtimeRoot.Close()) }()
	info, err := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
	if err != nil {
		return err
	}
	s, err := openMCPServer(ctx, info.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("open daemon server: %w", err)
	}
	defer func() {
		result = errors.Join(result, s.Shutdown())
		if s.Workspace != nil {
			result = errors.Join(result, s.Workspace.Close())
		}
	}()
	info.IPCTransport, info.Endpoint, err = DeriveMCPDaemonEndpoint(info.WorkspaceRoot, info.StorageRoot, runtimeDir)
	if err != nil {
		return err
	}
	info.ProtocolVersion = mcpDaemonProtocolVersion
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	listener, err := ListenMCPDaemonIPC(runCtx, runtimeRoot, runtimeDir, info.IPCTransport, info.Endpoint)
	if err != nil {
		return fmt.Errorf("listen for daemon sessions: %w", err)
	}
	defer func() { result = errors.Join(result, listener.Close()) }()
	if err := removeMCPDaemonSidecar(runtimeRoot); err != nil {
		return err
	}
	if err := writeMCPDaemonSidecar(runtimeRoot, runtimeDir, info); err != nil {
		return err
	}
	runtime := &mcpDaemonRuntime{server: s, info: info, stop: make(chan struct{}), idleSince: time.Now()}
	accepted := make(chan error, 1)
	var workers sync.WaitGroup
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				accepted <- err
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				runtime.serve(runCtx, conn)
			}()
		}
	}()
	tick := time.NewTicker(min(idleTimeout, 100*time.Millisecond))
	defer tick.Stop()
	acceptFinished := false
wait:
	for {
		select {
		case <-ctx.Done():
			result = fmt.Errorf("run MCP daemon: %w", ctx.Err())
			break wait
		case <-runtime.stop:
			break wait
		case err := <-accepted:
			acceptFinished = true
			result = fmt.Errorf("accept daemon connection: %w", err)
			break wait
		case <-tick.C:
			runtime.mu.Lock()
			idle := runtime.sessions == 0 && time.Since(runtime.idleSince) >= idleTimeout
			runtime.mu.Unlock()
			if idle {
				break wait
			}
		}
	}
	cancel()
	result = errors.Join(result, listener.Close())
	if !acceptFinished {
		<-accepted // Accept owns all worker admissions; join it before Wait.
	}
	result = errors.Join(result, s.Shutdown())
	workers.Wait()
	return result
}

const mcpDaemonProtocolVersion = "backlogit-mcp-ipc/v1"

type mcpDaemonHeader struct {
	ProtocolVersion string `json:"protocol_version"`
	Command         string `json:"command"`
	WorkspaceRoot   string `json:"workspace_root"`
	StorageRoot     string `json:"storage_root"`
	Version         string `json:"version"`
}

type mcpDaemonRuntime struct {
	server    *mcpinternal.Server
	info      MCPDaemonInfo
	stop      chan struct{}
	stopOnce  sync.Once
	mu        sync.Mutex
	sessions  int
	nextID    uint64
	idleSince time.Time
}

func (d *mcpDaemonRuntime) serve(ctx context.Context, conn net.Conn) {
	// Closing the connection interrupts both an incomplete handshake and the
	// buffered session reader. Join the callback before returning ownership.
	closed := make(chan struct{})
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close() // The handler also closes its owned connection below.
		close(closed)
	})
	defer func() {
		if !stopClose() {
			<-closed
		}
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Debug("close daemon connection", "error", err)
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		slog.Warn("set daemon handshake deadline", "error", err)
		return
	}
	reader := bufio.NewReaderSize(conn, maxMCPDaemonInfoBytes)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		slog.Debug("read daemon handshake", "error", err)
		return
	}
	var header mcpDaemonHeader
	if err := json.Unmarshal(line, &header); err != nil {
		slog.Debug("decode daemon handshake", "error", err)
		return
	}
	attachment := d.info
	attachment.WorkspaceRoot, attachment.StorageRoot = header.WorkspaceRoot, header.StorageRoot
	attachment.Version, attachment.ProtocolVersion = header.Version, header.ProtocolVersion
	if err := VerifyMCPDaemonAttachment(attachment, d.info.WorkspaceRoot, d.info.StorageRoot,
		d.info.Version, d.info.ProtocolVersion); err != nil {
		slog.Warn("refuse daemon attachment", "error", err)
		return
	}
	switch header.Command {
	case "status", "stop":
		status := mcpDaemonStatusFromInfo(d.info)
		status.ActiveSessions = d.server.ActiveSessionCount()
		if err := json.NewEncoder(conn).Encode(status); err != nil {
			slog.Debug("write daemon control response", "error", err)
		}
		if header.Command == "stop" {
			d.stopOnce.Do(func() { close(d.stop) })
		}
	case "session":
		if err := conn.SetDeadline(time.Time{}); err != nil {
			slog.Warn("clear daemon session deadline", "error", err)
			return
		}
		d.mu.Lock()
		d.sessions++
		d.nextID++
		id := fmt.Sprintf("ipc-%d", d.nextID)
		d.mu.Unlock()
		if err := d.server.ServeSessionStream(ctx, id, reader, conn); err != nil {
			slog.Debug("serve daemon session", "session", id, "error", err)
		}
		d.mu.Lock()
		d.sessions--
		if d.sessions == 0 {
			d.idleSince = time.Now()
		}
		d.mu.Unlock()
	default:
		slog.Debug("refuse unknown daemon command", "command", header.Command)
	}
}

func mcpDaemonStatusFromInfo(info MCPDaemonInfo) MCPDaemonStatus {
	return MCPDaemonStatus{
		Running: true, PID: info.PID, StartedAt: info.StartedAt,
		WorkspaceRoot: info.WorkspaceRoot, StorageRoot: info.StorageRoot,
		Version: info.Version, LockPath: info.LockPath, IPCTransport: info.IPCTransport,
		Endpoint: info.Endpoint, ActiveSessions: info.ActiveSessions, ProtocolVersion: info.ProtocolVersion,
	}
}

func mcpDaemonControl(ctx context.Context, info MCPDaemonInfo, command string) (status MCPDaemonStatus, result error) {
	transport, endpoint, err := DeriveMCPDaemonEndpoint(info.WorkspaceRoot, info.StorageRoot, filepath.Dir(info.LockPath))
	if err != nil {
		return status, err
	}
	if transport != info.IPCTransport || endpoint != info.Endpoint || info.ProtocolVersion != mcpDaemonProtocolVersion {
		return status, mcpDaemonValidationError("daemon control endpoint or protocol does not match workspace")
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	conn, err := DialMCPDaemonIPC(ctx, transport, endpoint, 500*time.Millisecond)
	if err != nil {
		return status, fmt.Errorf("connect daemon control: %w", err)
	}
	defer func() { result = errors.Join(result, conn.Close()) }()
	deadline, _ := ctx.Deadline() // WithTimeout always supplies a deadline.
	if err := conn.SetDeadline(deadline); err != nil {
		return status, fmt.Errorf("set daemon control deadline: %w", err)
	}
	header := mcpDaemonHeader{info.ProtocolVersion, command, info.WorkspaceRoot, info.StorageRoot, info.Version}
	if err := json.NewEncoder(conn).Encode(header); err != nil {
		return status, fmt.Errorf("write daemon control: %w", err)
	}
	if err := json.NewDecoder(io.LimitReader(conn, maxMCPDaemonInfoBytes)).Decode(&status); err != nil {
		return status, fmt.Errorf("read daemon control: %w", err)
	}
	if status.PID != info.PID || status.Endpoint != info.Endpoint || status.ProtocolVersion != info.ProtocolVersion {
		return MCPDaemonStatus{}, mcpDaemonValidationError("daemon control response identity changed")
	}
	return status, nil
}

// PrewarmMCPDaemon starts or attaches to the workspace-pinned background daemon.
// Startup contenders share the daemon's OS lock and poll readiness without
// acquiring it. The daemon's lifetime is independent of the caller's context.
func PrewarmMCPDaemon(ctx context.Context, rootPath string, replace bool, idleTimeout time.Duration) (status MCPDaemonStatus, result error) {
	if ctx == nil || rootPath == "" || idleTimeout < 0 {
		return status, mcpDaemonValidationError("prewarm requires a context, workspace, and nonnegative idle timeout")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	workspace, storage, dir, handle, err := openMCPDaemonRuntimeRoot(rootPath)
	if err != nil {
		return status, err
	}
	defer func() { result = errors.Join(result, handle.Close()) }()
	binary, err := pinnedMCPDaemonBinary(workspace)
	if err != nil {
		return status, err
	}
	expectedVersion, err := pinnedMCPDaemonVersion(ctx, binary, workspace)
	if err != nil {
		return status, err
	}
	if replace {
		if _, err := StopMCPDaemon(ctx, workspace); err != nil {
			return status, fmt.Errorf("replace MCP daemon: %w", err)
		}
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	var childDone <-chan error
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return MCPDaemonStatus{}, fmt.Errorf("wait for MCP daemon readiness: %w", errors.Join(err, lastErr))
		}
		// Read without a lock probe: a stale record is replaced only by the
		// child after it wins singleton ownership. Partial publication retries.
		info, readErr := readMCPDaemonInfoAt(handle, dir)
		if readErr != nil && errors.Is(readErr, corerrors.ErrValidation) {
			if !strings.HasSuffix(readErr.Error(), "changed while opening") {
				return MCPDaemonStatus{}, readErr
			}
		} else if readErr == nil && isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath) {
			if !sameDaemonPath(info.ExecutablePath, binary) {
				return MCPDaemonStatus{}, fmt.Errorf("daemon binary %s differs from pinned %s: %w",
					info.ExecutablePath, binary, corerrors.ErrDaemonVersionMismatch)
			}
			// AcquireMCPDaemonLock initially publishes a lock-only record.
			// Do not mistake that startup interval for incompatible protocol.
			if info.Endpoint != "" || info.ProtocolVersion != "" {
				if err := VerifyMCPDaemonAttachment(info, workspace, storage, expectedVersion, mcpDaemonProtocolVersion); err != nil {
					return MCPDaemonStatus{}, err
				}
				status, err = mcpDaemonControl(ctx, info, "status")
				if err == nil {
					return status, nil
				}
				lastErr = err
			}
		} else if childDone == nil {
			command := exec.Command(binary, "--cwd", workspace, "--no-update-check",
				"mcp", "daemon", "--idle-timeout", idleTimeout.String())
			command.Dir = workspace
			// Nil streams use the null device, not the launching shim's pipes.
			// CommandContext would incorrectly kill the shared daemon on detach.
			if err := command.Start(); err != nil {
				return MCPDaemonStatus{}, fmt.Errorf("start pinned MCP daemon: %w", err)
			}
			done := make(chan error, 1)
			childDone = done
			// This reaper belongs to the independent child lifetime. It owns
			// no client streams and exits when the daemon stops (or loses admission).
			go func() { done <- command.Wait() }()
		}
		select {
		case <-ctx.Done():
		case err := <-childDone:
			lastErr = err
			childDone = nil // A losing contender may retry after stale publication.
		case <-tick.C:
		}
	}
}

func pinnedMCPDaemonVersion(ctx context.Context, binary, workspace string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve prewarm executable: %w", err)
	}
	if sameDaemonPath(binary, exe) {
		return version.Resolve(), nil
	}
	// Library hosts (including go test) do not necessarily share the pinned
	// image's module/build version. Ask that image, not the daemon's sidecar.
	command := exec.CommandContext(ctx, binary, "--cwd", workspace, "version", "--no-update-check", "--format", "json")
	command.Dir = workspace
	data, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read pinned daemon version: %w", err)
	}
	var info versionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("decode pinned daemon version: %w", err)
	}
	if info.Version == "" {
		return "", mcpDaemonValidationError("pinned daemon reported an empty version")
	}
	return info.Version, nil
}

// pinnedMCPDaemonBinary resolves the workspace-pinned backlogit binary under
// os.OpenRoot(absWorkspace) in strict priority order:
//  1. .mcp.json backlogit server command
//  2. <storage>/toolchain.yaml (backlogit executable_path / tools_dir)
//  3. <storage>/config.yaml tools_dir
//  4. .bin/<name> default, then bin/<name> fallback
func pinnedMCPDaemonBinary(workspace string) (binary string, result error) {
	if strings.TrimSpace(workspace) == "" {
		return "", mcpDaemonValidationError("empty pinned binary workspace")
	}
	absWorkspace, err := filepath.Abs(filepath.Clean(workspace))
	if err != nil {
		return "", fmt.Errorf("resolve pinned binary workspace %s: %w", workspace, err)
	}
	wsInfo, err := os.Lstat(absWorkspace)
	if err != nil {
		return "", fmt.Errorf("inspect pinned binary workspace %s: %w", absWorkspace, err)
	}
	if err := rejectLink(wsInfo, absWorkspace); err != nil {
		return "", err
	}
	if wsInfo.Mode()&os.ModeIrregular != 0 || !wsInfo.IsDir() {
		return "", mcpDaemonValidationError("workspace root %s is not a regular directory", absWorkspace)
	}
	root, err := os.OpenRoot(absWorkspace)
	if err != nil {
		return "", fmt.Errorf("open pinned binary workspace %s: %w", absWorkspace, err)
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	openedWS, err := root.Stat(".")
	if err != nil {
		return "", fmt.Errorf("stat pinned binary workspace %s: %w", absWorkspace, err)
	}
	if !openedWS.IsDir() || !os.SameFile(wsInfo, openedWS) {
		return "", mcpDaemonValidationError("workspace root %s changed while opening", absWorkspace)
	}

	rel, found, err := resolveConfiguredDaemonBinaryRel(root, absWorkspace)
	if err != nil {
		return "", err
	}
	if found {
		return verifyPinnedDaemonBinaryRel(root, absWorkspace, rel)
	}

	name := pinnedDaemonBinaryName()
	dotBinRel := filepath.Join(".bin", name)
	binPath, err := verifyPinnedDaemonBinaryRel(root, absWorkspace, dotBinRel)
	if err == nil {
		return binPath, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	return verifyPinnedDaemonBinaryRel(root, absWorkspace, filepath.Join("bin", name))
}

func pinnedDaemonBinaryName() string {
	if runtime.GOOS == "windows" {
		return "backlogit.exe"
	}
	return "backlogit"
}

type daemonMCPJSONServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	URL     string            `json:"url"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type daemonWorkspaceConfig struct {
	ToolsDir string `yaml:"tools_dir"`
}

var daemonRemoteToolIDPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

func inspectDaemonDirCandidate(root *os.Root, absWorkspace, rel string) error {
	info, err := root.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", filepath.Join(absWorkspace, rel), err)
	}
	path := filepath.Join(absWorkspace, rel)
	if err := rejectLink(info, path); err != nil {
		return err
	}
	if info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
		return mcpDaemonValidationError("%s is not a directory", path)
	}
	return nil
}

func validateDaemonStorageBootstrapEntries(root *os.Root, absWorkspace, storageDir string) error {
	storageInfo, err := root.Lstat(storageDir)
	if err != nil {
		return fmt.Errorf("inspect storage root %s: %w", filepath.Join(absWorkspace, storageDir), err)
	}
	storageAbs := filepath.Join(absWorkspace, storageDir)
	if err := rejectLink(storageInfo, storageAbs); err != nil {
		return err
	}
	if storageInfo.Mode()&os.ModeIrregular != 0 || !storageInfo.IsDir() {
		return mcpDaemonValidationError("%s is not a directory", storageAbs)
	}
	storageHandle, err := root.OpenRoot(storageDir)
	if err != nil {
		return fmt.Errorf("open storage root %s: %w", storageAbs, err)
	}
	defer func() { _ = storageHandle.Close() }()

	opened, err := storageHandle.Stat(".")
	if err != nil || !os.SameFile(storageInfo, opened) {
		return mcpDaemonValidationError("storage root %s identity changed while opening", storageAbs)
	}

	for _, sub := range []string{"templates", "queue", "logs"} {
		subInfo, subErr := storageHandle.Lstat(sub)
		if errors.Is(subErr, os.ErrNotExist) {
			continue
		}
		if subErr != nil {
			return fmt.Errorf("inspect %s: %w", filepath.Join(storageAbs, sub), subErr)
		}
		subAbs := filepath.Join(storageAbs, sub)
		if err := rejectLink(subInfo, subAbs); err != nil {
			return err
		}
		if subInfo.Mode()&os.ModeIrregular != 0 || !subInfo.IsDir() {
			return mcpDaemonValidationError("%s is not a directory", subAbs)
		}
		if sub == "templates" {
			tplHandle, openErr := storageHandle.OpenRoot("templates")
			if openErr != nil {
				return fmt.Errorf("open %s: %w", subAbs, openErr)
			}
			tplDir, openFileErr := tplHandle.Open(".")
			if openFileErr != nil {
				_ = tplHandle.Close()
				return fmt.Errorf("read %s: %w", subAbs, openFileErr)
			}
			entries, readDirErr := tplDir.ReadDir(-1)
			_ = tplDir.Close()
			if readDirErr != nil {
				_ = tplHandle.Close()
				return fmt.Errorf("list %s: %w", subAbs, readDirErr)
			}
			for _, entry := range entries {
				if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
					continue
				}
				fInfo, fErr := tplHandle.Lstat(entry.Name())
				if fErr != nil {
					_ = tplHandle.Close()
					return fmt.Errorf("inspect %s: %w", filepath.Join(subAbs, entry.Name()), fErr)
				}
				fAbs := filepath.Join(subAbs, entry.Name())
				if err := rejectLink(fInfo, fAbs); err != nil {
					_ = tplHandle.Close()
					return err
				}
				if fInfo.Mode()&os.ModeIrregular != 0 || !fInfo.Mode().IsRegular() {
					_ = tplHandle.Close()
					return mcpDaemonValidationError("%s is not a regular file", fAbs)
				}
			}
			_ = tplHandle.Close()
		}
	}

	for _, fileName := range []string{"config.yaml", "registry.yaml", "header-def.yaml", "stash.jsonl", "hooks.yaml", "migration.yaml"} {
		fInfo, fErr := storageHandle.Lstat(fileName)
		if errors.Is(fErr, os.ErrNotExist) {
			continue
		}
		if fErr != nil {
			return fmt.Errorf("inspect %s: %w", filepath.Join(storageAbs, fileName), fErr)
		}
		fAbs := filepath.Join(storageAbs, fileName)
		if err := rejectLink(fInfo, fAbs); err != nil {
			return err
		}
		if fInfo.Mode()&os.ModeIrregular != 0 || !fInfo.Mode().IsRegular() {
			return mcpDaemonValidationError("%s is not a regular file", fAbs)
		}
	}
	return nil
}

func resolveConfiguredDaemonBinaryRel(root *os.Root, absWorkspace string) (string, bool, error) {
	// Evaluate all 5 sources unconditionally (no short-circuiting).
	mcpRel, mcpFound, mcpErr := resolveMCPJSONDaemonBinaryRel(root, absWorkspace)

	storageDir, hasStorage, storageErr := resolveDaemonStorageDirName(root, absWorkspace)
	var bootstrapErr error
	var tcRel string
	var tcFound bool
	var tcErr error
	var cfgRel string
	var cfgFound bool
	var cfgErr error
	if storageErr == nil && hasStorage {
		bootstrapErr = validateDaemonStorageBootstrapEntries(root, absWorkspace, storageDir)
		tcRel, tcFound, tcErr = resolveToolchainDaemonBinaryRel(root, absWorkspace, storageDir)
		cfgRel, cfgFound, cfgErr = resolveConfigDaemonBinaryRel(root, absWorkspace, storageDir)
	}

	dotBinErr := inspectDaemonDirCandidate(root, absWorkspace, ".bin")
	binErr := inspectDaemonDirCandidate(root, absWorkspace, "bin")

	for _, err := range []error{mcpErr, storageErr, bootstrapErr, tcErr, cfgErr, dotBinErr, binErr} {
		if err != nil {
			return "", false, err
		}
	}

	if mcpFound {
		return mcpRel, true, nil
	}
	if tcFound {
		return tcRel, true, nil
	}
	if cfgFound {
		return cfgRel, true, nil
	}
	return "", false, nil
}

func defaultTargetToolsDir(cleanRoot string) (string, error) {
	absRoot, wsRoot, err := openOrCreateValidatedWorkspaceRoot(cleanRoot, false)
	if errors.Is(err, os.ErrNotExist) {
		return ".bin", nil
	}
	if err != nil {
		return "", err
	}
	defer func() { _ = wsRoot.Close() }()

	rel, found, err := resolveConfiguredDaemonBinaryRel(wsRoot, absRoot)
	if err != nil {
		return "", err
	}
	if found {
		return filepath.ToSlash(filepath.Dir(rel)), nil
	}
	if _, err := wsRoot.Lstat(".bin"); err == nil {
		return ".bin", nil
	}
	if _, err := wsRoot.Lstat("bin"); err == nil {
		return "bin", nil
	}
	return ".bin", nil
}

func decodeDaemonMCPServersMap(raw json.RawMessage, section string) (map[string]daemonMCPJSONServer, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] != '{' {
		return nil, mcpDaemonValidationError(".mcp.json %s must be a JSON object", section)
	}
	var rawEntries map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &rawEntries); err != nil {
		return nil, mcpDaemonValidationError("decode .mcp.json %s: %v", section, err)
	}
	out := make(map[string]daemonMCPJSONServer, len(rawEntries))
	for name, entryRaw := range rawEntries {
		entryTrimmed := bytes.TrimSpace(entryRaw)
		if len(entryTrimmed) == 0 || entryTrimmed[0] != '{' {
			return nil, mcpDaemonValidationError(".mcp.json %s.%s must be a JSON object", section, name)
		}
		var srv daemonMCPJSONServer
		if err := json.Unmarshal(entryTrimmed, &srv); err != nil {
			return nil, mcpDaemonValidationError("decode .mcp.json %s.%s: %v", section, name, err)
		}
		out[name] = srv
	}
	return out, nil
}

func resolveMCPJSONDaemonBinaryRel(root *os.Root, absWorkspace string) (string, bool, error) {
	data, err := readLocalFileInRoot(root, absWorkspace, ".mcp.json", maxMCPDaemonInfoBytes)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err := scanDaemonJSON(data); err != nil {
		return "", false, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "", false, mcpDaemonValidationError(".mcp.json top-level must be a JSON object")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return "", false, mcpDaemonValidationError("decode .mcp.json: %v", err)
	}

	var matched []daemonMCPJSONServer
	for _, section := range []string{"mcpServers", "servers"} {
		var foundRaw json.RawMessage
		var hasSection bool
		for k, v := range top {
			if foldDaemonKey(k) == foldDaemonKey(section) {
				foundRaw = v
				hasSection = true
				break
			}
		}
		if !hasSection {
			continue
		}
		servers, err := decodeDaemonMCPServersMap(foundRaw, section)
		if err != nil {
			return "", false, err
		}
		names := make([]string, 0, len(servers))
		for name := range servers {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			srv := servers[name]
			if foldDaemonKey(strings.TrimSpace(name)) == foldDaemonKey("backlogit") || isBacklogitDaemonCommand(srv.Command) {
				matched = append(matched, srv)
			}
		}
	}
	if len(matched) == 0 {
		return "", false, nil
	}
	if len(matched) > 1 {
		return "", false, mcpDaemonValidationError("ambiguous multiple backlogit entries in .mcp.json")
	}
	srvType := strings.TrimSpace(matched[0].Type)
	if strings.TrimSpace(matched[0].URL) != "" || (srvType != "" && !strings.EqualFold(srvType, "stdio")) {
		return "", false, mcpDaemonValidationError(".mcp.json backlogit entry must use stdio transport without url")
	}
	rel, err := validateBacklogitCommandRel(absWorkspace, matched[0].Command)
	if err != nil {
		return "", false, err
	}
	return rel, true, nil
}

func resolveDaemonStorageDirName(root *os.Root, absWorkspace string) (string, bool, error) {
	candidates := core.WorkspaceRootCandidates()
	if override, ok := os.LookupEnv("BACKLOGIT_WORKSPACE_DIR"); ok && override != "" {
		valid := false
		for _, c := range candidates {
			if override == c {
				valid = true
				break
			}
		}
		if !valid {
			return "", false, mcpDaemonValidationError("invalid BACKLOGIT_WORKSPACE_DIR %q", override)
		}
		info, err := root.Lstat(override)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("inspect storage root %s: %w", filepath.Join(absWorkspace, override), err)
		}
		path := filepath.Join(absWorkspace, override)
		if err := rejectLink(info, path); err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			return "", false, mcpDaemonValidationError("%s is not a directory", path)
		}
		return override, true, nil
	}

	var found string
	for _, candidate := range candidates {
		info, err := root.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("inspect storage root %s: %w", filepath.Join(absWorkspace, candidate), err)
		}
		path := filepath.Join(absWorkspace, candidate)
		if err := rejectLink(info, path); err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			return "", false, mcpDaemonValidationError("%s is not a directory", path)
		}
		if found != "" {
			return "", false, mcpDaemonValidationError("ambiguous workspace storage roots %s and %s", found, candidate)
		}
		found = candidate
	}
	if found == "" {
		return "", false, nil
	}
	return found, true, nil
}

func resolveDaemonManifestWorkspaceRoot(absWorkspace, rawRoot string) (string, error) {
	if rawRoot == "" || rawRoot != strings.TrimSpace(rawRoot) {
		return "", mcpDaemonValidationError("toolchain.yaml invalid workspace_root %q", rawRoot)
	}
	if filepath.IsAbs(rawRoot) || filepath.VolumeName(rawRoot) != "" || hasDriveOrUNCPrefix(rawRoot) {
		declared, err := filepath.Abs(rawRoot)
		if err != nil {
			return "", mcpDaemonValidationError("toolchain.yaml resolve workspace_root %q: %v", rawRoot, err)
		}
		return filepath.Clean(declared), nil
	}
	return filepath.Clean(filepath.Join(absWorkspace, filepath.FromSlash(strings.ReplaceAll(rawRoot, "\\", "/")))), nil
}

func validateDaemonRelExistingDirs(root *os.Root, absWorkspace, rel string) error {
	clean := filepath.Clean(rel)
	if clean == "" || clean == "." || !filepath.IsLocal(clean) {
		return mcpDaemonValidationError("invalid workspace-local path %q", rel)
	}
	parts := strings.Split(clean, string(filepath.Separator))
	current := root
	currentAbs := absWorkspace
	var owned []*os.Root
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	for _, part := range parts {
		currentAbs = filepath.Join(currentAbs, part)
		info, err := current.Lstat(part)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", currentAbs, err)
		}
		if err := rejectLink(info, currentAbs); err != nil {
			return err
		}
		if info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			return mcpDaemonValidationError("%s is not a directory", currentAbs)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return fmt.Errorf("open directory %s: %w", currentAbs, err)
		}
		owned = append(owned, next)
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			return mcpDaemonValidationError("directory %s changed while opening", currentAbs)
		}
		current = next
	}
	return nil
}

func validateDaemonBacklogitBindings(root *os.Root, absWorkspace string, args []string) error {
	bindings := make(map[string]string)
	for i := 0; i < len(args); i++ {
		key, value, hasValue := strings.Cut(args[i], "=")
		key = strings.TrimSpace(key)
		if key != "--cwd" {
			key = strings.ToUpper(key)
		}
		if key != "--cwd" && key != "COPILOT_HOME" && key != "ENGRAM_DATA_DIR" {
			return mcpDaemonValidationError("toolchain.yaml unknown workspace binding %q", key)
		}
		if !hasValue {
			i++
			if i >= len(args) {
				return mcpDaemonValidationError("toolchain.yaml missing workspace binding value for %s", key)
			}
			value = args[i]
		}
		trimmed := strings.TrimSpace(value)
		if _, exists := bindings[key]; exists || (key == "--cwd" && (trimmed == "" || strings.HasPrefix(trimmed, "-"))) {
			return mcpDaemonValidationError("toolchain.yaml invalid workspace binding %q=%q", key, value)
		}
		if filepath.IsAbs(value) {
			cleaned := filepath.Clean(value)
			if key == "--cwd" && !sameDaemonPath(cleaned, absWorkspace) {
				return mcpDaemonValidationError("toolchain.yaml --cwd binding %q mismatches workspace %q", value, absWorkspace)
			}
			if key != "--cwd" && !sameDaemonPath(cleaned, absWorkspace) {
				rel, err := filepath.Rel(absWorkspace, cleaned)
				if err != nil || !filepath.IsLocal(rel) || rel == "." {
					return mcpDaemonValidationError("toolchain.yaml binding %q outside workspace", value)
				}
			}
			bindings[key] = cleaned
			continue
		}
		if filepath.VolumeName(value) != "" || hasDriveOrUNCPrefix(value) {
			return mcpDaemonValidationError("toolchain.yaml binding %q must not carry volume prefix", value)
		}
		joined := filepath.Clean(filepath.Join(absWorkspace, filepath.FromSlash(strings.ReplaceAll(value, "\\", "/"))))
		if key == "--cwd" {
			if !sameDaemonPath(joined, absWorkspace) {
				return mcpDaemonValidationError("toolchain.yaml --cwd binding %q mismatches workspace %q", value, absWorkspace)
			}
			bindings[key] = joined
			continue
		}
		if sameDaemonPath(joined, absWorkspace) {
			return mcpDaemonValidationError("toolchain.yaml %s binding must not equal workspace root", key)
		}
		rel, err := filepath.Rel(absWorkspace, joined)
		if err != nil || !filepath.IsLocal(rel) || rel == "." {
			return mcpDaemonValidationError("toolchain.yaml binding %q escapes workspace", value)
		}
		if err := validateDaemonRelExistingDirs(root, absWorkspace, rel); err != nil {
			return err
		}
		bindings[key] = joined
	}
	if bindings["--cwd"] == "" {
		return mcpDaemonValidationError("toolchain.yaml backlogit entry requires --cwd workspace binding")
	}
	return nil
}

func resolveToolchainDaemonBinaryRel(root *os.Root, absWorkspace, storageDir string) (string, bool, error) {
	data, err := readLocalFileInRoot(root, absWorkspace, filepath.Join(storageDir, "toolchain.yaml"), maxMCPDaemonInfoBytes)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", false, mcpDaemonValidationError("parse toolchain.yaml: %v", err)
	}
	if err := validateDaemonYAMLKeys(&doc); err != nil {
		return "", false, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var manifest toolchain.ToolchainManifest
	if err := decoder.Decode(&manifest); err != nil {
		return "", false, mcpDaemonValidationError("decode toolchain.yaml: %v", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", false, mcpDaemonValidationError("additional YAML document in toolchain.yaml")
	}
	declaredRoot, err := resolveDaemonManifestWorkspaceRoot(absWorkspace, manifest.WorkspaceRoot)
	if err != nil {
		return "", false, err
	}
	if manifest.SchemaVersion != "1.0" ||
		manifest.ContainmentLevel != toolchain.Level1WorkspaceLocalPinned ||
		!manifest.OSContainerSandboxDeferred ||
		!sameDaemonPath(absWorkspace, declaredRoot) ||
		manifest.Tools == nil ||
		manifest.HostPrerequisites == nil {
		return "", false, mcpDaemonValidationError("invalid required fields in toolchain.yaml")
	}
	manifest.WorkspaceRoot = absWorkspace

	var toolsDirRel string
	hasToolsDir := hasYAMLMappingKey(&doc, "tools_dir") || manifest.ToolsDir != ""
	if hasToolsDir {
		toolsDirRel, err = validateDaemonToolsDirRel(manifest.ToolsDir)
		if err != nil {
			return "", false, err
		}
		if err := validateDaemonRelExistingDirs(root, absWorkspace, toolsDirRel); err != nil {
			return "", false, err
		}
	}

	seen := make(map[string]bool, len(manifest.Tools)+len(manifest.HostPrerequisites))
	var backlogitTools []toolchain.ToolEntry
	unchangedTools := make([]toolchain.ToolEntry, 0, len(manifest.Tools))
	for _, tool := range manifest.Tools {
		trimmedID := strings.TrimSpace(tool.ID)
		id := strings.ToLower(trimmedID)
		if id == "" || tool.ID != trimmedID || strings.TrimSpace(tool.Kind) == "" || seen[id] {
			return "", false, mcpDaemonValidationError("invalid or duplicate tool identity %q in toolchain.yaml", tool.ID)
		}
		seen[id] = true

		isBacklogit := strings.EqualFold(trimmedID, "backlogit") || isBacklogitDaemonCommand(tool.ExecutablePath)
		if isBacklogit {
			backlogitTools = append(backlogitTools, tool)
		} else {
			switch tool.ContainmentClass {
			case toolchain.RemoteDeclared:
				if !daemonRemoteToolIDPattern.MatchString(tool.ID) || strings.TrimSpace(tool.RemotePolicy) == "" {
					return "", false, mcpDaemonValidationError("invalid remote tool %q in toolchain.yaml", tool.ID)
				}
			case toolchain.Unsupported, toolchain.NonConformant:
				return "", false, mcpDaemonValidationError("disallowed containment_class %q for tool %q in toolchain.yaml", tool.ContainmentClass, tool.ID)
			}
			unchangedTools = append(unchangedTools, tool)
		}
	}
	for _, prereq := range manifest.HostPrerequisites {
		trimmedID := strings.TrimSpace(prereq.ID)
		id := strings.ToLower(trimmedID)
		if id == "" || prereq.ID != trimmedID ||
			strings.TrimSpace(prereq.Command) == "" ||
			strings.TrimSpace(prereq.Purpose) == "" ||
			strings.TrimSpace(prereq.Reason) == "" ||
			strings.EqualFold(id, "backlogit") ||
			strings.HasPrefix(strings.ToLower(filepath.Base(strings.TrimSpace(prereq.Command))), "backlogit") ||
			seen[id] {
			return "", false, mcpDaemonValidationError("invalid host prerequisite %q in toolchain.yaml", prereq.ID)
		}
		seen[id] = true
	}

	if len(backlogitTools) > 1 {
		return "", false, mcpDaemonValidationError("ambiguous multiple backlogit entries in toolchain.yaml")
	}

	var execRel string
	hasExec := len(backlogitTools) == 1
	if hasExec {
		bt := backlogitTools[0]
		if bt.ContainmentClass != toolchain.WorkspaceLocalPinned {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit entry must have containment_class workspace-local-pinned, got %q", bt.ContainmentClass)
		}
		if !strings.EqualFold(strings.TrimSpace(bt.Kind), "mcp") {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit entry must have kind mcp, got %q", bt.Kind)
		}
		if strings.TrimSpace(bt.EntrypointPath) != "" {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit entry must not specify entrypoint_path")
		}
		if strings.TrimSpace(bt.PinnedVersion) == "" && strings.TrimSpace(bt.LockfilePath) == "" {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit entry missing pinned_version or lockfile_path")
		}
		if strings.TrimSpace(bt.SHA256) == "" && strings.TrimSpace(bt.LockfilePath) == "" {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit entry missing sha256 or lockfile_path")
		}
		execRel, err = validateBacklogitCommandRel(absWorkspace, bt.ExecutablePath)
		if err != nil {
			return "", false, err
		}
		if err := validateDaemonBacklogitBindings(root, absWorkspace, bt.WorkspaceBindingArgs); err != nil {
			return "", false, err
		}
		if strings.TrimSpace(bt.LockfilePath) != "" {
			lockData, lockErr := readLocalFileInRoot(root, absWorkspace, bt.LockfilePath, maxMCPDaemonInfoBytes)
			if lockErr != nil {
				return "", false, mcpDaemonValidationError("read backlogit lockfile_path %q: %v", bt.LockfilePath, lockErr)
			}
			versionPin := strings.TrimSpace(bt.PinnedVersion)
			if len(bytes.TrimSpace(lockData)) == 0 ||
				!strings.Contains(string(lockData), "backlogit") ||
				(versionPin != "" && !strings.Contains(string(lockData), versionPin)) {
				return "", false, mcpDaemonValidationError("toolchain.yaml backlogit lockfile_path %q lacks canonical backlogit provenance", bt.LockfilePath)
			}
		}
	}

	// Verify all unchanged non-backlogit inventory entries read-only without
	// inserting an external installer path into the workspace-local manifest.
	unchangedManifest := manifest
	unchangedManifest.Tools = unchangedTools
	report, verifyErr := toolchain.VerifyWorkspaceToolchain(context.Background(), absWorkspace, &unchangedManifest)
	if verifyErr != nil {
		return "", false, mcpDaemonValidationError("verify unchanged toolchain inventory: %v", verifyErr)
	}
	if report == nil || !report.Valid || report.Violations > 0 {
		return "", false, mcpDaemonValidationError("toolchain.yaml contains %d inventory violation(s)", report.Violations)
	}

	if hasToolsDir && hasExec {
		if !sameDaemonRelPath(filepath.Dir(execRel), toolsDirRel) {
			return "", false, mcpDaemonValidationError("toolchain.yaml backlogit executable_path %q disagrees with tools_dir %q", execRel, toolsDirRel)
		}
		return execRel, true, nil
	}
	if hasExec {
		return execRel, true, nil
	}
	if hasToolsDir {
		return filepath.Join(toolsDirRel, pinnedDaemonBinaryName()), true, nil
	}
	return "", false, nil
}

func findYAMLMappingValueNode(doc *yaml.Node, key string) (*yaml.Node, bool) {
	if doc == nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false
	}
	mapping := doc.Content[0]
	foldedTarget := foldDaemonKey(key)
	for i := 0; i < len(mapping.Content); i += 2 {
		if foldDaemonKey(mapping.Content[i].Value) == foldedTarget {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

func resolveConfigDaemonBinaryRel(root *os.Root, absWorkspace, storageDir string) (string, bool, error) {
	data, err := readLocalFileInRoot(root, absWorkspace, filepath.Join(storageDir, "config.yaml"), maxMCPDaemonInfoBytes)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", false, mcpDaemonValidationError("parse config.yaml: %v", err)
	}
	if err := validateDaemonYAMLKeys(&doc); err != nil {
		msg := strings.ReplaceAll(err.Error(), "in toolchain.yaml", "in config.yaml")
		return "", false, fmt.Errorf("%s: %w", msg, corerrors.ErrValidation)
	}
	if valNode, ok := findYAMLMappingValueNode(&doc, "tools_dir"); ok {
		if valNode.Kind != yaml.ScalarNode || valNode.Tag != "!!str" {
			return "", false, mcpDaemonValidationError("config.yaml tools_dir must be a string scalar")
		}
	}
	var cfg daemonWorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", false, mcpDaemonValidationError("decode config.yaml: %v", err)
	}
	if !hasYAMLMappingKey(&doc, "tools_dir") && cfg.ToolsDir == "" {
		return "", false, nil
	}
	toolsDirRel, err := validateDaemonToolsDirRel(cfg.ToolsDir)
	if err != nil {
		return "", false, err
	}
	return filepath.Join(toolsDirRel, pinnedDaemonBinaryName()), true, nil
}

func hasYAMLMappingKey(doc *yaml.Node, key string) bool {
	if doc == nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return false
	}
	mapping := doc.Content[0]
	foldedTarget := foldDaemonKey(key)
	for i := 0; i < len(mapping.Content); i += 2 {
		if foldDaemonKey(mapping.Content[i].Value) == foldedTarget {
			return true
		}
	}
	return false
}

func foldDaemonKey(s string) string {
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

func validateDaemonYAMLKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			rawKey := node.Content[i].Value
			key := foldDaemonKey(rawKey)
			if seen[key] {
				return mcpDaemonValidationError("duplicate YAML key %q in toolchain.yaml", rawKey)
			}
			seen[key] = true
		}
	}
	for _, child := range node.Content {
		if err := validateDaemonYAMLKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func scanDaemonJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var value func(int) error
	value = func(depth int) error {
		if depth > 10000 {
			return mcpDaemonValidationError("JSON nesting too deep in .mcp.json")
		}
		token, err := d.Token()
		if err != nil {
			return mcpDaemonValidationError("scan .mcp.json: %v", err)
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delim != '{' && delim != '[' {
			return mcpDaemonValidationError("unexpected JSON delimiter in .mcp.json")
		}
		seen := make(map[string]bool)
		for d.More() {
			if delim == '{' {
				key, err := d.Token()
				if err != nil {
					return mcpDaemonValidationError("scan .mcp.json key: %v", err)
				}
				name, ok := key.(string)
				if !ok {
					return mcpDaemonValidationError("non-string JSON key in .mcp.json")
				}
				folded := foldDaemonKey(name)
				if seen[folded] {
					return mcpDaemonValidationError("duplicate JSON key %q in .mcp.json", name)
				}
				seen[folded] = true
			}
			if err := value(depth + 1); err != nil {
				return err
			}
		}
		if _, err := d.Token(); err != nil {
			return mcpDaemonValidationError("scan .mcp.json end delimiter: %v", err)
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return mcpDaemonValidationError("trailing JSON data in .mcp.json")
	}
	return nil
}

func isBacklogitDaemonCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	norm := filepath.FromSlash(strings.ReplaceAll(trimmed, "\\", "/"))
	base := strings.ToLower(filepath.Base(norm))
	return base == "backlogit" || base == "backlogit.exe" ||
		strings.HasPrefix(base, "backlogit.") ||
		strings.HasPrefix(base, "backlogit-") ||
		strings.HasPrefix(base, "backlogit_")
}

func hasDriveOrUNCPrefix(s string) bool {
	if strings.HasPrefix(s, `\\`) || strings.HasPrefix(s, "//") {
		return true
	}
	return len(s) >= 2 && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) && s[1] == ':'
}

func validateDaemonToolsDirRel(toolsDir string) (string, error) {
	if toolsDir != strings.TrimSpace(toolsDir) || strings.TrimSpace(toolsDir) == "" {
		return "", mcpDaemonValidationError("tools_dir %q must not be blank or whitespace-padded", toolsDir)
	}
	for _, r := range toolsDir {
		if r < 0x20 || r == 0x7f {
			return "", mcpDaemonValidationError("tools_dir %q must not contain control characters or line separators", toolsDir)
		}
	}
	if filepath.IsAbs(toolsDir) || filepath.VolumeName(toolsDir) != "" || hasDriveOrUNCPrefix(toolsDir) {
		return "", mcpDaemonValidationError("tools_dir %q must be a workspace-relative subdirectory path", toolsDir)
	}
	for _, part := range strings.FieldsFunc(toolsDir, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", mcpDaemonValidationError("tools_dir %q must not contain '..' traversal", toolsDir)
		}
	}
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(toolsDir, "\\", "/")))
	if !filepath.IsLocal(clean) || clean == "." {
		return "", mcpDaemonValidationError("tools_dir %q must resolve to a local subdirectory within the workspace", toolsDir)
	}
	firstSeg, _, _ := strings.Cut(filepath.ToSlash(clean), "/")
	if isReservedDaemonDirSegment(firstSeg) {
		return "", mcpDaemonValidationError("tools_dir %q cannot target reserved directory %q", toolsDir, firstSeg)
	}
	return clean, nil
}

func validateBacklogitCommandRel(absWorkspace, command string) (string, error) {
	if command != strings.TrimSpace(command) || strings.TrimSpace(command) == "" {
		return "", mcpDaemonValidationError("backlogit command %q must not be empty or whitespace-padded", command)
	}
	if strings.ContainsRune(command, '\x00') {
		return "", mcpDaemonValidationError("backlogit command %q must not contain NUL bytes", command)
	}
	for _, part := range strings.FieldsFunc(command, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", mcpDaemonValidationError("backlogit command %q must not contain '..' traversal", command)
		}
	}

	if strings.HasPrefix(command, `\\`) || strings.HasPrefix(command, "//") {
		return "", mcpDaemonValidationError("backlogit command %q must not be a UNC path", command)
	}

	var rel string
	if filepath.IsAbs(command) {
		relPath, err := filepath.Rel(absWorkspace, filepath.Clean(command))
		if err != nil {
			return "", mcpDaemonValidationError("backlogit command %q cannot be resolved within workspace: %v", command, err)
		}
		rel = relPath
	} else {
		if filepath.VolumeName(command) != "" || hasDriveOrUNCPrefix(command) {
			return "", mcpDaemonValidationError("backlogit command %q must not carry a volume prefix", command)
		}
		if !strings.ContainsAny(command, "/\\") {
			return "", mcpDaemonValidationError("backlogit command %q must be a workspace-relative subdirectory path, not a bare command", command)
		}
		rel = filepath.Clean(filepath.FromSlash(strings.ReplaceAll(command, "\\", "/")))
	}

	if !filepath.IsLocal(rel) || rel == "." {
		return "", mcpDaemonValidationError("backlogit command %q resolves outside workspace", command)
	}
	if dir := filepath.Dir(rel); dir == "." || dir == ".." {
		return "", mcpDaemonValidationError("backlogit command %q must reside in a workspace subdirectory", command)
	}
	firstSeg, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	if isReservedDaemonDirSegment(firstSeg) {
		return "", mcpDaemonValidationError("backlogit command %q cannot reside under reserved directory %q", command, firstSeg)
	}
	base := filepath.Base(rel)
	if !sameDaemonRelPath(base, "backlogit") && !sameDaemonRelPath(base, "backlogit.exe") {
		return "", mcpDaemonValidationError("backlogit command %q must target backlogit or backlogit.exe", command)
	}
	return rel, nil
}

func isReservedDaemonDirSegment(seg string) bool {
	if strings.EqualFold(seg, ".git") {
		return true
	}
	for _, candidate := range core.WorkspaceRootCandidates() {
		if strings.EqualFold(seg, candidate) {
			return true
		}
	}
	return false
}

func sameDaemonRelPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func readLocalFileInRoot(root *os.Root, absWorkspace, rel string, maxBytes int64) (data []byte, result error) {
	file, path, err := openLocalFileInRoot(root, absWorkspace, rel)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	limited, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(limited)) > maxBytes {
		return nil, mcpDaemonValidationError("%s exceeds maximum allowed size of %d bytes", path, maxBytes)
	}
	return limited, nil
}

func verifyPinnedDaemonBinaryRel(root *os.Root, absWorkspace, rel string) (string, error) {
	file, path, err := openLocalFileInRoot(root, absWorkspace, rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("required pinned daemon binary %s: %w", filepath.Join(absWorkspace, rel), errors.Join(corerrors.ErrDaemonBinaryMissing, err))
		}
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close pinned daemon binary %s: %w", path, err)
	}
	return path, nil
}

func openLocalFileInRoot(root *os.Root, absWorkspace, rel string) (*os.File, string, error) {
	clean := filepath.Clean(rel)
	if clean == "" || clean == "." || !filepath.IsLocal(clean) {
		return nil, "", mcpDaemonValidationError("invalid workspace-local path %q", rel)
	}
	parts := strings.Split(clean, string(filepath.Separator))
	current := root
	path := absWorkspace
	owned := make([]*os.Root, 0, len(parts))
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := current.Lstat(part)
		if err != nil {
			return nil, "", fmt.Errorf("inspect %s: %w", path, err)
		}
		if err := rejectLink(info, path); err != nil {
			return nil, "", err
		}
		if info.Mode()&os.ModeIrregular != 0 {
			return nil, "", mcpDaemonValidationError("%s has irregular file mode", path)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return nil, "", mcpDaemonValidationError("%s is not a regular file", path)
			}
			file, err := current.Open(part)
			if err != nil {
				return nil, "", fmt.Errorf("open %s: %w", path, err)
			}
			opened, err := file.Stat()
			if err != nil {
				_ = file.Close()
				return nil, "", fmt.Errorf("stat %s: %w", path, err)
			}
			if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
				_ = file.Close()
				return nil, "", mcpDaemonValidationError("%s changed while opening or is not regular", path)
			}
			return file, path, nil
		}
		if !info.IsDir() {
			return nil, "", mcpDaemonValidationError("%s is not a directory", path)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, "", fmt.Errorf("open directory %s: %w", path, err)
		}
		owned = append(owned, next)
		opened, err := next.Stat(".")
		if err != nil {
			return nil, "", fmt.Errorf("stat directory %s: %w", path, err)
		}
		if !opened.IsDir() || !os.SameFile(info, opened) {
			return nil, "", mcpDaemonValidationError("directory %s changed while opening", path)
		}
		current = next
	}
	return nil, "", mcpDaemonValidationError("empty workspace-local path")
}

// RunMCPSessionShim proxies one stdio session to the verified workspace daemon.
// Stream ownership transfers for the session. Unix pipe inputs are polled for
// cancellation; other blocking streams must support an interruptible Close so
// cancellation or peer EOF can join both copy workers.
func RunMCPSessionShim(ctx context.Context, rootPath string, replace bool, input io.Reader, output io.Writer) (result error) {
	if ctx == nil || input == nil || output == nil {
		return mcpDaemonValidationError("session shim requires a context and streams")
	}
	status, err := PrewarmMCPDaemon(ctx, rootPath, replace, 0)
	if err != nil {
		return err
	}
	conn, err := DialMCPDaemonIPC(ctx, status.IPCTransport, status.Endpoint, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect MCP session: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			result = errors.Join(result, err)
		}
	}()
	header := mcpDaemonHeader{status.ProtocolVersion, "session", status.WorkspaceRoot, status.StorageRoot, status.Version}
	if err := conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return fmt.Errorf("set MCP session handshake deadline: %w", err)
	}
	if err := json.NewEncoder(conn).Encode(header); err != nil {
		return fmt.Errorf("write MCP session handshake: %w", err)
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear MCP session handshake deadline: %w", err)
	}
	return proxyMCPSession(ctx, conn, input, output)
}

func proxyMCPSession(ctx context.Context, conn net.Conn, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	input, err := prepareMCPSessionInput(ctx, input)
	if err != nil {
		return fmt.Errorf("prepare MCP session input: %w", err)
	}
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, input)
		done <- err
	}()
	go func() {
		_, err := io.Copy(output, conn)
		done <- err
	}()
	var result error
	remaining := 2
	select {
	case <-ctx.Done():
		result = ctx.Err()
	case result = <-done:
		remaining--
	}
	// EOF is a session detach, not an instruction to stop the daemon. Close
	// the entire IPC stream (named pipes do not provide a write half-close).
	cancel()
	result = errors.Join(result, conn.Close())
	if closer, ok := input.(io.Closer); ok {
		if err := closer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			result = errors.Join(result, err)
		}
	}
	if closer, ok := output.(io.Closer); ok {
		if err := closer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			result = errors.Join(result, err)
		}
	}
	for range remaining {
		<-done // Errors here are consequences of closing the owned streams.
	}
	if result != nil {
		return fmt.Errorf("proxy MCP session: %w", result)
	}
	return nil
}

// MCPDaemonStopResult reports the outcome of a request to stop a workspace MCP daemon.
type MCPDaemonStopResult struct {
	Stopped       bool   `json:"stopped"`
	Running       bool   `json:"running"`
	PID           int    `json:"pid"`
	WorkspaceRoot string `json:"workspace_root"`
	StorageRoot   string `json:"storage_root"`
	LockPath      string `json:"lock_path"`
}

// mcpDaemonOptions holds test seams for the daemon lock implementation.
type mcpDaemonOptions struct {
	// PID returns the current process ID.
	PID func() int
	// StopTimeout bounds how long a stop waits for the lock to be released.
	StopTimeout time.Duration
	// OpenLock opens the lock relative to runtimeRoot, using runtimeDir for validation.
	OpenLock func(runtimeRoot *os.Root, runtimeDir string) (*os.File, bool, error)
	// TerminateVerifiedProcess verifies pid against the recorded identity and
	// terminates it through one retained OS process handle. reverify runs while
	// that handle is held.
	TerminateVerifiedProcess func(pid int, startTime int64, exePath string, reverify func() error) error
}

func defaultMCPDaemonOptions() mcpDaemonOptions {
	return mcpDaemonOptions{
		PID:                      os.Getpid,
		StopTimeout:              5 * time.Second,
		OpenLock:                 openMCPDaemonLockFile,
		TerminateVerifiedProcess: terminateVerifiedProcess,
	}
}

const (
	mcpDaemonRuntimeDirName = "runtime"
	mcpDaemonLockName       = "mcp-daemon.lock"
	mcpDaemonSidecarName    = "mcp-daemon.json"
	maxMCPDaemonInfoBytes   = 64 << 10
)

func mcpDaemonValidationError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", corerrors.ErrValidation, fmt.Sprintf(format, args...))
}

// rejectLink fails closed when info describes a symlink or reparse point.
func rejectLink(info os.FileInfo, path string) error {
	isLink, err := core.IsSymlinkOrReparsePoint(info, path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if isLink {
		return mcpDaemonValidationError("%s is a symlink or reparse point", path)
	}
	return nil
}

// hasStorageOverride reports whether an explicit storage-root override is set.
// An absent override target must surface as an error rather than fall back to
// the default directory, which could address a different daemon.
func hasStorageOverride() bool {
	_, ok := os.LookupEnv("BACKLOGIT_WORKSPACE_DIR")
	return ok
}

// openMCPDaemonRuntimeRoot resolves the workspace storage root and returns a
// pinned handle to its runtime directory. Every later lock and sidecar
// operation is directory-relative to that handle.
func openMCPDaemonRuntimeRoot(rootPath string) (workspaceRoot, storageRoot, runtimeDir string, runtimeRoot *os.Root, err error) {
	absRoot, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", "", "", nil, fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	for _, candidate := range core.WorkspaceRootCandidates() {
		candidatePath := filepath.Join(absRoot, candidate)
		if info, lerr := os.Lstat(candidatePath); lerr == nil {
			if err := rejectLink(info, candidatePath); err != nil {
				return "", "", "", nil, err
			}
		}
	}
	storageRoot, err = core.ResolveStorageRoot(absRoot)
	switch {
	case err == nil:
		workspaceRoot = filepath.Dir(storageRoot)
	case errors.Is(err, os.ErrNotExist) && !hasStorageOverride():
		workspaceRoot = absRoot
		storageRoot = filepath.Join(absRoot, core.WorkspaceRootCandidates()[0])
	default:
		return "", "", "", nil, fmt.Errorf("resolve storage root: %w", err)
	}
	runtimeDir = filepath.Join(storageRoot, mcpDaemonRuntimeDirName)

	info, err := os.Lstat(storageRoot)
	switch {
	case err == nil:
		if err := rejectLink(info, storageRoot); err != nil {
			return "", "", "", nil, err
		}
		if !info.IsDir() {
			return "", "", "", nil, mcpDaemonValidationError("%s is not a directory", storageRoot)
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(storageRoot, 0o755); err != nil {
			return "", "", "", nil, fmt.Errorf("create storage root %s: %w", storageRoot, err)
		}
	default:
		return "", "", "", nil, fmt.Errorf("lstat storage root %s: %w", storageRoot, err)
	}

	storageHandle, err := os.OpenRoot(storageRoot)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("open storage root %s: %w", storageRoot, err)
	}
	defer func() { _ = storageHandle.Close() }()
	if err := verifyPinnedDir(storageRoot, storageHandle, "."); err != nil {
		return "", "", "", nil, err
	}

	if info, err := storageHandle.Lstat(mcpDaemonRuntimeDirName); err == nil {
		if err := rejectLink(info, runtimeDir); err != nil {
			return "", "", "", nil, err
		}
		if !info.IsDir() {
			return "", "", "", nil, mcpDaemonValidationError("%s is not a directory", runtimeDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", "", nil, fmt.Errorf("lstat %s: %w", runtimeDir, err)
	} else if err := storageHandle.Mkdir(mcpDaemonRuntimeDirName, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return "", "", "", nil, fmt.Errorf("create %s: %w", runtimeDir, err)
	}

	runtimeRoot, err = storageHandle.OpenRoot(mcpDaemonRuntimeDirName)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("open %s: %w", runtimeDir, err)
	}
	if err := verifyPinnedDir(runtimeDir, storageHandle, mcpDaemonRuntimeDirName); err != nil {
		_ = runtimeRoot.Close()
		return "", "", "", nil, err
	}
	if opened, err := runtimeRoot.Stat("."); err != nil {
		_ = runtimeRoot.Close()
		return "", "", "", nil, fmt.Errorf("stat %s: %w", runtimeDir, err)
	} else if linked, err := storageHandle.Lstat(mcpDaemonRuntimeDirName); err != nil || !os.SameFile(opened, linked) {
		_ = runtimeRoot.Close()
		return "", "", "", nil, mcpDaemonValidationError("%s changed while opening", runtimeDir)
	}
	return workspaceRoot, storageRoot, runtimeDir, runtimeRoot, nil
}

// verifyPinnedDir checks that name under parent is the same real directory the
// pathname currently resolves to and is not a link.
func verifyPinnedDir(path string, parent *os.Root, name string) error {
	handleInfo, err := parent.Stat(name)
	if err != nil {
		return fmt.Errorf("stat pinned %s: %w", path, err)
	}
	if !handleInfo.IsDir() {
		return mcpDaemonValidationError("%s is not a directory", path)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("lstat %s: %w", path, err)
	}
	if err := rejectLink(pathInfo, path); err != nil {
		return err
	}
	if !os.SameFile(handleInfo, pathInfo) {
		return mcpDaemonValidationError("%s changed while opening", path)
	}
	return nil
}

// resolveMCPDaemonLeafPath uses the absolute directory verified when the root
// was pinned, not the potentially relative name of a nested os.Root.
func resolveMCPDaemonLeafPath(runtimeRoot *os.Root, runtimeDir, leaf string) (string, error) {
	if runtimeRoot == nil || runtimeDir == "" || !filepath.IsAbs(runtimeDir) {
		return "", mcpDaemonValidationError("runtime root requires an absolute runtime directory")
	}
	if leaf == "" || leaf == "." || leaf == ".." || filepath.Base(leaf) != leaf {
		return "", mcpDaemonValidationError("invalid runtime leaf %q", leaf)
	}
	info, err := runtimeRoot.Stat(".")
	if err != nil {
		return "", fmt.Errorf("stat pinned runtime directory: %w", err)
	}
	if !info.IsDir() {
		return "", mcpDaemonValidationError("pinned runtime root is not a directory")
	}
	return filepath.Join(runtimeDir, leaf), nil
}

// openMCPDaemonLockFile opens the lock leaf relative to runtimeRoot without
// following links and attempts the exclusive non-blocking lock. On a failed
// acquisition the file is closed and acquired is false.
func openMCPDaemonLockFile(runtimeRoot *os.Root, runtimeDir string) (file *os.File, acquired bool, err error) {
	lockPath, err := resolveMCPDaemonLeafPath(runtimeRoot, runtimeDir, mcpDaemonLockName)
	if err != nil {
		return nil, false, err
	}
	if info, err := runtimeRoot.Lstat(mcpDaemonLockName); err == nil {
		if err := rejectLink(info, lockPath); err != nil {
			return nil, false, err
		}
		if !info.Mode().IsRegular() {
			return nil, false, mcpDaemonValidationError("%s is not a regular file", lockPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("lstat %s: %w", lockPath, err)
	}

	f, err := runtimeRoot.OpenFile(mcpDaemonLockName, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open %s: %w", lockPath, err)
	}
	opened, err := f.Stat()
	if err == nil {
		var linked os.FileInfo
		linked, err = runtimeRoot.Lstat(mcpDaemonLockName)
		if err == nil {
			if rerr := rejectLink(linked, lockPath); rerr != nil {
				_ = f.Close()
				return nil, false, rerr
			}
			if !os.SameFile(opened, linked) {
				_ = f.Close()
				return nil, false, mcpDaemonValidationError("%s changed while opening", lockPath)
			}
		}
	}
	if err != nil {
		_ = f.Close()
		return nil, false, fmt.Errorf("verify %s: %w", lockPath, err)
	}

	locked, err := tryLockFile(f)
	if err != nil || !locked {
		_ = f.Close()
		return nil, false, err
	}
	return f, true, nil
}

// readMCPDaemonInfo supports callers that opened a top-level absolute root.
//
//nolint:unused // Compatibility entrypoint for the existing !windows FIFO regression test.
func readMCPDaemonInfo(runtimeRoot *os.Root) (MCPDaemonInfo, error) {
	if runtimeRoot == nil || !filepath.IsAbs(runtimeRoot.Name()) {
		return MCPDaemonInfo{}, mcpDaemonValidationError("runtime root requires an absolute name")
	}
	return readMCPDaemonInfoAt(runtimeRoot, runtimeRoot.Name())
}

// readMCPDaemonInfoAt decodes the sidecar directly from a validated no-follow
// handle relative to runtimeRoot. A missing sidecar yields os.ErrNotExist.
func readMCPDaemonInfoAt(runtimeRoot *os.Root, runtimeDir string) (MCPDaemonInfo, error) {
	var info MCPDaemonInfo
	path, err := resolveMCPDaemonLeafPath(runtimeRoot, runtimeDir, mcpDaemonSidecarName)
	if err != nil {
		return info, err
	}
	var f *os.File
	for attempt := 0; attempt < 4; attempt++ {
		linked, err := runtimeRoot.Lstat(mcpDaemonSidecarName)
		if err != nil {
			return info, fmt.Errorf("lstat %s: %w", path, err)
		}
		if err := rejectLink(linked, path); err != nil {
			return info, err
		}
		if !linked.Mode().IsRegular() {
			return info, mcpDaemonValidationError("%s is not a regular file", path)
		}
		f, err = runtimeRoot.Open(mcpDaemonSidecarName)
		if err != nil {
			return info, fmt.Errorf("open %s: %w", path, err)
		}
		opened, err := f.Stat()
		if err != nil {
			_ = f.Close() // Preserve the stat failure while releasing this attempt.
			return info, fmt.Errorf("stat %s: %w", path, err)
		}
		if !opened.Mode().IsRegular() {
			_ = f.Close() // Invalid objects never retain a handle or enter retry.
			return info, mcpDaemonValidationError("%s is not a regular file", path)
		}
		if !os.SameFile(opened, linked) {
			_ = f.Close() // Release the replaced regular file before sleeping.
			if attempt < 3 {
				time.Sleep(2 * time.Millisecond)
				continue
			}
			return info, mcpDaemonValidationError("%s changed while opening", path)
		}
		break
	}
	defer func() { _ = f.Close() }()
	if err := json.NewDecoder(io.LimitReader(f, maxMCPDaemonInfoBytes)).Decode(&info); err != nil {
		return MCPDaemonInfo{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return info, nil
}

// removeMCPDaemonSidecar removes the sidecar relative to runtimeRoot.
func removeMCPDaemonSidecar(runtimeRoot *os.Root) error {
	if err := runtimeRoot.Remove(mcpDaemonSidecarName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", mcpDaemonSidecarName, err)
	}
	return nil
}

// writeMCPDaemonSidecar writes info relative to runtimeRoot.
func writeMCPDaemonSidecar(runtimeRoot *os.Root, runtimeDir string, info MCPDaemonInfo) error {
	path, err := resolveMCPDaemonLeafPath(runtimeRoot, runtimeDir, mcpDaemonSidecarName)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("encode daemon info: %w", err)
	}
	if linked, err := runtimeRoot.Lstat(mcpDaemonSidecarName); err == nil {
		if err := rejectLink(linked, path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat %s: %w", path, err)
	}
	// os.Root has no Rename before Go 1.25, so the sidecar is created
	// exclusively in place. The caller holds the OS lock and has removed any
	// stale sidecar, and readers tolerate a partially written record.
	f, err := runtimeRoot.OpenFile(mcpDaemonSidecarName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", mcpDaemonSidecarName, err)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = runtimeRoot.Remove(mcpDaemonSidecarName)
		return fmt.Errorf("write %s: %w", mcpDaemonSidecarName, werr)
	}
	return nil
}

// AcquireMCPDaemonLock acquires the single-daemon lock for the workspace at
// rootPath and returns an idempotent release function. A held lock yields an
// error wrapping ErrDaemonAlreadyRunning unless replace is set, in which
// case the verified holder is stopped first.
func AcquireMCPDaemonLock(ctx context.Context, rootPath string, replace bool) (func() error, error) {
	return acquireMCPDaemonLockWithOptions(ctx, rootPath, replace, defaultMCPDaemonOptions())
}

func acquireMCPDaemonLockWithOptions(ctx context.Context, rootPath string, replace bool, opts mcpDaemonOptions) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("acquire mcp daemon lock: %w", err)
	}
	workspaceRoot, storageRoot, runtimeDir, runtimeRoot, err := openMCPDaemonRuntimeRoot(rootPath)
	if err != nil {
		return nil, err
	}
	lockFile, acquired, err := opts.OpenLock(runtimeRoot, runtimeDir)
	if err != nil {
		_ = runtimeRoot.Close()
		return nil, err
	}
	if !acquired {
		defer func() { _ = runtimeRoot.Close() }()
		if replace {
			// Stop the verified holder, then retry once without replace so a
			// racing new holder yields ErrDaemonAlreadyRunning.
			if _, err := stopMCPDaemonWithOptions(ctx, rootPath, opts); err != nil {
				return nil, fmt.Errorf("replace mcp daemon: %w", err)
			}
			return acquireMCPDaemonLockWithOptions(ctx, rootPath, false, opts)
		}
		holder, readErr := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
		if errors.Is(readErr, corerrors.ErrValidation) {
			return nil, readErr
		}
		who := "another process"
		if readErr == nil && holder.PID != 0 {
			who = fmt.Sprintf("pid %d (started %s)", holder.PID, holder.StartedAt.Format(time.RFC3339))
		}
		return nil, fmt.Errorf("%w: %s holds %s; run `backlogit mcp status`, `backlogit mcp stop`, or `backlogit mcp --replace`",
			corerrors.ErrDaemonAlreadyRunning, who, filepath.Join(runtimeDir, mcpDaemonLockName))
	}

	fail := func(err error) (func() error, error) {
		_ = unlockFile(lockFile)
		_ = lockFile.Close()
		_ = runtimeRoot.Close()
		return nil, err
	}
	if err := removeMCPDaemonSidecar(runtimeRoot); err != nil {
		return fail(err)
	}
	pid := os.Getpid()
	startTime, err := processStartTime(pid)
	if err != nil {
		return fail(fmt.Errorf("resolve process identity: %w", err))
	}
	exePath, err := os.Executable()
	if err != nil {
		return fail(fmt.Errorf("resolve executable path: %w", err))
	}
	info := MCPDaemonInfo{
		PID:              pid,
		ProcessStartTime: startTime,
		ExecutablePath:   exePath,
		StartedAt:        time.Now().UTC().Truncate(time.Second),
		WorkspaceRoot:    workspaceRoot,
		StorageRoot:      storageRoot,
		Version:          version.Resolve(),
		LockPath:         filepath.Join(runtimeDir, mcpDaemonLockName),
	}
	if err := writeMCPDaemonSidecar(runtimeRoot, runtimeDir, info); err != nil {
		return fail(err)
	}

	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() {
			releaseErr = errors.Join(
				removeMCPDaemonSidecar(runtimeRoot),
				unlockFile(lockFile),
				lockFile.Close(),
				runtimeRoot.Close(),
			)
		})
		return releaseErr
	}, nil
}

// InspectMCPDaemonStatus probes metadata, process identity, and IPC without
// locking a live daemon. Only crashed-holder cleanup takes the OS lock.
func InspectMCPDaemonStatus(rootPath string) (MCPDaemonStatus, error) {
	workspaceRoot, storageRoot, runtimeDir, runtimeRoot, err := openMCPDaemonRuntimeRoot(rootPath)
	if err != nil {
		return MCPDaemonStatus{}, err
	}
	defer func() { _ = runtimeRoot.Close() }()

	status := MCPDaemonStatus{
		WorkspaceRoot: workspaceRoot,
		StorageRoot:   storageRoot,
		LockPath:      filepath.Join(runtimeDir, mcpDaemonLockName),
	}
	info, err := probeMCPDaemonInfo(runtimeRoot, runtimeDir, defaultMCPDaemonOptions())
	if errors.Is(err, os.ErrNotExist) {
		return status, nil
	}
	if err != nil {
		return MCPDaemonStatus{}, err
	}
	if !isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath) {
		return MCPDaemonStatus{}, mcpDaemonValidationError("held daemon lock has unverifiable process metadata")
	}
	if info.Endpoint == "" {
		return mcpDaemonStatusFromInfo(info), nil // Legacy lock-only holder.
	}
	if !sameDaemonPath(info.WorkspaceRoot, workspaceRoot) || !sameDaemonPath(info.StorageRoot, storageRoot) ||
		!sameDaemonPath(info.LockPath, status.LockPath) {
		return MCPDaemonStatus{}, mcpDaemonValidationError("daemon metadata does not belong to this workspace")
	}
	return mcpDaemonControl(context.Background(), info, "status")
}

// probeMCPDaemonInfo preserves no-follow leaf validation without opening the
// lock on absent metadata or a verified live holder.
func probeMCPDaemonInfo(runtimeRoot *os.Root, runtimeDir string, opts mcpDaemonOptions) (MCPDaemonInfo, error) {
	lockPath, err := resolveMCPDaemonLeafPath(runtimeRoot, runtimeDir, mcpDaemonLockName)
	if err != nil {
		return MCPDaemonInfo{}, err
	}
	if linked, err := runtimeRoot.Lstat(mcpDaemonLockName); err == nil {
		if err := rejectLink(linked, lockPath); err != nil {
			return MCPDaemonInfo{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return MCPDaemonInfo{}, fmt.Errorf("inspect daemon lock leaf: %w", err)
	}
	info, err := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
	if err != nil {
		return MCPDaemonInfo{}, err
	}
	if isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath) {
		return info, nil
	}
	probe, acquired, err := opts.OpenLock(runtimeRoot, runtimeDir)
	if err != nil {
		return MCPDaemonInfo{}, err
	}
	if !acquired {
		// A mismatched identity while the lock is held must never be treated
		// as an unowned stale sidecar (or permit termination without reverify).
		return info, nil
	}
	// Re-read under the cleanup lock: a new owner may have published metadata
	// between the first read and acquisition.
	current, readErr := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
	if readErr == nil && isVerifiedProcessAlive(current.PID, current.ProcessStartTime, current.ExecutablePath) {
		return current, errors.Join(unlockFile(probe), probe.Close())
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return MCPDaemonInfo{}, errors.Join(readErr, unlockFile(probe), probe.Close())
	}
	if err := closeLockProbe(runtimeRoot, probe); err != nil {
		return MCPDaemonInfo{}, err
	}
	return MCPDaemonInfo{}, fmt.Errorf("daemon holder is not running: %w", os.ErrNotExist)
}

// StopMCPDaemon stops the MCP daemon for the workspace at rootPath. It is
// idempotent: with no daemon running it removes any stale sidecar and reports
// Stopped=false. A live holder is terminated only after its recorded process
// identity is verified on a retained OS process handle.
func StopMCPDaemon(ctx context.Context, rootPath string) (MCPDaemonStopResult, error) {
	return stopMCPDaemonWithOptions(ctx, rootPath, defaultMCPDaemonOptions())
}

// closeLockProbe removes the sidecar while the probe lock is still held and
// then releases the probe.
func closeLockProbe(runtimeRoot *os.Root, probe *os.File) error {
	return errors.Join(removeMCPDaemonSidecar(runtimeRoot), unlockFile(probe), probe.Close())
}

// sameDaemonPath reports whether recorded and current name the same filesystem
// object, regardless of symlink aliasing or (on case-insensitive volumes)
// spelling. It fails closed: an empty or unresolvable path never matches.
func sameDaemonPath(recorded, current string) bool {
	if recorded == "" || current == "" {
		return false
	}
	recordedInfo, err := os.Stat(filepath.Clean(recorded))
	if err != nil {
		return false
	}
	currentInfo, err := os.Stat(filepath.Clean(current))
	if err != nil {
		return false
	}
	return os.SameFile(recordedInfo, currentInfo)
}

func stopMCPDaemonWithOptions(ctx context.Context, rootPath string, opts mcpDaemonOptions) (MCPDaemonStopResult, error) {
	workspaceRoot, storageRoot, runtimeDir, runtimeRoot, err := openMCPDaemonRuntimeRoot(rootPath)
	if err != nil {
		return MCPDaemonStopResult{}, err
	}
	defer func() { _ = runtimeRoot.Close() }()
	result := MCPDaemonStopResult{
		WorkspaceRoot: workspaceRoot,
		StorageRoot:   storageRoot,
		LockPath:      filepath.Join(runtimeDir, mcpDaemonLockName),
	}

	if err := ctx.Err(); err != nil {
		return MCPDaemonStopResult{}, fmt.Errorf("stop MCP daemon: %w", err)
	}
	info, err := probeMCPDaemonInfo(runtimeRoot, runtimeDir, opts)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return MCPDaemonStopResult{}, err
	}
	if info.PID <= 0 || info.ProcessStartTime <= 0 || info.ExecutablePath == "" {
		return MCPDaemonStopResult{}, mcpDaemonValidationError("refusing to stop daemon with invalid or incomplete metadata (pid %d)", info.PID)
	}
	if !sameDaemonPath(info.WorkspaceRoot, workspaceRoot) || !sameDaemonPath(info.StorageRoot, storageRoot) || !sameDaemonPath(info.LockPath, result.LockPath) {
		return MCPDaemonStopResult{}, mcpDaemonValidationError("refusing to stop pid %d: daemon metadata does not belong to this workspace", info.PID)
	}
	if info.Endpoint != "" && isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath) {
		if _, controlErr := mcpDaemonControl(ctx, info, "stop"); controlErr == nil {
			if waitErr := waitMCPDaemonDrain(ctx, runtimeRoot, runtimeDir, info, opts.StopTimeout); waitErr == nil {
				result.Stopped, result.PID = true, info.PID
				return result, nil
			} else if ctx.Err() != nil || info.PID == opts.PID() {
				return MCPDaemonStopResult{}, waitErr
			}
		} else if ctx.Err() != nil {
			return MCPDaemonStopResult{}, controlErr
		}
	}
	if info.PID == opts.PID() {
		return MCPDaemonStopResult{}, mcpDaemonValidationError("refusing to terminate daemon host process (pid %d)", info.PID)
	}

	reverify := func() error {
		cur, err := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
		if err != nil {
			return mcpDaemonValidationError("re-read daemon metadata: %v", err)
		}

		if cur.PID != info.PID || cur.ProcessStartTime != info.ProcessStartTime {
			return mcpDaemonValidationError("daemon metadata changed before termination")
		}
		p, got, err := opts.OpenLock(runtimeRoot, runtimeDir)
		if err != nil {
			return err
		}
		if got {
			_ = unlockFile(p)
			_ = p.Close()
			return mcpDaemonValidationError("daemon lock released before termination")
		}
		return nil
	}
	if err := opts.TerminateVerifiedProcess(info.PID, info.ProcessStartTime, info.ExecutablePath, reverify); err != nil {
		return MCPDaemonStopResult{}, fmt.Errorf("terminate mcp daemon pid %d: %w", info.PID, err)
	}

	timer := time.NewTimer(opts.StopTimeout)
	defer timer.Stop()
	for {
		probe, acquired, err := opts.OpenLock(runtimeRoot, runtimeDir)
		if err != nil {
			return MCPDaemonStopResult{}, err
		}
		if acquired {
			if err := closeLockProbe(runtimeRoot, probe); err != nil {
				return MCPDaemonStopResult{}, err
			}
			result.Stopped = true
			result.PID = info.PID
			return result, nil
		}
		select {
		case <-ctx.Done():
			return MCPDaemonStopResult{}, fmt.Errorf("wait for mcp daemon lock release: %w", ctx.Err())
		case <-timer.C:
			return MCPDaemonStopResult{}, fmt.Errorf("mcp daemon pid %d did not release %s within %s", info.PID, result.LockPath, opts.StopTimeout)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitMCPDaemonDrain(ctx context.Context, runtimeRoot *os.Root, runtimeDir string, info MCPDaemonInfo, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		current, err := readMCPDaemonInfoAt(runtimeRoot, runtimeDir)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("observe daemon drain: %w", err)
		}
		if current.PID != info.PID || current.ProcessStartTime != info.ProcessStartTime {
			return mcpDaemonValidationError("daemon holder changed during drain")
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("wait for daemon drain: %w", waitCtx.Err())
		case <-tick.C:
		}
	}
}
