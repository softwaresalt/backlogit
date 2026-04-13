package db

import (
	"context"
	"strings"
	"time"
)

// DefaultRetryDelays is the backoff schedule used by RetryWrite. The length
// of the slice determines the maximum number of retries (3 by default, for a
// total of 4 attempts: 1 initial + 3 retries with 1 s / 2 s / 4 s waits).
var DefaultRetryDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
}

// RetryWrite calls fn, retrying on SQLITE_BUSY errors using the backoff
// schedule in DefaultRetryDelays. ctx cancellation stops further retries
// immediately and returns ctx.Err().
func RetryWrite(ctx context.Context, fn func() error) error {
	return RetryWriteWithDelays(ctx, fn, DefaultRetryDelays)
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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delays[attempt]):
		}
	}
}

// isSQLiteBusy reports whether err represents an SQLITE_BUSY condition. The
// check is string-based so it works regardless of how the driver wraps the
// error code.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}
