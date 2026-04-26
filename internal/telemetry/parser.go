package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

// telemetryMarker is the old-format log-line segment: single-line JSON with "event" key.
const telemetryMarker = "[telemetry] "

// newFormatMarker is the new-format log-line segment: event type on the marker
// line followed by multi-line pretty-printed JSON on subsequent lines.
const newFormatMarker = "[Telemetry] "

// rawLogEvent is used for a first-pass decode to discover the event type.
type rawLogEvent struct {
	Event string `json:"event"`
}

// rawModelCall mirrors the JSON shape of a cli.model_call payload.
type rawModelCall struct {
	SessionID        string `json:"session_id"`
	RequestID        string `json:"request_id"`
	APIID            string `json:"api_id"`
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
// Supports both old single-line format ([telemetry]) and new multi-line
// format ([Telemetry]). Malformed lines are skipped with a slog debug log
// rather than aborting the parse.
type CopilotCLIParser struct{}

// NewCopilotCLIParser returns a new CopilotCLIParser.
func NewCopilotCLIParser() *CopilotCLIParser {
	return &CopilotCLIParser{}
}

// Parse scans r line-by-line for Copilot CLI telemetry events, calling emit for
// each valid event found. Supports both old single-line [telemetry] format and
// new multi-line [Telemetry] format where JSON is spread across subsequent lines.
func (p *CopilotCLIParser) Parse(r io.Reader, emit func(TelemetryEvent) error) error {
	reader := bufio.NewReader(r)

	// State for multi-line JSON accumulation (new format).
	var (
		accumulating bool
		accEventType string
		accTimestamp time.Time
		accJSON      strings.Builder
		braceDepth   int
	)

	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("telemetry read: %w", readErr)
		}
		isEOF := errors.Is(readErr, io.EOF)
		line := strings.TrimRight(rawLine, "\r\n")

		if line != "" {
			// If we're accumulating multi-line JSON for a new-format event,
			// continue collecting lines until brace depth returns to zero.
			if accumulating {
				accJSON.WriteString(line)
				accJSON.WriteByte('\n')
				braceDepth += countBraces(line)
				if braceDepth <= 0 {
					accumulating = false
					event, err := parseNewFormatPayload(accEventType, accJSON.String())
					if err != nil {
						slog.Debug("skipping malformed new-format telemetry block",
							"err", err, "event_type", accEventType)
					} else {
						event.Timestamp = accTimestamp
						if err := emit(event); err != nil {
							return fmt.Errorf("telemetry emit: %w", err)
						}
					}
				}
			} else if idx := strings.Index(line, newFormatMarker); idx >= 0 {
				// Check for new-format marker: [Telemetry] cli.model_call: / cli.tool_call:
				suffix := line[idx+len(newFormatMarker):]
				eventType := strings.TrimSuffix(strings.TrimSpace(suffix), ":")
				if eventType == "cli.model_call" || eventType == "cli.tool_call" {
					accumulating = true
					accEventType = eventType
					accJSON.Reset()
					braceDepth = 0
					accTimestamp = extractTimestamp(line[:idx])
				}
			} else if idx := strings.Index(line, telemetryMarker); idx >= 0 {
				// Old-format marker: [telemetry] followed by single-line JSON.
				payload := line[idx+len(telemetryMarker):]
				event, err := parseTelemetryPayload(payload)
				if err != nil {
					slog.Debug("skipping malformed telemetry line", "err", err, "line", line)
				} else {
					event.Timestamp = extractTimestamp(line[:idx])
					if err := emit(event); err != nil {
						return fmt.Errorf("telemetry emit: %w", err)
					}
				}
			}
		}

		if isEOF {
			break
		}
	}
	if accumulating {
		slog.Debug("incomplete new-format telemetry block at EOF; last event skipped",
			"event_type", accEventType)
	}
	return nil
}

// extractTimestamp parses the RFC3339Nano timestamp from a log-line prefix.
func extractTimestamp(prefix string) time.Time {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return time.Time{}
	}
	if fields := strings.Fields(prefix); len(fields) > 0 {
		if ts, err := time.Parse(time.RFC3339Nano, fields[0]); err == nil {
			return ts
		}
	}
	return time.Time{}
}

// countBraces returns the net brace depth change for a line (open minus close).
func countBraces(line string) int {
	depth := 0
	inString := false
	escaped := false
	for _, ch := range line {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case ch == '\\' && inString:
			escaped = true
		case ch == '"':
			inString = !inString
		case !inString && ch == '{':
			depth++
		case !inString && ch == '}':
			depth--
		}
	}
	return depth
}

// parseTelemetryPayload decodes a JSON telemetry payload (old format) into a TelemetryEvent.
func parseTelemetryPayload(payload string) (TelemetryEvent, error) {
	var raw rawLogEvent
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return TelemetryEvent{}, fmt.Errorf("decode event type: %w", err)
	}
	switch raw.Event {
	case "cli.model_call":
		return parseModelCallJSON(payload)
	case "cli.tool_call":
		return parseToolCallJSON(payload)
	default:
		return TelemetryEvent{}, fmt.Errorf("unknown event type: %q", raw.Event)
	}
}

// parseNewFormatPayload decodes multi-line JSON for a known event type (new format).
func parseNewFormatPayload(eventType, jsonPayload string) (TelemetryEvent, error) {
	switch eventType {
	case "cli.model_call":
		return parseModelCallJSON(jsonPayload)
	case "cli.tool_call":
		return parseToolCallJSON(jsonPayload)
	default:
		return TelemetryEvent{}, fmt.Errorf("unknown event type: %q", eventType)
	}
}

// parseModelCallJSON unmarshals JSON into a ModelCall event.
func parseModelCallJSON(payload string) (TelemetryEvent, error) {
	var mc rawModelCall
	if err := json.Unmarshal([]byte(payload), &mc); err != nil {
		return TelemetryEvent{}, fmt.Errorf("decode model_call: %w", err)
	}
	// Use api_id as RequestID when present — tool calls reference it via model_call_id.
	requestID := mc.RequestID
	if mc.APIID != "" {
		requestID = mc.APIID
	}
	return TelemetryEvent{
		Kind: EventKindModelCall,
		ModelCall: &ModelCall{
			SessionID:        mc.SessionID,
			RequestID:        requestID,
			Model:            mc.Model,
			PromptTokens:     mc.PromptTokens,
			CompletionTokens: mc.CompletionTokens,
			TotalTokens:      mc.TotalTokens,
			CachedTokens:     mc.CachedTokens,
			DurationMs:       mc.DurationMs,
		},
	}, nil
}

// parseToolCallJSON unmarshals JSON into a ToolCall event.
func parseToolCallJSON(payload string) (TelemetryEvent, error) {
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
}
