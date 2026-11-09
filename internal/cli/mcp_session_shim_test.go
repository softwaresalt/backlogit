package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// 211.006-T / Wave 9: behavior against U5a declarations; no production substitutes.
const u6Wait = 15 * time.Second

type u6Fixture struct {
	root   string
	binary string
}

type u6Log struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *u6Log) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := (64 << 10) - b.text.Len()
	if remaining > len(p) {
		remaining = len(p)
	}
	if remaining > 0 {
		b.text.Write(p[:remaining])
	}
	return len(p), nil
}

func (b *u6Log) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}

type u6Frame struct {
	data []byte
	err  error
}

type u6Stream struct {
	input      io.WriteCloser
	output     io.ReadCloser
	frames     chan u6Frame
	readerDone chan struct{}
	done       chan struct{}
	cancel     context.CancelFunc
	err        error // Published by closing done.
	stderr     *u6Log
	process    *os.Process
}

func TestU6_PrewarmAndLazyAutoStartColdRaceConvergence(t *testing.T) {
	source := u6BuildBinary(t)
	t.Run("prewarm_idempotent_with_two_attached_clients", func(t *testing.T) {
		f := u6NewFixture(t, source, true)
		ctx, cancel := context.WithTimeout(t.Context(), u6Wait)
		defer cancel()
		first, err := PrewarmMCPDaemon(ctx, f.root, false, time.Minute)
		require.NoError(t, err, "U6_PREWARM_START: prewarm must start a pinned background daemon, not return ErrNotImplemented")
		require.True(t, first.Running)
		require.Positive(t, first.PID)
		require.NotEqual(t, os.Getpid(), first.PID, "daemon must be independent of the test host")
		clients := []*u6Stream{u6DirectShim(t, f), u6DirectShim(t, f)}
		u6Initialize(t, clients)
		u6Status(t, f, first.PID, 2)
		second, err := PrewarmMCPDaemon(ctx, f.root, false, time.Minute)
		require.NoError(t, err, "U6_PREWARM_IDEMPOTENT: prewarm must not reject or replace an attached daemon")
		assert.Equal(t, first.PID, second.PID)
		u6RoundTripPair(t, clients)
		u6Status(t, f, first.PID, 2)
	})
	t.Run("lazy_first_client_eof_preserves_daemon_and_later_attach", func(t *testing.T) {
		f := u6NewFixture(t, source, true)
		first := u6DirectShim(t, f)
		u6Initialize(t, []*u6Stream{first})
		started := u6Status(t, f, 0, 1)
		require.NoError(t, first.input.Close())
		u6Join(t, first, "U6_LAZY_EOF: shim must finish cleanly on client EOF")
		u6Status(t, f, started.PID, 0)
		second := u6DirectShim(t, f)
		u6Initialize(t, []*u6Stream{second})
		u6Status(t, f, started.PID, 1)
		second.send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"backlogit_get_version","arguments":{}}}`+"\n")
		u6ToolText(t, second.response(t))
	})
	t.Run("four_mixed_cold_starts_recover_stale_sidecar_to_one_pid", func(t *testing.T) {
		f := u6NewFixture(t, source, true)
		u2WriteStaleSidecar(t, filepath.Join(f.root, ".backlogit", "runtime"))
		start := make(chan struct{})
		type outcome struct {
			status MCPDaemonStatus
			err    error
		}
		results := make(chan outcome, 2)
		ctx, cancel := context.WithTimeout(t.Context(), u6Wait)
		var workers sync.WaitGroup
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				status, err := PrewarmMCPDaemon(ctx, f.root, false, time.Minute)
				results <- outcome{status, err}
			}()
		}
		t.Cleanup(func() { cancel(); workers.Wait() })
		// Two lazy shims and two prewarm calls share the same cold-start barrier.
		clients := []*u6Stream{u6DirectShimAfter(t, f, start), u6DirectShimAfter(t, f, start)}
		close(start)
		var pids []int
		for range 2 {
			select {
			case result := <-results:
				require.NoError(t, result.err, "U6_COLD_CONVERGENCE: all four callers must succeed, never ErrDaemonAlreadyRunning/ErrNotImplemented")
				require.True(t, result.status.Running)
				require.Positive(t, result.status.PID)
				pids = append(pids, result.status.PID)
			case <-ctx.Done():
				t.Fatal("U6_COLD_CONVERGENCE: four cold starts exceeded their bounded readiness window")
			}
		}
		u6Initialize(t, clients)
		require.Equal(t, pids[0], pids[1], "U6_COLD_SINGLETON: concurrent prewarm callers must return one PID")
		status := u6Status(t, f, pids[0], 2)
		info := u5ReadInfo(t, f.root)
		assert.Equal(t, status.PID, info.PID)
		assert.True(t, sameDaemonPath(f.binary, info.ExecutablePath), "U6_PINNED_IDENTITY: singleton must use this fixture's binary")
		u6RoundTripPair(t, clients)
		t.Logf("U6_WITNESS cold_callers=4 prewarm=2 lazy=2 stale_recovered=true daemon_pid=%d", status.PID)
	})
}

func TestU6_MissingPinnedBinaryRefusalAndCLIWiring(t *testing.T) {
	source := u6BuildBinary(t)
	t.Run("missing_workspace_binary_refuses_even_with_real_binary_on_path", func(t *testing.T) {
		f := u6NewFixture(t, source, false)
		// The decoy is a real repo-built binary, not a fake replacement or script.
		pathDir := filepath.Join(f.root, "path-decoy")
		require.NoError(t, os.Mkdir(pathDir, 0o700))
		binaryName := filepath.Base(f.binary)
		u6CopyBinary(t, source, filepath.Join(pathDir, binaryName))
		t.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		resolved, err := exec.LookPath(binaryName)
		require.NoError(t, err)
		require.True(t, sameDaemonPath(filepath.Join(pathDir, binaryName), resolved))
		_, err = os.Stat(f.binary)
		require.ErrorIs(t, err, os.ErrNotExist)
		ctx, cancel := context.WithTimeout(t.Context(), u6Wait)
		defer cancel()
		status, err := PrewarmMCPDaemon(ctx, f.root, false, time.Minute)
		assert.ErrorIs(t, err, corerrors.ErrDaemonBinaryMissing, "U6_PINNED_BINARY_MISSING: prewarm must refuse without PATH fallback")
		assert.False(t, status.Running)
		var output strings.Builder
		err = RunMCPSessionShim(ctx, f.root, false, strings.NewReader(""), &output)
		assert.ErrorIs(t, err, corerrors.ErrDaemonBinaryMissing, "U6_SHIM_BINARY_MISSING: lazy shim must refuse without PATH fallback")
		assert.Empty(t, output.String(), "binary refusal must not emit protocol output")
		probe, err := InspectMCPDaemonStatus(f.root)
		require.NoError(t, err)
		assert.False(t, probe.Running, "U6_NO_PATH_FALLBACK: no daemon may be started from the resolvable decoy")
	})
	t.Run("lifecycle_commands_discoverable_and_unpaired", func(t *testing.T) {
		root := NewRootCommand()
		commands := make(map[string]core.CommandInfo)
		for _, command := range core.DescribeCLICommands(root) {
			commands[command.Command] = command
		}
		operations := loadRegistryOperations(t)
		for _, path := range []string{"backlogit mcp daemon", "backlogit mcp prewarm"} {
			info, exists := commands[path]
			assert.True(t, exists, "U6_CLI_LIFECYCLE_METADATA: missing CLI-only command %s", path)
			if exists {
				assert.NotEmpty(t, info.Short)
				command, _, err := root.Find(strings.Fields(strings.TrimPrefix(path, "backlogit ")))
				require.NoError(t, err)
				assert.Equal(t, path, command.CommandPath())
				assert.NotNil(t, command.RunE, "lifecycle command must be executable, not metadata-only")
			}
			for name, operation := range operations {
				if operation.MCPTool != "" {
					assert.NotEqual(t, path, resolveCLIPath(operation.CLICommand),
						"U6_CLI_ONLY: lifecycle command must not be paired to MCP tool %s (%s)", operation.MCPTool, name)
				}
			}
		}
	})
}

func TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival(t *testing.T) {
	f := u6NewFixture(t, u6BuildBinary(t), true)
	clients := []*u6Stream{u6ChildShim(t, f), u6ChildShim(t, f)}
	u6Initialize(t, clients)
	status := u6Status(t, f, 0, 2)
	for _, client := range clients {
		require.NotEqual(t, client.process.Pid, status.PID,
			"U6_INDEPENDENT_DAEMON: neither stdio client may own the background daemon")
	}
	u6RoundTripPair(t, clients)
	require.NoError(t, clients[0].process.Kill(), "terminate only the first owned client PID")
	select {
	case <-clients[0].done:
	case <-time.After(u6Wait):
		t.Fatal("U6_CLIENT_REAP: terminated client did not exit")
	}
	u6Status(t, f, status.PID, 1)
	clients[1].send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"backlogit_fetch_stash","arguments":{}}}`+"\n")
	text := u6ToolText(t, clients[1].response(t))
	assert.Contains(t, text, "u6-owned-mutation", "U6_PEER_SURVIVES: remaining client must read the persisted mutation after Client 1 is killed")
	ctx, cancel := context.WithTimeout(t.Context(), u6Wait)
	defer cancel()
	stopped, err := StopMCPDaemon(ctx, f.root)
	require.NoError(t, err, "U6_STOP_DAEMON: explicit stop must drain the surviving client")
	assert.True(t, stopped.Stopped)
	assert.Equal(t, status.PID, stopped.PID)
	u6Join(t, clients[1], "U6_STOP_DRAIN: surviving shim must exit cleanly after daemon stop")
	u5StoppedArtifacts(t, f.root)
	t.Logf("U6_WITNESS real_stdio_children=2 overlapping_id=1 daemon_pid=%d killed_client_pid=%d peer_survived=true stopped=true",
		status.PID, clients[0].process.Pid)
}

func u6BuildBinary(t *testing.T) string {
	t.Helper()
	repo := filepath.Dir(filepath.Dir(findRegistryPath(t)))
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "bin"), 0o700))
	binaryName := "backlogit"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(dir, "bin", binaryName)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	var command *exec.Cmd
	target := filepath.Join(repo, "cmd", "backlogit")
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-File",
			filepath.Join(repo, "bin", "tmp", "212-go-env.ps1"), "go", "build", "-o", binary, target)
	} else {
		command = exec.CommandContext(ctx, "go", "build", "-o", binary, target)
	}
	command.Dir = repo
	output, err := command.CombinedOutput()
	require.NoError(t, err, "U6_BINARY_BUILD: real repository CLI must build with the platform environment: %s", output)
	return binary
}

func u6CopyBinary(t *testing.T, source, destination string) {
	t.Helper()
	in, err := os.Open(source)
	require.NoError(t, err)
	defer func() { assert.NoError(t, in.Close()) }()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	require.NoError(t, err)
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	require.NoError(t, copyErr)
	require.NoError(t, closeErr)
}

func u6NewFixture(t *testing.T, source string, pinned bool) u6Fixture {
	t.Helper()
	f := u6Fixture{root: u5Workspace(t)}
	f.binary = filepath.Join(f.root, "bin", filepath.Base(source))
	require.NoError(t, os.Mkdir(filepath.Dir(f.binary), 0o700))
	if pinned {
		u6CopyBinary(t, source, f.binary)
	}
	// All callers and streams register later cleanups and join BEFORE this runs.
	t.Cleanup(func() {
		status, err := InspectMCPDaemonStatus(f.root)
		if !assert.NoError(t, err, "U6_CLEANUP_STATUS: fixture daemon must remain inspectable") || !status.Running {
			return
		}
		info := u5ReadInfo(t, f.root)
		relative, err := filepath.Rel(f.root, info.ExecutablePath)
		if !assert.NoError(t, err) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
			filepath.IsAbs(relative) || !sameDaemonPath(info.WorkspaceRoot, f.root) || info.PID == os.Getpid() {
			t.Error("U6_CLEANUP_OWNERSHIP: refusing to stop a process outside this fixture")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), u6Wait)
		defer cancel()
		_, err = StopMCPDaemon(ctx, f.root)
		assert.NoError(t, err, "U6_CLEANUP_STOP: stop only the daemon belonging to this temp workspace")
		assert.Eventually(t, func() bool {
			return !isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath)
		}, u6Wait, 20*time.Millisecond, "U6_CLEANUP_EXIT: owned daemon must terminate before TempDir removal")
	})
	return f
}

func u6DirectShim(t *testing.T, f u6Fixture) *u6Stream {
	t.Helper()
	start := make(chan struct{})
	close(start)
	return u6DirectShimAfter(t, f, start)
}

func u6DirectShimAfter(t *testing.T, f u6Fixture, start <-chan struct{}) *u6Stream {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	inputR, inputW := io.Pipe()
	outputR, outputW := io.Pipe()
	stream := &u6Stream{input: inputW, output: outputR, done: make(chan struct{}), cancel: cancel, stderr: &u6Log{}}
	go func() {
		defer close(stream.done)
		defer func() { _ = inputR.Close(); _ = outputW.Close() }()
		select {
		case <-ctx.Done():
			stream.err = ctx.Err()
		case <-start:
			stream.err = RunMCPSessionShim(ctx, f.root, false, inputR, outputW)
		}
	}()
	u6ReadStream(t, stream)
	return stream
}

func u6ChildShim(t *testing.T, f u6Fixture) *u6Stream {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	stream := &u6Stream{done: make(chan struct{}), cancel: cancel, stderr: &u6Log{}}
	command := exec.CommandContext(ctx, f.binary, "--cwd", f.root, "--no-update-check", "mcp")
	command.Dir = f.root
	command.Stderr = stream.stderr
	var err error
	stream.input, err = command.StdinPipe()
	require.NoError(t, err)
	stream.output, err = command.StdoutPipe()
	require.NoError(t, err)
	if err := command.Start(); err != nil {
		cancel()
		_ = stream.input.Close()
		_ = stream.output.Close()
		require.NoError(t, err, "U6_CHILD_START: pinned CLI process must launch")
	}
	stream.process = command.Process
	go func() { stream.err = command.Wait(); close(stream.done) }()
	u6ReadStream(t, stream)
	return stream
}

func u6ReadStream(t *testing.T, stream *u6Stream) {
	t.Helper()
	stream.frames = make(chan u6Frame, 32)
	stream.readerDone = make(chan struct{})
	readerCtx, readerCancel := context.WithCancel(context.Background())
	go func() {
		defer close(stream.readerDone)
		reader := bufio.NewReader(stream.output)
		for {
			data, err := reader.ReadBytes('\n')
			select {
			case stream.frames <- u6Frame{data, err}:
			case <-readerCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		stream.cancel()
		readerCancel()
		_ = stream.input.Close()  // Unblock a pending pipe write on assertion RED.
		_ = stream.output.Close() // Unblock the response reader before joining.
		select {
		case <-stream.done:
		case <-time.After(u6Wait):
			t.Error("U6_STREAM_CLEANUP: owned shim failed to join after cancellation")
		}
		select {
		case <-stream.readerDone:
		case <-time.After(u6Wait):
			t.Error("U6_READER_CLEANUP: response reader failed to join")
		}
		if t.Failed() {
			t.Logf("U6 shim stderr: %s", stream.stderr.String())
			select {
			case <-stream.done:
				t.Logf("U6 shim exit: %v", stream.err)
			default:
			}
		}
	})
}

func (stream *u6Stream) send(t *testing.T, frame string) {
	t.Helper()
	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(stream.input, frame)
		written <- err
	}()
	select {
	case err := <-written:
		if err != nil {
			select {
			case <-stream.done:
				require.NoError(t, stream.err, "U6_SHIM_MUST_BRIDGE: shim must consume input, not return ErrNotImplemented; stderr=%s", stream.stderr.String())
			default:
			}
		}
		require.NoError(t, err, "U6_SHIM_WRITE: shim must accept stdio input; stderr=%s", stream.stderr.String())
	case <-stream.done:
		_ = stream.input.Close()
		<-written
		require.NoError(t, stream.err, "U6_SHIM_MUST_BRIDGE: shim exited before consuming input (ErrNotImplemented is RED); stderr=%s", stream.stderr.String())
		t.Fatal("U6_SHIM_MUST_BRIDGE: shim exited without consuming input")
	case <-time.After(u6Wait):
		_ = stream.input.Close()
		<-written
		t.Fatalf("U6_SHIM_WRITE: bounded stdio write timed out; stderr=%s", stream.stderr.String())
	}
}

func (stream *u6Stream) response(t *testing.T) json.RawMessage {
	t.Helper()
	timer := time.NewTimer(u6Wait)
	defer timer.Stop()
	for range 32 {
		select {
		case frame := <-stream.frames:
			require.NoError(t, frame.err, "U6_STDIO_RESPONSE: shim must return a framed response, not exit; stderr=%s", stream.stderr.String())
			var response struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			require.NoError(t, json.Unmarshal(frame.data, &response), "U6_STDIO_FRAMING: stdout must contain JSON-RPC only; stderr=%s", stream.stderr.String())
			if len(response.ID) == 0 {
				continue
			}
			require.JSONEq(t, "1", string(response.ID), "U6_RESPONSE_ROUTING: overlapping request id must remain 1")
			require.Empty(t, response.Error, "U6_RESPONSE_ROUTING: unexpected JSON-RPC error; stderr=%s", stream.stderr.String())
			require.NotEmpty(t, response.Result)
			return response.Result
		case <-timer.C:
			t.Fatalf("U6_STDIO_RESPONSE: response deadline exceeded; stderr=%s", stream.stderr.String())
		}
	}
	t.Fatal("U6_STDIO_RESPONSE: no response after bounded notification frames")
	return nil
}

func u6Initialize(t *testing.T, clients []*u6Stream) {
	t.Helper()
	for _, client := range clients {
		client.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"u6","version":"1"}}}`+"\n")
	}
	for _, client := range clients {
		var result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		}
		require.NoError(t, json.Unmarshal(client.response(t), &result))
		require.Equal(t, "backlogit", result.ServerInfo.Name, "U6_INITIALIZE: real MCP server must answer each stream")
		client.send(t, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n")
	}
}

func u6RoundTripPair(t *testing.T, clients []*u6Stream) {
	t.Helper()
	require.Len(t, clients, 2)
	for _, client := range clients {
		client.send(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n")
	}
	for _, client := range clients {
		var result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		require.NoError(t, json.Unmarshal(client.response(t), &result))
		var names []string
		for _, tool := range result.Tools {
			names = append(names, tool.Name)
		}
		require.Contains(t, names, "backlogit_stash")
		require.Contains(t, names, "backlogit_get_version")
	}
	// Distinct payload shapes on equal IDs expose accidental cross-session routing.
	clients[0].send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"backlogit_stash","arguments":{"kind":"task","priority":"medium","text":"u6-owned-mutation"}}}`+"\n")
	clients[1].send(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"backlogit_get_version","arguments":{}}}`+"\n")
	mutation := u6ToolText(t, clients[0].response(t))
	version := u6ToolText(t, clients[1].response(t))
	require.Contains(t, mutation, "u6-owned-mutation", "U6_MUTATION_ROUTING: mutating response must return only to Client 1")
	require.NotContains(t, version, "u6-owned-mutation", "U6_READ_ROUTING: Client 2 must not receive Client 1's result")
	require.Contains(t, version, "version", "U6_READ_ROUTING: Client 2 must receive the real version result")
}

func u6ToolText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	require.False(t, result.IsError, "U6_TOOL_CALL: tool must succeed: %s", raw)
	require.NotEmpty(t, result.Content)
	var text strings.Builder
	for _, content := range result.Content {
		require.Equal(t, "text", content.Type)
		text.WriteString(content.Text)
	}
	require.NotEmpty(t, text.String())
	return text.String()
}

func u6Status(t *testing.T, f u6Fixture, pid, sessions int) MCPDaemonStatus {
	t.Helper()
	var status MCPDaemonStatus
	var err error
	matched := assert.Eventually(t, func() bool {
		status, err = InspectMCPDaemonStatus(f.root)
		return err == nil && status.Running && status.Endpoint != "" && status.ActiveSessions == sessions
	}, u6Wait, 20*time.Millisecond, "U6_DAEMON_STATUS: independent daemon must report %d sessions", sessions)
	require.True(t, matched, "last status=%+v error=%v", status, err)
	if pid != 0 {
		require.Equal(t, pid, status.PID, "U6_DAEMON_SURVIVAL: client/prewarm lifecycle must not replace the daemon")
	}
	require.Positive(t, status.PID)
	require.Equal(t, u5Protocol, status.ProtocolVersion)
	return status
}

func u6Join(t *testing.T, stream *u6Stream, marker string) {
	t.Helper()
	select {
	case <-stream.done:
		require.NoError(t, stream.err, "%s; stderr=%s", marker, stream.stderr.String())
	case <-time.After(u6Wait):
		t.Fatalf("%s: bounded wait expired; stderr=%s", marker, stream.stderr.String())
	}
}
