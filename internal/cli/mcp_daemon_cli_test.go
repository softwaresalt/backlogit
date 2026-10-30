package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

const u3HelperRootEnv = "BACKLOGIT_U3_HELPER_ROOT"

// TestMain runs the U3 daemon-holder helper process when requested.
func TestMain(m *testing.M) {
	if root := os.Getenv(u3HelperRootEnv); root != "" {
		release, err := AcquireMCPDaemonLock(context.Background(), root, false)
		if err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			os.Exit(2)
		}
		defer func() { _ = release() }()
		os.Stdout.WriteString("ready\n")
		buf := make([]byte, 1)
		_, _ = os.Stdin.Read(buf)
		return
	}
	os.Exit(m.Run())
}

type u3Holder struct {
	cmd   *exec.Cmd
	done  chan struct{}
	stdin io.WriteCloser
}

// u3NativeTerminationSupported reports whether verified process termination
// is implemented on this platform; elsewhere it must fail closed.
func u3NativeTerminationSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "windows"
}

// release lets the helper exit cleanly by closing its stdin.
func (h *u3Holder) release() { _ = h.stdin.Close() }

func (h *u3Holder) alive() bool {
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}

func (h *u3Holder) waitExit(t *testing.T) {
	t.Helper()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
		t.Fatal("holder did not exit")
	}
}

func u3StartHolder(t *testing.T, root string) *u3Holder {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), u3HelperRootEnv+"="+root)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	h := &u3Holder{cmd: cmd, done: make(chan struct{}), stdin: stdin}
	go func() { _ = cmd.Wait(); close(h.done) }()
	t.Cleanup(func() {
		h.release()
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-h.done
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", line)
	return h
}

func u3TamperSidecar(t *testing.T, runtimeDir string, mutate func(*MCPDaemonInfo)) {
	t.Helper()
	info := u2ReadSidecar(t, runtimeDir)
	mutate(&info)
	data, err := json.Marshal(info)
	require.NoError(t, err)
	path := filepath.Join(runtimeDir, u2SidecarName)
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func u3RunCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	out := new(bytes.Buffer)
	cmd := NewRootCommand()
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func u3StubServe(t *testing.T, fn func(*mcpinternal.Server) error) {
	t.Helper()
	orig := serveMCPStdio
	serveMCPStdio = fn
	t.Cleanup(func() { serveMCPStdio = orig })
}

func TestU3_StopMCPDaemon_NotRunning(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	u2WriteStaleSidecar(t, runtimeDir)

	res, err := StopMCPDaemon(t.Context(), root)
	require.NoError(t, err)
	assert.False(t, res.Stopped)
	assert.False(t, res.Running)
	assert.Equal(t, root, res.WorkspaceRoot)
	assert.Equal(t, filepath.Join(runtimeDir, u2LockName), res.LockPath)
	_, statErr := os.Stat(filepath.Join(runtimeDir, u2SidecarName))
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "stale sidecar must be removed")
}

func TestU3_StopMCPDaemon_TerminatesVerifiedHolder(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	h := u3StartHolder(t, root)

	res, err := StopMCPDaemon(t.Context(), root)
	if !u3NativeTerminationSupported() {
		require.Error(t, err)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
		assert.True(t, h.alive(), "unsupported platform must fail closed")
		return
	}
	require.NoError(t, err)
	assert.True(t, res.Stopped)
	assert.False(t, res.Running)
	assert.Equal(t, h.cmd.Process.Pid, res.PID)
	h.waitExit(t)
	_, statErr := os.Stat(filepath.Join(runtimeDir, u2SidecarName))
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestU3_StopMCPDaemon_RefusesProcessIdentityMismatch(t *testing.T) {
	cases := map[string]func(*MCPDaemonInfo){
		"start time": func(i *MCPDaemonInfo) { i.ProcessStartTime++ },
		"executable": func(i *MCPDaemonInfo) { i.ExecutablePath = filepath.Join(os.TempDir(), "other-binary") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, _, runtimeDir := u2Workspace(t)
			h := u3StartHolder(t, root)
			u3TamperSidecar(t, runtimeDir, mutate)

			_, err := StopMCPDaemon(t.Context(), root)
			require.Error(t, err)
			assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
			time.Sleep(200 * time.Millisecond)
			assert.True(t, h.alive(), "mismatched holder must not be killed")
		})
	}
}

func TestU3_StopMCPDaemon_RefusesForeignWorkspaceMetadata(t *testing.T) {
	cases := map[string]func(*MCPDaemonInfo){
		"workspace": func(i *MCPDaemonInfo) { i.WorkspaceRoot = filepath.Join(os.TempDir(), "other") },
		"storage":   func(i *MCPDaemonInfo) { i.StorageRoot = filepath.Join(os.TempDir(), "other", ".backlog") },
		"lock":      func(i *MCPDaemonInfo) { i.LockPath = filepath.Join(os.TempDir(), "other.lock") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, _, runtimeDir := u2Workspace(t)
			h := u3StartHolder(t, root)
			u3TamperSidecar(t, runtimeDir, mutate)

			_, err := StopMCPDaemon(t.Context(), root)
			require.Error(t, err)
			assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
			assert.True(t, h.alive(), "foreign metadata must not kill the holder")
		})
	}
}

func TestU3_MissingStorageOverrideDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
	_, err := StopMCPDaemon(t.Context(), root)
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
	_, statErr := os.Stat(filepath.Join(root, ".backlog"))
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "default dir must not be created")
}

func TestU3_StopMCPDaemon_RefusesMissingOrSelfMetadata(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	h := u3StartHolder(t, root)

	opts := defaultMCPDaemonOptions()
	opts.PID = func() int { return h.cmd.Process.Pid }
	called := false
	opts.TerminateVerifiedProcess = func(int, int64, string, func() error) error { called = true; return nil }
	_, err := stopMCPDaemonWithOptions(t.Context(), root, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrValidation), "self pid: %v", err)

	u3TamperSidecar(t, runtimeDir, func(i *MCPDaemonInfo) { i.ProcessStartTime = 0 })
	opts.PID = os.Getpid
	_, err = stopMCPDaemonWithOptions(t.Context(), root, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrValidation), "zero start time: %v", err)
	assert.False(t, called)
	assert.True(t, h.alive())
}

func TestU3_StopMCPDaemon_TerminatesThroughRetainedHandleSeam(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	h := u3StartHolder(t, root)
	want := u2ReadSidecar(t, runtimeDir)

	calls := 0
	opts := defaultMCPDaemonOptions()
	opts.TerminateVerifiedProcess = func(pid int, start int64, exe string, reverify func() error) error {
		calls++
		assert.Equal(t, want.PID, pid)
		assert.Equal(t, want.ProcessStartTime, start)
		assert.Equal(t, want.ExecutablePath, exe)
		require.NoError(t, reverify())
		return h.cmd.Process.Kill()
	}
	res, err := stopMCPDaemonWithOptions(t.Context(), root, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.True(t, res.Stopped)
	assert.Equal(t, want.PID, res.PID)
}

func TestU3_AcquireMCPDaemonLock_ReplaceTakesOverVerifiedHolder(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	h := u3StartHolder(t, root)

	release, err := AcquireMCPDaemonLock(t.Context(), root, true)
	if !u3NativeTerminationSupported() {
		require.Error(t, err)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
		assert.True(t, h.alive(), "unsupported platform must fail closed")
		return
	}
	require.NoError(t, err)
	defer func() { _ = release() }()
	h.waitExit(t)
	assert.Equal(t, os.Getpid(), u2ReadSidecar(t, runtimeDir).PID)
}

func TestU3_AcquireMCPDaemonLock_ReplaceRefusesMismatch(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)
	h := u3StartHolder(t, root)
	u3TamperSidecar(t, runtimeDir, func(i *MCPDaemonInfo) { i.ProcessStartTime++ })

	_, err := AcquireMCPDaemonLock(t.Context(), root, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
	assert.True(t, h.alive())
}

func TestU3_MCPSubcommandsRegistered(t *testing.T) {
	mcpCmd, _, err := NewRootCommand().Find([]string{"mcp"})
	require.NoError(t, err)
	assert.NotNil(t, mcpCmd.Flags().Lookup("replace"))
	for _, name := range []string{"status", "stop"} {
		sub, _, err := NewRootCommand().Find([]string{"mcp", name})
		require.NoError(t, err)
		assert.Equal(t, name, sub.Name())
		assert.NotNil(t, sub.Flags().Lookup("format"))
	}
}

func TestU3_MCPStopCommandIdempotent(t *testing.T) {
	root, _, _ := u2Workspace(t)
	h := u3StartHolder(t, root)

	out, err := u3RunCLI(t, root, "mcp", "stop", "--format", "json")
	if !u3NativeTerminationSupported() {
		require.Error(t, err)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
		assert.True(t, h.alive(), "unsupported platform must fail closed")
		return
	}
	require.NoError(t, err)
	var res MCPDaemonStopResult
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	assert.True(t, res.Stopped)
	assert.False(t, res.Running)
	h.waitExit(t)

	out, err = u3RunCLI(t, root, "mcp", "stop", "--format", "json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	assert.False(t, res.Stopped)
	assert.False(t, res.Running)
}

func TestU3_MCPCommandSingleInstanceAndStatus(t *testing.T) {
	root, _, _ := u2Workspace(t)
	entered := make(chan struct{})
	unblock := make(chan struct{})
	u3StubServe(t, func(*mcpinternal.Server) error {
		close(entered)
		<-unblock
		return nil
	})
	errCh := make(chan error, 1)
	go func() { _, err := u3RunCLI(t, root, "mcp"); errCh <- err }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("serve not entered")
	}

	out, err := u3RunCLI(t, root, "mcp", "status", "--format", "json")
	require.NoError(t, err)
	var st MCPDaemonStatus
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	assert.True(t, st.Running)
	assert.Equal(t, os.Getpid(), st.PID)

	_, err = u3RunCLI(t, root, "mcp")
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrDaemonAlreadyRunning), "got %v", err)

	close(unblock)
	require.NoError(t, <-errCh)
	out, err = u3RunCLI(t, root, "mcp", "status", "--format", "json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	assert.False(t, st.Running)
}

func TestU3_MCPCommandReplaceFlag(t *testing.T) {
	root, _, _ := u2Workspace(t)
	h := u3StartHolder(t, root)
	served := false
	u3StubServe(t, func(*mcpinternal.Server) error { served = true; return nil })

	_, err := u3RunCLI(t, root, "mcp", "--replace")
	require.NoError(t, err)
	assert.True(t, served)
	h.waitExit(t)
}
