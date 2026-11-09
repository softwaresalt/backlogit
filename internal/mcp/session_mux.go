package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const maxSessionFrameBytes = 4 * 1024 * 1024

// MultiSessionConn is an independently cancellable, line-framed SDK session.
// ServeSessionStream owns its workers and never closes the notification channel:
// SDK senders may still hold a reference after session unregistration.
type MultiSessionConn struct {
	sessionID     string
	initialized   atomic.Bool
	notifications chan mcplib.JSONRPCNotification
	writer        io.Writer
	writeMu       sync.Mutex
	inFlightMu    sync.Mutex
	inFlight      map[string]context.CancelFunc
	cancel        context.CancelFunc
	workers       sync.WaitGroup
	done          chan struct{}
	errMu         sync.Mutex
	err           error
	result        error // Published by closing done after all workers exit.
}

var _ mcpserver.ClientSession = (*MultiSessionConn)(nil)

// SessionID returns the identity registered with the SDK.
func (c *MultiSessionConn) SessionID() string {
	return c.sessionID
}

// NotificationChannel returns the session's buffered SDK notification sink.
func (c *MultiSessionConn) NotificationChannel() chan<- mcplib.JSONRPCNotification {
	return c.notifications
}

// Initialize atomically marks this session ready for SDK notifications.
func (c *MultiSessionConn) Initialize() {
	c.initialized.Store(true)
}

// Initialized reports whether this session is ready for SDK notifications.
func (c *MultiSessionConn) Initialized() bool {
	return c.initialized.Load()
}

// ServeSessionStream serves one session and joins all its workers before returning.
// Blocking transports must implement io.Closer so cancellation can interrupt I/O;
// finite, nonblocking readers and writers need not implement it. Stream ownership
// transfers to this call, which closes closable endpoints on rejection or detach.
func (s *Server) ServeSessionStream(ctx context.Context, sessionID string, r io.Reader, w io.Writer) (result error) {
	sess := &MultiSessionConn{}
	registered := false
	defer func() {
		if !registered {
			sess.closeStreams(r, w)
			result = errors.Join(result, sess.err)
		}
	}()
	if s == nil || s.mcp == nil || ctx == nil || strings.TrimSpace(sessionID) == "" || r == nil || w == nil {
		return fmt.Errorf("serve MCP session: invalid server, context, identity or stream: %w", corerrors.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("serve MCP session: %w", err)
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sess = &MultiSessionConn{
		sessionID: sessionID, notifications: make(chan mcplib.JSONRPCNotification, 128),
		writer: w, inFlight: make(map[string]context.CancelFunc),
		cancel: cancel, done: make(chan struct{}),
	}
	if err := s.registerStream(sessionCtx, sess); err != nil {
		return err
	}
	registered = true

	sess.workers.Add(2)
	go func() {
		defer sess.workers.Done()
		<-sessionCtx.Done()
		sess.closeStreams(r, w)
	}()
	go func() {
		defer sess.workers.Done()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case notification := <-sess.notifications:
				if err := sess.writeMessage(sessionCtx, notification); err != nil {
					sess.recordError(err)
					cancel()
					return
				}
			}
		}
	}()

	sess.recordError(s.readSessionFrames(sessionCtx, sess, r))
	cancel()
	sess.workers.Wait()
	s.sessionsMu.Lock()
	s.mcp.UnregisterSession(sessionCtx, sessionID)
	delete(s.sessions, sessionID)
	sess.result = sess.err
	close(sess.done)
	s.sessionsMu.Unlock()
	return sess.result
}

func (s *Server) registerStream(ctx context.Context, sess *MultiSessionConn) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if s.sessionDrainDone != nil {
		return fmt.Errorf("register MCP session during shutdown: %w", io.ErrClosedPipe)
	}
	if err := s.mcp.RegisterSession(ctx, sess); err != nil {
		return fmt.Errorf("register MCP session %q: %w", sess.sessionID, err)
	}
	if s.sessions == nil {
		s.sessions = make(map[string]*MultiSessionConn)
	}
	s.sessions[sess.sessionID] = sess
	return nil
}

func (s *Server) readSessionFrames(ctx context.Context, sess *MultiSessionConn, r io.Reader) error {
	// Reset the budget for every frame, accounting for prefetched bytes. Unlike
	// checking len after an unrestricted ReadBytes, this also bounds a sender
	// that never supplies a newline, without losing prefetched peer frames.
	limited := &io.LimitedReader{R: r}
	reader := bufio.NewReader(limited)
	for ctx.Err() == nil {
		limited.N = int64(maxSessionFrameBytes + 1 - reader.Buffered())
		frame, err := reader.ReadBytes('\n')
		if len(frame) > maxSessionFrameBytes {
			return fmt.Errorf("MCP session frame exceeds 4 MiB: %w", corerrors.ErrValidation)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			if ctx.Err() != nil {
				return nil // Closing an endpoint is the cancellation wakeup.
			}
			return fmt.Errorf("read MCP session frame: %w", err)
		}
		if len(bytes.TrimSpace(frame)) != 0 {
			if dispatchErr := s.dispatchSessionFrame(ctx, sess, frame); dispatchErr != nil {
				return dispatchErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
	return nil
}

type sessionFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id"`
	Params  json.RawMessage `json:"params"`
}

func (s *Server) dispatchSessionFrame(ctx context.Context, sess *MultiSessionConn, frame []byte) error {
	var envelope sessionFrame
	if err := json.Unmarshal(frame, &envelope); err != nil {
		// The SDK owns parse errors and JSON-RPC error formatting.
		sess.startResponse(ctx, func() mcplib.JSONRPCMessage {
			return s.mcp.HandleMessage(s.mcp.WithContext(ctx, sess), frame)
		})
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(frame))
	decoder.UseNumber()
	if err := validateSessionJSONKeys(decoder, 0); err != nil {
		response := mcplib.JSONRPCError{JSONRPC: mcplib.JSONRPC_VERSION}
		response.Error.Code = mcplib.INVALID_REQUEST
		response.Error.Message = err.Error()
		sess.queueMessage(ctx, response)
		return nil
	}
	if len(envelope.ID) == 0 || bytes.Equal(envelope.ID, []byte("null")) {
		if envelope.JSONRPC == mcplib.JSONRPC_VERSION && envelope.Method == "notifications/cancelled" {
			var params struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			if err := json.Unmarshal(envelope.Params, &params); err == nil {
				key := sessionRequestIDKey(params.RequestID)
				sess.inFlightMu.Lock()
				if cancel := sess.inFlight[key]; cancel != nil {
					cancel()
				}
				sess.inFlightMu.Unlock()
			}
			return nil
		}
		sess.startResponse(ctx, func() mcplib.JSONRPCMessage {
			return s.mcp.HandleMessage(s.mcp.WithContext(ctx, sess), frame)
		})
		return nil
	}
	key := sessionRequestIDKey(envelope.ID)
	if key == "" {
		response := mcplib.JSONRPCError{JSONRPC: mcplib.JSONRPC_VERSION}
		response.Error.Code = mcplib.INVALID_REQUEST
		response.Error.Message = "Invalid request ID"
		sess.queueMessage(ctx, response)
		return nil
	}
	// The SDK's base-message decoder uses float64 for IDs. Dispatch every
	// numeric ID with a representable surrogate, retaining the exact wire ID
	// and canonical cancellation key solely in the session dispatcher.
	if strings.HasPrefix(key, "n:") {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(frame, &fields); err != nil {
			return fmt.Errorf("decode MCP session dispatch frame: %w", err)
		}
		for name := range fields {
			if strings.EqualFold(name, "id") {
				fields[name] = json.RawMessage("0")
			}
		}
		var err error
		frame, err = json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("encode MCP session dispatch frame: %w", err)
		}
	}
	if envelope.JSONRPC != mcplib.JSONRPC_VERSION {
		sess.startResponse(ctx, func() mcplib.JSONRPCMessage {
			return sessionWireResponse(s.mcp.HandleMessage(s.mcp.WithContext(ctx, sess), frame), envelope.ID)
		})
		return nil
	}
	if envelope.Method == string(mcplib.MethodToolsCall) {
		var request mcplib.CallToolRequest
		// Leave malformed tool parameters to the SDK. A supplied filesystem
		// path, however, must be contained even when a handler would discard
		// it as outside that tool's corpus. Decode the full SDK request so
		// struct merging and exact-key arguments match the handler's view.
		if err := json.Unmarshal(frame, &request); err == nil {
			if path, ok := request.Params.Arguments["path"].(string); ok && path != "" {
				if err := s.validateSessionPath(path); err != nil {
					sess.queueMessage(ctx, mcplib.JSONRPCResponse{
						JSONRPC: mcplib.JSONRPC_VERSION, ID: envelope.ID, Result: ValidationFailed(err.Error()),
					})
					return nil
				}
			}
		}
	}
	reqCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	sess.inFlightMu.Lock()
	if _, exists := sess.inFlight[key]; exists {
		sess.inFlightMu.Unlock()
		cancel()
		response := mcplib.JSONRPCError{JSONRPC: mcplib.JSONRPC_VERSION, ID: envelope.ID}
		response.Error.Code = mcplib.INVALID_REQUEST
		response.Error.Message = "Request ID already in flight"
		sess.queueMessage(ctx, response)
		return nil
	}
	stopSessionCancel := context.AfterFunc(ctx, cancel)
	if ctx.Err() != nil {
		cancel()
	}
	sess.inFlight[key] = cancel
	sess.inFlightMu.Unlock()
	sess.workers.Add(1)
	go func() {
		defer sess.workers.Done()
		// Unregister when the worker exits so the session cannot retain completed
		// requests, without canceling a successfully completed handler's context.
		defer stopSessionCancel()
		response := s.mcp.HandleMessage(s.mcp.WithContext(reqCtx, sess), frame)
		sess.inFlightMu.Lock()
		delete(sess.inFlight, key)
		sess.inFlightMu.Unlock()
		if err := sess.writeMessage(ctx, sessionWireResponse(response, envelope.ID)); err != nil {
			cancel()
			sess.recordError(err)
			sess.cancel()
		}
	}()
	return nil
}

// validateSessionJSONKeys rejects duplicate decoded names, including escaped
// aliases. Envelope names are case-insensitive in the SDK; argument-map names
// are not. Reject ambiguity before raw params and typed SDK decoders can disagree.
func validateSessionJSONKeys(decoder *json.Decoder, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode MCP session JSON token: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	keys := make(map[string]struct{})
	for decoder.More() {
		if delim == '{' {
			token, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("decode MCP session JSON key: %w", err)
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("MCP session JSON object key: %w", corerrors.ErrValidation)
			}
			if depth == 0 {
				key = strings.ToLower(key)
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate MCP session JSON key: %w", corerrors.ErrValidation)
			}
			keys[key] = struct{}{}
		}
		if err := validateSessionJSONKeys(decoder, depth+1); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("decode MCP session JSON closing token: %w", err)
	}
	return nil
}

func sessionWireResponse(response mcplib.JSONRPCMessage, id json.RawMessage) mcplib.JSONRPCMessage {
	switch message := response.(type) {
	case mcplib.JSONRPCResponse:
		message.ID = id
		return message
	case mcplib.JSONRPCError:
		message.ID = id
		return message
	default:
		return response
	}
}

func (c *MultiSessionConn) queueMessage(ctx context.Context, message mcplib.JSONRPCMessage) {
	c.startResponse(ctx, func() mcplib.JSONRPCMessage { return message })
}

// startResponse is called only by frame intake, before its final worker join.
// Neither SDK handling nor a blocked error response may stall cancellation intake.
func (c *MultiSessionConn) startResponse(ctx context.Context, produce func() mcplib.JSONRPCMessage) {
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		if err := c.writeMessage(ctx, produce()); err != nil {
			c.recordError(err)
			c.cancel()
		}
	}()
}

func (s *Server) validateSessionPath(path string) (result error) {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", string(filepath.Separator))
	if !filepath.IsLocal(path) {
		return fmt.Errorf("MCP tool path must be workspace-relative: %w", corerrors.ErrValidation)
	}
	components := strings.Split(path, string(filepath.Separator))
	for _, component := range components {
		if component == ".." {
			return fmt.Errorf("MCP tool path contains parent traversal: %w", corerrors.ErrValidation)
		}
	}
	absRoot, err := filepath.Abs(s.RootPath)
	if err != nil {
		return fmt.Errorf("resolve MCP workspace root: %w", err)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return fmt.Errorf("open MCP workspace root: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close MCP workspace root: %w", err))
		}
	}()
	relative := "."
	// Inspect original components, not just Clean(path): link/../file must
	// never hide a link that the handler's own path resolver could traverse.
	for _, component := range components {
		relative = filepath.Join(relative, component)
		info, err := root.Lstat(relative)
		if errors.Is(err, os.ErrNotExist) {
			return nil // Existing ancestors are safe; let the handler report absence.
		}
		if err != nil {
			return fmt.Errorf("inspect MCP tool path: %w", err)
		}
		linked, err := core.IsSymlinkOrReparsePoint(info, filepath.Join(absRoot, relative))
		if err != nil {
			return fmt.Errorf("inspect MCP tool path reparse attributes: %w", err)
		}
		if linked {
			return fmt.Errorf("MCP tool path traverses a symlink or reparse point: %w", corerrors.ErrValidation)
		}
	}
	return nil
}

func sessionRequestIDKey(raw json.RawMessage) string {
	var id any // JSON-RPC IDs are the protocol's string-or-number union.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&id); err != nil {
		return ""
	}
	return canonicalRequestIDKey(id)
}

// canonicalRequestIDKey keeps strings distinct from exact decimal numbers.
func canonicalRequestIDKey(id any) string {
	switch value := id.(type) {
	case string:
		return "s:" + value
	case json.Number:
		return canonicalNumericRequestID(string(value))
	case int64:
		return canonicalNumericRequestID(strconv.FormatInt(value, 10))
	case int:
		return canonicalNumericRequestID(strconv.FormatInt(int64(value), 10))
	case float64:
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			return canonicalNumericRequestID(strconv.FormatFloat(value, 'g', -1, 64))
		}
	}
	return ""
}

func canonicalNumericRequestID(number string) string {
	if number == "" || (number[0] != '-' && (number[0] < '0' || number[0] > '9')) || !json.Valid([]byte(number)) {
		return ""
	}
	negative := strings.HasPrefix(number, "-")
	coefficient := strings.TrimPrefix(number, "-")
	var exponent big.Int
	if i := strings.IndexAny(coefficient, "eE"); i >= 0 {
		if _, ok := exponent.SetString(coefficient[i+1:], 10); !ok {
			return ""
		}
		coefficient = coefficient[:i]
	}
	if i := strings.IndexByte(coefficient, '.'); i >= 0 {
		exponent.Sub(&exponent, big.NewInt(int64(len(coefficient)-i-1)))
		coefficient = coefficient[:i] + coefficient[i+1:]
	}
	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		return "n:0"
	}
	digits := strings.TrimRight(coefficient, "0")
	exponent.Add(&exponent, big.NewInt(int64(len(coefficient)-len(digits))))
	if negative {
		digits = "-" + digits
	}
	// Normalize the exponent symbolically: an enormous exponent must not
	// allocate an enormous integer/rational merely to compare request IDs.
	return "n:" + digits + "e" + exponent.String()
}

func (c *MultiSessionConn) writeMessage(ctx context.Context, message mcplib.JSONRPCMessage) error {
	if message == nil || ctx.Err() != nil {
		return nil
	}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode MCP session message: %w", err)
	}
	data = append(data, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if ctx.Err() != nil {
		return nil
	}
	n, err := c.writer.Write(data)
	if ctx.Err() != nil {
		return nil // Transport closure may interrupt an active write.
	}
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write MCP session message: %w", err)
	}
	return nil
}

func (c *MultiSessionConn) recordError(err error) {
	if err == nil {
		return
	}
	c.errMu.Lock()
	c.err = errors.Join(c.err, err)
	c.errMu.Unlock()
}

func (c *MultiSessionConn) closeEndpoint(closer io.Closer) {
	if err := closer.Close(); err != nil &&
		!errors.Is(err, net.ErrClosed) && !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
		c.recordError(fmt.Errorf("close MCP session stream: %w", err))
	}
}

func (c *MultiSessionConn) closeStreams(r io.Reader, w io.Writer) {
	if closer, ok := r.(io.Closer); ok {
		c.closeEndpoint(closer)
	}
	if closer, ok := w.(io.Closer); ok {
		c.closeEndpoint(closer)
	}
}

// ActiveSessionCount reports this server's sessions, including draining workers.
func (s *Server) ActiveSessionCount() int {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return len(s.sessions)
}

// CloseAllSessions stops admission, cancels every session, and joins their drains.
// The terminal shutdown also closes the operation coordinator after sessions
// drain, and is safe to call concurrently or repeatedly.
func (s *Server) CloseAllSessions() error {
	s.sessionsMu.Lock()
	if done := s.sessionDrainDone; done != nil {
		s.sessionsMu.Unlock()
		<-done
		return s.sessionDrainErr
	}
	s.sessionDrainDone = make(chan struct{})
	sessions := make([]*MultiSessionConn, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.sessionsMu.Unlock()

	for _, sess := range sessions {
		sess.cancel()
	}
	var result error
	for _, sess := range sessions {
		<-sess.done
		if sess.result != nil {
			result = errors.Join(result, fmt.Errorf("drain MCP session %q: %w", sess.sessionID, sess.result))
		}
	}
	if s.coordinator != nil {
		result = errors.Join(result, s.coordinator.Close())
	}
	s.sessionsMu.Lock()
	s.sessionDrainErr = result
	close(s.sessionDrainDone)
	s.sessionsMu.Unlock()
	return result
}
