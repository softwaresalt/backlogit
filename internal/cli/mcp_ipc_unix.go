//go:build !windows

package cli

import (
	"context"
	"net"
	"os"
	"time"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// DeriveMCPDaemonEndpoint declares the platform-local MCP daemon endpoint.
func DeriveMCPDaemonEndpoint(workspaceRoot, storageRoot, runtimeDir string) (string, string, error) {
	return "", "", corerrors.ErrNotImplemented
}

// ListenMCPDaemonIPC declares the platform-local MCP daemon listener.
func ListenMCPDaemonIPC(ctx context.Context, runtimeRoot *os.Root, runtimeDir, transport, endpoint string) (net.Listener, error) {
	return nil, corerrors.ErrNotImplemented
}

// DialMCPDaemonIPC declares the platform-local MCP daemon connection.
func DialMCPDaemonIPC(ctx context.Context, transport, endpoint string, timeout time.Duration) (net.Conn, error) {
	return nil, corerrors.ErrNotImplemented
}

func isVerifiedProcessAlive(pid int, startTime int64, exePath string) bool {
	return false
}

var _ = isVerifiedProcessAlive
