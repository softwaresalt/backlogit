package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// memoriesMu serializes concurrent SaveMemory calls on the same process.
var memoriesMu sync.Mutex

// SaveMemory persists a key-value pair to memories.json via atomic read-modify-write.
// A process-level mutex prevents lost updates from concurrent callers.
func SaveMemory(_ context.Context, memoriesPath string, key string, summary string) error {
	memoriesMu.Lock()
	defer memoriesMu.Unlock()

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
// If the state dump contains a V1 schema (schema_version=1), it is parsed and
// validated before writing. Missing created_at, updated_at, and status fields
// are auto-populated. Legacy (non-V1) dumps are written as-is with atomic writes.
func CreateCheckpoint(_ context.Context, checkpointDir string, stateDump string) (string, error) {
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return "", fmt.Errorf("create checkpoint dir: %w", err)
	}
	name := fmt.Sprintf("checkpoint-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(checkpointDir, name)

	data := []byte(stateDump)

	// Probe for V1 schema.
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.SchemaVersion == 1 {
		cp, err := ParseCheckpoint(data)
		if err != nil {
			// Preserve the ErrCheckpointCorrupt sentinel from ParseCheckpoint.
			return "", fmt.Errorf("parse v1 checkpoint: %w", err)
		}
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = time.Now().UTC()
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = time.Now().UTC()
		}
		if cp.Status == "" {
			cp.Status = "active"
		}
		if err := ValidateCheckpoint(cp); err != nil {
			return "", err
		}
		var marshalErr error
		data, marshalErr = json.Marshal(cp)
		if marshalErr != nil {
			return "", fmt.Errorf("marshal v1 checkpoint: %w", marshalErr)
		}
	}

	if err := syncWriteFileAtomic(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write checkpoint: %w", err)
	}
	return path, nil
}
