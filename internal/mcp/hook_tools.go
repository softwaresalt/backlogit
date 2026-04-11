package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/events"
)

// PollHookEventsRequest contains validated parameters for backlogit_poll_hook_events.
type PollHookEventsRequest struct {
	ConsumerID string `json:"consumer_id" validate:"required"`
}

// PollHookEventsResponse is the JSON response for backlogit_poll_hook_events.
type PollHookEventsResponse struct {
	Events         []events.HookEvent `json:"events"`
	DerivedSignals []events.HookEvent `json:"derived_signals"`
}

// AckHookEventsRequest contains validated parameters for backlogit_ack_hook_events.
type AckHookEventsRequest struct {
	ConsumerID string `json:"consumer_id" validate:"required"`
	Seq        int64  `json:"seq"         validate:"required,gte=1"`
}

// AckHookEventsResponse is the JSON response for backlogit_ack_hook_events.
type AckHookEventsResponse struct {
	AckedSeq int64 `json:"acked_seq"`
}

// registerHookTools adds backlogit_poll_hook_events and backlogit_ack_hook_events
// to the MCP server.
func (s *Server) registerHookTools() {
	s.addTool(
		mcplib.NewTool("backlogit_poll_hook_events",
			mcplib.WithDescription("Poll for unacknowledged hook events since the consumer's last checkpoint"),
			mcplib.WithString("consumer_id",
				mcplib.Required(),
				mcplib.Description("Agent consumer ID (e.g. 'stage', 'ship')"),
			),
		),
		s.handlePollHookEvents,
	)
	s.addTool(
		mcplib.NewTool("backlogit_ack_hook_events",
			mcplib.WithDescription("Acknowledge processing of hook events up to and including seq"),
			mcplib.WithString("consumer_id",
				mcplib.Required(),
				mcplib.Description("Agent consumer ID"),
			),
			mcplib.WithNumber("seq",
				mcplib.Required(),
				mcplib.Description("Highest sequence number processed"),
			),
		),
		s.handleAckHookEvents,
	)
}

// handlePollHookEvents implements the backlogit_poll_hook_events MCP tool.
// It returns events with seq strictly greater than the consumer's last checkpoint.
func (s *Server) handlePollHookEvents(
	ctx context.Context,
	request mcplib.CallToolRequest,
) (*mcplib.CallToolResult, error) {
	consumerID, _ := request.Params.Arguments["consumer_id"].(string)
	if consumerID == "" {
		return ValidationFailed("consumer_id is required"), nil
	}

	cs := events.NewCheckpointStore(s.backlogitDir())
	result, err := events.PollHookEvents(ctx, s.HookEvents, cs, consumerID, nil)
	if err != nil {
		return InternalError(fmt.Sprintf("poll hook events: %v", err)), nil
	}

	resp := PollHookEventsResponse{
		Events:         result.Events,
		DerivedSignals: result.DerivedSignals,
	}
	// Ensure nil slices marshal as empty arrays for consistent JSON.
	if resp.Events == nil {
		resp.Events = []events.HookEvent{}
	}
	if resp.DerivedSignals == nil {
		resp.DerivedSignals = []events.HookEvent{}
	}
	return toolResultJSON(resp)
}

// handleAckHookEvents implements the backlogit_ack_hook_events MCP tool.
// It advances the consumer's checkpoint to seq.
func (s *Server) handleAckHookEvents(
	ctx context.Context,
	request mcplib.CallToolRequest,
) (*mcplib.CallToolResult, error) {
	consumerID, _ := request.Params.Arguments["consumer_id"].(string)
	if consumerID == "" {
		return ValidationFailed("consumer_id is required"), nil
	}

	seqRaw, seqPresent := request.Params.Arguments["seq"]
	if !seqPresent || seqRaw == nil {
		return ValidationFailed("seq is required"), nil
	}
	seqFloat, ok := seqRaw.(float64)
	if !ok {
		return ValidationFailed("seq must be a number"), nil
	}
	seq := int64(seqFloat)

	cs := events.NewCheckpointStore(s.backlogitDir())
	if ackErr := events.AckHookEvents(ctx, cs, consumerID, seq); ackErr != nil {
		return InternalError(fmt.Sprintf("ack hook events: %v", ackErr)), nil
	}
	return toolResultJSON(AckHookEventsResponse{AckedSeq: seq})
}
