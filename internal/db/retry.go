package db

import (
	"context"
	"strings"
	"time"
)

// defaultRetryDelays is the backoff schedule used by RetryWrite: 1 s / 2 s / 4 s.
// The slice length determines the maximum number of retries (3), for a total of
// 4 attempts (1 initial + 3 retries). Unexported to prevent external mutation,
// which would affect all concurrent RetryWrite calls globally.
var defaultRetryDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
}

// RetryWrite calls fn, retrying on SQLITE_BUSY and SQLITE_LOCKED errors using
// the default backoff schedule (1 s / 2 s / 4 s, up to 3 retries). ctx
// cancellation stops further retries immediately and returns ctx.Err().
func RetryWrite(ctx context.Context, fn func() error) error {
	return RetryWriteWithDelays(ctx, fn, defaultRetryDelays)
}

// RetryWriteWithDelays is the testable inner implementation of RetryWrite.
// Production code should call RetryWrite; tests pass a zero-duration slice to
// eliminate timing overhead.
func RetryWriteWithDelays(ctx context.Context, fn func() error, delays []time.Duration) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil || !isSQLiteBusy(err) {
			return err
		}
		if attempt >= len(delays) {
			return err
		}
		// Fast-path: if ctx is already cancelled do not enter the select at all.
		// This matters when delay is zero and both channels would be simultaneously
		// ready — Go picks randomly, so the explicit check guarantees cancellation wins.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		timer := time.NewTimer(delays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// isSQLiteBusy reports whether err represents a retryable SQLite lock
// condition. The check is string-based so it works regardless of how the
// driver wraps the error code.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "SQLITE_LOCKED") ||
		strings.Contains(msg, "database is locked")
}
