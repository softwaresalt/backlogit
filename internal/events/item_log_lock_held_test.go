package events

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItemLogCrossProcessLockHeld(t *testing.T) {
	root := t.TempDir()
	locksRoot := filepath.Join(root, "locks")
	logsDir := filepath.Join(root, "logs")
	otherLogsDir := filepath.Join(root, "other-logs")
	const itemID = "001-F"

	heldCtx, unlock, err := LockItemLogCrossProcess(context.Background(), locksRoot, logsDir, itemID)
	require.NoError(t, err)
	released := false
	t.Cleanup(func() {
		if !released {
			unlock()
		}
	})

	tests := []struct {
		name    string
		ctx     context.Context
		logsDir string
		itemID  string
		want    bool
	}{
		{"fresh context holds nothing", context.Background(), logsDir, itemID, false},
		{"held item in same logs dir", heldCtx, logsDir, itemID, true},
		{"different item is not held", heldCtx, logsDir, "002-F", false},
		{"different logs dir is not held", heldCtx, otherLogsDir, itemID, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ItemLogCrossProcessLockHeld(tt.ctx, tt.logsDir, tt.itemID))
		})
	}

	unlock()
	released = true
}
