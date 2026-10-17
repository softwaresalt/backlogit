package evidencedag

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ageFile(t *testing.T, p string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(p, old, old))
}

func withLockTiming(t *testing.T, wait, stale time.Duration) {
	t.Helper()
	ow, ost := lockWait, lockStale
	lockWait, lockStale = wait, stale
	t.Cleanup(func() { lockWait, lockStale = ow, ost })
}

func TestHardening_ReapNeverDeletesALockThatChangedIdentity(t *testing.T) {
	l, p := seededLedger(t, 1)
	lp := p + ".lock"
	require.NoError(t, os.WriteFile(lp, []byte(`{"pid":1,"token":"dead"}`), 0o600))
	ageFile(t, lp)
	withLockTiming(t, 250*time.Millisecond, 30*time.Second)

	var fresh *heldLock
	reapHook = func() {
		reapHook = nil
		require.NoError(t, os.Remove(lp)) // the stale holder releases ...
		h, err := l.lock()                // ... and a new holder acquires, all inside the reaper's window
		require.NoError(t, err)
		fresh = h
	}
	t.Cleanup(func() { reapHook = nil })

	_, err := l.lock()
	assert.ErrorIs(t, err, ErrLedgerBusy, "the reaper must not take the fresh holder's lock")
	require.NotNil(t, fresh)
	assert.NoError(t, fresh.verify(), "the fresh holder still owns its lock (restored by hard link)")
	assert.NoError(t, fresh.release())
	matches, _ := filepath.Glob(lp + ".stale-*")
	assert.Empty(t, matches)
}

func TestHardening_FailedRestoreDeletesNothingAndFailsClosed(t *testing.T) {
	l, p := seededLedger(t, 1)
	lp := p + ".lock"
	require.NoError(t, os.WriteFile(lp, []byte(`{"pid":1,"token":"dead"}`), 0o600))
	ageFile(t, lp)
	withLockTiming(t, 250*time.Millisecond, 30*time.Second)

	var fresh *heldLock
	reapHook = func() {
		reapHook = nil
		require.NoError(t, os.Remove(lp))
		h, err := l.lock()
		require.NoError(t, err)
		fresh = h
	}
	origLink := linkFile
	linkFile = func(string, string) error { return errors.New("link not permitted") }
	t.Cleanup(func() { reapHook, linkFile = nil, origLink })

	_, err := l.lock()
	require.ErrorIs(t, err, ErrLedgerBusy)
	matches, _ := filepath.Glob(lp + ".stale-*")
	require.Len(t, matches, 1, "the moved-aside live lock is preserved, never deleted")
	b, rerr := os.ReadFile(matches[0])
	require.NoError(t, rerr)
	assert.Contains(t, string(b), fresh.token)
	assert.Error(t, fresh.verify(), "the displaced holder learns its lock is gone before it can commit")
	_ = fresh.release()
}

func TestHardening_HolderOutlivingLockStaleKeepsItsLockViaHeartbeat(t *testing.T) {
	l, _ := seededLedger(t, 1)
	withLockTiming(t, 400*time.Millisecond, 300*time.Millisecond)
	holder, err := l.lock()
	require.NoError(t, err)
	time.Sleep(800 * time.Millisecond) // well past lockStale
	_, err = l.lock()
	assert.ErrorIs(t, err, ErrLedgerBusy, "a live holder past lockStale is not reaped")
	assert.NoError(t, holder.verify())
	assert.NoError(t, holder.release())
	h2, err := l.lock()
	require.NoError(t, err)
	assert.NoError(t, h2.release())
}

type lockStealingAnchor struct {
	inner    MemoryAnchorStore
	lockPath string
}

func (a *lockStealingAnchor) Load() (Anchor, bool, error) {
	_ = os.Remove(a.lockPath)
	_ = os.WriteFile(a.lockPath, []byte(`{"pid":2,"token":"thief"}`), 0o600)
	return a.inner.Load()
}

func (a *lockStealingAnchor) Store(x Anchor) error { return a.inner.Store(x) }

func TestHardening_LostLockAbortsBeforeTheRecordIsWritten(t *testing.T) {
	l, p := seededLedger(t, 1)
	before, err := os.ReadFile(p)
	require.NoError(t, err)
	st := &lockStealingAnchor{lockPath: p + ".lock"}
	_, _, err = l.appendUnique(entry(2), appendOpts{store: st})
	require.ErrorIs(t, err, ErrLockLost)
	assert.NotErrorIs(t, err, ErrCommitted, "nothing was committed")
	after, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the ledger is byte-for-byte unchanged")
	assert.NoError(t, os.Remove(p+".lock"))
}

func TestHardening_LostLockBeforeAnchorStoreIsReportedCommittedAndAnchorStale(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.ledger.AdoptHead(f.anchors))
	orig := renameFile
	lp := f.ledger.path + ".lock"
	renameFile = func(a, b string) error { // the sidecar rename happens after the record commit
		_ = os.Remove(lp)
		_ = os.WriteFile(lp, []byte(`{"pid":2,"token":"thief"}`), 0o600)
		return os.Rename(a, b)
	}
	t.Cleanup(func() { renameFile = orig })

	_, _, err := f.ledger.appendUnique(entry(1), appendOpts{store: f.anchors})
	var ce *CommittedError
	require.ErrorAs(t, err, &ce)
	assert.ErrorIs(t, err, ErrCommitted)
	assert.ErrorIs(t, err, ErrAnchorStale)
	assert.ErrorIs(t, err, ErrLockLost)
	assert.Equal(t, 1, ce.Record.Seq, "the durable record is reported so callers do not retry")
	a, ok, _ := f.anchors.Load()
	assert.True(t, !ok || a.Seq == 0, "the anchor was not advanced without ownership")
	assert.NoError(t, os.Remove(lp))
}

func TestHardening_CommittedWithWarningIsTyped(t *testing.T) {
	l, _ := seededLedger(t, 1)
	orig, ow := renameFile, renameWait
	renameWait = 20 * time.Millisecond
	renameFile = func(string, string) error { return errors.New("sharing violation") }
	t.Cleanup(func() { renameFile, renameWait = orig, ow })
	rec, err := l.Append(entry(2))
	var ce *CommittedError
	require.ErrorAs(t, err, &ce)
	assert.ErrorIs(t, err, ErrSidecarStale)
	assert.ErrorIs(t, err, ErrCommitted)
	assert.Equal(t, rec, ce.Record)
	assert.Equal(t, 2, rec.Seq)
}

type panicStore struct{ onLoad bool }

func (s panicStore) Load() (Anchor, bool, error) {
	if s.onLoad {
		panic("load exploded")
	}
	return Anchor{}, false, nil
}
func (s panicStore) Store(Anchor) error { panic("store exploded") }

func TestHardening_PanickingAnchorStoreReleasesTheLock(t *testing.T) {
	cases := map[string]func(*Ledger){
		"AdoptHead.Store":    func(l *Ledger) { _ = l.AdoptHead(panicStore{}) },
		"AdoptHead.Load":     func(l *Ledger) { _ = l.AdoptHead(panicStore{onLoad: true}) },
		"append anchor.Load": func(l *Ledger) { _, _, _ = l.appendUnique(entry(9), appendOpts{store: panicStore{onLoad: true}}) },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			l, p := seededLedger(t, 1)
			assert.Panics(t, func() { run(l) })
			_, err := os.Lstat(p + ".lock")
			assert.True(t, errors.Is(err, os.ErrNotExist), "the deferred release ran")
			withLockTiming(t, 200*time.Millisecond, 30*time.Second)
			_, err = l.Records(nil)
			assert.NoError(t, err, "the ledger is usable again immediately")
		})
	}
}

func TestHardening_PlantedNonRegularLockLeafFailsClosed(t *testing.T) {
	withLockTiming(t, 300*time.Millisecond, 30*time.Second)
	t.Run("directory", func(t *testing.T) {
		l, p := seededLedger(t, 1)
		require.NoError(t, os.Mkdir(p+".lock", 0o700))
		start := time.Now()
		_, err := l.lock()
		assert.ErrorIs(t, err, ErrLedgerTampered)
		assert.Less(t, time.Since(start), 250*time.Millisecond, "refused outright, not waited out")
	})
	t.Run("symlink", func(t *testing.T) {
		l, p := seededLedger(t, 1)
		target := filepath.Join(t.TempDir(), "victim")
		require.NoError(t, os.WriteFile(target, []byte("precious"), 0o600))
		if err := os.Symlink(target, p+".lock"); err != nil {
			t.Skipf("cannot create symlinks here: %v", err)
		}
		ageFile(t, target)
		_, err := l.lock()
		assert.ErrorIs(t, err, ErrLedgerTampered)
		b, rerr := os.ReadFile(target)
		require.NoError(t, rerr)
		assert.Equal(t, "precious", string(b), "the symlink target is never touched")
	})
	t.Run("anchor lock leaf", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewFileAnchorStore(filepath.Join(dir, "a.json"), testForbiddenRoot)
		require.NoError(t, err)
		require.NoError(t, os.Mkdir(filepath.Join(dir, "a.json.lock"), 0o700))
		assert.ErrorIs(t, s.Store(Anchor{Seq: 1, Hash: strings.Repeat("c", 64)}), ErrLedgerTampered)
	})
}
