package compatcorpus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	errConcFixturesNotImplemented = errors.New("TODO: implement concurrency fixtures")
	errConcLockBusy               = errors.New("concurrency fixture: lock busy")
)

type concLockMode string

const (
	concLockBlocking concLockMode = "blocking"
	concLockBusy     concLockMode = "busy"
)

type concLockOutcome struct {
	waitedForRelease    bool
	secondAcquireErr    error
	maximumHolders      int
	allGoroutinesJoined bool
}

type concCancellationOutcome struct {
	adapterErrors map[string]error
	report        Report
}

// TestConcurrencyFixtures verifies the deterministic concurrency, cancellation,
// and fail-closed gate fixtures owned by 158.002-T.
func TestConcurrencyFixtures(t *testing.T) {
	t.Run("lock contention", func(t *testing.T) {
		t.Run("blocking waits for release", func(t *testing.T) {
			outcome, err := concExerciseLockContention(context.Background(), concLockBlocking)
			concReportUnimplemented(t, err, "blocking lock-contention fixture")

			if outcome.secondAcquireErr != nil {
				t.Errorf("second blocking Acquire() error = %v, want nil after Release", outcome.secondAcquireErr)
			}
			if !outcome.waitedForRelease {
				t.Error("second blocking Acquire() returned before the first holder released")
			}
			if outcome.maximumHolders != 1 {
				t.Errorf("maximum simultaneous holders = %d, want 1", outcome.maximumHolders)
			}
			if !outcome.allGoroutinesJoined {
				t.Error("blocking lock-contention fixture left a goroutine unjoined")
			}
		})

		t.Run("busy fails with the pinned sentinel", func(t *testing.T) {
			outcome, err := concExerciseLockContention(context.Background(), concLockBusy)
			concReportUnimplemented(t, err, "busy lock-contention fixture")

			if !errors.Is(outcome.secondAcquireErr, errConcLockBusy) {
				t.Errorf(
					"second busy Acquire() error = %v, want errors.Is(_, ErrLockBusy fixture sentinel)",
					outcome.secondAcquireErr,
				)
			}
			if outcome.maximumHolders != 1 {
				t.Errorf("maximum simultaneous holders = %d, want 1", outcome.maximumHolders)
			}
			if !outcome.allGoroutinesJoined {
				t.Error("busy lock-contention fixture left a goroutine unjoined")
			}
		})
	})

	t.Run("context cancellation returns without hanging", func(t *testing.T) {
		boundedCtx, stopBound := context.WithTimeout(context.Background(), time.Second)
		defer stopBound()

		cancelledCtx, cancel := context.WithCancel(boundedCtx)
		cancel()

		type result struct {
			outcome concCancellationOutcome
			err     error
		}
		resultCh := make(chan result, 1)
		go func() {
			outcome, err := concExerciseCancellation(cancelledCtx)
			resultCh <- result{outcome: outcome, err: err}
		}()

		select {
		case got := <-resultCh:
			concReportUnimplemented(t, got.err, "context-cancellation fixture")
			for _, adapterName := range []string{"events_jsonl", "frontmatter", "scanner"} {
				if !errors.Is(got.outcome.adapterErrors[adapterName], context.Canceled) {
					t.Errorf(
						"adapter %q error = %v, want errors.Is(_, context.Canceled)",
						adapterName,
						got.outcome.adapterErrors[adapterName],
					)
				}
			}
			if got.outcome.report.Total != 3 ||
				got.outcome.report.Passed != 0 ||
				got.outcome.report.Failed != 3 {
				t.Errorf(
					"cancelled report counts = total:%d passed:%d failed:%d, want 3/0/3",
					got.outcome.report.Total,
					got.outcome.report.Passed,
					got.outcome.report.Failed,
				)
			}
			for _, entryResult := range got.outcome.report.Results {
				if entryResult.Passed {
					t.Errorf("cancelled entry %q unexpectedly passed", entryResult.EntryID)
				}
				if !strings.Contains(entryResult.GotErr, context.Canceled.Error()) {
					t.Errorf(
						"cancelled entry %q GotErr = %q, want context cancellation",
						entryResult.EntryID,
						entryResult.GotErr,
					)
				}
			}
		case <-boundedCtx.Done():
			t.Fatal("context-cancellation fixture hung past its bounded context")
		}
	})

	t.Run("ambiguous gate input fails closed", func(t *testing.T) {
		entry, err := concAmbiguousGateEntry()
		concReportUnimplemented(t, err, "ambiguous-gate-input fixture")

		if entry.Expect != ExpectRejected {
			t.Errorf("ambiguous entry Expect = %q, want %q", entry.Expect, ExpectRejected)
		}
		if !errors.Is(entry.WantErr, ErrDuplicateKey) {
			t.Errorf("ambiguous entry WantErr = %v, want ErrDuplicateKey", entry.WantErr)
		}

		report := Run(context.Background(), []Entry{entry}, DefaultAdapters())
		if report.Total != 1 || report.Passed != 1 || report.Failed != 0 {
			t.Errorf(
				"ambiguous gate report counts = total:%d passed:%d failed:%d, want 1/1/0",
				report.Total,
				report.Passed,
				report.Failed,
			)
		}
	})
}

func concExerciseLockContention(context.Context, concLockMode) (concLockOutcome, error) {
	return concLockOutcome{}, errConcFixturesNotImplemented
}

func concExerciseCancellation(context.Context) (concCancellationOutcome, error) {
	return concCancellationOutcome{}, errConcFixturesNotImplemented
}

func concAmbiguousGateEntry() (Entry, error) {
	return Entry{}, errConcFixturesNotImplemented
}

func concReportUnimplemented(t *testing.T, err error, fixture string) {
	t.Helper()
	switch {
	case errors.Is(err, errConcFixturesNotImplemented):
		t.Errorf("%s: %v", fixture, err)
	case err != nil:
		t.Fatalf("%s setup failed: %v", fixture, err)
	}
}
