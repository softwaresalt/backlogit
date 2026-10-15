package events_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestUCYA5_AppendFastOpenFailureIsWriteNotApplied covers the durable-off append
// path. When the item log cannot be opened nothing was written, so the error
// must carry ErrWriteNotApplied to tell callers the append is safe to retry.
func TestUCYA5_AppendFastOpenFailureIsWriteNotApplied(t *testing.T) {
	logsDir := t.TempDir()
	const itemID = "199.005-T"
	obstructed := events.LogPathForItem(logsDir, itemID)
	require.NoError(t, os.Mkdir(obstructed, 0o755))

	writer := events.NewEventWriter(logsDir, events.WithDurableWrites(false))
	err := writer.AppendEvent(context.Background(), events.Event{
		ItemID:    itemID,
		EventType: "ucya5_probe",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrWriteNotApplied),
		"a durable-off append whose log open fails must satisfy ErrWriteNotApplied, got: %v", err)
	assert.False(t, errors.Is(err, blerrors.ErrWriteIndeterminate),
		"a pre-write open failure must not be reported as indeterminate")
}
