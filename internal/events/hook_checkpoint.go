package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	backlogiterrors "github.com/backlogit/backlogit/internal/errors"
)

// consumerLocks serialises concurrent SaveCheckpoint calls for the same
// consumer ID within a single process. It maps consumer IDs to *sync.Mutex.
var consumerLocks sync.Map

// getConsumerLock returns (or lazily creates) the per-consumer mutex.
func getConsumerLock(consumerID string) *sync.Mutex {
	mu, _ := consumerLocks.LoadOrStore(consumerID, new(sync.Mutex))
	return mu.(*sync.Mutex)
}

// CheckpointStore persists per-consumer acknowledgement positions as JSON files.
// Files are stored under .backlogit/runtime/hooks/{consumer_id}.checkpoint.json.
// The runtime/hooks directory is ephemeral (gitignored). If the directory or
// files are deleted, consumers restart from seq=0 (idempotent processing assumed).
type CheckpointStore struct {
	dir string
}

// NewCheckpointStore creates a checkpoint store rooted at backlogitDir/runtime/hooks/.
func NewCheckpointStore(backlogitDir string) *CheckpointStore {
	return &CheckpointStore{
		dir: filepath.Join(backlogitDir, "runtime", "hooks"),
	}
}

// checkpointData is the JSON structure persisted for each consumer checkpoint.
type checkpointData struct {
	Seq int64 `json:"seq"`
}

// LoadCheckpoint returns the last acknowledged sequence number for a consumer.
// Returns 0 and no error if no checkpoint file exists (consumer starts from the beginning).
func (s *CheckpointStore) LoadCheckpoint(consumerID string) (int64, error) {
	cpPath := filepath.Join(s.dir, consumerID+".checkpoint.json")
	data, err := os.ReadFile(cpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read checkpoint %q: %w", consumerID, err)
	}
	var cp checkpointData
	if unmarshalErr := json.Unmarshal(data, &cp); unmarshalErr != nil {
		return 0, fmt.Errorf("unmarshal checkpoint %q: %w", consumerID, unmarshalErr)
	}
	return cp.Seq, nil
}

// SaveCheckpoint atomically persists the ack position for a consumer using
// a temp-file-then-rename pattern to prevent partial state.
// Returns an error wrapping ErrValidation if seq is strictly less than the current
// checkpoint (monotonic enforcement: ack positions must never go backward).
// Saving the same seq twice (idempotent ack) is allowed.
// Per-consumer in-process locking prevents concurrent read-modify-write races.
func (s *CheckpointStore) SaveCheckpoint(consumerID string, seq int64) error {
	mu := getConsumerLock(consumerID)
	mu.Lock()
	defer mu.Unlock()

	current, err := s.LoadCheckpoint(consumerID)
	if err != nil {
		return fmt.Errorf("load checkpoint before save: %w", err)
	}
	if seq < current {
		return backlogiterrors.NewValidationError(
			"seq",
			seq,
			fmt.Sprintf("must not regress (current=%d, got=%d)", current, seq),
			backlogiterrors.ErrValidation,
		)
	}

	if mkdirErr := os.MkdirAll(s.dir, 0o755); mkdirErr != nil {
		return fmt.Errorf("create checkpoint dir: %w", mkdirErr)
	}

	cpPath := filepath.Join(s.dir, consumerID+".checkpoint.json")
	tmpPath := cpPath + ".tmp"

	payload, err := json.Marshal(checkpointData{Seq: seq})
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	if writeErr := os.WriteFile(tmpPath, payload, 0o644); writeErr != nil {
		return fmt.Errorf("write checkpoint tmp: %w", writeErr)
	}
	if renameErr := os.Rename(tmpPath, cpPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename checkpoint: %w", renameErr)
	}
	return nil
}
