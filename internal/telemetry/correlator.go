package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
)

// Correlate joins model calls, tool calls, session metadata, and backlogit task
// completions into per-session SessionSummary records.
//
// Task completions are detected by scanning per-item log files under
// .backlogit/logs/ for status_changed events where delta.to == "done".
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
		tokensByServer   map[string]int // call counts per server; converted to tokens in summary loop
	}
	accums := make(map[string]*sessionAccum)

	ensureAccum := func(sessionID string) *sessionAccum {
		if a, ok := accums[sessionID]; ok {
			return a
		}
		a := &sessionAccum{
			tokensByModel:  make(map[string]int),
			tokensByServer: make(map[string]int),
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
			a.tokensByServer[server]++
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

		// Distribute total tokens across servers proportional to call counts.
		tokensByServer := make(map[string]int, len(a.tokensByServer))
		if a.totalTokens > 0 {
			totalCalls := 0
			for _, c := range a.tokensByServer {
				totalCalls += c
			}
			if totalCalls > 0 {
				for sv, c := range a.tokensByServer {
					tokensByServer[sv] = int(math.Round(float64(a.totalTokens) * float64(c) / float64(totalCalls)))
				}
			}
		}

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
			TokensByServer:   tokensByServer,
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

// rawEventRecord is used for first-pass decoding of per-item log entries
// under .backlogit/logs/<item_id>.jsonl.
type rawEventRecord struct {
	EventType string            `json:"event_type"`
	ItemID    string            `json:"item_id"`
	Delta     map[string]string `json:"delta"`
}

// loadCompletedTasks scans all per-item JSONL log files under
// .backlogit/logs/ and returns the IDs of artifacts whose status was
// changed to "done". Each ID is returned at most once.
//
// Events are stored per-item in .backlogit/logs/<item_id>.jsonl with the
// structure: {"event_type":"status_changed","delta":{"to":"done",...},...}
func loadCompletedTasks(workspacePath string) ([]string, error) {
	logsDir := filepath.Join(workspacePath, ".backlogit", "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read logs dir: %w", err)
	}

	seen := make(map[string]struct{})
	var tasks []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		found, err := doneTasksFromLog(filepath.Join(logsDir, entry.Name()))
		if err != nil {
			slog.Warn("skip unreadable log file", "path", entry.Name(), "err", err)
			continue
		}
		for _, id := range found {
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
				tasks = append(tasks, id)
			}
		}
	}
	return tasks, nil
}

// doneTasksFromLog reads a single per-item JSONL log file and returns the item
// IDs whose status was changed to "done" within that file.
func doneTasksFromLog(logPath string) ([]string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var tasks []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var rec rawEventRecord
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec.EventType == "status_changed" && rec.Delta["to"] == "done" && rec.ItemID != "" {
			tasks = append(tasks, rec.ItemID)
		}
	}
	return tasks, sc.Err()
}
