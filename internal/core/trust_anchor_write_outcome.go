package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Write-outcome taxonomy for trust-anchor mutations (168.017-T). The sentinels
// reuse the two-class durability contract so errors.Is works across packages.
var (
	// ErrWriteNotApplied reports a mutation that failed before step 2 of the
	// protocol: state is definitively unchanged, the marker is absent, and the
	// request is retry-safe.
	ErrWriteNotApplied = blerrors.ErrWriteNotApplied
	// ErrWriteIndeterminate reports a failure at or after step 2 and before the
	// marker cleared: the mutation may be (partly) applied. The marker is left
	// in place for operator visibility; nothing is rolled forward or re-emitted
	// automatically (recovery is out of scope, P-021).
	ErrWriteIndeterminate = blerrors.ErrWriteIndeterminate
	// ErrAuditNotPersisted reports a denied mutation whose denial-audit write
	// failed. The denial stands and no state changed; the audit error is
	// surfaced alongside it.
	ErrAuditNotPersisted = errors.New("trust anchor denial audit not persisted")
)

// TrustAnchorMarkerFile is the indeterminate-marker file name in the storage root.
const TrustAnchorMarkerFile = "trust_anchor_write.marker"

// TrustAnchorMarker durably records a mutation that is in flight. While it is
// present the outcome of the recorded mutation is not known to be complete.
type TrustAnchorMarker struct {
	Action      TrustAnchorAction `json:"action"`
	AnchorID    string            `json:"anchor_id"`
	NewAnchorID string            `json:"new_anchor_id,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	At          time.Time         `json:"at"`
	// Nonce uniquely identifies the request that wrote the marker.
	Nonce string `json:"nonce,omitempty"`
}

// TrustAnchorMarkerStore persists the indeterminate marker. WriteMarker and
// ClearMarker must be durable (fsynced) before returning.
type TrustAnchorMarkerStore interface {
	WriteMarker(ctx context.Context, m TrustAnchorMarker) error
	ClearMarker(ctx context.Context) error
}

// ErrTrustAnchorMarkerOutstanding reports that another mutation's indeterminate
// marker is already present. The mutation is refused before any state change
// and the existing marker is left untouched. It matches ErrWriteIndeterminate
// because the earlier mutation's outcome is not known.
var ErrTrustAnchorMarkerOutstanding = fmt.Errorf("%w: a trust anchor write marker is already outstanding", ErrWriteIndeterminate)

// FileTrustAnchorMarkerStore is the file-backed marker store. Use a pointer:
// it remembers the nonce of the marker it wrote so ClearMarker only ever
// removes its own marker.
type FileTrustAnchorMarkerStore struct {
	// Path is the marker file.
	Path string
	// SyncDir fsyncs the parent directory of the given path after a marker
	// create or unlink. Nil uses atomicfile.SyncParentDir (a no-op on Windows).
	SyncDir func(path string) error

	nonce string
}

// LockKey identifies the storage location serialized by TrustAnchorMutator.Apply.
func (s *FileTrustAnchorMarkerStore) LockKey() string {
	if abs, err := filepath.Abs(s.Path); err == nil {
		return abs
	}
	return filepath.Clean(s.Path)
}

func (s *FileTrustAnchorMarkerStore) syncDir() func(string) error {
	if s.SyncDir != nil {
		return s.SyncDir
	}
	return atomicfile.SyncParentDir
}

// WriteMarker implements TrustAnchorMarkerStore. Acquisition is atomic and
// no-clobber (O_CREATE|O_EXCL): if a marker exists it returns
// ErrTrustAnchorMarkerOutstanding and leaves that marker untouched.
func (s *FileTrustAnchorMarkerStore) WriteMarker(_ context.Context, m TrustAnchorMarker) error {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Errorf("marker nonce: %w", err)
	}
	m.Nonce = hex.EncodeToString(raw[:])
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode marker: %w", err)
	}
	f, err := os.OpenFile(s.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // fixed marker path under the storage root
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrTrustAnchorMarkerOutstanding, s.Path)
	}
	if err != nil {
		return fmt.Errorf("create marker: %w", err)
	}
	fail := func(op string, err error) error {
		_ = f.Close()
		_ = os.Remove(s.Path) // best effort: we created this file just now
		return fmt.Errorf("%s marker: %w", op, err)
	}
	if _, err := f.Write(data); err != nil {
		return fail("write", err)
	}
	if err := f.Sync(); err != nil {
		return fail("fsync", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(s.Path) // best effort: we created this file just now
		return fmt.Errorf("close marker: %w", err)
	}
	if err := s.syncDir()(s.Path); err != nil {
		_ = os.Remove(s.Path) // best effort: we created this file just now
		return fmt.Errorf("fsync marker dir: %w", err)
	}
	s.nonce = m.Nonce
	return nil
}

// ClearMarker implements TrustAnchorMarkerStore. Clearing an absent marker is
// not an error. It removes the marker only when its nonce matches the one this
// store wrote, then fsyncs the parent directory and propagates any failure.
func (s *FileTrustAnchorMarkerStore) ClearMarker(_ context.Context) error {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear marker: read: %w", err)
	}
	var cur TrustAnchorMarker
	if err := json.Unmarshal(data, &cur); err != nil || s.nonce == "" || cur.Nonce != s.nonce {
		return fmt.Errorf("clear marker: %w: marker belongs to another request", ErrTrustAnchorMarkerOutstanding)
	}
	if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear marker: %w", err)
	}
	s.nonce = ""
	if err := s.syncDir()(s.Path); err != nil {
		return fmt.Errorf("clear marker: fsync parent dir: %w", err)
	}
	return nil
}

// OutstandingTrustAnchorMarker reports the indeterminate marker left in
// storageRoot by an interrupted or failed mutation, or nil when none exists.
// It is read-only; operators (or a future doctor check) use it to see that a
// trust-anchor mutation needs manual inspection.
func OutstandingTrustAnchorMarker(storageRoot string) (*TrustAnchorMarker, error) {
	data, err := os.ReadFile(filepath.Join(storageRoot, TrustAnchorMarkerFile)) //nolint:gosec // fixed file name under the storage root
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // absence of a marker is the healthy state
	}
	if err != nil {
		return nil, fmt.Errorf("read marker: %w", err)
	}
	var m TrustAnchorMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse marker: %w", err)
	}
	return &m, nil
}
