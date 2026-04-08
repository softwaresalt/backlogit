package core

import "fmt"

// lockStashFile creates an advisory lock sidecar (.lock) next to path.
// Returns an unlock function and nil on success.
// Returns an error if the lock file already exists (another process holds the lock).
func lockStashFile(path string) (unlock func() error, err error) {
	return nil, fmt.Errorf("not implemented: lockStashFile")
}

// unlockStashFile removes the advisory lock sidecar created by lockStashFile.
// On Windows, os.Remove may fail if the handle is still open; failures are logged
// as warnings and the caller proceeds without error.
func unlockStashFile(path string) error {
	_ = path
	return fmt.Errorf("not implemented: unlockStashFile")
}
