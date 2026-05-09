package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.ItemID == "" {
			e.ItemID = itemID
		}
		result = append(result, e)
	}
	return result, scanner.Err()
}
