package evidencedag

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemediation_LedgerConcurrentAppendsFormOneLinearChain(t *testing.T) {
	old := lockWait
	lockWait = 2 * time.Minute // generous: -race on a loaded host can be many times slower
	defer func() { lockWait = old }()
	p := filepath.Join(t.TempDir(), "w.jsonl")
	const writers, per = 12, 2
	var wg sync.WaitGroup
	errs := make(chan error, writers*per)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			l, err := openLedger(p)
			if err != nil {
				errs <- err
				return
			}
			for k := 0; k < per; k++ {
				_, err := l.Append(entry(w*per + k + 1))
				errs <- err
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	l, err := openLedger(p)
	require.NoError(t, err)
	recs, err := l.Records(nil)
	require.NoError(t, err)
	require.Len(t, recs, writers*per)
	prev := genesisHash
	nonces := map[string]bool{}
	for i, r := range recs {
		assert.Equal(t, i+1, r.Seq)
		assert.Equal(t, prev, r.PrevHash, "a single linear chain, never a fork")
		prev = r.Hash
		nonces[r.Nonce] = true
	}
	assert.Len(t, nonces, writers*per)
	_, statErr := os.Lstat(p + ".lock")
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "lock is released")
}

func TestRemediation_StaleLockIsReapedByRenameAndUnlockVerifiesOwnership(t *testing.T) {
	l, p := seededLedger(t, 1)
	lp := p + ".lock"
	require.NoError(t, os.WriteFile(lp, []byte(`{"pid":1,"token":"dead"}`), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(lp, old, old))
	_, err := l.Append(entry(2))
	require.NoError(t, err, "a stale lock is reclaimed")
	_, statErr := os.Lstat(lp)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
	matches, _ := filepath.Glob(lp + ".stale-*")
	assert.Empty(t, matches, "reaped lock files are removed")

	release, err := l.lock()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lp, []byte(`{"pid":2,"token":"someone-else"}`), 0o600))
	assert.Error(t, release.release(), "unlock refuses to remove a lock this holder does not own")
	_, statErr = os.Lstat(lp)
	assert.NoError(t, statErr, "the other owner's lock is left in place")
}

func TestRemediation_FreshLockHonorsDeadlineWithoutSpinning(t *testing.T) {
	l, _ := seededLedger(t, 1)
	release, err := l.lock()
	require.NoError(t, err)
	defer func() { _ = release.release() }()
	orig := lockWait
	lockWait = 150 * time.Millisecond
	defer func() { lockWait = orig }()
	start := time.Now()
	_, err = l.Append(entry(2))
	assert.ErrorIs(t, err, ErrLedgerBusy)
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond, "the loop waits instead of failing or spinning")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestRemediation_TornTailIsRecoveredByNextAppend(t *testing.T) {
	l, p := seededLedger(t, 2)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(`{"v":1,"seq":3`)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	_, err = l.Records(nil)
	assert.ErrorIs(t, err, ErrLedgerTampered, "readers still see the torn tail as tampering")
	rec, err := l.Append(entry(3))
	require.NoError(t, err)
	assert.Equal(t, 3, rec.Seq)
	recs, err := l.Records(nil)
	require.NoError(t, err)
	assert.Len(t, recs, 3)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"seq":3`+"\n{")
	assert.Equal(t, byte('\n'), b[len(b)-1])
}

func TestRemediation_FailedWriteTruncatesBackToPreAppendSize(t *testing.T) {
	l, p := seededLedger(t, 2)
	before, err := os.ReadFile(p)
	require.NoError(t, err)
	orig := ledgerWrite
	ledgerWrite = func(f *os.File, b []byte, off int64) (int, error) {
		n, _ := f.WriteAt(b[:len(b)/2], off)
		return n, errors.New("disk full")
	}
	_, err = l.Append(entry(3))
	ledgerWrite = orig
	require.Error(t, err)
	after, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "no torn bytes are left behind")
	_, err = l.Append(entry(3))
	require.NoError(t, err)
	recs, err := l.Records(nil)
	require.NoError(t, err)
	assert.Len(t, recs, 3)
}

func TestRemediation_SidecarRenameRetriedThenReportedStaleAndHealed(t *testing.T) {
	l, _ := seededLedger(t, 1)
	orig := renameFile
	origWait := renameWait
	renameWait = 50 * time.Millisecond
	defer func() { renameFile, renameWait = orig, origWait }()

	fails := 2
	renameFile = func(a, b string) error {
		if fails > 0 {
			fails--
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}
	_, err := l.Append(entry(2))
	require.NoError(t, err, "a transient rename failure is retried")

	renameFile = func(string, string) error { return errors.New("sharing violation") }
	rec, err := l.Append(entry(3))
	assert.ErrorIs(t, err, ErrSidecarStale, "committed but sidecar stale is reported distinctly")
	assert.Equal(t, 3, rec.Seq, "the committed record is still returned")
	leftovers, _ := filepath.Glob(l.path + ".head.tmp-*")
	assert.Empty(t, leftovers)

	renameFile = orig
	_, err = l.Records(nil)
	require.NoError(t, err, "a sidecar one record behind is tolerated")
	_, err = l.Append(entry(4))
	require.NoError(t, err, "the next append heals the sidecar")
	head, err := l.Head()
	require.NoError(t, err)
	assert.Equal(t, 4, head.Seq)
}

func TestRemediation_OpenLedgerRequiresForbiddenRootsAndContainment(t *testing.T) {
	dir := t.TempDir()
	evidence := t.TempDir()
	_, err := OpenLedger(filepath.Join(dir, "w.jsonl"))
	assert.Error(t, err, "no forbidden roots is a configuration error")
	_, err = OpenLedger(filepath.Join(dir, "w.jsonl"), "")
	assert.Error(t, err)
	_, err = OpenLedger(filepath.Join(evidence, "w.jsonl"), evidence)
	assert.Error(t, err, "a ledger inside agent-writable evidence is refused")
	_, err = OpenLedger(filepath.Join(dir, "w.jsonl"), evidence)
	assert.NoError(t, err)
}

func TestRemediation_FileAnchorStore(t *testing.T) {
	dir := t.TempDir()
	evidence := t.TempDir()
	_, err := NewFileAnchorStore(filepath.Join(evidence, "a.json"), evidence)
	assert.Error(t, err, "anchor inside agent-writable roots is refused")
	_, err = NewFileAnchorStore(filepath.Join(dir, "a.json"))
	assert.Error(t, err)
	s, err := NewFileAnchorStore(filepath.Join(dir, "a.json"), evidence)
	require.NoError(t, err)
	_, ok, err := s.Load()
	require.NoError(t, err)
	assert.False(t, ok)
	h := strings.Repeat("a", 64)
	require.NoError(t, s.Store(Anchor{Seq: 2, Hash: h}))
	got, ok, err := s.Load()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, Anchor{Seq: 2, Hash: h}, got)
	assert.Error(t, s.Store(Anchor{Seq: 1, Hash: h}), "anchors never move backwards")
	assert.Error(t, s.Store(Anchor{Seq: 3, Hash: "xyz"}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"seq":1,"seq":2,"hash":"`+h+`"}`), 0o600))
	_, _, err = s.Load()
	assert.Error(t, err, "duplicate keys are refused")
}

// accepted accepts one waiver through the anchored service and returns it.
func (f *fixture) accept() {
	f.t.Helper()
	d := decl([]Node{pnode("t", true)})
	_, err := f.svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
	require.NoError(f.t, err)
}

func (f *fixture) acceptDenied(want error) {
	f.t.Helper()
	d := decl([]Node{pnode("t", true)})
	_, err := f.svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
	require.Error(f.t, err)
	assert.ErrorIs(f.t, err, ErrWaiverDenied)
	if want != nil {
		assert.ErrorIs(f.t, err, want)
	}
}

func TestRemediation_AnchoredLedgerDetectsDeletionRollbackAndForgedTail(t *testing.T) {
	t.Run("anchor is mandatory", func(t *testing.T) {
		f := newFixture(t)
		_, err := NewWaiverService(f.trust, f.ledger, nil, nil)
		assert.ErrorIs(t, err, ErrAnchorRequired)
	})
	t.Run("genesis then append anchors the head", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		a, ok, err := f.anchors.Load()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, 1, a.Seq)
	})
	t.Run("deleted ledger", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		f.accept()
		require.NoError(t, os.Remove(f.ledger.path))
		require.NoError(t, os.Remove(f.ledger.path+".head"))
		f.acceptDenied(nil)
		_, err := f.ledger.Records(&Anchor{Seq: 2, Hash: strings.Repeat("b", 64)})
		assert.ErrorIs(t, err, ErrLedgerTampered)
	})
	t.Run("rolled back ledger with matching sidecar", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		oldLedger, err := os.ReadFile(f.ledger.path)
		require.NoError(t, err)
		oldSide, err := os.ReadFile(f.ledger.path + ".head")
		require.NoError(t, err)
		f.accept()
		require.NoError(t, os.WriteFile(f.ledger.path, oldLedger, 0o600))
		require.NoError(t, os.WriteFile(f.ledger.path+".head", oldSide, 0o600))
		_, err = f.ledger.Records(nil)
		require.NoError(t, err, "without the anchor a consistent rollback is invisible")
		f.acceptDenied(nil)
	})
	t.Run("replayed nonce after rollback is not accepted again", func(t *testing.T) {
		f := newFixture(t)
		d := decl([]Node{pnode("t", true)})
		tok := signClaims(f.alice, f.claims())
		_, err := f.svc.Accept(tok, ship(), lookupFor(d))
		require.NoError(t, err)
		require.NoError(t, os.Remove(f.ledger.path))
		require.NoError(t, os.Remove(f.ledger.path+".head"))
		_, err = f.svc.Accept(tok, ship(), lookupFor(d))
		assert.ErrorIs(t, err, ErrWaiverDenied, "erasing the ledger cannot reopen a consumed nonce")
	})
	t.Run("forged tail", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		_, err := f.ledger.Append(entry(900001)) // an attacker can compute valid hashes
		require.NoError(t, err)
		f.acceptDenied(ErrLedgerTampered)
		require.NoError(t, f.ledger.AdoptHead(f.anchors), "an operator re-anchors after inspecting the tail")
		f.accept()
	})
	t.Run("ledger without any anchor", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.ledger.Append(entry(1))
		require.NoError(t, err)
		f.acceptDenied(ErrLedgerTampered)
	})
	t.Run("explicit genesis anchor with a populated ledger", func(t *testing.T) {
		f := newFixture(t)
		require.NoError(t, f.anchors.Store(Anchor{}))
		_, err := f.ledger.Append(entry(1))
		require.NoError(t, err)
		_, err = f.ledger.Append(entry(2))
		require.NoError(t, err)
		f.acceptDenied(ErrLedgerTampered)
	})
	t.Run("anchor update failure fails closed", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		failing := &failingAnchor{inner: f.anchors}
		svc, err := NewWaiverService(f.trust, f.ledger, failing, func() time.Time { return f.now })
		require.NoError(t, err)
		d := decl([]Node{pnode("t", true)})
		failing.failStore = true
		_, err = svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
		assert.ErrorIs(t, err, ErrWaiverDenied)
		assert.ErrorIs(t, err, ErrAnchorStale)
		failing.failStore = false
		_, err = svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
		assert.ErrorIs(t, err, ErrLedgerTampered, "stays closed until an operator re-anchors")
	})
	t.Run("sidecar-only tolerance does not weaken the anchor", func(t *testing.T) {
		f := newFixture(t)
		f.accept()
		f.accept()
		recs, err := f.ledger.Records(nil)
		require.NoError(t, err)
		// Truncate the last record and point the sidecar at the new tail.
		lines := readLines(t, f.ledger.path)
		writeLines(t, f.ledger.path, lines[:1])
		require.NoError(t, os.WriteFile(f.ledger.path+".head", []byte(fmt.Sprintf(`{"seq":1,"hash":%q}`, recs[0].Hash)), 0o600))
		f.acceptDenied(ErrLedgerTampered)
	})
}

type failingAnchor struct {
	inner     AnchorStore
	failStore bool
}

func (a *failingAnchor) Load() (Anchor, bool, error) { return a.inner.Load() }

func (a *failingAnchor) Store(x Anchor) error {
	if a.failStore {
		return errors.New("anchor service unavailable")
	}
	return a.inner.Store(x)
}
