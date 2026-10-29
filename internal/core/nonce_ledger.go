package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sentinel errors for the nonce ledger (170.003-T).
var (
	// ErrNonceReplayed reports a nonce that is already recorded as consumed.
	ErrNonceReplayed = errors.New("authorization token nonce already consumed")
	// ErrNonceInvalid reports an empty nonce.
	ErrNonceInvalid = errors.New("authorization token nonce invalid")
	// ErrNonceLedgerCorrupt reports an unreadable ledger; it fails closed.
	ErrNonceLedgerCorrupt = errors.New("nonce ledger corrupt")
)

// NonceLedgerError is the typed error returned by NonceLedger.
type NonceLedgerError struct {
	// Nonce is the nonce the error concerns (may be empty).
	Nonce string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *NonceLedgerError) Error() string {
	return fmt.Sprintf("nonce ledger nonce %q: %v", e.Nonce, e.Err)
}

// Unwrap returns the sentinel category.
func (e *NonceLedgerError) Unwrap() error { return e.Err }

type nonceLedgerRecord struct {
	Nonce    string    `json:"nonce"`
	NotAfter time.Time `json:"not_after"`
}

// NonceLedger is the durable append-then-fsync record of consumed nonces.
// It is a workspace projection only: the authoritative atomic
// compare-and-consume is owned by 170.008-T. No file handle is held between
// calls.
type NonceLedger struct {
	path string
	mu   sync.Mutex
}

// OpenNonceLedger returns a ledger over the file at path. The file is created
// on first Record.
func OpenNonceLedger(path string) (*NonceLedger, error) {
	if path == "" {
		return nil, fmt.Errorf("open nonce ledger: %w", ErrNonceLedgerCorrupt)
	}
	return &NonceLedger{path: path}, nil
}

// Record appends nonce to the ledger and fsyncs it. A nonce that is already
// recorded fails closed with ErrNonceReplayed.
func (l *NonceLedger) Record(nonce string, notAfter time.Time) error {
	if nonce == "" {
		return &NonceLedgerError{Err: ErrNonceInvalid}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	records, err := l.load()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Nonce == nonce {
			return &NonceLedgerError{Nonce: nonce, Err: ErrNonceReplayed}
		}
	}
	line, err := json.Marshal(nonceLedgerRecord{Nonce: nonce, NotAfter: notAfter.UTC()})
	if err != nil {
		return fmt.Errorf("encode nonce record: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open nonce ledger: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close() // write error takes precedence
		return fmt.Errorf("append nonce record: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() // sync error takes precedence
		return fmt.Errorf("sync nonce ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close nonce ledger: %w", err)
	}
	return nil
}

// Contains reports whether nonce is recorded as consumed.
func (l *NonceLedger) Contains(nonce string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	records, err := l.load()
	if err != nil {
		return false, err
	}
	for _, r := range records {
		if r.Nonce == nonce {
			return true, nil
		}
	}
	return false, nil
}

// Prune removes records whose not_after is before now and returns how many
// were removed. A pruned nonce belongs to a token that already fails expiry.
func (l *NonceLedger) Prune(now time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	records, err := l.load()
	if err != nil {
		return 0, err
	}
	kept := make([]nonceLedgerRecord, 0, len(records))
	for _, r := range records {
		if !r.NotAfter.Before(now) {
			kept = append(kept, r)
		}
	}
	removed := len(records) - len(kept)
	if removed == 0 {
		return 0, nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(l.path), ".nonce-ledger-*")
	if err != nil {
		return 0, fmt.Errorf("create nonce ledger temp: %w", err)
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	for _, r := range kept {
		if err := enc.Encode(r); err != nil {
			_ = tmp.Close()        // encode error takes precedence
			_ = os.Remove(tmpName) // best-effort cleanup
			return 0, fmt.Errorf("write nonce ledger: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()        // sync error takes precedence
		_ = os.Remove(tmpName) // best-effort cleanup
		return 0, fmt.Errorf("sync nonce ledger: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup
		return 0, fmt.Errorf("close nonce ledger temp: %w", err)
	}
	if err := os.Rename(tmpName, l.path); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup
		return 0, fmt.Errorf("replace nonce ledger: %w", err)
	}
	return removed, nil
}

func (l *NonceLedger) load() ([]nonceLedgerRecord, error) {
	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open nonce ledger: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	var records []nonceLedgerRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r nonceLedgerRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil || r.Nonce == "" {
			return nil, &NonceLedgerError{Err: ErrNonceLedgerCorrupt}
		}
		records = append(records, r)
	}
	if err := sc.Err(); err != nil {
		return nil, &NonceLedgerError{Err: fmt.Errorf("%w: %v", ErrNonceLedgerCorrupt, err)}
	}
	return records, nil
}
