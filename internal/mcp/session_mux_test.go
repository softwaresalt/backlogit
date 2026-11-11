package mcp

// 211.003-T / 211-F / 212-S Wave 6. These behavior tests intentionally compile
// against U3a's declaration-only implementation. No new production seam is
// needed: sessions are observed through SDK contexts and their wire output.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

const u3Wait = 5 * time.Second

// u3Wire preserves frame boundaries, detects concurrent writer entry, and never
// blocks a server goroutine on a test consumer. JSON-RPC is decoded in the test
// goroutine so malformed output is an assertion, not a goroutine panic.
type u3Wire struct {
	mu       sync.Mutex
	pending  []byte
	lines    chan []byte
	writing  atomic.Int32
	overlap  atomic.Bool
	returned atomic.Bool
	late     atomic.Int32
}

func (w *u3Wire) Write(p []byte) (int, error) {
	if w.writing.Add(1) != 1 {
		w.overlap.Store(true)
	}
	defer w.writing.Add(-1)
	if w.returned.Load() {
		w.late.Add(1)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := bytes.Clone(w.pending[:i])
		w.pending = w.pending[i+1:]
		select {
		case w.lines <- line:
		default:
			return 0, fmt.Errorf("U3 wire output exceeded its test buffer")
		}
	}
	return len(p), nil
}

type u3Client struct {
	id     string
	input  *io.PipeWriter
	reader *io.PipeReader
	wire   *u3Wire
	cancel context.CancelFunc
	done   chan struct{}
	err    error // Read only after done closes.
}

type u3Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func u3Server(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	storage := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storage, 0o755))
	require.NoError(t, config.WriteDefaults(storage))
	ws, err := core.NewWorkspace(t.Context(), root)
	require.NoError(t, err)
	s := NewServer(ws)
	s.NoUpdateCheck = true
	t.Cleanup(func() {
		assert.NoError(t, s.CloseAllSessions())
		assert.NoError(t, s.coordinator.Close())
		assert.NoError(t, ws.Close())
	})
	return s
}

// u3RequireDispatcher is a real EOF/lifecycle assertion, not a source probe or
// synthetic RED. Run it before starting pipes or workers in every stream test:
// U3a returns ErrNotImplemented synchronously, so RED cannot be a hung reader.
func u3RequireDispatcher(t *testing.T, s *Server) {
	t.Helper()
	var output bytes.Buffer
	err := s.ServeSessionStream(t.Context(), "u3-eof-probe", strings.NewReader(""), &output)
	require.NoError(t, err, "U3_DISPATCH_REQUIRED: empty stream must detach cleanly; declaration stub is not a dispatcher")
	require.Zero(t, s.ActiveSessionCount(), "EOF probe must not retain a session")
	require.Empty(t, output.String(), "EOF must not manufacture a response")
}

func u3Start(t *testing.T, s *Server, id string) *u3Client {
	t.Helper()
	r, w := io.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	c := &u3Client{
		id: id, input: w, reader: r, cancel: cancel,
		wire: &u3Wire{lines: make(chan []byte, 512)}, done: make(chan struct{}),
	}
	go func() {
		c.err = s.ServeSessionStream(ctx, id, r, c.wire)
		c.wire.returned.Store(true)
		// Unblock a client write even when dispatch fails before reading.
		if err := r.Close(); err != nil {
			c.err = errors.Join(c.err, err)
		}
		close(c.done)
	}()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, w.Close())
		assert.NoError(t, r.Close())
		u3Await(t, c.done, "session cleanup "+id)
		assert.False(t, c.wire.overlap.Load(), "session writer must serialize responses and notifications")
		assert.Zero(t, c.wire.late.Load(), "no writes after ServeSessionStream returns")
	})
	return c
}

func u3Await(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(u3Wait):
		t.Fatalf("U3 cleanup/watchdog: %s did not finish", what)
	}
}

func u3Receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(u3Wait):
		t.Fatalf("U3 watchdog: %s was not observed", what)
		var zero T
		return zero
	}
}

func u3Send(t *testing.T, c *u3Client, frame string) {
	t.Helper()
	sent := make(chan error, 1)
	go func() {
		_, err := io.WriteString(c.input, frame)
		sent <- err
	}()
	select {
	case err := <-sent:
		require.NoError(t, err, "write frame to "+c.id)
	case <-time.After(u3Wait):
		require.NoError(t, c.input.Close())
		err := u3Receive(t, sent, "blocked writer cleanup")
		require.ErrorIs(t, err, io.ErrClosedPipe, "closing the input must unblock the writer")
		t.Fatal("U3 watchdog: dispatcher stopped reading frames")
	}
}

func u3Read(t *testing.T, c *u3Client) u3Message {
	t.Helper()
	line := u3Receive(t, c.wire.lines, "wire frame for "+c.id)
	require.NotEmpty(t, bytes.TrimSpace(line), "blank output is not a JSON-RPC frame")
	var msg u3Message
	require.NoError(t, json.Unmarshal(line, &msg), "one complete JSON object per line: %s", line)
	require.Equal(t, "2.0", msg.JSONRPC)
	return msg
}

func u3Response(t *testing.T, c *u3Client, id string) u3Message {
	t.Helper()
	msg := u3Read(t, c)
	require.Equal(t, id, string(msg.ID), "response must preserve request ID type and value")
	require.Empty(t, msg.Method, "unexpected notification in response slot")
	require.Empty(t, msg.Error, "SDK dispatch error: %s", msg.Error)
	require.NotEmpty(t, msg.Result)
	return msg
}

func u3Initialize(t *testing.T, c *u3Client) {
	t.Helper()
	u3Send(t, c, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"u3-test","version":"1"}}}`+"\n")
	response := u3Response(t, c, "1")
	assert.Contains(t, string(response.Result), `"serverInfo"`, "initialize must use the SDK")
	u3Send(t, c, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
}

// args is the SDK's intentionally dynamic JSON tool argument object.
func u3Call(t *testing.T, c *u3Client, id, name string, args map[string]any) {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	require.NoError(t, err)
	u3Send(t, c, fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%s,\"method\":\"tools/call\",\"params\":%s}\n", id, params))
}

// Decode concrete wire content, not the SDK's []Content interface, which
// encoding/json cannot populate with concrete content implementations.
type u3Result struct {
	Content []mcplib.TextContent `json:"content"`
	IsError bool                 `json:"isError"`
}

func u3ToolResult(t *testing.T, msg u3Message) u3Result {
	t.Helper()
	var result u3Result
	require.NoError(t, json.Unmarshal(msg.Result, &result))
	require.NotEmpty(t, result.Content, "tools/call must return actual tool content")
	return result
}

func u3Detach(t *testing.T, c *u3Client) {
	t.Helper()
	require.NoError(t, c.input.Close())
	u3Await(t, c.done, "EOF detach "+c.id)
	require.NoError(t, c.err, "ordinary client EOF is not a process failure")
}

// TestU3_SessionInitializeStateIsolation gates independent, concurrent init state.
func TestU3_SessionInitializeStateIsolation(t *testing.T) {
	a, b := &MultiSessionConn{}, &MultiSessionConn{}
	require.False(t, a.Initialized())
	require.False(t, b.Initialized())
	a.Initialize()
	require.True(t, a.Initialized(), "U3_INITIALIZE_REQUIRED: Initialize must publish session-local initialized state")
	require.False(t, b.Initialized(), "initializing A must not initialize B")
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			a.Initialize()
			// Exercise the atomic load concurrently with initialization.
			a.Initialized()
		}()
	}
	workers.Wait()
	b.Initialize()
	assert.True(t, a.Initialized())
	assert.True(t, b.Initialized())
}

// TestU3_ConcurrentSessionsOverlappingRequestIDsAndDisconnectSurvival gates R4/R5.
func TestU3_ConcurrentSessionsOverlappingRequestIDsAndDisconnectSurvival(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	s.addTool(mcplib.NewTool("u3_identity"), func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		session := mcpserver.ClientSessionFromContext(ctx)
		if session == nil {
			return nil, errors.New("SDK handler has no client session context")
		}
		return mcplib.NewToolResultText(session.SessionID()), nil
	})
	clients := make([]*u3Client, 3)
	for i := range clients {
		clients[i] = u3Start(t, s, fmt.Sprintf("u3-client-%d", i+1))
		u3Initialize(t, clients[i])
	}
	require.Equal(t, 3, s.ActiveSessionCount())
	for _, c := range clients {
		u3Call(t, c, "2", "u3_identity", nil)
	}
	for _, c := range clients {
		msg := u3Response(t, c, "2")
		require.Contains(t, string(msg.Result), c.id)
		for _, other := range clients {
			if other != c {
				assert.NotContains(t, string(msg.Result), other.id)
			}
		}
		require.NoError(t, s.mcp.SendNotificationToSpecificClient(c.id, "notifications/u3", map[string]any{"owner": c.id}))
	}
	for _, c := range clients {
		msg := u3Read(t, c)
		assert.Equal(t, "notifications/u3", msg.Method)
		assert.Contains(t, string(msg.Params), c.id)
	}
	u3Detach(t, clients[0])
	require.Equal(t, 2, s.ActiveSessionCount())
	require.ErrorIs(t, s.mcp.SendNotificationToSpecificClient(clients[0].id, "notifications/u3", nil), mcpserver.ErrSessionNotFound)
	for _, c := range clients[1:] {
		u3Send(t, c, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")
		assert.Contains(t, string(u3Response(t, c, "1").Result), `"backlogit_create_item"`)
		u3Call(t, c, "2", "backlogit_create_item", map[string]any{"title": c.id, "artifact_type": "feature"})
		assert.False(t, u3ToolResult(t, u3Response(t, c, "2")).IsError, "surviving client can still mutate")
		u3Call(t, c, "1", "backlogit_list_items", nil)
		msg := u3Response(t, c, "1")
		assert.False(t, u3ToolResult(t, msg).IsError)
		assert.Contains(t, string(msg.Result), c.id, "surviving client can read its write")
	}
}

type u3Entered struct {
	ctx     context.Context
	session mcpserver.ClientSession
	key     string
}

func u3BlockingRead(s *Server, entered chan<- u3Entered, release <-chan struct{}) {
	s.addTool(mcplib.NewTool("backlogit_get_version"), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		session := mcpserver.ClientSessionFromContext(ctx)
		if session == nil {
			return nil, errors.New("missing SDK session context")
		}
		key, ok := req.Params.Arguments["key"].(string)
		if !ok {
			return nil, errors.New("U3 fixture requires a string key")
		}
		select {
		case entered <- u3Entered{ctx: ctx, session: session, key: key}:
		case <-ctx.Done():
			return mcplib.NewToolResultError("cancelled before entry"), nil
		}
		select {
		case <-ctx.Done():
			return mcplib.NewToolResultError("cancelled:" + key), nil
		case <-release:
			return mcplib.NewToolResultText("completed:" + key), nil
		}
	})
}

// TestU3_PerSessionNotificationAndCancellationIsolation gates R6, including the
// send-only public channel backed by the dispatcher's internal receive channel.
func TestU3_PerSessionNotificationAndCancellationIsolation(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	entered, release := make(chan u3Entered, 2), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	u3BlockingRead(s, entered, release)
	a, b := u3Start(t, s, "u3-cancel-a"), u3Start(t, s, "u3-cancel-b")
	u3Initialize(t, a)
	u3Initialize(t, b)
	u3Call(t, a, "1", "backlogit_get_version", map[string]any{"key": "a"})
	first := u3Receive(t, entered, "first in-flight handler")
	u3Call(t, b, "1", "backlogit_get_version", map[string]any{"key": "b"})
	second := u3Receive(t, entered, "second in-flight handler")
	require.Equal(t, a.id, first.session.SessionID())
	require.Equal(t, b.id, second.session.SessionID())
	require.True(t, first.session.Initialized())
	require.True(t, second.session.Initialized())
	var sender chan<- mcplib.JSONRPCNotification = first.session.NotificationChannel()
	require.NotNil(t, sender, "initialized session needs a buffered notification channel")
	require.Positive(t, cap(sender), "SDK notification delivery cannot require a synchronous receiver")
	require.NoError(t, s.mcp.SendNotificationToClient(first.ctx, "notifications/u3-private", map[string]any{"owner": a.id}))
	msg := u3Read(t, a)
	require.Equal(t, "notifications/u3-private", msg.Method)
	require.Contains(t, string(msg.Params), a.id)
	u3Send(t, a, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1,"reason":"u3"}}`+"\n")
	u3Await(t, first.ctx.Done(), "only session A request cancellation")
	require.NoError(t, second.ctx.Err(), "same request ID in B must remain live")
	assert.Contains(t, string(u3Response(t, a, "1").Result), "cancelled:a")
	releaseOnce.Do(func() { close(release) })
	// Any notification leaked from A would occupy this response slot and fail.
	assert.Contains(t, string(u3Response(t, b, "1").Result), "completed:b")
	u3Send(t, a, `{"jsonrpc":"2.0","id":2,"method":"ping"}`+"\n")
	u3Response(t, a, "2")
}

// TestU3_CancellationRequestIDTypeAndReuse gates typed canonical cancellation
// keys without referencing U3's as-yet undeclared canonicalRequestIDKey helper.
func TestU3_CancellationRequestIDTypeAndReuse(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	entered, release := make(chan u3Entered, 3), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	u3BlockingRead(s, entered, release)
	c := u3Start(t, s, "u3-typed-ids")
	u3Initialize(t, c)
	u3Call(t, c, "1", "backlogit_get_version", map[string]any{"key": "number"})
	numeric := u3Receive(t, entered, "numeric ID")
	u3Call(t, c, `"1"`, "backlogit_get_version", map[string]any{"key": "string"})
	text := u3Receive(t, entered, "string ID")
	u3Send(t, c, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"unknown"}}`+"\n")
	u3Send(t, c, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1.0}}`+"\n")
	u3Await(t, numeric.ctx.Done(), "canonical numeric cancellation")
	require.NoError(t, text.ctx.Err(), "string 1 and numeric 1 must not alias")
	assert.Contains(t, string(u3Response(t, c, "1").Result), "cancelled:number")
	once.Do(func() { close(release) })
	assert.Contains(t, string(u3Response(t, c, `"1"`).Result), "completed:string")
	// A completed/cancelled ID can be reused without inheriting an old cancel.
	u3Call(t, c, "1", "backlogit_get_version", map[string]any{"key": "reused"})
	reused := u3Receive(t, entered, "reused request")
	require.NoError(t, reused.ctx.Err())
	assert.Contains(t, string(u3Response(t, c, "1").Result), "completed:reused")
}

// TestU3_StreamFramingAndSDKDispatch gates fragmented CRLF, multiple frames,
// frames larger than Scanner's limit, and concurrent responses/notifications.
func TestU3_StreamFramingAndSDKDispatch(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	s.addTool(mcplib.NewTool("u3_payload"), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		payload, ok := req.Params.Arguments["payload"].(string)
		if !ok {
			return nil, errors.New("U3 fixture requires a string payload")
		}
		if err := s.mcp.SendNotificationToClient(ctx, "notifications/u3-payload", map[string]any{"size": len(payload)}); err != nil {
			return nil, err
		}
		return mcplib.NewToolResultText(payload), nil
	})
	c := u3Start(t, s, "u3-framing")
	u3Initialize(t, c)
	payload := strings.Repeat("x", 96*1024)
	frame := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"u3_payload\",\"arguments\":{\"payload\":\"%s\"}}}\r\n", payload)
	u3Send(t, c, frame[:31])
	u3Send(t, c, frame[31:4096])
	u3Send(t, c, frame[4096:]+`{"jsonrpc":"2.0","id":3,"method":"ping"}`+"\n")
	responses, notifications := 0, 0
	for range 3 {
		msg := u3Read(t, c)
		require.Empty(t, msg.Error)
		switch {
		case msg.Method == "notifications/u3-payload":
			notifications++
			assert.Contains(t, string(msg.Params), "98304")
		case string(msg.ID) == "2":
			responses++
			result := u3ToolResult(t, msg)
			require.False(t, result.IsError)
			assert.Contains(t, string(msg.Result), payload)
		case string(msg.ID) == "3":
			responses++
		default:
			t.Fatalf("unexpected or interleaved JSON-RPC frame: %+v", msg)
		}
	}
	assert.Equal(t, 2, responses)
	assert.Equal(t, 1, notifications)
	u3Detach(t, c)
	c.wire.mu.Lock()
	defer c.wire.mu.Unlock()
	assert.Empty(t, c.wire.pending, "every response must end in a newline")
}

// TestU3_OversizeFrameRejectedAndPeerSurvives gates the 4 MiB frame bound.
func TestU3_OversizeFrameRejectedAndPeerSurvives(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	peer := u3Start(t, s, "u3-size-peer")
	u3Initialize(t, peer)
	var output bytes.Buffer
	frame := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"padding":"` + strings.Repeat("x", 4*1024*1024) + "\"}}\n"
	err := s.ServeSessionStream(t.Context(), "u3-oversize", strings.NewReader(frame), &output)
	require.Error(t, err, "a frame larger than 4 MiB must be rejected, not dispatched")
	require.Equal(t, 1, s.ActiveSessionCount(), "oversize frame must detach only its origin")
	u3Send(t, peer, `{"jsonrpc":"2.0","id":2,"method":"ping"}`+"\n")
	u3Response(t, peer, "2")
}

// TestU3_ProcessCoordinatorSharedAcrossStreamsAndInvokeTool gates one global
// admission domain, rather than a new coordinator per stream.
func TestU3_ProcessCoordinatorSharedAcrossStreamsAndInvokeTool(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	entered, release := make(chan string, 3), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	s.addTool(mcplib.NewTool("u3_mutation"), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		key, ok := req.Params.Arguments["key"].(string)
		if !ok {
			return nil, errors.New("U3 fixture requires a string key")
		}
		entered <- key
		if key == "first" {
			select {
			case <-release:
			case <-ctx.Done():
				return mcplib.NewToolResultError("cancelled"), nil
			}
		}
		return mcplib.NewToolResultText(key), nil
	})
	a, b := u3Start(t, s, "u3-global-a"), u3Start(t, s, "u3-global-b")
	u3Initialize(t, a)
	u3Initialize(t, b)
	u3Call(t, a, "2", "u3_mutation", map[string]any{"key": "first"})
	require.Equal(t, "first", u3Receive(t, entered, "first mutation"))
	u3Call(t, b, "2", "u3_mutation", map[string]any{"key": "second"})
	require.Eventually(t, func() bool { return len(s.coordinator.mutationQueue) == 1 }, u3Wait, time.Millisecond,
		"stream B must join the server's existing mutation queue")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	direct := make(chan error, 1)
	go func() {
		req := mcplib.CallToolRequest{}
		req.Params.Arguments = map[string]any{"key": "direct"}
		result, err := s.InvokeTool(ctx, "u3_mutation", req)
		if err == nil && (result == nil || result.IsError) {
			err = errors.New("in-process mutation returned no successful result")
		}
		direct <- err
	}()
	// Cancel before server cleanup joins the coordinator if an assertion fails.
	t.Cleanup(func() {
		cancel()
		once.Do(func() { close(release) })
		require.NoError(t, u3Receive(t, direct, "in-process mutation cleanup"))
	})
	require.Eventually(t, func() bool { return len(s.coordinator.mutationQueue) == 2 }, u3Wait, time.Millisecond,
		"InvokeTool and both streams must share one coordinator")
	select {
	case key := <-entered:
		t.Fatalf("mutation %q ran while first still owned the process turn", key)
	default:
	}
	once.Do(func() { close(release) })
	require.Equal(t, "second", u3Receive(t, entered, "second mutation"))
	require.Equal(t, "direct", u3Receive(t, entered, "direct mutation"))
	assert.Contains(t, string(u3Response(t, a, "2").Result), "first")
	assert.Contains(t, string(u3Response(t, b, "2").Result), "second")
}

// TestU3_DetachCancelsOnlyOriginAndLeavesCoordinatorUsable gates EOF cleanup.
func TestU3_DetachCancelsOnlyOriginAndLeavesCoordinatorUsable(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	entered, release := make(chan u3Entered, 2), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	u3BlockingRead(s, entered, release)
	a, b := u3Start(t, s, "u3-detach-a"), u3Start(t, s, "u3-detach-b")
	u3Initialize(t, a)
	u3Initialize(t, b)
	u3Call(t, a, "2", "backlogit_get_version", map[string]any{"key": "a"})
	first := u3Receive(t, entered, "origin handler")
	u3Call(t, b, "2", "backlogit_get_version", map[string]any{"key": "b"})
	second := u3Receive(t, entered, "peer handler")
	u3Detach(t, a)
	require.ErrorIs(t, first.ctx.Err(), context.Canceled)
	require.NoError(t, second.ctx.Err())
	require.Equal(t, 1, s.ActiveSessionCount())
	once.Do(func() { close(release) })
	assert.Contains(t, string(u3Response(t, b, "2").Result), "completed:b")
	u3Call(t, b, "1", "backlogit_create_item", map[string]any{"title": "after detach", "artifact_type": "feature"})
	assert.False(t, u3ToolResult(t, u3Response(t, b, "1")).IsError, "detach must not close the process coordinator")
	// Reusing the disconnected ID proves SDK UnregisterSession was executed.
	replacement := u3Start(t, s, a.id)
	u3Initialize(t, replacement)
	assert.Equal(t, 2, s.ActiveSessionCount())
}

// TestU3_CloseAllCancelsAndJoinsHandlersAndNotificationWriters gates daemon drain.
func TestU3_CloseAllCancelsAndJoinsHandlersAndNotificationWriters(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	entered, release := make(chan u3Entered, 2), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	s.addTool(mcplib.NewTool("backlogit_get_version"), func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		select {
		case entered <- u3Entered{ctx: ctx, session: mcpserver.ClientSessionFromContext(ctx)}:
		case <-ctx.Done():
			return mcplib.NewToolResultError("cancelled before entry"), nil
		}
		<-ctx.Done()
		// Hold a synchronous frame after cancellation. CloseAll must join it.
		<-release
		return mcplib.NewToolResultError("drained"), nil
	})
	clients := []*u3Client{u3Start(t, s, "u3-close-a"), u3Start(t, s, "u3-close-b")}
	for _, c := range clients {
		u3Initialize(t, c)
		u3Call(t, c, "2", "backlogit_get_version", nil)
	}
	a, b := u3Receive(t, entered, "first close handler"), u3Receive(t, entered, "second close handler")
	closed := make(chan error, 1)
	go func() { closed <- s.CloseAllSessions() }()
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		assert.NoError(t, u3Receive(t, closed, "CloseAll cleanup"))
	})
	u3Await(t, a.ctx.Done(), "CloseAll cancels A")
	u3Await(t, b.ctx.Done(), "CloseAll cancels B")
	select {
	case err := <-closed:
		// Preserve the result for cleanup without starting another CloseAll.
		closed <- err
		t.Fatal("CloseAll returned before synchronous request frames exited")
	default:
	}
	once.Do(func() { close(release) })
	for _, c := range clients {
		u3Await(t, c.done, "CloseAll unblocks reader and joins "+c.id)
	}
	assert.Zero(t, s.ActiveSessionCount())
	assert.NoError(t, s.CloseAllSessions(), "CloseAll must be idempotent")
	for _, c := range clients {
		require.ErrorIs(t, s.mcp.SendNotificationToSpecificClient(c.id, "notifications/u3", nil), mcpserver.ErrSessionNotFound)
		assert.Zero(t, c.wire.late.Load())
	}
}

// TestU3_WorkspaceScopedToolAndResourceContainment gates R17/SC9 through the
// actual path-consuming tool and SDK resource registry, not a test validator.
func TestU3_WorkspaceScopedToolAndResourceContainment(t *testing.T) {
	s := u3Server(t)
	u3RequireDispatcher(t, s)
	outside := t.TempDir() // Outside s.RootPath, but still inside pinned TEMP.
	sentinel := filepath.Join(outside, "private.md")
	const secret = "U3_EXTERNAL_SENTINEL_DO_NOT_RETURN"
	require.NoError(t, os.WriteFile(sentinel, []byte(secret), 0o600))
	link := filepath.Join(s.RootPath, "escape")
	if runtime.GOOS == "windows" {
		out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", link, outside).CombinedOutput()
		require.NoError(t, err, "non-skippable junction fixture: %s", out)
	} else {
		require.NoError(t, os.Symlink(outside, link))
	}
	info, err := os.Lstat(link)
	require.NoError(t, err)
	linked, err := core.IsSymlinkOrReparsePoint(info, link)
	require.NoError(t, err)
	require.True(t, linked, "fixture must exercise a real symlink/reparse escape")
	relativeOutside, err := filepath.Rel(s.RootPath, sentinel)
	require.NoError(t, err)
	c := u3Start(t, s, "u3-contained")
	u3Initialize(t, c)
	for _, path := range []string{"../private.md", relativeOutside, sentinel, "escape/private.md"} {
		u3Call(t, c, "2", "backlogit_docs_lint", map[string]any{"path": path})
		msg := u3Response(t, c, "2")
		require.True(t, u3ToolResult(t, msg).IsError, "tool must reject escape %q", path)
		assert.NotContains(t, string(msg.Result), secret)
		uri, marshalErr := json.Marshal("file://" + filepath.ToSlash(filepath.Join(s.RootPath, path)))
		require.NoError(t, marshalErr)
		u3Send(t, c, fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"resources/read\",\"params\":{\"uri\":%s}}\n", uri))
		resource := u3Read(t, c)
		require.Equal(t, "1", string(resource.ID))
		require.NotEmpty(t, resource.Error, "unregistered external resource URI must be rejected")
		require.Empty(t, resource.Result)
		assert.NotContains(t, string(resource.Error), secret)
	}
	u3Send(t, c, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"backlogit://config"}}`+"\n")
	assert.Contains(t, string(u3Response(t, c, "2").Result), "backlogit://config", "valid workspace resource must remain readable")
	data, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	assert.Equal(t, secret, string(data), "external target must remain unchanged")
}
