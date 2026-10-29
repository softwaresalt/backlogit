package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// memNonceProvider is an in-memory stand-in for the external rollback-resistant
// compare-and-swap authority. It performs no filesystem writes.
type memNonceProvider struct {
	mu       sync.Mutex
	consumed map[string]struct{}
	err      error
}

func newMemNonceProvider() *memNonceProvider {
	return &memNonceProvider{consumed: map[string]struct{}{}}
}

func (p *memNonceProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	if _, ok := p.consumed[nonce]; ok {
		return core.ErrNonceReplayed
	}
	p.consumed[nonce] = struct{}{}
	return nil
}

type memConsumeAuditor struct {
	mu     sync.Mutex
	events []core.NonceConsumeEvent
	err    error
}

func (a *memConsumeAuditor) RecordNonceConsume(ev core.NonceConsumeEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, ev)
	return nil
}

func newTestConsumer(t *testing.T, provider core.NonceConsumeProvider, auditor core.NonceConsumeAuditor) (*core.NonceConsumer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	projection, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	consumer, err := core.NewNonceConsumer(provider, projection, auditor)
	require.NoError(t, err)
	return consumer, path
}

func TestNonceConsumeAtomicCompareAndConsume(t *testing.T) {
	notAfter := time.Now().Add(5 * time.Minute)

	t.Run("succeeds once, audits, and survives projection deletion", func(t *testing.T) {
		provider := newMemNonceProvider()
		auditor := &memConsumeAuditor{}
		consumer, path := newTestConsumer(t, provider, auditor)

		require.NoError(t, consumer.Consume(context.Background(), "00ff", notAfter))
		require.Len(t, auditor.events, 1)
		require.Equal(t, "00ff", auditor.events[0].Nonce)

		require.NoError(t, os.Remove(path))
		err := consumer.Consume(context.Background(), "00ff", notAfter)
		require.ErrorIs(t, err, core.ErrNonceReplayed)
		require.Len(t, auditor.events, 1, "a replay is not a consume")
	})

	t.Run("concurrent consumes yield exactly one success", func(t *testing.T) {
		consumer, _ := newTestConsumer(t, newMemNonceProvider(), &memConsumeAuditor{})
		const workers = 16
		errs := make([]error, workers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = consumer.Consume(context.Background(), "race", notAfter)
			}()
		}
		close(start)
		wg.Wait()
		successes := 0
		for _, err := range errs {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIs(t, err, core.ErrNonceReplayed)
		}
		require.Equal(t, 1, successes)
	})

	t.Run("provider unavailable fails closed", func(t *testing.T) {
		provider := newMemNonceProvider()
		provider.err = errors.New("cas backend down")
		consumer, _ := newTestConsumer(t, provider, &memConsumeAuditor{})
		err := consumer.Consume(context.Background(), "00ff", notAfter)
		require.ErrorIs(t, err, core.ErrNonceProviderUnavailable)
		require.False(t, errors.Is(err, core.ErrNonceConsumeIndeterminate))
	})

	t.Run("nil provider is rejected at construction", func(t *testing.T) {
		projection, err := core.OpenNonceLedger(filepath.Join(t.TempDir(), "n.jsonl"))
		require.NoError(t, err)
		_, err = core.NewNonceConsumer(nil, projection, &memConsumeAuditor{})
		require.ErrorIs(t, err, core.ErrNonceProviderUnavailable)
	})

	t.Run("empty nonce is invalid", func(t *testing.T) {
		consumer, _ := newTestConsumer(t, newMemNonceProvider(), &memConsumeAuditor{})
		require.ErrorIs(t, consumer.Consume(context.Background(), "", notAfter), core.ErrNonceInvalid)
	})

	t.Run("consume with audit failure is indeterminate and never rolled back", func(t *testing.T) {
		provider := newMemNonceProvider()
		auditor := &memConsumeAuditor{err: errors.New("audit disk full")}
		consumer, _ := newTestConsumer(t, provider, auditor)

		err := consumer.Consume(context.Background(), "00ff", notAfter)
		require.ErrorIs(t, err, core.ErrNonceConsumeIndeterminate)
		var ind *core.NonceConsumeIndeterminateError
		require.True(t, errors.As(err, &ind))
		require.Equal(t, "00ff", ind.Nonce)

		// The nonce is definitively consumed: replay is still rejected.
		auditor.err = nil
		err = consumer.Consume(context.Background(), "00ff", notAfter)
		require.ErrorIs(t, err, core.ErrNonceReplayed)
	})
}
