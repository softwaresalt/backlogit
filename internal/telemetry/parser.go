package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// telemetryMarker is the log-line segment that identifies a telemetry event.
const telemetryMarker = "[telemetry] "

// rawLogEvent is used for a first-pass decode to discover the event type.
type rawLogEvent struct {
	Event string `json:"event"`
}

// rawModelCall mirrors the JSON shape of a cli.model_call payload.
type rawModelCall struct {
	SessionID        string `json:"session_id"`
	RequestID        string `json:"request_id"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens_count"`
	CompletionTokens int    `json:"completion_tokens_count"`
	TotalTokens      int    `json:"total_tokens_count"`
	CachedTokens     int    `json:"cached_tokens_count"`
	DurationMs       int    `json:"duration_ms"`
}

// rawToolCall mirrors the JSON shape of a cli.tool_call payload.
type rawToolCall struct {
	SessionID   string `json:"session_id"`
	ModelCallID string `json:"model_call_id"`
	ToolName    string `json:"tool_name"`
	ResultType  string `json:"result_type"`
	DurationMs  int    `json:"duration_ms"`
}

// CopilotCLIParser parses Copilot CLI process log files by scanning
// line-by-line for cli.model_call and cli.tool_call JSON telemetry events.
// Malformed lines are skipped with a slog debug log rather than aborting the parse.
type CopilotCLIParser struct{}

// NewCopilotCLIParser returns a new CopilotCLIParser.
func NewCopilotCLIParser() *CopilotCLIParser {
	return &CopilotCLIParser{}
}

// Parse scans r line-by-line for Copilot CLI telemetry events, calling emit for
// each valid event found. Lines that do not contain the [telemetry] marker or
// that contain malformed JSON are skipped with a slog debug log.
func (p *CopilotCLIParser) Parse(r io.Reader, emit func(TelemetryEvent) error) error {
	scanner := bufio.NewScanner(r)
	// Copilot CLI log lines can contain large JSON payloads. The default 64KB
	// scanner buffer is too small; set a 1MB buffer to avoid ErrTooLong.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, telemetryMarker)
		if idx < 0 {
			continue
		}
		payload := line[idx+len(telemetryMarker):]
		event, err := parseTelemetryPayload(payload)
		if err != nil {
			slog.Debug("skipping malformed telemetry line", "err", err, "line", line)
			continue
		}
		if err := emit(event); err != nil {
			return fmt.Errorf("telemetry emit: %w", err)
		}
	}
	return scanner.Err()
}

// parseTelemetryPayload decodes a JSON telemetry payload into a TelemetryEvent.
func parseTelemetryPayload(payload string) (TelemetryEvent, error) {
	var raw rawLogEvent
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return TelemetryEvent{}, fmt.Errorf("decode event type: %w", err)
	}
	switch raw.Event {
	case "cli.model_call":
		var mc rawModelCall
		if err := json.Unmarshal([]byte(payload), &mc); err != nil {
			return TelemetryEvent{}, fmt.Errorf("decode model_call: %w", err)
		}
		return TelemetryEvent{
			Kind: EventKindModelCall,
			ModelCall: &ModelCall{
				SessionID:        mc.SessionID,
				RequestID:        mc.RequestID,
				Model:            mc.Model,
				PromptTokens:     mc.PromptTokens,
				CompletionTokens: mc.CompletionTokens,
				TotalTokens:      mc.TotalTokens,
				CachedTokens:     mc.CachedTokens,
				DurationMs:       mc.DurationMs,
			},
		}, nil
	case "cli.tool_call":
		var tc rawToolCall
		if err := json.Unmarshal([]byte(payload), &tc); err != nil {
			return TelemetryEvent{}, fmt.Errorf("decode tool_call: %w", err)
		}
		return TelemetryEvent{
			Kind: EventKindToolCall,
			ToolCall: &ToolCall{
				SessionID:   tc.SessionID,
				ModelCallID: tc.ModelCallID,
				ToolName:    tc.ToolName,
				ResultType:  tc.ResultType,
				DurationMs:  tc.DurationMs,
			},
		}, nil
	default:
		return TelemetryEvent{}, fmt.Errorf("unknown event type: %q", raw.Event)
	}
}
