package evidencedag

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingIdentityExec struct{ release chan struct{} }

func (b *blockingIdentityExec) Identity() string {
	<-b.release
	return "backlogit/parity@v1"
}

func (b *blockingIdentityExec) Execute(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, errors.New("unreachable")
}

func TestReview3_BlockingIdentityIsBoundedAndAccounted(t *testing.T) {
	x := &blockingIdentityExec{release: make(chan struct{})}
	t.Cleanup(func() {
		close(x.release)
		require.Eventually(t, func() bool { return abandonedCount() == 0 }, 5*time.Second, 5*time.Millisecond)
	})
	start := time.Now()
	res, err := RunExecutorsWith(context.Background(), decl([]Node{pnode("a", true)}), ship(),
		map[string]NodeExecutor{"a": x}, ExecOptions{NodeTimeout: 50 * time.Millisecond})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.Contains(t, res.Failures["a"], "timed out")
	assert.Equal(t, 1, abandonedCount(), "the blocked Identity() goroutine is accounted")
}

func TestReview3_ConcurrentRunsNeverExceedCap(t *testing.T) {
	old := maxAbandonedExecutors
	maxAbandonedExecutors = 4
	defer func() { maxAbandonedExecutors = old }()
	release := make(chan struct{})
	var mu sync.Mutex
	live, peak := 0, 0
	stuck := fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
		mu.Lock()
		live++
		if live > peak {
			peak = live
		}
		mu.Unlock()
		<-release // ignores ctx on purpose
		mu.Lock()
		live--
		mu.Unlock()
		return faultline.EvidenceArtifact{}, errors.New("released")
	})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = RunExecutorsWith(context.Background(), decl([]Node{pnode("a", true)}), ship(),
				map[string]NodeExecutor{"a": stuck}, ExecOptions{NodeTimeout: 20 * time.Millisecond})
		}()
	}
	wg.Wait()
	mu.Lock()
	assert.LessOrEqual(t, peak, 4)
	mu.Unlock()
	assert.LessOrEqual(t, abandonedCount(), 4)
	close(release)
	require.Eventually(t, func() bool { return abandonedCount() == 0 }, 5*time.Second, 5*time.Millisecond)
}

func TestReview3_FoldedDuplicateKeysRejectedAtEveryEntryPoint(t *testing.T) {
	docs := map[string]string{
		"long s":       `{"schema_version":99,"` + "\u017f" + `chema_version":1}`,
		"kelvin sign":  `{"kid":1,"` + "\u212a" + `id":2}`,
		"ascii casing": `{"Kid":1,"kID":2}`,
	}
	entry := map[string]func([]byte) error{
		"declaration": func(b []byte) error { _, err := Load(b); return err },
		"trust":       func(b []byte) error { _, err := ParseTrust(b); return err },
		"token":       func(b []byte) error { _, err := decodeToken(b); return err },
		"claims":      func(b []byte) error { _, err := decodeClaims(b); return err },
		"evidence":    func(b []byte) error { _, err := decodeRecord(b); return err },
		"ledger":      func(b []byte) error { _, err := decodeLedgerLine(b); return err },
		"anchor": func(b []byte) error {
			p := filepath.Join(t.TempDir(), "anchor.json")
			if err := writeFileAtomic(filepath.Dir(p), filepath.Base(p), b); err != nil {
				return err
			}
			s, err := NewFileAnchorStore(p, testForbiddenRoot)
			if err != nil {
				return err
			}
			_, _, err = s.Load()
			return err
		},
	}
	for en, fn := range entry {
		for dn, doc := range docs {
			t.Run(en+"/"+dn, func(t *testing.T) {
				err := fn([]byte(doc))
				require.Error(t, err)
				assert.Contains(t, err.Error(), "duplicate key")
			})
		}
	}
	require.Error(t, rejectDuplicateKeys([]byte(docs["long s"])))
}

func failingDirSync(t *testing.T) {
	t.Helper()
	old := syncDirFn
	syncDirFn = func(string) error { return errors.New("injected dir sync failure") }
	t.Cleanup(func() { syncDirFn = old })
}

func TestReview3_DirSyncFailurePropagates(t *testing.T) {
	t.Run("writeFileAtomic", func(t *testing.T) {
		failingDirSync(t)
		err := writeFileAtomic(t.TempDir(), "f.json", []byte("{}"))
		require.ErrorIs(t, err, errDirSyncFailed)
	})
	t.Run("anchor store", func(t *testing.T) {
		failingDirSync(t)
		s, err := NewFileAnchorStore(filepath.Join(t.TempDir(), "anchor.json"), testForbiddenRoot)
		require.NoError(t, err)
		err = s.Store(Anchor{Seq: 1, Hash: genesisHash})
		require.ErrorIs(t, err, errDirSyncFailed)
	})
	t.Run("anchored append reports a stale anchor", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewFileAnchorStore(filepath.Join(dir, "anchor.json"), testForbiddenRoot)
		require.NoError(t, err)
		l, err := openLedger(filepath.Join(dir, "ledger.jsonl"))
		require.NoError(t, err)
		failingDirSync(t)
		_, _, err = l.appendUnique(entry(1), appendOpts{store: s})
		require.ErrorIs(t, err, ErrCommitted)
		assert.ErrorIs(t, err, ErrAnchorStale)
	})
	t.Run("ledger record directory sync is surfaced", func(t *testing.T) {
		l, err := openLedger(filepath.Join(t.TempDir(), "ledger.jsonl"))
		require.NoError(t, err)
		failingDirSync(t)
		_, err = l.Append(entry(1))
		require.ErrorIs(t, err, ErrCommitted)
		assert.Contains(t, err.Error(), "sync ledger directory")
	})
}
