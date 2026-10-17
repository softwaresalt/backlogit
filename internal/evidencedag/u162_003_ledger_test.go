package evidencedag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entry(i int) LedgerEntry {
	return LedgerEntry{
		AcceptedAt: int64(1_800_000_000 + i), ShipmentID: "144-S", NodeID: fmt.Sprintf("n%d", i), HeadCommit: testHead,
		Principal: "alice", KeyID: "k-alice", Family: "parity", Kind: "waiver",
		Nonce: fmt.Sprintf("%032x", i), Reason: "r", TokenDigest: fmt.Sprintf("%064x", i),
	}
}

func seededLedger(t *testing.T, n int) (*Ledger, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "w.jsonl")
	l, err := openLedger(p)
	require.NoError(t, err)
	for i := 1; i <= n; i++ {
		_, err := l.Append(entry(i))
		require.NoError(t, err)
	}
	return l, p
}

func readLines(t *testing.T, p string) []string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func writeLines(t *testing.T, p string, lines []string) {
	t.Helper()
	require.NoError(t, os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}

func TestU162_003_LedgerChainAppendAndVerify(t *testing.T) {
	l, p := seededLedger(t, 3)
	recs, err := l.Records(nil)
	require.NoError(t, err)
	require.Len(t, recs, 3)
	assert.Equal(t, strings.Repeat("0", 64), recs[0].PrevHash)
	for i, r := range recs {
		assert.Equal(t, i+1, r.Seq)
		assert.Equal(t, LedgerSchemaVersion, r.Version)
		if i > 0 {
			assert.Equal(t, recs[i-1].Hash, r.PrevHash)
		}
	}
	head, err := l.Head()
	require.NoError(t, err)
	assert.Equal(t, Anchor{Seq: 3, Hash: recs[2].Hash}, head)
	_, err = l.RecordsPrefix(&Anchor{Seq: 2, Hash: recs[1].Hash})
	assert.NoError(t, err, "an earlier anchor remains a valid prefix")
	_, err = l.Records(&Anchor{Seq: 2, Hash: recs[1].Hash})
	assert.ErrorIs(t, err, ErrLedgerTampered, "Records requires an exact anchor match")
	_, err = l.Records(&Anchor{Seq: 3, Hash: recs[2].Hash})
	assert.NoError(t, err, "the exact head anchor matches")
	assert.Len(t, readLines(t, p), 3)

	empty, err := openLedger(filepath.Join(t.TempDir(), "none.jsonl"))
	require.NoError(t, err)
	got, err := empty.Records(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	h, err := empty.Head()
	require.NoError(t, err)
	assert.Equal(t, Anchor{}, h)
}

func TestU162_003_LedgerDetectsTampering(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, p string)
		anchor func(recs []LedgerRecord) *Anchor
	}{
		{"edited record", func(t *testing.T, p string) {
			ls := readLines(t, p)
			ls[1] = strings.Replace(ls[1], `"principal":"alice"`, `"principal":"mallory"`, 1)
			writeLines(t, p, ls)
		}, nil},
		{"deleted middle record", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, []string{ls[0], ls[2]})
		}, nil},
		{"reordered records", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, []string{ls[1], ls[0], ls[2]})
		}, nil},
		{"truncated tail with stale sidecar", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, ls[:2])
		}, nil},
		{"truncated tail and sidecar removed, anchor mismatch", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, ls[:1])
			require.NoError(t, os.Remove(p+".head"))
		}, func(recs []LedgerRecord) *Anchor { return &Anchor{Seq: 3, Hash: recs[2].Hash} }},
		{"truncated tail rewritten with forged anchor hash", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, ls[:2])
			require.NoError(t, os.Remove(p+".head"))
		}, func(recs []LedgerRecord) *Anchor { return &Anchor{Seq: 2, Hash: strings.Repeat("a", 64)} }},
		{"emptied ledger", func(t *testing.T, p string) { require.NoError(t, os.WriteFile(p, nil, 0o600)) }, nil},
		{"deleted ledger", func(t *testing.T, p string) { require.NoError(t, os.Remove(p)) }, nil},
		{"appended garbage line", func(t *testing.T, p string) {
			f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
			require.NoError(t, err)
			_, err = f.WriteString("garbage\n")
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}, nil},
		{"appended forged valid-looking record", func(t *testing.T, p string) {
			ls := readLines(t, p)
			forged := strings.Replace(ls[2], `"seq":3`, `"seq":4`, 1)
			writeLines(t, p, append(ls, forged))
		}, nil},
		{"missing sidecar", func(t *testing.T, p string) { require.NoError(t, os.Remove(p+".head")) }, nil},
		{"unsupported record version", func(t *testing.T, p string) {
			ls := readLines(t, p)
			ls[0] = strings.Replace(ls[0], `"v":1`, `"v":2`, 1)
			writeLines(t, p, ls)
		}, nil},
		{"unknown field", func(t *testing.T, p string) {
			ls := readLines(t, p)
			ls[0] = strings.Replace(ls[0], `"v":1`, `"v":1,"extra":1`, 1)
			writeLines(t, p, ls)
		}, nil},
		{"blank line", func(t *testing.T, p string) {
			ls := readLines(t, p)
			writeLines(t, p, []string{ls[0], "", ls[1], ls[2]})
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, p := seededLedger(t, 3)
			recs, err := l.Records(nil)
			require.NoError(t, err)
			c.mutate(t, p)
			var a *Anchor
			if c.anchor != nil {
				a = c.anchor(recs)
			}
			_, err = l.Records(a)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrLedgerTampered)
			if c.anchor == nil {
				_, err = l.Append(entry(9))
				assert.Error(t, err, "appending to a tampered ledger must be refused")
				_, err = l.Head()
				assert.Error(t, err)
			}
		})
	}
}

func TestU162_003_LedgerSidecarOneBehindHeals(t *testing.T) {
	l, p := seededLedger(t, 3)
	recs, err := l.Records(nil)
	require.NoError(t, err)
	// Simulate a crash after the record append but before the sidecar update.
	side := fmt.Sprintf(`{"seq":%d,"hash":%q}`, 2, recs[1].Hash)
	require.NoError(t, os.WriteFile(p+".head", []byte(side), 0o600))
	_, err = l.Records(nil)
	require.NoError(t, err)
	_, err = l.Append(entry(4))
	require.NoError(t, err)
	head, err := l.Head()
	require.NoError(t, err)
	assert.Equal(t, 4, head.Seq)
}

func TestU162_003_LedgerConcurrentAppendsAreSingleWriterSafe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "w.jsonl")
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l, err := openLedger(p) // independent handles model independent processes
			if err != nil {
				errs <- err
				return
			}
			_, err = l.Append(entry(i))
			errs <- err
		}(i)
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
	require.Len(t, recs, 20)
	seen := map[string]bool{}
	for i, r := range recs {
		assert.Equal(t, i+1, r.Seq)
		seen[r.NodeID] = true
	}
	assert.Len(t, seen, 20)
}

func TestU162_003_LedgerBounds(t *testing.T) {
	l, p := seededLedger(t, 1)
	require.NoError(t, os.WriteFile(p, []byte(strings.Repeat("a", MaxLedgerBytes+10)), 0o600))
	_, err := l.Records(nil)
	assert.ErrorIs(t, err, ErrLedgerTampered)

	l2, _ := seededLedger(t, 1)
	big := entry(2)
	big.Reason = strings.Repeat("r", 5000)
	_, err = l2.Append(big)
	assert.Error(t, err, "oversized reason is refused")
	bad := entry(3)
	bad.NodeID = ""
	_, err = l2.Append(bad)
	assert.Error(t, err)
}

func TestU162_003_LedgerRefusesSymlinkedPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.jsonl")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := openLedger(link)
	assert.Error(t, err)
	_, err = openLedger("")
	assert.Error(t, err)
	_, err = openLedger(filepath.Join(dir, "missing-dir", "w.jsonl"))
	assert.Error(t, err)
}
