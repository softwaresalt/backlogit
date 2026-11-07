package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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

type mutationWaiter struct {
	grant     chan chan struct{}
	abandoned chan struct{}
}

// OperationCoordinator admits FIFO mutations and bounded concurrent readers.
// Handlers and their admission resources belong to the synchronous caller.
type OperationCoordinator struct {
	mutationQueue chan *mutationWaiter
	readTokens    chan struct{}
	closed        chan struct{}
	loopDone      chan struct{}
	closeOnce     sync.Once
	cfg           CoordinatorConfig
}

// NewOperationCoordinator starts admission scheduling with defaults for unset limits.
func NewOperationCoordinator(cfg CoordinatorConfig) *OperationCoordinator {
	if cfg.MutationQueueCapacity <= 0 {
		cfg.MutationQueueCapacity = DefaultMutationQueueCapacity
	}
	if cfg.MutationAdmissionTimeout <= 0 {
		cfg.MutationAdmissionTimeout = DefaultMutationAdmissionTimeout
	}
	if cfg.MaxConcurrentReads <= 0 {
		cfg.MaxConcurrentReads = DefaultMaxConcurrentReads
	}
	if cfg.ReadAdmissionTimeout <= 0 {
		cfg.ReadAdmissionTimeout = DefaultReadAdmissionTimeout
	}
	c := &OperationCoordinator{
		mutationQueue: make(chan *mutationWaiter, cfg.MutationQueueCapacity),
		readTokens:    make(chan struct{}, cfg.MaxConcurrentReads),
		closed:        make(chan struct{}),
		loopDone:      make(chan struct{}),
		cfg:           cfg,
	}
	for range cfg.MaxConcurrentReads {
		c.readTokens <- struct{}{}
	}
	go c.schedule()
	return c
}

// ClassifyToolOperation classifies known reads; unknown or malformed flags fail
// closed as mutations without coercing SDK dynamic argument values.
func ClassifyToolOperation(toolName string, args map[string]any) OperationKind {
	switch toolName {
	case "backlogit_sync_index":
		return OpExclusiveBarrier
	case "backlogit_get_item", "backlogit_list_items", "backlogit_search_items",
		"backlogit_query_sql", "backlogit_get_queue", "backlogit_get_shipment",
		"backlogit_list_shipments", "backlogit_get_dependencies", "backlogit_get_links",
		"backlogit_fetch_stash", "backlogit_stash_get", "backlogit_list_checkpoints",
		"backlogit_get_checkpoint", "backlogit_docs_scope", "backlogit_docs_classify",
		"backlogit_docs_lint", "backlogit_get_metadata_catalog", "backlogit_get_version",
		"backlogit_list_templates", "backlogit_get_wit_metadata", "backlogit_list_types",
		"backlogit_poll_hook_events", "backlogit_custody_preview", "backlogit_custody_verify":
		return OpReadOnly
	case "backlogit_doctor", "backlogit_docs_migrate":
		flag := "fix_orphans"
		if toolName == "backlogit_docs_migrate" {
			flag = "apply"
		}
		value, present := args[flag]
		enabled, valid := value.(bool)
		if !present || (valid && !enabled) {
			return OpReadOnly
		}
	case "backlogit_merge_sync", "backlogit_remove_from_shipment",
		"backlogit_move_shipment_item", "backlogit_reorder_shipment",
		"backlogit_reconstitute_shipment", "backlogit_custody_offer", "backlogit_custody_accept":
		if dryRun, ok := args["dry_run"].(bool); ok && dryRun {
			return OpReadOnly
		}
	}
	return OpMutating
}

func (c *OperationCoordinator) schedule() {
	defer close(c.loopDone)
	for {
		select {
		case <-c.closed:
			return
		case waiter := <-c.mutationQueue:
			done := make(chan struct{})
			select {
			case <-c.closed:
				return
			case <-waiter.abandoned:
				continue
			case waiter.grant <- done:
			}
			select {
			case <-done:
			case <-c.closed:
				return
			}
		}
	}
}

func coordinatorBusy(code string) (*mcplib.CallToolResult, error) {
	data, err := json.Marshal(BusyErrorResponse{
		Error: "server_busy", Code: code, Retryable: true,
		Message:     "Operation admission failed: " + code,
		Remediation: "Retry with a fresh request context after pending operations finish.",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal operation admission error: %w", err)
	}
	return mcplib.NewToolResultError(string(data)), nil
}

func coordinatorProgress(ctx context.Context, srv *mcpserver.MCPServer, req mcplib.CallToolRequest, stage string, progress int) {
	if srv == nil || req.Params.Meta == nil || req.Params.Meta.ProgressToken == nil {
		return
	}
	session := mcpserver.ClientSessionFromContext(ctx)
	if session == nil || !session.Initialized() {
		return
	}
	if err := srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
		"progressToken": req.Params.Meta.ProgressToken,
		"progress":      progress, "progressTotal": 2, "message": stage,
	}); err != nil {
		// Progress is advisory: a blocked session cannot abort an admitted write.
		logger.Debug("operation progress unavailable", "stage", stage, "error", err)
	}
}

// Execute admits and executes next in the caller's goroutine, releasing every
// granted turn and acquired token on all return paths, including panic unwinding.
func (c *OperationCoordinator) Execute(
	ctx context.Context,
	srv *mcpserver.MCPServer,
	toolName string,
	req mcplib.CallToolRequest,
	next mcpserver.ToolHandlerFunc,
) (*mcplib.CallToolResult, error) {
	if ctx.Err() != nil {
		return coordinatorBusy("request_cancelled")
	}
	select {
	case <-c.closed:
		return coordinatorBusy("mutation_queue_full")
	default:
	}
	kind := ClassifyToolOperation(toolName, req.Params.Arguments)
	coordinatorProgress(ctx, srv, req, "queued", 0)
	if kind != OpReadOnly {
		waiter := &mutationWaiter{grant: make(chan chan struct{}), abandoned: make(chan struct{})}
		// Never close the queue: senders race shutdown safely through closed.
		select {
		case <-c.closed:
			return coordinatorBusy("mutation_queue_full")
		case c.mutationQueue <- waiter:
		default:
			return coordinatorBusy("mutation_queue_full")
		}
		timer := time.NewTimer(c.cfg.MutationAdmissionTimeout)
		defer timer.Stop()
		defer func() {
			close(waiter.abandoned)
			select {
			case release := <-waiter.grant:
				close(release)
			default:
			}
		}()
		select {
		case releaseTurn := <-waiter.grant:
			defer close(releaseTurn)
		case <-ctx.Done():
			return coordinatorBusy("request_cancelled")
		case <-timer.C:
			return coordinatorBusy("mutation_admission_timeout")
		case <-c.closed:
			return coordinatorBusy("mutation_queue_full")
		}
		// A simultaneous grant and expired admission must not execute next.
		select {
		case <-timer.C:
			return coordinatorBusy("mutation_admission_timeout")
		default:
		}
	}
	acquiredCount := 0
	defer func() {
		for range acquiredCount {
			c.readTokens <- struct{}{}
		}
	}()
	if kind != OpMutating {
		tokenCount := 1
		if kind == OpExclusiveBarrier {
			tokenCount = c.cfg.MaxConcurrentReads
		}
		timer := time.NewTimer(c.cfg.ReadAdmissionTimeout)
		defer timer.Stop()
		for range tokenCount {
			select {
			case <-ctx.Done():
				return coordinatorBusy("request_cancelled")
			case <-timer.C:
				return coordinatorBusy("read_admission_timeout")
			case <-c.closed:
				return coordinatorBusy("mutation_queue_full")
			case <-c.readTokens:
				acquiredCount++
			}
		}
		select {
		case <-timer.C:
			return coordinatorBusy("read_admission_timeout")
		default:
		}
	}
	if ctx.Err() != nil {
		return coordinatorBusy("request_cancelled")
	}
	select {
	case <-c.closed:
		return coordinatorBusy("mutation_queue_full")
	default:
	}
	coordinatorProgress(ctx, srv, req, "started", 1)
	result, err := next(ctx, req)
	coordinatorProgress(ctx, srv, req, "completed", 2)
	return result, err
}

// Close idempotently stops admission and joins the scheduling goroutine.
// Already executing handlers retain their resources until their frame returns.
func (c *OperationCoordinator) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	<-c.loopDone
	return nil
}
