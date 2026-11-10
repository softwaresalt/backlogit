package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/version"
)

// 211.005-T / U5: behavior against the landed U5a declarations only.
// The wire fixture uses a newline-delimited protocol_version/command header;
// it deliberately pipelines initialize in the SAME write as that header.
const (
	u5Protocol = "backlogit-mcp-ipc/v1"
	u5Wait     = 5 * time.Second
)

type u5Daemon struct {
	root string
	info MCPDaemonInfo
	done chan struct{}
	err  error // Published by closing done.
}

type u5Client struct {
	conn   net.Conn
	reader *bufio.Reader
	once   sync.Once
}

type u5Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func TestU5_RunMCPDaemonServesMultipleSessionsAndGracefulStop(t *testing.T) {
	d := u5StartDaemon(t, time.Minute)
	clients := []*u5Client{u5Connect(t, d), u5Connect(t, d)}
	// Both initialize(id:1) requests are outstanding before either is read.
	for _, client := range clients {
		response := client.response(t, 1)
		var initialized struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		}
		require.NoError(t, json.Unmarshal(response.Result, &initialized))
		assert.Equal(t, "backlogit", initialized.ServerInfo.Name)
		client.write(t, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n")
	}
	u5SessionCount(t, d, 2)

	// Equal IDs but distinguishable results catch cross-stream routing.
	clients[0].write(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n")
	clients[1].write(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n")
	var tools struct {
		Tools []json.RawMessage `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(clients[0].response(t, 1).Result, &tools))
	assert.NotEmpty(t, tools.Tools, "U5_ROUTING: tools/list must return only to its originating stream")
	assert.JSONEq(t, "{}", string(clients[1].response(t, 1).Result),
		"U5_ROUTING: peer ping with the same id must not receive tools/list")

	clients[0].close(t)
	u5SessionCount(t, d, 1)
	clients[1].write(t, "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\"}\n")
	clients[1].response(t, 2)
	clients[1].close(t)
	u5SessionCount(t, d, 0)

	// Also prove stop drains a live stream, rather than only an idle listener.
	draining := u5Connect(t, d)
	draining.response(t, 1)
	u5SessionCount(t, d, 1)
	ctx, cancel := context.WithTimeout(t.Context(), u5Wait)
	defer cancel()
	stopped, err := StopMCPDaemon(ctx, d.root)
	require.NoError(t, err, "U5_GRACEFUL_STOP: same-process daemon must stop over IPC, never terminate its host")
	assert.True(t, stopped.Stopped)
	assert.False(t, stopped.Running)
	u5Exited(t, d, u5Wait)
	require.NoError(t, draining.conn.SetReadDeadline(time.Now().Add(u5Wait)))
	_, err = draining.reader.ReadByte()
	assert.ErrorIs(t, err, io.EOF, "U5_GRACEFUL_STOP: live session must be closed by graceful drain")
	u5StoppedArtifacts(t, d.root)
	t.Log("U5_WITNESS streams=2 overlapping_id=1 distinct_results=2 active_sessions=2->1->0 peer_survived=true graceful_live_drain=true")
}

func TestU5_VerifyMCPDaemonAttachmentRefusesWorkspaceAndVersionMismatch(t *testing.T) {
	t.Run("identity_and_sentinel_matrix", func(t *testing.T) {
		root := u5Workspace(t)
		release, err := AcquireMCPDaemonLock(t.Context(), root, false)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, release()) })
		info := u5ReadInfo(t, root)
		info.ProtocolVersion = u5Protocol
		foreign := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(foreign, "store"), 0o700))
		otherBinary := filepath.Join(foreign, "other-binary")
		require.NoError(t, os.WriteFile(otherBinary, []byte("not the running executable\n"), 0o600))
		assert.NoError(t, VerifyMCPDaemonAttachment(info, info.WorkspaceRoot, info.StorageRoot, info.Version, u5Protocol),
			"U5_ATTACHMENT_VALID: matching daemon identity must attach")
		cases := []struct {
			name   string
			change func(*MCPDaemonInfo)
			want   error
		}{
			{"workspace", func(i *MCPDaemonInfo) { i.WorkspaceRoot = foreign }, corerrors.ErrDaemonWorkspaceMismatch},
			{"storage", func(i *MCPDaemonInfo) { i.StorageRoot = filepath.Join(foreign, "store") }, corerrors.ErrDaemonWorkspaceMismatch},
			{"version", func(i *MCPDaemonInfo) { i.Version += "-incompatible" }, corerrors.ErrDaemonVersionMismatch},
			{"protocol", func(i *MCPDaemonInfo) { i.ProtocolVersion += "-incompatible" }, corerrors.ErrDaemonVersionMismatch},
			{"binary", func(i *MCPDaemonInfo) { i.ExecutablePath = otherBinary }, corerrors.ErrDaemonVersionMismatch},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				changed := info
				tc.change(&changed)
				err := VerifyMCPDaemonAttachment(changed, info.WorkspaceRoot, info.StorageRoot, info.Version, u5Protocol)
				assert.ErrorIs(t, err, tc.want, "U5_ATTACHMENT_REFUSAL: mismatch must preserve its domain sentinel")
				assert.Equal(t, info, u5InfoWithProtocol(t, root), "refusal must not rewrite/remove holder metadata")
			})
		}
		t.Logf("U5_WITNESS attachment_mismatch_cases=%d real_lock_holder_pid=%d", len(cases), info.PID)
	})
	t.Run("refusal_preserves_active_peer_sessions", func(t *testing.T) {
		d := u5StartDaemon(t, time.Minute)
		clients := []*u5Client{u5Connect(t, d), u5Connect(t, d)}
		for _, client := range clients {
			client.response(t, 1)
		}
		u5SessionCount(t, d, 2)
		foreign := t.TempDir()
		assert.ErrorIs(t, VerifyMCPDaemonAttachment(d.info, foreign, d.info.StorageRoot, d.info.Version, u5Protocol),
			corerrors.ErrDaemonWorkspaceMismatch)
		assert.ErrorIs(t, VerifyMCPDaemonAttachment(d.info, d.info.WorkspaceRoot, d.info.StorageRoot, d.info.Version+"-skew", u5Protocol),
			corerrors.ErrDaemonVersionMismatch)
		for _, client := range clients {
			client.write(t, "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\"}\n")
			client.response(t, 2)
		}
		u5SessionCount(t, d, 2)
		select {
		case <-d.done:
			t.Fatalf("U5_ATTACHMENT_PEERS: refusal killed the active daemon: %v", d.err)
		default:
		}
		t.Log("U5_WITNESS refusals=2 surviving_peer_pings=2 active_sessions=2")
	})
}

func TestU5_NonExclusiveStatusProbeAndIdleAutoShutdown(t *testing.T) {
	t.Run("cold_status_and_stop_do_not_create_or_lock_lockfile", func(t *testing.T) {
		root := u5Workspace(t)
		_, _, runtimeDir, runtimeRoot, err := openMCPDaemonRuntimeRoot(root)
		require.NoError(t, err)
		require.NoError(t, runtimeRoot.Close())
		status, err := InspectMCPDaemonStatus(root)
		require.NoError(t, err)
		assert.False(t, status.Running)
		_, err = os.Lstat(filepath.Join(runtimeDir, mcpDaemonLockName))
		assert.ErrorIs(t, err, os.ErrNotExist, "U5_COLD_NONEXCLUSIVE: status must not create/acquire the daemon lock")
		// A lock leaf that cannot be opened as a lock makes any attempted open
		// observable, without a timing-dependent acquire/release race.
		root2 := u5Workspace(t)
		_, _, dir2, handle2, err := openMCPDaemonRuntimeRoot(root2)
		require.NoError(t, err)
		require.NoError(t, handle2.Close())
		require.NoError(t, os.Mkdir(filepath.Join(dir2, mcpDaemonLockName), 0o700))
		status, err = InspectMCPDaemonStatus(root2)
		assert.NoError(t, err, "U5_COLD_NONEXCLUSIVE: absent sidecar must return without opening lock leaf")
		assert.False(t, status.Running)
		stopped, err := StopMCPDaemon(t.Context(), root2)
		assert.NoError(t, err, "U5_COLD_STOP_NONEXCLUSIVE: absent sidecar must not open lock leaf")
		assert.False(t, stopped.Stopped)
	})
	t.Run("live_identity_status_does_not_open_lock_leaf", func(t *testing.T) {
		root := u5Workspace(t)
		workspace, storage, dir, handle, err := openMCPDaemonRuntimeRoot(root)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, handle.Close()) })
		require.NoError(t, handle.Mkdir(mcpDaemonLockName, 0o700))
		start, err := processStartTime(os.Getpid())
		require.NoError(t, err)
		exe, err := os.Executable()
		require.NoError(t, err)
		info := MCPDaemonInfo{
			PID: os.Getpid(), ProcessStartTime: start, ExecutablePath: exe,
			WorkspaceRoot: workspace, StorageRoot: storage, Version: version.Resolve(),
			LockPath: filepath.Join(dir, mcpDaemonLockName), StartedAt: time.Now().UTC(),
		}
		// D5 explicitly supports a live identity with no IPC endpoint (legacy
		// lock-only holders). The directory is an open-denial witness, not a
		// fake IPC server or replacement implementation of status.
		require.NoError(t, writeMCPDaemonSidecar(handle, dir, info))
		status, err := InspectMCPDaemonStatus(root)
		require.NoError(t, err, "U5_LIVE_NONEXCLUSIVE: verified live identity must not open the lock leaf")
		assert.True(t, status.Running)
		assert.Equal(t, info.PID, status.PID)
		assert.Equal(t, info, u5ReadInfo(t, root), "live probe must leave the sidecar intact")
		t.Log("U5_WITNESS live_process_identity=true lock_open_denial_leaf=directory status_read_only=true")
	})
	t.Run("crashed_holder_sidecar_is_removed_under_lock", func(t *testing.T) {
		root := u5Workspace(t)
		_, _, dir, handle, err := openMCPDaemonRuntimeRoot(root)
		require.NoError(t, err)
		require.NoError(t, handle.Close())
		u2WriteStaleSidecar(t, dir)
		status, err := InspectMCPDaemonStatus(root)
		require.NoError(t, err)
		assert.False(t, status.Running)
		_, err = os.Lstat(filepath.Join(dir, mcpDaemonSidecarName))
		assert.ErrorIs(t, err, os.ErrNotExist, "U5_STALE_CLEANUP: crashed holder metadata must not survive status")
		release, err := AcquireMCPDaemonLock(t.Context(), root, false)
		require.NoError(t, err, "stale cleanup must release its exclusive cleanup lock")
		require.NoError(t, release())
		t.Log("U5_WITNESS stale_sidecar_removed=true subsequent_lock_acquired=true")
	})
	t.Run("idle_timer_pauses_with_session_and_restarts_on_disconnect", func(t *testing.T) {
		const idle = time.Second
		d := u5StartDaemon(t, idle)
		client := u5Connect(t, d)
		client.response(t, 1)
		u5SessionCount(t, d, 1)
		select {
		case <-d.done:
			t.Fatalf("U5_IDLE_ACTIVE: daemon exited with a live session: %v", d.err)
		case <-time.After(2 * idle):
		}
		client.write(t, "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\"}\n")
		client.response(t, 2)
		disconnected := time.Now()
		client.close(t)
		u5SessionCount(t, d, 0)
		u5Exited(t, d, idle+u5Wait)
		assert.GreaterOrEqual(t, time.Since(disconnected), idle/2, "idle grace must restart, not expire immediately")
		u5StoppedArtifacts(t, d.root)
		t.Log("U5_WITNESS active_survived_two_idle_periods=true idle_disconnect_exit=true")
	})
	t.Run("unattached_daemon_exits_after_boot_idle_window", func(t *testing.T) {
		d := u5StartDaemon(t, time.Second)
		u5SessionCount(t, d, 0)
		u5Exited(t, d, time.Second+u5Wait)
		u5StoppedArtifacts(t, d.root)
	})
}

func u5Workspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if runtime.GOOS != "windows" {
		root = u4TempDir(t)
	}
	storage := filepath.Join(root, ".backlogit")
	t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlogit")
	require.NoError(t, os.Mkdir(storage, 0o700))
	require.NoError(t, config.WriteDefaults(storage))
	ws, err := core.NewWorkspace(t.Context(), root)
	require.NoError(t, err)
	require.NoError(t, ws.Close())
	return root
}

func u5ReadInfo(t *testing.T, root string) MCPDaemonInfo {
	t.Helper()
	_, _, dir, handle, err := openMCPDaemonRuntimeRoot(root)
	require.NoError(t, err)
	defer func() { assert.NoError(t, handle.Close()) }()
	info, err := readMCPDaemonInfoAt(handle, dir)
	require.NoError(t, err)
	return info
}

func u5InfoWithProtocol(t *testing.T, root string) MCPDaemonInfo {
	t.Helper()
	info := u5ReadInfo(t, root)
	info.ProtocolVersion = u5Protocol
	return info
}

func u5StartDaemon(t *testing.T, idle time.Duration) *u5Daemon {
	t.Helper()
	d := &u5Daemon{root: u5Workspace(t), done: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		defer close(d.done)
		d.err = RunMCPDaemon(ctx, d.root, false, idle)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-d.done:
			if d.err != nil && !errors.Is(d.err, context.Canceled) {
				t.Logf("U5 daemon returned: %v", d.err)
			}
		case <-time.After(u5Wait):
			t.Error("U5_DAEMON_JOIN: daemon did not release its lifecycle after cancellation")
		}
	})
	_, _, dir, handle, err := openMCPDaemonRuntimeRoot(d.root)
	require.NoError(t, err)
	defer func() { assert.NoError(t, handle.Close()) }()
	deadline := time.NewTimer(u5Wait)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-d.done:
			t.Fatalf("U5_DAEMON_MUST_SERVE: RunMCPDaemon exited before IPC readiness: %v", d.err)
		case <-deadline.C:
			t.Fatal("U5_DAEMON_READINESS: no published IPC endpoint within bounded startup window")
		case <-tick.C:
			info, readErr := readMCPDaemonInfoAt(handle, dir)
			if readErr == nil && info.Endpoint != "" && info.IPCTransport != "" {
				d.info = info
				require.Equal(t, u5Protocol, info.ProtocolVersion)
				require.Equal(t, os.Getpid(), info.PID, "fixture must remain in-process")
				return d
			}
		}
	}
}

func u5Connect(t *testing.T, d *u5Daemon) *u5Client {
	t.Helper()
	conn, err := DialMCPDaemonIPC(t.Context(), d.info.IPCTransport, d.info.Endpoint, u5Wait)
	require.NoError(t, err, "U5_SESSION_CONNECT: advertised daemon endpoint must accept concurrent streams")
	client := &u5Client{conn: conn, reader: bufio.NewReader(conn)}
	t.Cleanup(func() { client.close(t) })
	header, err := json.Marshal(struct {
		ProtocolVersion string `json:"protocol_version"`
		Command         string `json:"command"`
		WorkspaceRoot   string `json:"workspace_root"`
		StorageRoot     string `json:"storage_root"`
		Version         string `json:"version"`
	}{u5Protocol, "session", d.info.WorkspaceRoot, d.info.StorageRoot, d.info.Version})
	require.NoError(t, err)
	client.write(t, string(header)+"\n"+
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"u5\",\"version\":\"1\"}}}\n")
	return client
}

func (c *u5Client) write(t *testing.T, frame string) {
	t.Helper()
	require.NoError(t, c.conn.SetWriteDeadline(time.Now().Add(u5Wait)))
	n, err := io.WriteString(c.conn, frame)
	require.NoError(t, err, "U5_PIPELINING: header and initialize must be accepted in one write")
	require.Equal(t, len(frame), n)
}

func (c *u5Client) response(t *testing.T, id int) u5Response {
	t.Helper()
	require.NoError(t, c.conn.SetReadDeadline(time.Now().Add(u5Wait)))
	for range 16 {
		line, err := c.reader.ReadBytes('\n')
		require.NoError(t, err, "U5_WIRE_RESPONSE: expected id=%d; handshake reader must retain pipelined bytes", id)
		var response u5Response
		require.NoError(t, json.Unmarshal(line, &response))
		if response.ID == 0 {
			continue // A control acknowledgement or notification is not a response.
		}
		require.Equal(t, id, response.ID, "U5_ROUTING: response belongs to a different request")
		require.Empty(t, response.Error, "U5_WIRE_RESPONSE: JSON-RPC returned an error")
		require.NotEmpty(t, response.Result)
		return response
	}
	t.Fatalf("U5_WIRE_RESPONSE: no response for id=%d after 16 frames", id)
	return u5Response{}
}

func (c *u5Client) close(t *testing.T) {
	t.Helper()
	c.once.Do(func() { assert.NoError(t, c.conn.Close()) })
}

func u5SessionCount(t *testing.T, d *u5Daemon, want int) {
	t.Helper()
	var status MCPDaemonStatus
	var err error
	matched := assert.Eventually(t, func() bool {
		status, err = InspectMCPDaemonStatus(d.root)
		return err == nil && status.Running && status.ActiveSessions == want
	}, u5Wait, 10*time.Millisecond, "U5_LIVE_STATUS: active_sessions must become %d without exclusive probing", want)
	require.True(t, matched, "last status=%+v error=%v", status, err)
	assert.Equal(t, d.info.PID, status.PID)
	assert.Equal(t, d.info.Endpoint, status.Endpoint)
	assert.Equal(t, d.info.IPCTransport, status.IPCTransport)
	assert.Equal(t, u5Protocol, status.ProtocolVersion)
	// Repeated control probes must not register transient MCP sessions.
	for range 3 {
		probe, probeErr := InspectMCPDaemonStatus(d.root)
		require.NoError(t, probeErr)
		require.Equal(t, want, probe.ActiveSessions, "U5_CONTROL_NOT_SESSION: status must not increment the session count")
	}
	t.Logf("U5_WITNESS active_sessions=%d non_session_status_probes=4", want)
}

func u5Exited(t *testing.T, d *u5Daemon, timeout time.Duration) {
	t.Helper()
	select {
	case <-d.done:
		require.NoError(t, d.err, "U5_EXIT: graceful stop/idle exit must succeed")
	case <-time.After(timeout):
		t.Fatalf("U5_EXIT: daemon did not exit within %s", timeout)
	}
}

func u5StoppedArtifacts(t *testing.T, root string) {
	t.Helper()
	_, _, dir, handle, err := openMCPDaemonRuntimeRoot(root)
	require.NoError(t, err)
	defer func() { assert.NoError(t, handle.Close()) }()
	_, err = readMCPDaemonInfoAt(handle, dir)
	assert.ErrorIs(t, err, os.ErrNotExist, "U5_EXIT: daemon must remove its own sidecar")
	status, err := InspectMCPDaemonStatus(root)
	require.NoError(t, err)
	assert.False(t, status.Running)
	release, err := AcquireMCPDaemonLock(t.Context(), root, false)
	require.NoError(t, err, fmt.Sprintf("U5_EXIT: daemon must release lock in %s", dir))
	require.NoError(t, release())
}
