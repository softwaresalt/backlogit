package events

import "context"

// SaveMemory persists a key-value pair to memories.json.
//
// Worker: Implement atomic JSON read-modify-write for memories.
func SaveMemory(ctx context.Context, memoriesPath string, key string, summary string) error {
	panic("not implemented: Worker: Implement memory persistence")
}

// CreateCheckpoint writes a timestamped state dump to the checkpoints directory.
//
// Worker: Implement checkpoint file creation.
func CreateCheckpoint(ctx context.Context, checkpointDir string, stateDump string) (string, error) {
	panic("not implemented: Worker: Implement checkpoint creation")
}
