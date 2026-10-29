package core

import (
	"context"
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
}

// TrustAnchorMarkerStore persists the indeterminate marker. WriteMarker and
// ClearMarker must be durable (fsynced) before returning.
type TrustAnchorMarkerStore interface {
	WriteMarker(ctx context.Context, m TrustAnchorMarker) error
	ClearMarker(ctx context.Context) error
}

// FileTrustAnchorMarkerStore is the file-backed marker store.
type FileTrustAnchorMarkerStore struct {
	// Path is the marker file.
	Path string
}

// WriteMarker implements TrustAnchorMarkerStore with an fsynced atomic write.
func (s FileTrustAnchorMarkerStore) WriteMarker(_ context.Context, m TrustAnchorMarker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode marker: %w", err)
	}
	if err := atomicfile.WriteFileAtomicWithOptions(s.Path, data, atomicfile.Options{DurableWrites: true}); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	return nil
}

// ClearMarker implements TrustAnchorMarkerStore. Clearing an absent marker is
// not an error.
func (s FileTrustAnchorMarkerStore) ClearMarker(_ context.Context) error {
	if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear marker: %w", err)
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
