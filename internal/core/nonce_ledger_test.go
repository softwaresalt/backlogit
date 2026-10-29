package core_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestNonceLedgerRecordPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	notAfter := time.Date(2026, 6, 1, 12, 5, 0, 0, time.UTC)

	ledger, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	require.NoError(t, ledger.Record("00ff", notAfter))

	// A fresh handle over the same file sees the durable record.
	reopened, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	seen, err := reopened.Contains("00ff")
	require.NoError(t, err)
	require.True(t, seen)
	seen, err = reopened.Contains("beef")
	require.NoError(t, err)
	require.False(t, seen)
}

func TestNonceLedgerReplayRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	notAfter := time.Date(2026, 6, 1, 12, 5, 0, 0, time.UTC)
	ledger, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	require.NoError(t, ledger.Record("00ff", notAfter))

	err = ledger.Record("00ff", notAfter)
	require.Error(t, err)
	require.True(t, errors.Is(err, core.ErrNonceReplayed))
	var replay *core.NonceLedgerError
	require.True(t, errors.As(err, &replay))
	require.Equal(t, "00ff", replay.Nonce)

	reopened, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	err = reopened.Record("00ff", notAfter)
	require.ErrorIs(t, err, core.ErrNonceReplayed)
}

func TestNonceLedgerRejectsEmptyNonceAndCorruptLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	ledger, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	require.ErrorIs(t, ledger.Record("", time.Now()), core.ErrNonceInvalid)

	require.NoError(t, os.WriteFile(path, []byte("not json\n"), 0o600))
	_, err = ledger.Contains("00ff")
	require.ErrorIs(t, err, core.ErrNonceLedgerCorrupt)
	require.ErrorIs(t, ledger.Record("00ff", time.Now()), core.ErrNonceLedgerCorrupt)
}

func TestNonceLedgerPrunesPastNotAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	ledger, err := core.OpenNonceLedger(path)
	require.NoError(t, err)
	require.NoError(t, ledger.Record("old", base.Add(time.Minute)))
	require.NoError(t, ledger.Record("live", base.Add(10*time.Minute)))

	removed, err := ledger.Prune(base.Add(5 * time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	seen, err := ledger.Contains("old")
	require.NoError(t, err)
	require.False(t, seen)
	seen, err = ledger.Contains("live")
	require.NoError(t, err)
	require.True(t, seen)

	// A pruned nonce is past not_after, so its token already fails the
	// expiry check; the ledger may accept it again without re-enabling replay.
	require.NoError(t, ledger.Record("old", base.Add(time.Minute)))
}
