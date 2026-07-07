package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// TailEvents reads the most recent events for a specific item from its JSONL log file.
func TailEvents(_ context.Context, logsDir string, itemID string, limit int) ([]Event, error) {
	all, err := ReadAllEvents(context.Background(), logsDir, itemID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

// ReadAllEvents reads all events from a work item's JSONL log file.
func ReadAllEvents(_ context.Context, logsDir string, itemID string) ([]Event, error) {
	path := LogPathForItem(logsDir, itemID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open item log file: %w", err)
	}
	defer f.Close()

	var result []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		e, ok, perr := ParseEventLine(scanner.Text(), itemID)
		if perr != nil {
			slog.Warn("skipping malformed event log line",
				"item_id", itemID, "path", path, "line", lineNum, "error", perr)
			continue
		}
		if !ok {
			continue
		}
		result = append(result, e)
	}
	return result, scanner.Err()
}

// ParseEventLine parses a single JSONL line from an item event log into an
// Event under the unified malformed-line policy shared by ReadAllEvents
// (doctor fallback) and rehydration's parseItemLogFile (SQLite projection):
//
//   - blank / whitespace-only line -> ok=false, err=nil   (skip silently)
//   - malformed JSON               -> ok=false, err!=nil  (caller logs + skips)
//   - valid event                  -> ok=true,  err=nil
//
// itemID backfills Event.ItemID when the serialized line omits it. The raw
// json.Unmarshal error is returned unwrapped: it is consumed immediately by
// each caller's structured slog.Warn (item + path + 1-based line), which
// supplies richer context than a wrapped string. Centralizing the parse/skip
// decision here keeps the two independent callers from silently re-diverging.
func ParseEventLine(line, itemID string) (Event, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{}, false, nil
	}
	var e Event
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return Event{}, false, err
	}
	if e.ItemID == "" {
		e.ItemID = itemID
	}
	return e, true, nil
}
