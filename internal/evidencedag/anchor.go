package evidencedag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
)

// AnchorStore retains the ledger head outside the ledger. It must live where
// the party being constrained cannot write (a protected path, a separate
// service); an anchor an attacker can rewrite proves nothing. Store must never
// move the anchor backwards.
type AnchorStore interface {
	// Load returns the retained anchor; ok is false when none was ever stored
	// (a genesis state). Anchor{Seq: 0} stored explicitly means "empty ledger".
	Load() (a Anchor, ok bool, err error)
	// Store durably replaces the anchor.
	Store(a Anchor) error
}

func validateAnchor(a Anchor) error {
	switch {
	case a.Seq < 0 || a.Seq > MaxLedgerRecords:
		return errors.New("anchor sequence out of range")
	case a.Seq == 0 && a.Hash != "":
		return errors.New("genesis anchor must carry no hash")
	case a.Seq > 0 && !hexPattern64.MatchString(a.Hash):
		return errors.New("anchor hash is invalid")
	}
	return nil
}

// MemoryAnchorStore is an in-process AnchorStore, suitable for tests and for
// callers that hold the anchor in protected memory.
type MemoryAnchorStore struct {
	mu sync.Mutex
	a  Anchor
	ok bool
}

// Load implements AnchorStore.
func (m *MemoryAnchorStore) Load() (Anchor, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.a, m.ok, nil
}

// Store implements AnchorStore.
func (m *MemoryAnchorStore) Store(a Anchor) error {
	if err := validateAnchor(a); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ok {
		if err := checkAdvance(m.a, a); err != nil {
			return err
		}
	}
	m.a, m.ok = a, true
	return nil
}

// ErrAnchorFork indicates an attempt to store an anchor that contradicts the
// retained one: an older sequence, or the same sequence with a different hash.
var ErrAnchorFork = errors.New("evidencedag: anchor must not move backwards or fork")

func checkAdvance(cur, next Anchor) error {
	switch {
	case next.Seq < cur.Seq:
		return fmt.Errorf("%w: sequence %d is behind the retained %d", ErrAnchorFork, next.Seq, cur.Seq)
	case next.Seq == cur.Seq && next.Hash != cur.Hash:
		return fmt.Errorf("%w: sequence %d is retained with a different hash", ErrAnchorFork, cur.Seq)
	}
	return nil
}

// anchorLocks serializes in-process Store calls per anchor file, even across
// distinct FileAnchorStore values for the same path.
var anchorLocks sync.Map

// FileAnchorStore keeps the anchor in a small file that must lie outside every
// caller-supplied agent-writable root.
type FileAnchorStore struct {
	dir, name string
}

func (s *FileAnchorStore) mutex() *sync.Mutex {
	mu, _ := anchorLocks.LoadOrStore(filepath.Join(s.dir, s.name), &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// NewFileAnchorStore returns a store for the anchor file at path. At least one
// forbidden root is required and the file must lie outside all of them.
func NewFileAnchorStore(path string, forbiddenRoots ...string) (*FileAnchorStore, error) {
	dir, name, err := splitCanonical(path)
	if err != nil {
		return nil, fmt.Errorf("evidencedag: anchor path: %w", err)
	}
	if err := checkOutsideRoots(filepath.Join(dir, name), forbiddenRoots); err != nil {
		return nil, fmt.Errorf("evidencedag: anchor path: %w", err)
	}
	return &FileAnchorStore{dir: dir, name: name}, nil
}

// Load implements AnchorStore.
func (s *FileAnchorStore) Load() (Anchor, bool, error) {
	raw, exists, err := readRegularBounded(s.dir, s.name, maxSidecarBytes)
	if err != nil {
		return Anchor{}, false, fmt.Errorf("evidencedag: read anchor: %w", err)
	}
	if !exists {
		return Anchor{}, false, nil
	}
	var a Anchor
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := rejectDuplicateKeys(raw); err != nil {
		return Anchor{}, false, fmt.Errorf("evidencedag: anchor is unreadable: %w", err)
	}
	if err := dec.Decode(&a); err != nil {
		return Anchor{}, false, fmt.Errorf("evidencedag: anchor is unreadable: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Anchor{}, false, errors.New("evidencedag: anchor has trailing data")
	}
	if err := validateAnchor(a); err != nil {
		return Anchor{}, false, fmt.Errorf("evidencedag: anchor is invalid: %w", err)
	}
	return a, true, nil
}

// Store implements AnchorStore. The check-and-write is serialized in-process
// and across processes by an anchor lock file; it refuses to move the anchor
// backwards or to replace a sequence with a different hash.
func (s *FileAnchorStore) Store(a Anchor) (err error) {
	if err := validateAnchor(a); err != nil {
		return fmt.Errorf("evidencedag: %w", err)
	}
	mu := s.mutex()
	mu.Lock()
	defer mu.Unlock()
	hl, err := dirLock{dir: s.dir, name: s.name + ".lock"}.acquire()
	if err != nil {
		return fmt.Errorf("evidencedag: lock anchor: %w", err)
	}
	defer func() {
		if uerr := hl.release(); uerr != nil {
			err = errors.Join(err, fmt.Errorf("evidencedag: release anchor lock: %w", uerr))
		}
	}()
	cur, ok, err := s.Load()
	if err != nil {
		return err
	}
	if ok {
		if err := checkAdvance(cur, a); err != nil {
			return err
		}
	}
	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("evidencedag: marshal anchor: %w", err)
	}
	if err := hl.verify(); err != nil {
		return fmt.Errorf("evidencedag: store anchor: %w", err)
	}
	if err := writeFileAtomic(s.dir, s.name, b); err != nil {
		return fmt.Errorf("evidencedag: store anchor: %w", err)
	}
	return nil
}
