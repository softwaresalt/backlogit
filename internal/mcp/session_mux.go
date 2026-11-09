package mcp

import (
	"context"
	"io"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// MultiSessionConn declares a client session for the future stream dispatcher.
// Session state and lifecycle behavior are not implemented yet.
type MultiSessionConn struct{}

var _ mcpserver.ClientSession = (*MultiSessionConn)(nil)

// SessionID returns an empty identifier until session state is implemented.
func (c *MultiSessionConn) SessionID() string {
	return ""
}

// NotificationChannel returns nil until notification delivery is implemented.
func (c *MultiSessionConn) NotificationChannel() chan<- mcplib.JSONRPCNotification {
	return nil
}

// Initialize is a no-op until session initialization is implemented.
func (c *MultiSessionConn) Initialize() {}

// Initialized returns false until session initialization is implemented.
func (c *MultiSessionConn) Initialized() bool {
	return false
}

// ServeSessionStream declares stream dispatch, which is not implemented yet.
func (s *Server) ServeSessionStream(ctx context.Context, sessionID string, r io.Reader, w io.Writer) error {
	return corerrors.ErrNotImplemented
}

// ActiveSessionCount returns zero until session tracking is implemented.
func (s *Server) ActiveSessionCount() int {
	return 0
}

// CloseAllSessions is a no-op until session lifecycle management is implemented.
func (s *Server) CloseAllSessions() error {
	return nil
}
