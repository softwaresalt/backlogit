package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// rawSessionEvent is used for first-pass decoding to discover the event type.
type rawSessionEvent struct {
	EventType string `json:"event_type"`
}

// rawCompactionEvent mirrors the JSON shape of a session.compaction_complete entry.
type rawCompactionEvent struct {
	Timestamp            string `json:"timestamp"`
	PreCompactionTokens  int    `json:"preCompactionTokens"`
	CompactionTokensUsed struct {
		Input       int `json:"input"`
		Output      int `json:"output"`
		CachedInput int `json:"cachedInput"`
	} `json:"compactionTokensUsed"`
}

// ParseSessionEvents reads a session-state events.jsonl stream and extracts
// CompactionEvents from session.compaction_complete entries. Malformed or
// non-compaction lines are skipped. Returns an empty slice (not an error) when
// no compaction events are present.
func ParseSessionEvents(r io.Reader) ([]CompactionEvent, error) {
	var events []CompactionEvent
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var raw rawSessionEvent
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			slog.Debug("skipping malformed session event line", "err", err)
			continue
		}
		if raw.EventType != "session.compaction_complete" {
			continue
		}
		var rc rawCompactionEvent
		if err := json.Unmarshal([]byte(line), &rc); err != nil {
			slog.Debug("skipping malformed compaction event", "err", err)
			continue
		}
		events = append(events, CompactionEvent{
			Timestamp:           rc.Timestamp,
			PreCompactionTokens: rc.PreCompactionTokens,
			InputTokens:         rc.CompactionTokensUsed.Input,
			OutputTokens:        rc.CompactionTokensUsed.Output,
			CachedInputTokens:   rc.CompactionTokensUsed.CachedInput,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session events: %w", err)
	}
	return events, nil
}

// LoadSessionEvents reads all events.jsonl files found directly under sessionStateDir
// (one level deep, one file per session directory) and returns a map of
// sessionID → CompactionEvents. The session ID is inferred from the parent directory name.
func LoadSessionEvents(sessionStateDir string) (map[string][]CompactionEvent, error) {
	result := make(map[string][]CompactionEvent)
	entries, err := os.ReadDir(sessionStateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("read session-state dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		eventsPath := filepath.Join(sessionStateDir, sessionID, "events.jsonl")
		f, err := os.Open(eventsPath)
		if err != nil {
			slog.Debug("session events.jsonl not found", "session_id", sessionID, "err", err)
			continue
		}
		events, err := ParseSessionEvents(f)
		f.Close()
		if err != nil {
			slog.Warn("failed to parse session events", "session_id", sessionID, "err", err)
			continue
		}
		result[sessionID] = events
	}
	return result, nil
}
