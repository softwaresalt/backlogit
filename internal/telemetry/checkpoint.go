package telemetry

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// HarvestOptions configures a telemetry harvest run.
// Both fields are optional; the zero value performs a full re-harvest identical
// to the prior behaviour.
type HarvestOptions struct {
	// Force ignores any saved checkpoint and re-processes all log files from
	// byte offset 0, overwriting telemetry-sessions.jsonl on completion.
	Force bool
	// Since, when non-nil, excludes events whose timestamp precedes this value.
	// Events with unparseable timestamps are always included (safe default).
	Since *time.Time
}

// HarvestCheckpoint tracks the last-read byte offset per log file to enable
// incremental harvest. Stored as JSON in .backlogit/.telemetry-checkpoint.json.
type HarvestCheckpoint struct {
	// FileOffsets maps a log file base-name to the byte offset from which the
	// next harvest should resume.
	FileOffsets map[string]int64 `json:"file_offsets"`
	// LastHarvest is the wall-clock time of the most recent successful harvest.
	LastHarvest time.Time `json:"last_harvest"`
	// Version is a schema version for forward-compatibility checks.
	Version int `json:"version"`
}

const checkpointFilename = ".telemetry-checkpoint.json"

func checkpointPath(workspacePath string) string {
	return filepath.Join(workspacePath, ".backlogit", checkpointFilename)
}

// LoadCheckpoint reads the harvest checkpoint from
// <workspacePath>/.backlogit/.telemetry-checkpoint.json.
// Returns a zero-value checkpoint when the file does not exist or contains
// malformed JSON — the checkpoint is derived state so a missing file is not an error.
func LoadCheckpoint(workspacePath string) (*HarvestCheckpoint, error) {
	path := checkpointPath(workspacePath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &HarvestCheckpoint{FileOffsets: make(map[string]int64)}, nil
	}
	if err != nil {
		slog.Warn("failed to read telemetry checkpoint; treating as missing", "err", err)
		return &HarvestCheckpoint{FileOffsets: make(map[string]int64)}, nil
	}
	var cp HarvestCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		slog.Warn("corrupt telemetry checkpoint; re-processing all logs", "err", err)
		return &HarvestCheckpoint{FileOffsets: make(map[string]int64)}, nil
	}
	if cp.FileOffsets == nil {
		cp.FileOffsets = make(map[string]int64)
	}
	return &cp, nil
}

// SaveCheckpoint atomically writes cp to
// <workspacePath>/.backlogit/.telemetry-checkpoint.json via temp-file-then-rename.
func SaveCheckpoint(workspacePath string, cp *HarvestCheckpoint) error {
	path := checkpointPath(workspacePath)
	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write checkpoint temp file: %w", err)
	}
	// os.Rename fails on Windows when the destination already exists.
	// Remove the destination first so the swap is safe cross-platform.
	_ = os.Remove(path)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("commit checkpoint: %w", err)
	}
	return nil
}
