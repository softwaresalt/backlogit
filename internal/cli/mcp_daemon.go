package cli

import (
	"context"
	"time"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
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
}

// MCPDaemonStatus reports whether an MCP daemon is running for a workspace.
type MCPDaemonStatus struct {
	Running       bool      `json:"running"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	WorkspaceRoot string    `json:"workspace_root"`
	StorageRoot   string    `json:"storage_root"`
	Version       string    `json:"version"`
	LockPath      string    `json:"lock_path"`
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
	// runtimeDir is a pinned handle to the runtime directory so lock and
	// sidecar operations are directory-relative.
	runtimeDir any
	// verifyProcess checks, via one retained process handle, that a PID still
	// refers to the recorded process instance.
	verifyProcess func(info MCPDaemonInfo) (bool, error)
	// terminateProcess terminates the process through the same retained handle
	// used for verification.
	terminateProcess func(ctx context.Context, info MCPDaemonInfo) error
}

// AcquireMCPDaemonLock acquires the single-daemon lock for the workspace at
// rootPath and returns a release function. It is not yet implemented.
func AcquireMCPDaemonLock(ctx context.Context, rootPath string, replace bool) (func() error, error) {
	return nil, corerrors.ErrNotImplemented
}

// InspectMCPDaemonStatus reports the MCP daemon status for the workspace at
// rootPath. It is not yet implemented.
func InspectMCPDaemonStatus(rootPath string) (MCPDaemonStatus, error) {
	return MCPDaemonStatus{}, corerrors.ErrNotImplemented
}

// StopMCPDaemon stops the MCP daemon for the workspace at rootPath. It is not
// yet implemented.
func StopMCPDaemon(ctx context.Context, rootPath string) (MCPDaemonStopResult, error) {
	return MCPDaemonStopResult{}, corerrors.ErrNotImplemented
}
