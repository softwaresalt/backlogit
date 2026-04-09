package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Correlate joins model calls, tool calls, session metadata, and backlogit task
// completions into per-session SessionSummary records.
//
// Task completions are detected by scanning .backlogit/events.jsonl for
// status_changed events where new_status == "done" (Plan Review F9).
// Sessions with no task completions report TokensPerTask as nil.
func Correlate(ctx context.Context, events []TelemetryEvent, metas map[string]SessionMeta, workspacePath string) ([]SessionSummary, error) {
	attr := AttributeTool

	// Index events by session.
	type sessionAccum struct {
		totalTokens      int
		promptTokens     int
		completionTokens int
		cachedTokens     int
		modelCalls       int
		toolCalls        int
		tokensByModel    map[string]int
		tokensByServer   map[string]string
	}
	accums := make(map[string]*sessionAccum)

	ensureAccum := func(sessionID string) *sessionAccum {
		if a, ok := accums[sessionID]; ok {
			return a
		}
		a := &sessionAccum{
			tokensByModel:  make(map[string]int),
			tokensByServer: make(map[string]string),
		}
		accums[sessionID] = a
		return a
	}

	for _, e := range events {
		switch e.Kind {
		case EventKindModelCall:
			if e.ModelCall == nil {
				continue
			}
			mc := e.ModelCall
			a := ensureAccum(mc.SessionID)
			a.totalTokens += mc.TotalTokens
			a.promptTokens += mc.PromptTokens
			a.completionTokens += mc.CompletionTokens
			a.cachedTokens += mc.CachedTokens
			a.modelCalls++
			a.tokensByModel[mc.Model] += mc.TotalTokens

		case EventKindToolCall:
			if e.ToolCall == nil {
				continue
			}
			tc := e.ToolCall
			a := ensureAccum(tc.SessionID)
			a.toolCalls++
			server := attr(tc.ToolName)
			a.tokensByServer[server] = server
		}
	}

	// Also ensure sessions that only have metadata still appear if no events.
	for sessionID := range metas {
		ensureAccum(sessionID)
	}

	// Load task completions from .backlogit/events.jsonl.
	completedTasks, err := loadCompletedTasks(workspacePath)
	if err != nil {
		slog.Warn("failed to load completed tasks", "err", err)
		completedTasks = nil
	}

	summaries := make([]SessionSummary, 0, len(accums))
	for sessionID, a := range accums {
		meta := metas[sessionID]
		s := SessionSummary{
			SessionID:        sessionID,
			Branch:           meta.Branch,
			Repository:       meta.Repository,
			TotalTokens:      a.totalTokens,
			PromptTokens:     a.promptTokens,
			CompletionTokens: a.completionTokens,
			CachedTokens:     a.cachedTokens,
			ModelCalls:       a.modelCalls,
			ToolCalls:        a.toolCalls,
			TokensByModel:    a.tokensByModel,
			TokensByServer:   a.tokensByServer,
			CompactionEvents: meta.CompactionEvents,
			CompletedTasks:   completedTasks,
		}
		if len(completedTasks) > 0 && a.totalTokens > 0 {
			tpt := float64(a.totalTokens) / float64(len(completedTasks))
			s.TokensPerTask = &tpt
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// rawEventRecord is used for first-pass decoding of .backlogit/events.jsonl entries.
type rawEventRecord struct {
	EventType string `json:"event_type"`
	NewStatus string `json:"new_status"`
}

// loadCompletedTasks reads .backlogit/events.jsonl and returns a list of artifact IDs
// whose status was changed to "done".
func loadCompletedTasks(workspacePath string) ([]string, error) {
	eventsPath := filepath.Join(workspacePath, ".backlogit", "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open events.jsonl: %w", err)
	}
	defer f.Close()

	var tasks []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec rawEventRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.EventType == "status_changed" && rec.NewStatus == "done" {
			// Extract item ID from the event record.
			var full map[string]interface{}
			if err := json.Unmarshal([]byte(line), &full); err != nil {
				continue
			}
			if id, ok := full["item_id"].(string); ok && id != "" {
				tasks = append(tasks, id)
			}
		}
	}
	return tasks, scanner.Err()
}
