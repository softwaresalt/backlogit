package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zeroDelays is passed by tests to eliminate timing overhead.
var zeroDelays = []time.Duration{}

func TestRetryWriteWithDelays_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := RetryWriteWithDelays(context.Background(), func() error {
		calls++
		return nil
	}, zeroDelays)

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetryWriteWithDelays_NonBusyErrorNotRetried(t *testing.T) {
	sentinel := errors.New("some other error")
	calls := 0
	err := RetryWriteWithDelays(context.Background(), func() error {
		calls++
		return sentinel
	}, zeroDelays)

	assert.Equal(t, sentinel, err)
	assert.Equal(t, 1, calls, "non-busy errors should not be retried")
}

func TestRetryWriteWithDelays_BusyErrorRetriedAndSucceeds(t *testing.T) {
	busyErr := errors.New("SQLITE_BUSY: database is locked")
	calls := 0
	err := RetryWriteWithDelays(context.Background(), func() error {
		calls++
		if calls < 3 {
			return busyErr
		}
		return nil
	}, []time.Duration{0, 0, 0})

	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetryWriteWithDelays_ExhaustsRetries(t *testing.T) {
	busyErr := errors.New("SQLITE_BUSY: database is locked")
	calls := 0
	err := RetryWriteWithDelays(context.Background(), func() error {
		calls++
		return busyErr
	}, []time.Duration{0, 0})

	assert.ErrorIs(t, err, busyErr)
	assert.Equal(t, 3, calls, "should attempt 1 + 2 retries = 3 total")
}

func TestRetryWriteWithDelays_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	busyErr := errors.New("SQLITE_BUSY")
	calls := 0
	err := RetryWriteWithDelays(ctx, func() error {
		calls++
		return busyErr
	}, []time.Duration{0, 0, 0})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "should not retry when context is cancelled")
}

func TestRetryWriteWithDelays_DatabaseIsLockedMessage(t *testing.T) {
	lockedErr := errors.New("database is locked")
	calls := 0
	err := RetryWriteWithDelays(context.Background(), func() error {
		calls++
		if calls == 1 {
			return lockedErr
		}
		return nil
	}, []time.Duration{0})

	require.NoError(t, err)
	assert.Equal(t, 2, calls, "database is locked should trigger retry")
}

func TestIsSQLiteBusy(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"other error", errors.New("some random error"), false},
		{"SQLITE_BUSY", errors.New("SQLITE_BUSY"), true},
		{"SQLITE_LOCKED", errors.New("SQLITE_LOCKED: shared-cache lock"), true},
		{"database is locked", errors.New("database is locked"), true},
		{"mixed case SQLITE_BUSY with context", errors.New("step: SQLITE_BUSY (5)"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSQLiteBusy(tt.err))
		})
	}
}
