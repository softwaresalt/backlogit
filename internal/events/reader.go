package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// TailEvents reads the most recent events for a specific item from events.jsonl.
func TailEvents(_ context.Context, path string, itemID string, limit int) ([]Event, error) {
	all, err := ReadAllEvents(context.Background(), path)
	if err != nil {
		return nil, err
	}
	var filtered []Event
	for _, e := range all {
		if e.ItemID == itemID {
			filtered = append(filtered, e)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}

// ReadAllEvents reads all events from a JSONL file.
func ReadAllEvents(_ context.Context, path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var result []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		result = append(result, e)
	}
	return result, scanner.Err()
}
