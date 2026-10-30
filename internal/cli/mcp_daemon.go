package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
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
	// PID returns the current process ID.
	PID func() int
	// StopTimeout bounds how long a stop waits for the lock to be released.
	StopTimeout time.Duration
	// OpenLock opens name relative to the runtime root and tries the lock.
	OpenLock func(runtimeRoot *os.Root, name string) (*os.File, bool, error)
	// TerminateVerifiedProcess verifies pid against the recorded identity and
	// terminates it through one retained OS process handle. reverify runs while
	// that handle is held.
	TerminateVerifiedProcess func(pid int, startTime int64, exePath string, reverify func() error) error
}

func defaultMCPDaemonOptions() mcpDaemonOptions {
	return mcpDaemonOptions{
		PID:         os.Getpid,
		StopTimeout: 5 * time.Second,
		OpenLock: func(runtimeRoot *os.Root, _ string) (*os.File, bool, error) {
			return openMCPDaemonLockFile(runtimeRoot)
		},
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

// openMCPDaemonLockFile opens the lock leaf relative to runtimeRoot without
// following links and attempts the exclusive non-blocking lock. On a failed
// acquisition the file is closed and acquired is false.
func openMCPDaemonLockFile(runtimeRoot *os.Root) (file *os.File, acquired bool, err error) {
	lockPath := filepath.Join(runtimeRoot.Name(), mcpDaemonLockName)
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

// readMCPDaemonInfo decodes the sidecar directly from a validated no-follow
// handle relative to runtimeRoot. A missing sidecar yields os.ErrNotExist.
func readMCPDaemonInfo(runtimeRoot *os.Root) (MCPDaemonInfo, error) {
	var info MCPDaemonInfo
	path := filepath.Join(runtimeRoot.Name(), mcpDaemonSidecarName)
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
func writeMCPDaemonSidecar(runtimeRoot *os.Root, info MCPDaemonInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("encode daemon info: %w", err)
	}
	if linked, err := runtimeRoot.Lstat(mcpDaemonSidecarName); err == nil {
		if err := rejectLink(linked, filepath.Join(runtimeRoot.Name(), mcpDaemonSidecarName)); err != nil {
			return err
		}
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
	lockFile, acquired, err := opts.OpenLock(runtimeRoot, mcpDaemonLockName)
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
		holder, readErr := readMCPDaemonInfo(runtimeRoot)
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
	if err := writeMCPDaemonSidecar(runtimeRoot, info); err != nil {
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

// InspectMCPDaemonStatus reports the MCP daemon status for the workspace at
// rootPath by probing the OS lock. A sidecar left by a crashed daemon is
// removed and reported as not running.
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
	probe, acquired, err := openMCPDaemonLockFile(runtimeRoot)
	if err != nil {
		return MCPDaemonStatus{}, err
	}
	if acquired {
		rmErr := removeMCPDaemonSidecar(runtimeRoot)
		unlockErr := unlockFile(probe)
		closeErr := probe.Close()
		if err := errors.Join(rmErr, unlockErr, closeErr); err != nil {
			return MCPDaemonStatus{}, err
		}
		return status, nil
	}

	info, err := readMCPDaemonInfo(runtimeRoot)
	if errors.Is(err, corerrors.ErrValidation) {
		return MCPDaemonStatus{}, err
	}
	status.Running = true
	if err == nil {
		status.PID = info.PID
		status.StartedAt = info.StartedAt
		status.Version = info.Version
	}
	return status, nil
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

// readHolderInfo reads the sidecar, tolerating a new daemon that is still
// replacing stale metadata.
func readHolderInfo(ctx context.Context, runtimeRoot *os.Root, timeout time.Duration) (MCPDaemonInfo, error) {
	deadline := time.Now().Add(timeout)
	for {
		info, err := readMCPDaemonInfo(runtimeRoot)
		if err == nil {
			return info, nil
		}
		if errors.Is(err, corerrors.ErrValidation) || !time.Now().Before(deadline) {
			return MCPDaemonInfo{}, mcpDaemonValidationError("daemon metadata unavailable: %v", err)
		}
		select {
		case <-ctx.Done():
			return MCPDaemonInfo{}, fmt.Errorf("read daemon metadata: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
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

	probe, acquired, err := opts.OpenLock(runtimeRoot, mcpDaemonLockName)
	if err != nil {
		return MCPDaemonStopResult{}, err
	}
	if acquired {
		if err := closeLockProbe(runtimeRoot, probe); err != nil {
			return MCPDaemonStopResult{}, err
		}
		return result, nil
	}

	info, err := readHolderInfo(ctx, runtimeRoot, time.Second)
	if err != nil {
		return MCPDaemonStopResult{}, err
	}
	if info.PID <= 0 || info.PID == opts.PID() || info.ProcessStartTime <= 0 || info.ExecutablePath == "" {
		return MCPDaemonStopResult{}, mcpDaemonValidationError("refusing to stop daemon with invalid or incomplete metadata (pid %d)", info.PID)
	}
	if !sameDaemonPath(info.WorkspaceRoot, workspaceRoot) || !sameDaemonPath(info.StorageRoot, storageRoot) || !sameDaemonPath(info.LockPath, result.LockPath) {
		return MCPDaemonStopResult{}, mcpDaemonValidationError("refusing to stop pid %d: daemon metadata does not belong to this workspace", info.PID)
	}

	reverify := func() error {
		cur, err := readMCPDaemonInfo(runtimeRoot)
		if err != nil {
			return mcpDaemonValidationError("re-read daemon metadata: %v", err)
		}
		if cur.PID != info.PID || cur.ProcessStartTime != info.ProcessStartTime {
			return mcpDaemonValidationError("daemon metadata changed before termination")
		}
		p, got, err := opts.OpenLock(runtimeRoot, mcpDaemonLockName)
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
		probe, acquired, err := opts.OpenLock(runtimeRoot, mcpDaemonLockName)
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
