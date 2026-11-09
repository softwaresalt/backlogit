//go:build windows

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// DeriveMCPDaemonEndpoint derives a pipe name from canonical storage identity.
func DeriveMCPDaemonEndpoint(workspaceRoot, storageRoot, runtimeDir string) (string, string, error) {
	if err := validateMCPIPCDirectories(workspaceRoot, storageRoot, runtimeDir); err != nil {
		return "", "", err
	}
	storage, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize pipe storage: %w", err)
	}
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(storage))))
	return "named_pipe", `\\.\pipe\backlogit-mcp-` + hex.EncodeToString(digest[:8]), nil
}

// ListenMCPDaemonIPC listens on a local, current-user-only multi-instance pipe.
func ListenMCPDaemonIPC(ctx context.Context, runtimeRoot *os.Root, runtimeDir, transport, endpoint string) (net.Listener, error) {
	if runtimeRoot == nil || !filepath.IsAbs(runtimeDir) || transport != "named_pipe" || !validMCPPipeEndpoint(endpoint) {
		return nil, fmt.Errorf("invalid local MCP pipe listener parameters: %w", corerrors.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("listen MCP pipe: %w", err)
	}
	// Use the process owner, not an impersonated thread token.
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("query pipe owner SID: %w", err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, fmt.Errorf("create pipe owner DACL: %w", err)
	}
	listenerCtx, cancel := context.WithCancel(ctx)
	listener := &mcpPipeListener{ctx: listenerCtx, cancel: cancel, endpoint: endpoint, descriptor: descriptor}
	listener.pending, err = listener.newInstance(true)
	if err != nil {
		cancel()
		return nil, err
	}
	listener.mu.Lock()
	listener.stop = context.AfterFunc(ctx, func() {
		_ = listener.Close() // The owner can retrieve the retained close error.
	})
	listener.mu.Unlock()
	return listener, nil
}

// DialMCPDaemonIPC connects to a local pipe with bounded busy-instance retries.
func DialMCPDaemonIPC(ctx context.Context, transport, endpoint string, timeout time.Duration) (net.Conn, error) {
	if transport != "named_pipe" || !validMCPPipeEndpoint(endpoint) || timeout <= 0 {
		return nil, fmt.Errorf("invalid local MCP pipe dial parameters: %w", corerrors.ErrValidation)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	name, err := windows.UTF16PtrFromString(endpoint)
	if err != nil {
		return nil, fmt.Errorf("encode pipe endpoint: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("dial MCP pipe: %w", err)
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("dial MCP pipe canceled: %w", errors.Join(err, windows.CloseHandle(handle)))
			}
			return newMCPPipeConn(handle, endpoint), nil
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil, fmt.Errorf("open MCP pipe: %w", err)
		}
		// x/sys/windows does not expose WaitNamedPipe. A bounded retry avoids
		// unsafe syscall pointers and, unlike an unbounded native wait, honors
		// caller cancellation while Accept replenishes available instances.
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("wait for MCP pipe instance: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func isVerifiedProcessAlive(pid int, startTime int64, exePath string) bool {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) || startTime <= 0 || !filepath.IsAbs(exePath) {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }() // Read-only identity probe owns this handle.
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil || creation.Nanoseconds() != startTime {
		return false
	}
	image := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(image))
	if err := windows.QueryFullProcessImageName(handle, 0, &image[0], &size); err != nil ||
		!strings.EqualFold(filepath.Clean(windows.UTF16ToString(image[:size])), filepath.Clean(exePath)) {
		return false
	}
	// Waiting with zero timeout observes death without signaling the process.
	state, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && state == uint32(windows.WAIT_TIMEOUT)
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
	if !strings.EqualFold(runtimePath, filepath.Join(storage, "runtime")) {
		return fmt.Errorf("MCP runtime must be storage/runtime: %w", corerrors.ErrValidation)
	}
	return nil
}

func validMCPPipeEndpoint(endpoint string) bool {
	const prefix = `\\.\pipe\backlogit-mcp-`
	if !strings.HasPrefix(endpoint, prefix) || len(endpoint) != len(prefix)+16 {
		return false
	}
	for _, c := range endpoint[len(prefix):] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

type mcpPipeAddr string

func (a mcpPipeAddr) Network() string { return "named_pipe" }
func (a mcpPipeAddr) String() string  { return string(a) }

type mcpPipeListener struct {
	mu         sync.Mutex
	acceptMu   sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	stop       func() bool
	endpoint   string
	descriptor *windows.SECURITY_DESCRIPTOR
	pending    *mcpPipeConn
	closed     bool
	closeErr   error
}

func (l *mcpPipeListener) newInstance(first bool) (*mcpPipeConn, error) {
	name, err := windows.UTF16PtrFromString(l.endpoint)
	if err != nil {
		return nil, fmt.Errorf("encode listener endpoint: %w", err)
	}
	// SECURITY_ATTRIBUTES occupies three pointer-sized ABI slots on both
	// Win32 and Win64, including alignment padding. No unsafe pointer is needed.
	attributes := windows.SecurityAttributes{Length: uint32(3 * (strconv.IntSize / 8)), SecurityDescriptor: l.descriptor}
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
	if first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	handle, err := windows.CreateNamedPipe(name, flags,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		windows.PIPE_UNLIMITED_INSTANCES, 65536, 65536, 0, &attributes)
	if err != nil {
		return nil, fmt.Errorf("create MCP pipe instance: %w", err)
	}
	return newMCPPipeConn(handle, l.endpoint), nil
}

func (l *mcpPipeListener) Accept() (net.Conn, error) {
	l.acceptMu.Lock()
	defer l.acceptMu.Unlock()
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, net.ErrClosed
	}
	if l.pending == nil {
		var err error
		l.pending, err = l.newInstance(false)
		if err != nil {
			l.mu.Unlock()
			return nil, err
		}
	}
	conn := l.pending
	l.mu.Unlock()
	_, err := conn.run(l.ctx, 0, func(overlapped *windows.Overlapped, _ *uint32) error {
		err := windows.ConnectNamedPipe(conn.handle, overlapped)
		if errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			return nil
		}
		return err
	})
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil, net.ErrClosed
	}
	l.pending = nil
	if err != nil {
		return nil, fmt.Errorf("accept MCP pipe: %w", errors.Join(err, conn.Close()))
	}
	// Keep an unconnected instance available before handing this one to the
	// caller, so concurrent clients can coexist rather than serialize sessions.
	l.pending, err = l.newInstance(false)
	if err != nil {
		return nil, fmt.Errorf("replenish MCP pipe: %w", errors.Join(err, conn.Close()))
	}
	return conn, nil
}

func (l *mcpPipeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.closeErr
	}
	l.closed = true
	l.cancel()
	if l.stop != nil {
		l.stop()
	}
	if l.pending != nil {
		l.closeErr = l.pending.Close()
		l.pending = nil
	}
	return l.closeErr
}

func (l *mcpPipeListener) Addr() net.Addr { return mcpPipeAddr(l.endpoint) }

// Each operation owns an event until its OVERLAPPED completion is drained.
// Close cancels pending operations and joins them before releasing the handle.
type mcpPipeConn struct {
	mu            sync.Mutex
	readMu        sync.Mutex
	writeMu       sync.Mutex
	operations    sync.WaitGroup
	handle        windows.Handle
	endpoint      string
	readDeadline  time.Time
	writeDeadline time.Time
	closed        bool
	closeDone     chan struct{}
	closeErr      error
}

func newMCPPipeConn(handle windows.Handle, endpoint string) *mcpPipeConn {
	return &mcpPipeConn{handle: handle, endpoint: endpoint, closeDone: make(chan struct{})}
}

func (c *mcpPipeConn) Read(data []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	n, err := c.run(context.Background(), 1, func(o *windows.Overlapped, n *uint32) error {
		return windows.ReadFile(c.handle, data, n, o)
	})
	runtime.KeepAlive(data)
	if errors.Is(err, io.EOF) {
		return int(n), io.EOF
	}
	if err == os.ErrDeadlineExceeded {
		return int(n), err
	}
	if err != nil {
		return int(n), fmt.Errorf("read MCP pipe: %w", err)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (c *mcpPipeConn) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	n, err := c.run(context.Background(), 2, func(o *windows.Overlapped, n *uint32) error {
		return windows.WriteFile(c.handle, data, n, o)
	})
	runtime.KeepAlive(data)
	if err == os.ErrDeadlineExceeded {
		return int(n), err
	}
	if err != nil {
		return int(n), fmt.Errorf("write MCP pipe: %w", err)
	}
	return int(n), nil
}

func (c *mcpPipeConn) run(ctx context.Context, direction int, start func(*windows.Overlapped, *uint32) error) (uint32, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.operations.Add(1)
	c.mu.Unlock()
	defer c.operations.Done()
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("create pipe I/O event: %w", err)
	}
	defer func() { _ = windows.CloseHandle(event) }() // Completion is drained before releasing its event.
	overlapped := windows.Overlapped{HEvent: event}
	var transferred uint32
	if err := c.operationError(ctx, direction); err != nil {
		return 0, err
	}
	err = start(&overlapped, &transferred)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		err = c.await(ctx, direction, &overlapped, &transferred)
	}
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) {
		err = io.EOF
	}
	return transferred, err
}

func (c *mcpPipeConn) operationError(ctx context.Context, direction int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("pipe operation canceled: %w", err)
	}
	var deadline time.Time
	switch direction {
	case 1:
		deadline = c.readDeadline
	case 2:
		deadline = c.writeDeadline
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return os.ErrDeadlineExceeded
	}
	return nil
}

func (c *mcpPipeConn) await(ctx context.Context, direction int, o *windows.Overlapped, n *uint32) error {
	for {
		operationErr := c.operationError(ctx, direction)
		if operationErr != nil {
			cancelErr := windows.CancelIoEx(c.handle, o)
			if errors.Is(cancelErr, windows.ERROR_NOT_FOUND) {
				cancelErr = nil // The completion won the cancellation race.
			}
			drainErr := windows.GetOverlappedResult(c.handle, o, n, true)
			if errors.Is(drainErr, windows.ERROR_OPERATION_ABORTED) {
				drainErr = nil // Expected completion of canceled I/O.
			}
			if cancelErr == nil && drainErr == nil {
				return operationErr
			}
			return errors.Join(operationErr, cancelErr, drainErr)
		}
		state, err := windows.WaitForSingleObject(o.HEvent, 10)
		if err != nil {
			cancelErr := windows.CancelIoEx(c.handle, o)
			if errors.Is(cancelErr, windows.ERROR_NOT_FOUND) {
				cancelErr = nil
			}
			drainErr := windows.GetOverlappedResult(c.handle, o, n, true)
			return fmt.Errorf("wait for pipe I/O: %w", errors.Join(err, cancelErr, drainErr))
		}
		if state == windows.WAIT_OBJECT_0 {
			if err := windows.GetOverlappedResult(c.handle, o, n, false); err != nil {
				return fmt.Errorf("complete pipe I/O: %w", err)
			}
			return nil
		}
	}
}

func (c *mcpPipeConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		<-c.closeDone
		return c.closeErr
	}
	c.closed = true
	err := windows.CancelIoEx(c.handle, nil)
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		err = nil // No operation was pending.
	}
	c.mu.Unlock()
	c.operations.Wait()
	err = errors.Join(err, windows.CloseHandle(c.handle))
	if err != nil {
		c.closeErr = fmt.Errorf("close MCP pipe: %w", err)
	}
	close(c.closeDone)
	return c.closeErr
}

func (c *mcpPipeConn) LocalAddr() net.Addr  { return mcpPipeAddr(c.endpoint) }
func (c *mcpPipeConn) RemoteAddr() net.Addr { return mcpPipeAddr(c.endpoint) }

func (c *mcpPipeConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.readDeadline, c.writeDeadline = deadline, deadline
	return nil
}

func (c *mcpPipeConn) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.readDeadline = deadline
	return nil
}

func (c *mcpPipeConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.writeDeadline = deadline
	return nil
}
