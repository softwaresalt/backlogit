package cli

import (
	"bufio"
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
	"runtime"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
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
		result = errors.Join(result, s.CloseAllSessions())
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
	result = errors.Join(result, s.CloseAllSessions())
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
			return MCPDaemonStatus{}, readErr
		}
		if readErr == nil && isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath) {
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

// pinnedMCPDaemonBinary resolves only a regular, no-follow workspace bin image.
func pinnedMCPDaemonBinary(workspace string) (binary string, result error) {
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return "", fmt.Errorf("open pinned binary workspace: %w", err)
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	name := "backlogit"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary = filepath.Join(workspace, "bin", name)
	for _, leaf := range []string{"bin", filepath.Join("bin", name)} {
		info, err := root.Lstat(leaf)
		if err != nil {
			return "", fmt.Errorf("required pinned daemon binary %s: %w", binary, errors.Join(corerrors.ErrDaemonBinaryMissing, err))
		}
		if err := rejectLink(info, filepath.Join(workspace, leaf)); err != nil {
			return "", err
		}
	}
	file, err := root.Open(filepath.Join("bin", name))
	if err != nil {
		return "", fmt.Errorf("open pinned daemon binary %s: %w", binary, errors.Join(corerrors.ErrDaemonBinaryMissing, err))
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat pinned daemon binary: %w", err)
	}
	linked, err := root.Lstat(filepath.Join("bin", name))
	if err != nil {
		return "", fmt.Errorf("verify pinned daemon binary: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(opened, linked) {
		return "", mcpDaemonValidationError("pinned daemon binary changed or is not regular")
	}
	return binary, nil
}

// RunMCPSessionShim proxies one stdio session to the verified workspace daemon.
// Blocking streams must implement io.Closer; ownership transfers for the session
// so cancellation or peer EOF can interrupt and join both copy workers.
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
	f, err := runtimeRoot.Open(mcpDaemonSidecarName)
	if err != nil {
		return info, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return info, fmt.Errorf("stat %s: %w", path, err)
	}
	if !os.SameFile(opened, linked) || !opened.Mode().IsRegular() {
		return info, mcpDaemonValidationError("%s changed while opening", path)
	}
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
