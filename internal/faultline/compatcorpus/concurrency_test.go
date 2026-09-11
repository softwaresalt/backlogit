package compatcorpus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

var errConcLockBusy = errors.New("concurrency fixture: lock busy")

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
	lockerError   error
	report        Report
}

type concFixtureLocker struct {
	mode      concLockMode
	token     chan struct{}
	attempted chan struct{}

	mu             sync.Mutex
	holders        int
	maximumHolders int
}

func concNewFixtureLocker(mode concLockMode) *concFixtureLocker {
	locker := &concFixtureLocker{
		mode:      mode,
		token:     make(chan struct{}, 1),
		attempted: make(chan struct{}, 2),
	}
	locker.token <- struct{}{}
	return locker
}

func (l *concFixtureLocker) Acquire(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	l.attempted <- struct{}{}

	switch l.mode {
	case concLockBlocking:
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire fixture lock: %w", ctx.Err())
		case <-l.token:
			l.recordAcquire()
			return nil
		}
	case concLockBusy:
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire fixture lock: %w", ctx.Err())
		case <-l.token:
			l.recordAcquire()
			return nil
		default:
			return errConcLockBusy
		}
	default:
		return fmt.Errorf("acquire fixture lock: unsupported mode %q", l.mode)
	}
}

func (l *concFixtureLocker) Release() {
	l.mu.Lock()
	l.holders--
	l.mu.Unlock()
	l.token <- struct{}{}
}

func (l *concFixtureLocker) recordAcquire() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.holders++
	if l.holders > l.maximumHolders {
		l.maximumHolders = l.holders
	}
}

func (l *concFixtureLocker) maxHolders() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.maximumHolders
}

// TestConcurrencyFixtures verifies the deterministic concurrency, cancellation,
// and fail-closed gate fixtures owned by 158.002-T.
func TestConcurrencyFixtures(t *testing.T) {
	t.Run("lock contention", func(t *testing.T) {
		t.Run("blocking waits for release", func(t *testing.T) {
			outcome, err := concExerciseLockContention(context.Background(), concLockBlocking)
			if err != nil {
				t.Fatalf("blocking lock-contention fixture: %v", err)
			}

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
			if err != nil {
				t.Fatalf("busy lock-contention fixture: %v", err)
			}

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
			if got.err != nil {
				t.Fatalf("context-cancellation fixture: %v", got.err)
			}
			if !errors.Is(got.outcome.lockerError, context.Canceled) {
				t.Errorf(
					"blocking Locker.Acquire() error = %v, want errors.Is(_, context.Canceled)",
					got.outcome.lockerError,
				)
			}
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
		if err != nil {
			t.Fatalf("ambiguous-gate-input fixture: %v", err)
		}

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

func concExerciseLockContention(ctx context.Context, mode concLockMode) (concLockOutcome, error) {
	if mode != concLockBlocking && mode != concLockBusy {
		return concLockOutcome{}, fmt.Errorf("exercise lock contention: unsupported mode %q", mode)
	}

	locker := concNewFixtureLocker(mode)
	releaseHolder := make(chan struct{})
	holderReady := make(chan error, 1)
	holderDone := make(chan struct{})
	go func() {
		defer close(holderDone)
		if err := locker.Acquire(ctx); err != nil {
			holderReady <- err
			return
		}
		holderReady <- nil
		<-releaseHolder
		locker.Release()
	}()

	if err := <-holderReady; err != nil {
		return concLockOutcome{}, fmt.Errorf("first fixture lock acquire: %w", err)
	}
	<-locker.attempted

	secondResult := make(chan error, 1)
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		err := locker.Acquire(ctx)
		if err == nil {
			locker.Release()
		}
		secondResult <- err
	}()
	<-locker.attempted

	outcome := concLockOutcome{}
	if mode == concLockBlocking {
		select {
		case outcome.secondAcquireErr = <-secondResult:
			outcome.waitedForRelease = false
		default:
			outcome.waitedForRelease = true
		}
		close(releaseHolder)
		if outcome.waitedForRelease {
			outcome.secondAcquireErr = <-secondResult
		}
	} else {
		outcome.secondAcquireErr = <-secondResult
		close(releaseHolder)
	}

	<-secondDone
	<-holderDone
	outcome.maximumHolders = locker.maxHolders()
	outcome.allGoroutinesJoined = true
	return outcome, nil
}

func concExerciseCancellation(ctx context.Context) (concCancellationOutcome, error) {
	adapters := DefaultAdapters()
	adapterInputs := map[string][]byte{
		"events_jsonl": []byte(`{"schema_version":1,"gate":"allow"}`),
		"frontmatter":  []byte("---\nid: cancellation\n---\n"),
		"scanner":      []byte("bounded token\n"),
	}
	adapterOrder := []string{"events_jsonl", "frontmatter", "scanner"}

	outcome := concCancellationOutcome{
		adapterErrors: make(map[string]error, len(adapterOrder)),
	}
	entries := make([]Entry, 0, len(adapterOrder))
	for _, adapterName := range adapterOrder {
		adapter := adapters[adapterName]
		if adapter == nil {
			return concCancellationOutcome{}, fmt.Errorf(
				"exercise cancellation: adapter %q is not registered",
				adapterName,
			)
		}
		input := adapterInputs[adapterName]
		_, outcome.adapterErrors[adapterName] = adapter.Decode(ctx, input)
		entries = append(entries, Entry{
			ID:      "cancelled-" + adapterName,
			Adapter: adapterName,
			Input:   input,
			Expect:  ExpectAccepted,
		})
	}

	locker := concNewFixtureLocker(concLockBlocking)
	if err := locker.Acquire(context.Background()); err != nil {
		return concCancellationOutcome{}, fmt.Errorf(
			"hold cancellation fixture lock: %w",
			err,
		)
	}
	outcome.lockerError = locker.Acquire(ctx)
	locker.Release()

	outcome.report = Run(ctx, entries, adapters)
	return outcome, nil
}

func concAmbiguousGateEntry() (Entry, error) {
	return Entry{
		ID:       "ambiguous-gate-duplicate-key",
		Category: CatDuplicateKey,
		Adapter:  "events_jsonl",
		Input:    []byte(`{"gate":"allow","gate":"deny"}`),
		Expect:   ExpectRejected,
		WantErr:  ErrDuplicateKey,
	}, nil
}
