package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveMemory persists a key-value pair to memories.json via atomic read-modify-write.
func SaveMemory(_ context.Context, memoriesPath string, key string, summary string) error {
	memories := make(map[string]string)
	if data, err := os.ReadFile(memoriesPath); err == nil {
		_ = json.Unmarshal(data, &memories)
	}
	memories[key] = summary
	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memories: %w", err)
	}
	tmp := memoriesPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write memories tmp: %w", err)
	}
	return os.Rename(tmp, memoriesPath)
}

// CreateCheckpoint writes a timestamped state dump to the checkpoints directory.
func CreateCheckpoint(_ context.Context, checkpointDir string, stateDump string) (string, error) {
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return "", fmt.Errorf("create checkpoint dir: %w", err)
	}
	name := fmt.Sprintf("checkpoint-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(checkpointDir, name)
	if err := os.WriteFile(path, []byte(stateDump), 0o644); err != nil {
		return "", fmt.Errorf("write checkpoint: %w", err)
	}
	return path, nil
}
