package evidencedag

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReview4_IdentifierLengthIsCheckedBeforePatterns(t *testing.T) {
	taskID := func(n int) string { return strings.Repeat("1", n-2) + "-S" }
	t.Run("task id boundary", func(t *testing.T) {
		assert.True(t, ValidTaskID(taskID(MaxIDLength)))
		assert.False(t, ValidTaskID(taskID(MaxIDLength+1)))
		assert.False(t, ValidTaskID(taskID(1<<20)))
	})
	t.Run("shipment validation", func(t *testing.T) {
		for _, c := range []struct {
			n  int
			ok bool
		}{{MaxIDLength, true}, {MaxIDLength + 1, false}, {1 << 20, false}} {
			s := ship()
			s.ID = taskID(c.n)
			err := validateShipment(s)
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidInput)
			}
		}
	})
	t.Run("evaluator rejects an oversized shipment id", func(t *testing.T) {
		s := ship()
		s.ID = taskID(1 << 20)
		_, err := mustEval(t, decl([]Node{pnode("t", true)})).Evaluate(Input{Shipment: s})
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
	t.Run("principal", func(t *testing.T) {
		assert.True(t, validPrincipalName(strings.Repeat("a", MaxIDLength)))
		assert.False(t, validPrincipalName(strings.Repeat("a", MaxIDLength+1)))
		assert.False(t, validPrincipalName(strings.Repeat("a", 1<<20)))
	})
	t.Run("nonce", func(t *testing.T) {
		assert.True(t, validNonce(strings.Repeat("a", 64)))
		assert.False(t, validNonce(strings.Repeat("a", 65)))
		assert.False(t, validNonce(strings.Repeat("a", 15)))
		assert.False(t, validNonce(strings.Repeat("a", 1<<20)))
	})
	t.Run("hex digest", func(t *testing.T) {
		assert.True(t, validHex64(strings.Repeat("a", 64)))
		assert.False(t, validHex64(strings.Repeat("a", 65)))
		assert.False(t, validHex64(strings.Repeat("a", 1<<20)))
	})
	t.Run("ledger entry", func(t *testing.T) {
		e := entry(1)
		e.ShipmentID = taskID(MaxIDLength)
		assert.NoError(t, validateEntry(e))
		e.ShipmentID = taskID(1 << 20)
		assert.Error(t, validateEntry(e))
		e = entry(1)
		e.Principal = strings.Repeat("a", 1<<20)
		assert.Error(t, validateEntry(e))
	})
	t.Run("node producing task", func(t *testing.T) {
		n := pnode("t", true)
		n.ProducingTask = taskID(1 << 20)
		assert.Error(t, decl([]Node{n}).Validate())
	})
}

type blockingErr struct{ release chan struct{} }

func (b *blockingErr) Error() string { <-b.release; return "late" }

type panickingErr struct{}

func (panickingErr) Error() string { panic("boom") }

type abandonedErr struct{}

func (abandonedErr) Error() string   { return "mimic" }
func (abandonedErr) Is(t error) bool { return t == ErrExecutorsAbandoned }

func TestReview4_ExecutorErrorsAreFlattenedInsideTheGuardedGoroutine(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	run := func(t *testing.T, err error) (ExecutionResult, error) {
		t.Helper()
		x := fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
			return faultline.EvidenceArtifact{}, err
		})
		done := make(chan struct{})
		var res ExecutionResult
		var rerr error
		go func() {
			defer close(done)
			res, rerr = RunExecutorsWith(context.Background(), d, ship(), map[string]NodeExecutor{"t": x}, ExecOptions{NodeTimeout: 200 * time.Millisecond})
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("RunExecutorsWith hung on the executor error")
		}
		return res, rerr
	}
	t.Run("blocking Error is bounded by the node timeout", func(t *testing.T) {
		be := &blockingErr{release: make(chan struct{})}
		t.Cleanup(func() { close(be.release) })
		res, err := run(t, be)
		require.NoError(t, err)
		assert.Contains(t, res.Failures["t"], "timed out")
		assert.Empty(t, res.Evidence)
	})
	t.Run("panicking Error fails the node", func(t *testing.T) {
		res, err := run(t, panickingErr{})
		require.NoError(t, err)
		assert.Contains(t, res.Failures["t"], "unavailable")
	})
	t.Run("an error mimicking the abandoned sentinel cannot abort the run", func(t *testing.T) {
		res, err := run(t, abandonedErr{})
		require.NoError(t, err)
		assert.Contains(t, res.Failures["t"], "mimic")
	})
	t.Run("message is bounded", func(t *testing.T) {
		res, err := run(t, errors.New(strings.Repeat("x", 1<<20)))
		require.NoError(t, err)
		assert.Less(t, len(res.Failures["t"]), 4096)
	})
}

func TestReview4_IdenticalWaiverRetryRequiresAnchorDurability(t *testing.T) {
	ledgerDir, anchorDir := t.TempDir(), t.TempDir()
	st, err := NewFileAnchorStore(filepath.Join(anchorDir, "anchor.json"), testForbiddenRoot)
	require.NoError(t, err)
	l, err := openLedger(filepath.Join(ledgerDir, "ledger.jsonl"))
	require.NoError(t, err)
	require.NoError(t, l.AdoptHead(st))

	old := syncDirFn
	t.Cleanup(func() { syncDirFn = old })
	syncDirFn = func(dir string) error {
		if filepath.Base(dir) == filepath.Base(anchorDir) {
			return errors.New("injected anchor dir sync failure")
		}
		return old(dir)
	}
	o := appendOpts{unique: true, store: st}
	_, _, err = l.appendUnique(entry(1), o)
	require.ErrorIs(t, err, ErrAnchorStale)
	require.True(t, committedAndUsable(err) == false)

	// The anchor file is readable although never made durable; an identical
	// retry must not report success while the sync keeps failing.
	_, existed, err := l.appendUnique(entry(1), o)
	require.ErrorIs(t, err, ErrAnchorStale, fmt.Sprint(existed))
	assert.False(t, existed)

	syncDirFn = old
	r, existed, err := l.appendUnique(entry(1), o)
	require.NoError(t, err)
	assert.True(t, existed)
	assert.Equal(t, 1, r.Seq)
}
