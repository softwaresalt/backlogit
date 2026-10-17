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
)

// Lock timing and the write seam are variables only so tests can shorten the
// wait and inject write faults.
var (
	lockWait    = 10 * time.Second
	lockStale   = 30 * time.Second
	ledgerWrite = func(f *os.File, b []byte, off int64) (int, error) { return f.WriteAt(b, off) }
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
// parent directory must exist, an existing ledger must be a regular file, and
// the canonical ledger path must lie outside every forbidden root (the
// agent-writable evidence and workspace-output roots). At least one forbidden
// root is required; an empty list is a configuration error, never a wildcard.
func OpenLedger(path string, forbiddenRoots ...string) (*Ledger, error) {
	if path == "" {
		return nil, errors.New("evidencedag: ledger path is required")
	}
	dir, name, err := splitCanonical(path)
	if err != nil {
		return nil, fmt.Errorf("evidencedag: ledger path: %w", err)
	}
	if di, err := os.Stat(dir); err != nil || !di.IsDir() {
		return nil, errors.New("evidencedag: ledger directory does not exist")
	}
	canon := filepath.Join(dir, name)
	if err := checkOutsideRoots(canon, forbiddenRoots); err != nil {
		return nil, fmt.Errorf("evidencedag: ledger path: %w", err)
	}
	if fi, err := os.Lstat(canon); err == nil && !fi.Mode().IsRegular() {
		return nil, errors.New("evidencedag: ledger path is not a regular file")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("evidencedag: stat ledger: %w", err)
	}
	return &Ledger{path: canon, dir: dir, name: name}, nil
}

// readBounded reads a regular file beneath the ledger directory up to max
// bytes; a missing file reports exists=false. Anything else (symlink,
// directory, oversize) is tampering.
func (l *Ledger) readBounded(name string, max int) (data []byte, exists bool, err error) {
	data, exists, err = readRegularBounded(l.dir, name, max)
	switch {
	case errors.Is(err, errNotRegular):
		return nil, true, tampered("%s is not a regular file", name)
	case errors.Is(err, errTooLarge):
		return nil, true, tampered("%s exceeds %d bytes", name, max)
	case err != nil:
		return nil, true, fmt.Errorf("evidencedag: read %s: %w", name, err)
	}
	return data, exists, nil
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
// the external anchor. Any inconsistency is ErrLedgerTampered. With exact set
// the ledger must hold precisely the anchored records (a ledger ahead of its
// anchor is refused). With repairTorn set, a single trailing unterminated
// line (a torn append) is ignored and its verified prefix length returned so a
// writer can truncate it away. validLen is the byte length of the verified
// prefix.
func (l *Ledger) load(anchor *Anchor, exact, repairTorn bool) (recs []LedgerRecord, validLen int64, err error) {
	data, _, err := l.readBounded(l.name, MaxLedgerBytes)
	if err != nil {
		return nil, 0, err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if !repairTorn {
			return nil, 0, tampered("ledger does not end with a newline (partial or appended data)")
		}
		cut := bytes.LastIndexByte(data, '\n') + 1
		if len(data)-cut > maxLedgerLine {
			return nil, 0, tampered("ledger tail is oversized and unterminated")
		}
		data = data[:cut]
	}
	validLen = int64(len(data))
	if len(data) > 0 {
		lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
		if len(lines) > MaxLedgerRecords {
			return nil, 0, tampered("ledger exceeds %d records", MaxLedgerRecords)
		}
		prev := genesisHash
		for i, line := range lines {
			rec, err := decodeLedgerLine(line)
			if err != nil {
				return nil, 0, tampered("record %d: %v", i+1, err)
			}
			if rec.Seq != i+1 || rec.PrevHash != prev {
				return nil, 0, tampered("record %d breaks the chain (seq or prev_hash)", i+1)
			}
			want, err := recordHash(rec)
			if err != nil || want != rec.Hash {
				return nil, 0, tampered("record %d hash mismatch", i+1)
			}
			prev = rec.Hash
			recs = append(recs, rec)
		}
	}
	if err := l.checkSidecar(recs); err != nil {
		return nil, 0, err
	}
	if anchor != nil {
		if err := validateAnchor(*anchor); err != nil {
			return nil, 0, tampered("%v", err)
		}
		switch {
		case anchor.Seq == 0 && len(recs) != 0:
			return nil, 0, tampered("ledger is not empty but its anchor says genesis (rolled back or replaced)")
		case anchor.Seq > 0 && (anchor.Seq > len(recs) || recs[anchor.Seq-1].Hash != anchor.Hash):
			return nil, 0, tampered("ledger does not contain the anchored head (truncated, deleted or rewritten)")
		case exact && len(recs) != anchor.Seq:
			return nil, 0, tampered("ledger is ahead of its external anchor (uncommitted or forged tail); verify it and re-anchor with AdoptHead")
		}
	}
	return recs, validLen, nil
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
// the sidecar update and is healed by the next append. The sidecar is a
// convenience cross-check only: it is rewritable by whoever can rewrite the
// ledger, so tamper evidence against such an actor rests on the external
// anchor.
func (l *Ledger) checkSidecar(recs []LedgerRecord) error {
	raw, exists, err := l.readBounded(l.name+".head", maxSidecarBytes)
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

// lock takes the single-writer lock beside the ledger.
func (l *Ledger) lock() (*heldLock, error) {
	return dirLock{dir: l.dir, name: l.name + ".lock"}.acquire()
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
	case e.Kind != claimKindWaiver:
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

// Append adds one record under the single-writer lock, without an external
// anchor (use a WaiverService for anchored acceptance). The whole chain is
// re-verified under the lock first, so an append can never extend a tampered
// ledger or race another writer into a forked chain; a torn trailing line from
// a crashed append is discarded first. A returned *CommittedError (matching
// ErrCommitted, and for example ErrSidecarStale) means the record is durable
// (the error carries it) and the append must not be retried.
func (l *Ledger) Append(e LedgerEntry) (LedgerRecord, error) {
	rec, _, err := l.appendUnique(e, appendOpts{})
	return rec, err
}

type appendOpts struct {
	// unique makes the nonce single-use: a record already holding it is
	// returned unchanged when it has the same token digest, and rejected as a
	// replay otherwise.
	unique bool
	// store, when set, makes the append anchored: the ledger must match the
	// anchor exactly (or be absent for a genesis anchor) and the anchor is
	// advanced after the commit.
	store AnchorStore
	// stamp, when set, is called under the lock and yields AcceptedAt; an
	// error aborts the append.
	stamp func() (int64, error)
}

func (l *Ledger) appendUnique(e LedgerEntry, o appendOpts) (rec LedgerRecord, existed bool, err error) {
	probe := e
	if o.stamp != nil {
		probe.AcceptedAt = 1
	}
	if err := validateEntry(probe); err != nil {
		return rec, false, fmt.Errorf("evidencedag: invalid ledger entry: %w", err)
	}
	hl, err := l.lock()
	if err != nil {
		return rec, false, err
	}
	committed := false
	var warns []error
	defer func() {
		if uerr := hl.release(); uerr != nil {
			uerr = fmt.Errorf("evidencedag: release ledger lock: %w", uerr)
			if committed {
				warns = append(warns, uerr)
			} else {
				err = errors.Join(err, uerr)
			}
		}
		if committed && len(warns) > 0 {
			err = &CommittedError{Record: rec, Err: errors.Join(warns...)}
		}
	}()
	var anchor *Anchor
	genesis := false
	if o.store != nil {
		a, ok, aerr := o.store.Load()
		if aerr != nil {
			return rec, false, fmt.Errorf("evidencedag: load ledger anchor: %w", aerr)
		}
		if ok {
			anchor = &a
		} else {
			genesis = true
		}
	}
	recs, valid, err := l.load(anchor, anchor != nil, true)
	if err != nil {
		return rec, false, err
	}
	if genesis && len(recs) > 0 {
		return rec, false, tampered("ledger has records but no external anchor exists")
	}
	if o.unique {
		for _, r := range recs {
			if r.Nonce == e.Nonce {
				if r.TokenDigest == e.TokenDigest {
					return r, true, nil
				}
				return rec, false, ErrNonceReplayed
			}
		}
	}
	if o.stamp != nil {
		at, serr := o.stamp()
		if serr != nil {
			return rec, false, serr
		}
		e.AcceptedAt = at
		if err := validateEntry(e); err != nil {
			return rec, false, fmt.Errorf("evidencedag: invalid ledger entry: %w", err)
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
	if len(line) > maxLedgerLine || valid+int64(len(line)) > MaxLedgerBytes {
		return LedgerRecord{}, false, errors.New("evidencedag: ledger record would exceed size bounds")
	}
	if err := hl.verify(); err != nil {
		return LedgerRecord{}, false, fmt.Errorf("evidencedag: aborting before commit: %w", err)
	}
	closeErr, err := l.writeRecord(line, valid)
	if err != nil {
		return LedgerRecord{}, false, err
	}
	committed = true
	if closeErr != nil {
		warns = append(warns, closeErr)
	}
	if err := l.writeSidecar(rec); err != nil {
		warns = append(warns, fmt.Errorf("%w: %v", ErrSidecarStale, err))
	}
	if o.store != nil {
		if err := hl.verify(); err != nil {
			warns = append(warns, fmt.Errorf("%w: %v", ErrAnchorStale, err))
		} else if err := o.store.Store(Anchor{Seq: rec.Seq, Hash: rec.Hash}); err != nil {
			warns = append(warns, fmt.Errorf("%w: %v", ErrAnchorStale, err))
		}
	}
	return rec, false, nil
}

// writeRecord writes line at offset valid (discarding any torn tail beyond the
// verified prefix) and, on any failure before the record is synced, truncates
// back so a partial record is never left behind. A close failure after a
// successful sync is returned separately: the record is already durable.
func (l *Ledger) writeRecord(line []byte, valid int64) (closeErr, err error) {
	f, err := openBeneath(l.dir, l.name, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("evidencedag: open ledger: %w", err)
	}
	fail := func(what string, cause error) error {
		terr := f.Truncate(valid)
		cerr := f.Close()
		out := fmt.Errorf("evidencedag: %s ledger: %w", what, cause)
		if terr != nil || cerr != nil {
			out = errors.Join(out, fmt.Errorf("evidencedag: restore ledger size: %w", errors.Join(terr, cerr)))
		}
		return out
	}
	if fi, serr := f.Stat(); serr != nil {
		return nil, fail("stat", serr)
	} else if fi.Size() > valid {
		if err := f.Truncate(valid); err != nil {
			return nil, fail("discard torn tail of", err)
		}
	}
	if _, err := ledgerWrite(f, line, valid); err != nil {
		return nil, fail("append", err)
	}
	if err := f.Sync(); err != nil {
		return nil, fail("sync", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("evidencedag: close ledger: %w", err), nil
	}
	return nil, nil
}

func (l *Ledger) writeSidecar(rec LedgerRecord) error {
	b, err := json.Marshal(sidecar{Seq: rec.Seq, Hash: rec.Hash})
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}
	return writeFileAtomic(l.dir, l.name+".head", b)
}

// Records verifies the chain under the writer lock, so it never observes a
// half-finished append, and returns every record. A missing ledger is an empty
// history, never tampering, unless its head sidecar or the anchor says records
// once existed. When anchor is non-nil it must match the ledger exactly: the
// ledger must hold precisely the anchored records (use RecordsPrefix to accept
// a ledger that extends the anchor). The lock file is created beside the
// ledger, so callers need write access to its directory.
func (l *Ledger) Records(anchor *Anchor) ([]LedgerRecord, error) {
	return l.records(anchor, anchor != nil)
}

// RecordsPrefix is Records for an anchor that only has to be a prefix of the
// ledger: the anchored head must be contained in the chain, which may extend
// beyond it. This is the verification AdoptHead applies before re-anchoring.
func (l *Ledger) RecordsPrefix(anchor *Anchor) ([]LedgerRecord, error) {
	return l.records(anchor, false)
}

func (l *Ledger) records(anchor *Anchor, exact bool) (recs []LedgerRecord, err error) {
	if l == nil {
		return nil, errors.New("evidencedag: nil ledger")
	}
	hl, err := l.lock()
	if err != nil {
		return nil, err
	}
	defer func() {
		if uerr := hl.release(); uerr != nil && err == nil {
			recs, err = nil, fmt.Errorf("evidencedag: release ledger lock: %w", uerr)
		}
	}()
	recs, _, err = l.load(anchor, exact, false)
	return recs, err
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

// AdoptHead is the operator recovery action for a ledger that is ahead of its
// external anchor (for example after a crash between the record commit and the
// anchor update). It verifies the chain with RecordsPrefix semantics, requires
// any existing anchor to still be contained in it, and advances the anchor to
// the verified head. Run it only after confirming that the extra records are
// legitimate: when no anchor exists it trusts the current ledger on first use.
func (l *Ledger) AdoptHead(store AnchorStore) (err error) {
	if l == nil || store == nil {
		return ErrAnchorRequired
	}
	hl, err := l.lock()
	if err != nil {
		return err
	}
	defer func() {
		if uerr := hl.release(); uerr != nil {
			err = errors.Join(err, fmt.Errorf("evidencedag: release ledger lock: %w", uerr))
		}
	}()
	var anchor *Anchor
	a, ok, err := store.Load()
	if err != nil {
		return fmt.Errorf("evidencedag: adopt ledger head: %w", err)
	}
	if ok {
		anchor = &a
	}
	recs, _, err := l.load(anchor, false, false)
	if err != nil {
		return fmt.Errorf("evidencedag: adopt ledger head: %w", err)
	}
	head := Anchor{}
	if len(recs) > 0 {
		head = Anchor{Seq: len(recs), Hash: recs[len(recs)-1].Hash}
	}
	if err := hl.verify(); err != nil {
		return fmt.Errorf("evidencedag: adopt ledger head: %w", err)
	}
	if err := store.Store(head); err != nil {
		return fmt.Errorf("evidencedag: adopt ledger head: %w", err)
	}
	return nil
}
