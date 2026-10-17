package evidencedag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

const (
	ledgerDomain    = "backlogit/evidencedag/ledger/v1\n"
	maxLedgerLine   = 8 << 10
	maxSidecarBytes = 1 << 10
	lockWait        = 10 * time.Second
	lockStale       = 30 * time.Second
)

var (
	genesisHash  = strings.Repeat("0", 64)
	hexPattern64 = regexp.MustCompile(`^[0-9a-f]{64}$`)
	noncePattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)
)

type sidecar struct {
	Seq  int    `json:"seq"`
	Hash string `json:"hash"`
}

func tampered(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrLedgerTampered, fmt.Sprintf(format, args...))
}

// OpenLedger returns a Ledger handle for path without creating anything. The
// parent directory must exist and an existing ledger must be a regular file.
func OpenLedger(path string) (*Ledger, error) {
	if path == "" {
		return nil, errors.New("evidencedag: ledger path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("evidencedag: resolve ledger path: %w", err)
	}
	if di, err := os.Stat(filepath.Dir(abs)); err != nil || !di.IsDir() {
		return nil, errors.New("evidencedag: ledger directory does not exist")
	}
	if fi, err := os.Lstat(abs); err == nil && !fi.Mode().IsRegular() {
		return nil, errors.New("evidencedag: ledger path is not a regular file")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("evidencedag: stat ledger: %w", err)
	}
	return &Ledger{path: abs}, nil
}

// readBounded reads a regular file up to max bytes; a missing file reports
// exists=false. Anything else (symlink, directory, oversize) is tampering.
func readBounded(path string, max int) (data []byte, exists bool, err error) {
	li, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("evidencedag: stat %s: %w", filepath.Base(path), err)
	}
	if !li.Mode().IsRegular() {
		return nil, true, tampered("%s is not a regular file", filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, true, fmt.Errorf("evidencedag: open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, true, fmt.Errorf("evidencedag: read %s: %w", filepath.Base(path), err)
	}
	if len(raw) > max {
		return nil, true, tampered("%s exceeds %d bytes", filepath.Base(path), max)
	}
	return raw, true, nil
}

func recordHash(r LedgerRecord) (string, error) {
	r.Hash = ""
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(ledgerDomain), b...))
	return hex.EncodeToString(sum[:]), nil
}

// load reads and fully verifies the chain, the head sidecar and, when given,
// the external anchor. Any inconsistency is ErrLedgerTampered.
func (l *Ledger) load(anchor *Anchor) ([]LedgerRecord, error) {
	data, _, err := readBounded(l.path, MaxLedgerBytes)
	if err != nil {
		return nil, err
	}
	var recs []LedgerRecord
	if len(data) > 0 {
		if data[len(data)-1] != '\n' {
			return nil, tampered("ledger does not end with a newline (partial or appended data)")
		}
		lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
		if len(lines) > MaxLedgerRecords {
			return nil, tampered("ledger exceeds %d records", MaxLedgerRecords)
		}
		prev := genesisHash
		for i, line := range lines {
			rec, err := decodeLedgerLine(line)
			if err != nil {
				return nil, tampered("record %d: %v", i+1, err)
			}
			if rec.Seq != i+1 || rec.PrevHash != prev {
				return nil, tampered("record %d breaks the chain (seq or prev_hash)", i+1)
			}
			want, err := recordHash(rec)
			if err != nil || want != rec.Hash {
				return nil, tampered("record %d hash mismatch", i+1)
			}
			prev = rec.Hash
			recs = append(recs, rec)
		}
	}
	if err := l.checkSidecar(recs); err != nil {
		return nil, err
	}
	if anchor != nil {
		if anchor.Seq < 1 || anchor.Seq > len(recs) || recs[anchor.Seq-1].Hash != anchor.Hash {
			return nil, tampered("ledger does not contain the anchored head (truncated or rewritten)")
		}
	}
	return recs, nil
}

func decodeLedgerLine(line []byte) (LedgerRecord, error) {
	var rec LedgerRecord
	if len(line) == 0 || len(line) > maxLedgerLine {
		return rec, errors.New("blank or oversized line")
	}
	if err := rejectDuplicateKeys(line); err != nil {
		return rec, err
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return rec, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return rec, errors.New("trailing data on line")
	}
	if rec.Version != LedgerSchemaVersion {
		return rec, fmt.Errorf("unsupported record version %d", rec.Version)
	}
	canon, err := json.Marshal(rec)
	if err != nil || !bytes.Equal(canon, line) {
		return rec, errors.New("record is not in canonical form")
	}
	return rec, nil
}

// checkSidecar compares the head sidecar with the verified chain. A sidecar
// exactly one record behind is the crash window between the record write and
// the sidecar update and is healed by the next append.
func (l *Ledger) checkSidecar(recs []LedgerRecord) error {
	raw, exists, err := readBounded(l.path+".head", maxSidecarBytes)
	if err != nil {
		return err
	}
	n := len(recs)
	if !exists {
		if n > 1 {
			return tampered("head sidecar is missing")
		}
		return nil
	}
	var sc sidecar
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return tampered("head sidecar is unreadable")
	}
	switch {
	case n > 0 && sc.Seq == n && sc.Hash == recs[n-1].Hash:
		return nil
	case n > 1 && sc.Seq == n-1 && sc.Hash == recs[n-2].Hash:
		return nil
	}
	return tampered("ledger does not match its head sidecar (truncated, deleted or rewritten)")
}

func (l *Ledger) lock() (func(), error) {
	lp := l.path + ".lock"
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(lp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lp) }, nil
		}
		if !errors.Is(err, fs.ErrExist) && !errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("evidencedag: acquire ledger lock: %w", err)
		}
		if fi, serr := os.Stat(lp); serr == nil && time.Since(fi.ModTime()) > lockStale {
			_ = os.Remove(lp)
			continue
		}
		if time.Now().After(deadline) {
			return nil, ErrLedgerBusy
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func validateEntry(e LedgerEntry) error {
	switch {
	case e.AcceptedAt <= 0:
		return errors.New("accepted_at is required")
	case !taskPattern.MatchString(e.ShipmentID):
		return errors.New("shipment id is invalid")
	case !validID(e.NodeID):
		return errors.New("node id is invalid")
	case !faultline.ValidCommitSHA(e.HeadCommit):
		return errors.New("head commit is invalid")
	case !principalPattern.MatchString(e.Principal) || !principalPattern.MatchString(e.KeyID):
		return errors.New("principal or key id is invalid")
	case !knownFamily(Family(e.Family)):
		return errors.New("family is unknown")
	case e.Kind != claimKindWaiver && e.Kind != claimKindNotApplicable:
		return errors.New("kind is invalid")
	case !noncePattern.MatchString(e.Nonce):
		return errors.New("nonce is invalid")
	case e.Reason == "" || len(e.Reason) > MaxReasonBytes:
		return errors.New("reason is required and bounded")
	case !hexPattern64.MatchString(e.TokenDigest):
		return errors.New("token digest is invalid")
	}
	return nil
}

// Append adds one record under the single-writer lock. The whole chain is
// re-verified under the lock first, so an append can never extend a tampered
// ledger or race another writer into a forked chain.
func (l *Ledger) Append(e LedgerEntry) (LedgerRecord, error) {
	rec, _, err := l.appendUnique(e, false)
	return rec, err
}

// appendUnique appends e. With uniqueNonce set, a record already holding the
// nonce is returned unchanged (existed=true) when it carries the same token
// digest, and rejected as a replay otherwise.
func (l *Ledger) appendUnique(e LedgerEntry, uniqueNonce bool) (rec LedgerRecord, existed bool, err error) {
	if err := validateEntry(e); err != nil {
		return rec, false, fmt.Errorf("evidencedag: invalid ledger entry: %w", err)
	}
	unlock, err := l.lock()
	if err != nil {
		return rec, false, err
	}
	defer unlock()
	recs, err := l.load(nil)
	if err != nil {
		return rec, false, err
	}
	if uniqueNonce {
		for _, r := range recs {
			if r.Nonce == e.Nonce {
				if r.TokenDigest == e.TokenDigest {
					return r, true, nil
				}
				return rec, false, ErrNonceReplayed
			}
		}
	}
	if len(recs) >= MaxLedgerRecords {
		return rec, false, fmt.Errorf("evidencedag: ledger is full (%d records)", MaxLedgerRecords)
	}
	prev := genesisHash
	if len(recs) > 0 {
		prev = recs[len(recs)-1].Hash
	}
	rec = LedgerRecord{
		Version: LedgerSchemaVersion, Seq: len(recs) + 1, PrevHash: prev, AcceptedAt: e.AcceptedAt,
		ShipmentID: e.ShipmentID, NodeID: e.NodeID, HeadCommit: e.HeadCommit, Principal: e.Principal,
		KeyID: e.KeyID, Family: e.Family, Kind: e.Kind, Nonce: e.Nonce, Reason: e.Reason, TokenDigest: e.TokenDigest,
	}
	if rec.Hash, err = recordHash(rec); err != nil {
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: hash ledger record: %w", err)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: marshal ledger record: %w", err)
	}
	line = append(line, '\n')
	var size int64
	if fi, serr := os.Lstat(l.path); serr == nil {
		size = fi.Size()
	}
	if len(line) > maxLedgerLine || size+int64(len(line)) > MaxLedgerBytes {
		return LedgerRecord{}, false, errors.New("evidencedag: ledger record would exceed size bounds")
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: open ledger: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: append ledger: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: sync ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: close ledger: %w", err)
	}
	if err := l.writeSidecar(rec); err != nil {
		return LedgerRecord{}, false, err
	}
	return rec, false, nil
}

func (l *Ledger) writeSidecar(rec LedgerRecord) error {
	b, err := json.Marshal(sidecar{Seq: rec.Seq, Hash: rec.Hash})
	if err != nil {
		return fmt.Errorf("evidencedag: marshal sidecar: %w", err)
	}
	tmp := l.path + ".head.tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("evidencedag: stage sidecar: %w", err)
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return fmt.Errorf("evidencedag: write sidecar: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("evidencedag: sync sidecar: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("evidencedag: close sidecar: %w", err)
	}
	if err := os.Rename(tmp, l.path+".head"); err != nil {
		return fmt.Errorf("evidencedag: publish sidecar: %w", err)
	}
	return nil
}

// Records verifies the chain (and the optional external anchor) and returns
// every record. A missing ledger is an empty history, never tampering, unless
// its head sidecar says records once existed.
func (l *Ledger) Records(anchor *Anchor) ([]LedgerRecord, error) {
	if l == nil {
		return nil, errors.New("evidencedag: nil ledger")
	}
	return l.load(anchor)
}

// Head returns the verified chain head, suitable for retention as an Anchor.
func (l *Ledger) Head() (Anchor, error) {
	recs, err := l.Records(nil)
	if err != nil {
		return Anchor{}, err
	}
	if len(recs) == 0 {
		return Anchor{}, nil
	}
	return Anchor{Seq: len(recs), Hash: recs[len(recs)-1].Hash}, nil
}
