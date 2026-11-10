//go:build !windows

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func prepareMCPSessionInput(ctx context.Context, input io.Reader) (io.Reader, error) {
	file, ok := input.(*os.File)
	if !ok {
		return input, nil
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat session input: %w", err)
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeSocket) == 0 {
		return input, nil
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("access session input descriptor: %w", err)
	}
	reader := &mcpSessionPipeReader{ctx: ctx, file: file}
	if err := raw.Control(func(fd uintptr) { reader.fd = int32(fd) }); err != nil {
		return nil, fmt.Errorf("inspect session input descriptor: %w", err)
	}
	return reader, nil
}

// Inherited stdin is not Go-pollable: Close can wait for its blocking Read.
// Poll before each read without changing shared descriptor flags. Session
// ownership guarantees that no competing reader can consume the ready bytes.
// Do not embed File: its WriterTo fast path would bypass the cancellable Read.
type mcpSessionPipeReader struct {
	ctx  context.Context
	file *os.File
	fd   int32
}

func (r *mcpSessionPipeReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	fds := []unix.PollFd{{Fd: r.fd, Events: unix.POLLIN}}
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		_, err := unix.Poll(fds, 50)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("poll session input: %w", err)
		}
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		if fds[0].Revents != 0 {
			return r.file.Read(p)
		}
	}
}

func (r *mcpSessionPipeReader) Close() error {
	return r.file.Close()
}

// DeriveMCPDaemonEndpoint derives a socket strictly inside the workspace runtime.
func DeriveMCPDaemonEndpoint(workspaceRoot, storageRoot, runtimeDir string) (string, string, error) {
	if err := validateMCPIPCDirectories(workspaceRoot, storageRoot, runtimeDir); err != nil {
		return "", "", err
	}
	endpoint := filepath.Join(runtimeDir, "mcp.sock")
	if len(endpoint) <= 100 {
		return "unix", endpoint, nil
	}
	endpoint = filepath.Join(runtimeDir, "s")
	if len(endpoint) > 100 {
		return "", "", fmt.Errorf("unix domain socket path %q (%d bytes) exceeds 100-byte sun_path limit; move the workspace to a shorter path or set BACKLOGIT_WORKSPACE_DIR to a shorter workspace-contained storage path: %w", endpoint, len(endpoint), corerrors.ErrValidation)
	}
	return "unix", endpoint, nil
}

// mcpIPCUmaskMu serializes the process-wide umask change required at socket bind.
var mcpIPCUmaskMu sync.Mutex

// ListenMCPDaemonIPC binds an owner-only socket in the pinned runtime directory.
func ListenMCPDaemonIPC(ctx context.Context, runtimeRoot *os.Root, runtimeDir, transport, endpoint string) (net.Listener, error) {
	if runtimeRoot == nil || transport != "unix" || !filepath.IsAbs(runtimeDir) ||
		!filepath.IsAbs(endpoint) || len(endpoint) > 100 ||
		filepath.Clean(endpoint) != endpoint ||
		filepath.Dir(endpoint) != filepath.Clean(runtimeDir) {
		return nil, fmt.Errorf("invalid workspace runtime socket endpoint: %w", corerrors.ErrValidation)
	}
	// Retain the cleaned-directory invariant alongside the stricter raw-path
	// checks above; cleaning alone must never admit a non-canonical endpoint.
	if filepath.Clean(filepath.Dir(endpoint)) != filepath.Clean(runtimeDir) {
		return nil, fmt.Errorf("socket directory must match runtime directory: %w", corerrors.ErrValidation)
	}
	leafName := filepath.Base(endpoint)
	if leafName != "mcp.sock" && leafName != "s" {
		return nil, fmt.Errorf("invalid runtime socket leaf %q: %w", leafName, corerrors.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("listen MCP IPC: %w", err)
	}
	// Root.Name can be relative for nested roots. Compare directory identities,
	// not names, before performing the path-based bind.
	opened, err := runtimeRoot.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("stat pinned runtime: %w", err)
	}
	linked, err := os.Lstat(runtimeDir)
	if err != nil {
		return nil, fmt.Errorf("stat runtime directory: %w", err)
	}
	if !linked.IsDir() || !os.SameFile(opened, linked) {
		return nil, fmt.Errorf("runtime directory does not match pinned root: %w", corerrors.ErrValidation)
	}
	if info, err := runtimeRoot.Lstat(leafName); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("refuse non-socket runtime leaf: %w", corerrors.ErrValidation)
		}
		if err := rejectLink(info, endpoint); err != nil {
			return nil, fmt.Errorf("validate stale runtime socket: %w", err)
		}
		if err := runtimeRoot.Remove(leafName); err != nil {
			return nil, fmt.Errorf("remove stale runtime socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect runtime socket: %w", err)
	}
	listener, err := bindMCPUnixSocket(endpoint)
	if err != nil {
		return nil, err
	}
	// Cleanup is handle-relative too; net.UnixListener's path-based unlink is
	// disabled so directory renames cannot redirect removal.
	listener.SetUnlinkOnClose(false)
	boundInfo, err := os.Lstat(endpoint)
	if err != nil {
		return nil, fmt.Errorf("inspect bound socket identity: %w", errors.Join(err, listener.Close()))
	}
	if boundInfo.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("bound endpoint is not a socket: %w", errors.Join(corerrors.ErrValidation, listener.Close()))
	}
	socket := &mcpUnixListener{UnixListener: listener, root: runtimeRoot, leaf: leafName, info: boundInfo}
	pinnedInfo, err := runtimeRoot.Lstat(leafName)
	if err != nil {
		return nil, fmt.Errorf("inspect bound runtime socket: %w", errors.Join(err, socket.Close()))
	}
	if !os.SameFile(boundInfo, pinnedInfo) || pinnedInfo.Mode()&os.ModeSocket == 0 || pinnedInfo.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("bound runtime socket identity or permissions invalid: %w", errors.Join(corerrors.ErrValidation, socket.Close()))
	}
	if err := rejectLink(pinnedInfo, endpoint); err != nil {
		return nil, fmt.Errorf("validate bound runtime socket: %w", errors.Join(err, socket.Close()))
	}
	// Serialize callback installation with Close even for an already-canceled
	// context. AfterFunc owns the callback lifecycle; Close stops it.
	socket.mu.Lock()
	socket.stop = context.AfterFunc(ctx, func() {
		_ = socket.Close() // Close errors remain available to the listener owner.
	})
	socket.mu.Unlock()
	return socket, nil
}

func bindMCPUnixSocket(endpoint string) (*net.UnixListener, error) {
	mcpIPCUmaskMu.Lock()
	defer mcpIPCUmaskMu.Unlock()
	oldMask := syscall.Umask(0o177)
	defer syscall.Umask(oldMask)
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("bind runtime socket: %w", err)
	}
	return listener.(*net.UnixListener), nil
}

type mcpUnixListener struct {
	*net.UnixListener
	mu       sync.Mutex
	root     *os.Root
	leaf     string
	info     os.FileInfo
	stop     func() bool
	closed   bool
	closeErr error
}

func (l *mcpUnixListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.closeErr
	}
	l.closed = true
	if l.stop != nil {
		l.stop()
	}
	err := l.UnixListener.Close()
	info, statErr := l.root.Lstat(l.leaf)
	if statErr == nil && os.SameFile(l.info, info) {
		err = errors.Join(err, l.root.Remove(l.leaf))
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		err = errors.Join(err, statErr)
	}
	if err != nil {
		l.closeErr = fmt.Errorf("close runtime socket: %w", err)
	}
	return l.closeErr
}

// DialMCPDaemonIPC connects to a local socket within the caller's time budget.
func DialMCPDaemonIPC(ctx context.Context, transport, endpoint string, timeout time.Duration) (net.Conn, error) {
	leaf := filepath.Base(endpoint)
	if transport != "unix" || !filepath.IsAbs(endpoint) || len(endpoint) > 100 ||
		(leaf != "mcp.sock" && leaf != "s") || timeout <= 0 {
		return nil, fmt.Errorf("invalid MCP socket dial parameters: %w", corerrors.ErrValidation)
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("dial runtime socket: %w", err)
	}
	return conn, nil
}

func isVerifiedProcessAlive(pid int, startTime int64, exePath string) bool {
	if pid <= 0 || startTime <= 0 || !filepath.IsAbs(exePath) {
		return false
	}
	procDir := filepath.Join("/proc", strconv.Itoa(pid))
	// Parse field 22 directly: processStartTime's non-procfs fallback is not
	// process identity. Missing or malformed procfs data must fail closed.
	readStart := func() int64 {
		data, err := os.ReadFile(filepath.Join(procDir, "stat"))
		if err != nil {
			return 0
		}
		stat := string(data)
		end := strings.LastIndexByte(stat, ')')
		if end < 0 {
			return 0
		}
		fields := strings.Fields(stat[end+1:])
		if len(fields) < 20 {
			return 0
		}
		start, err := strconv.ParseInt(fields[19], 10, 64)
		if err != nil || start <= 0 {
			return 0
		}
		return start
	}
	start := readStart()
	if start <= 0 || start != startTime {
		return false
	}
	// /proc supplies read-only executable identity on Linux. On systems without
	// this facility fail closed, rather than signal a potentially recycled PID.
	image, err := os.Readlink(filepath.Join(procDir, "exe"))
	if err != nil || filepath.Clean(strings.TrimSuffix(image, " (deleted)")) != filepath.Clean(exePath) {
		return false
	}
	return readStart() == start
}

func validateMCPIPCDirectories(workspaceRoot, storageRoot, runtimeDir string) error {
	if !filepath.IsAbs(workspaceRoot) || !filepath.IsAbs(storageRoot) || !filepath.IsAbs(runtimeDir) {
		return fmt.Errorf("MCP workspace, storage, and runtime directories must be absolute: %w", corerrors.ErrValidation)
	}
	workspace, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return fmt.Errorf("canonicalize MCP workspace: %w", err)
	}
	storage, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return fmt.Errorf("canonicalize MCP storage: %w", err)
	}
	rel, err := filepath.Rel(workspace, storage)
	if err != nil || rel == ".." || !filepath.IsLocal(rel) {
		return fmt.Errorf("MCP storage must remain inside workspace: %w", corerrors.ErrValidation)
	}
	runtimePath, err := filepath.EvalSymlinks(runtimeDir)
	if err != nil {
		return fmt.Errorf("canonicalize MCP runtime: %w", err)
	}
	if runtimePath != filepath.Join(storage, "runtime") {
		return fmt.Errorf("MCP runtime must be storage/runtime: %w", corerrors.ErrValidation)
	}
	return nil
}
