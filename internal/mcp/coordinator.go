package mcp

import (
	"context"
	"fmt"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// OperationKind identifies the admission class of an MCP tool operation.
type OperationKind string

const (
	// OpReadOnly identifies an operation that does not mutate workspace state.
	OpReadOnly OperationKind = "read_only"
	// OpMutating identifies an operation requiring a FIFO mutation turn.
	OpMutating OperationKind = "mutating"
	// OpExclusiveBarrier identifies a mutation that must also drain readers.
	OpExclusiveBarrier OperationKind = "exclusive_barrier"
)

const (
	// DefaultMutationQueueCapacity is the maximum number of pending mutations.
	DefaultMutationQueueCapacity = 32
	// DefaultMutationAdmissionTimeout bounds the wait for a mutation turn.
	DefaultMutationAdmissionTimeout = 30 * time.Second
	// DefaultMaxConcurrentReads matches the SQLite WAL connection pool capacity.
	DefaultMaxConcurrentReads = 4
	// DefaultReadAdmissionTimeout bounds the wait for read admission.
	DefaultReadAdmissionTimeout = 15 * time.Second
)

// CoordinatorConfig defines operation admission limits and timeouts.
type CoordinatorConfig struct {
	MutationQueueCapacity    int
	MutationAdmissionTimeout time.Duration
	MaxConcurrentReads       int
	ReadAdmissionTimeout     time.Duration
}

// BusyErrorResponse describes a recoverable operation admission failure.
type BusyErrorResponse struct {
	// Error is the canonical category "server_busy".
	Error string `json:"error"`
	// Code is one of "mutation_queue_full", "mutation_admission_timeout",
	// "read_admission_timeout", or "request_cancelled".
	Code string `json:"code"`
	// Message describes the admission failure.
	Message string `json:"message"`
	// Retryable indicates whether the caller can retry the operation.
	Retryable bool `json:"retryable"`
	// Remediation provides actionable retry guidance.
	Remediation string `json:"remediation"`
}

// OperationCoordinator declares the runtime-managed MCP operation admission seam.
// Scheduling state and behavior are implemented in U2.
type OperationCoordinator struct{}

// NewOperationCoordinator declares coordinator construction; U2 applies cfg.
func NewOperationCoordinator(cfg CoordinatorConfig) *OperationCoordinator {
	return &OperationCoordinator{}
}

// ClassifyToolOperation returns the conservative initial mutation class.
// U2 implements classification using toolName and the SDK's dynamic argument map.
func ClassifyToolOperation(toolName string, args map[string]any) OperationKind {
	return OpMutating
}

// Execute declares synchronous tool admission and execution, implemented in U2.
func (c *OperationCoordinator) Execute(
	ctx context.Context,
	srv *mcpserver.MCPServer,
	toolName string,
	req mcplib.CallToolRequest,
	next mcpserver.ToolHandlerFunc,
) (*mcplib.CallToolResult, error) {
	return nil, fmt.Errorf("execute coordinated MCP operation: %w", corerrors.ErrNotImplemented)
}

// Close declares coordinator shutdown, implemented in U2.
func (c *OperationCoordinator) Close() error {
	return fmt.Errorf("close MCP operation coordinator: %w", corerrors.ErrNotImplemented)
}
