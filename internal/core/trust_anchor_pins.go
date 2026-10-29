package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// Sentinel errors for the trust-anchor external-pin gate (168.012-T).
var (
	// ErrTrustPinSetAbsent reports that no external pin set was provided. The
	// trust-anchor feature is disabled and every verification fails closed.
	ErrTrustPinSetAbsent = errors.New("external trust pin set absent: trust anchors disabled (fail closed)")
	// ErrTrustAnchorNotPinned reports an anchor fingerprint missing from the
	// external pin set.
	ErrTrustAnchorNotPinned = errors.New("trust anchor fingerprint not in external pin set")
	// ErrTrustAnchorBindingUnverifiable reports a pinned anchor whose role and
	// validity window cannot be verified because the external pin carries no
	// binding for it (deny by default; 168.008-T).
	ErrTrustAnchorBindingUnverifiable = errors.New("trust anchor role/validity cannot be verified against the external pin set (fail closed)")
	// ErrTrustAnchorBindingMismatch reports an in-workspace role that differs
	// from the externally bound role, or a validity window that widens the
	// externally bound window (168.008-T).
	ErrTrustAnchorBindingMismatch = errors.New("trust anchor role/validity diverges from external binding (fail closed)")
)

// PinBinding is the externally authoritative role and validity window of one
// pinned fingerprint. Role is required. A nil NotBefore/NotAfter means the
// external authority sets no bound on that side; the workspace may only
// narrow, never widen, the window.
type PinBinding struct {
	Fingerprint string
	Role        string
	NotBefore   *time.Time
	NotAfter    *time.Time
}

// ExternalPinSet is an immutable set of trusted anchor fingerprints that is
// provisioned from outside the workspace (never read from workspace config).
type ExternalPinSet struct {
	fingerprints map[string]struct{}
	bindings     map[string]PinBinding
	tombstones   map[string]struct{}
}

// NewExternalPinSet builds a pin set from the given fingerprints. Empty
// fingerprints are ignored so they can never be pinned.
func NewExternalPinSet(fingerprints ...string) *ExternalPinSet {
	set := make(map[string]struct{}, len(fingerprints))
	for _, fp := range fingerprints {
		if fp != "" {
			set[fp] = struct{}{}
		}
	}
	return &ExternalPinSet{fingerprints: set, bindings: map[string]PinBinding{}}
}

// NewBoundExternalPinSet builds a pin set whose entries each carry the
// externally authoritative role and validity window. Bindings with an empty
// fingerprint or empty role are ignored so they can never be pinned.
func NewBoundExternalPinSet(bindings ...PinBinding) *ExternalPinSet {
	p := NewExternalPinSet()
	for _, b := range bindings {
		if b.Fingerprint == "" || b.Role == "" {
			continue
		}
		p.fingerprints[b.Fingerprint] = struct{}{}
		p.bindings[b.Fingerprint] = b
	}
	return p
}

// MergePinSets returns a new pin set containing the pins and bindings of both
// inputs; on a fingerprint collision the binding from a wins.
func MergePinSets(a, b *ExternalPinSet) *ExternalPinSet {
	out := NewExternalPinSet()
	for _, s := range []*ExternalPinSet{b, a} {
		if s == nil {
			continue
		}
		for fp := range s.fingerprints {
			out.fingerprints[fp] = struct{}{}
		}
		for fp, bd := range s.bindings {
			out.bindings[fp] = bd
		}
		if len(s.tombstones) > 0 && out.tombstones == nil {
			out.tombstones = make(map[string]struct{}, len(s.tombstones))
		}
		for fp := range s.tombstones {
			out.tombstones[fp] = struct{}{}
		}
	}
	return out
}

// Binding returns the external role/validity binding for fingerprint, if the
// pin carries one.
func (p *ExternalPinSet) Binding(fingerprint string) (PinBinding, bool) {
	if p == nil {
		return PinBinding{}, false
	}
	b, ok := p.bindings[fingerprint]
	return b, ok
}

// Contains reports whether fingerprint is pinned. A nil set contains nothing.
func (p *ExternalPinSet) Contains(fingerprint string) bool {
	if p == nil || fingerprint == "" {
		return false
	}
	_, ok := p.fingerprints[fingerprint]
	return ok
}

// ExternalPinSetProvider supplies the external pin set. Implementations read
// from a location outside the workspace (e.g. environment or an operator-owned
// file) and are injected by the caller. A provider reporting "no pin set" may
// return (nil, nil) or an error wrapping ErrTrustPinSetAbsent.
type ExternalPinSetProvider interface {
	ExternalPinSet(ctx context.Context) (*ExternalPinSet, error)
}

// TrustAnchorPinError is the typed error for an anchor rejected by the pin gate.
type TrustAnchorPinError struct {
	// Index is the zero-based position of the rejected anchor.
	Index int
	// ID is the rejected anchor id.
	ID string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *TrustAnchorPinError) Error() string {
	return fmt.Sprintf("trust anchor %q (index %d): %v", e.ID, e.Index, e.Err)
}

// Unwrap returns the sentinel category.
func (e *TrustAnchorPinError) Unwrap() error { return e.Err }

// VerifyTrustAnchorPins enforces that every anchor fingerprint is present in
// the external pin set. It fails closed: a nil provider, a provider error, or
// an absent (nil) pin set returns ErrTrustPinSetAbsent-category errors.
func VerifyTrustAnchorPins(ctx context.Context, anchors []config.TrustAnchor, provider ExternalPinSetProvider) error {
	if provider == nil {
		return ErrTrustPinSetAbsent
	}
	pins, err := provider.ExternalPinSet(ctx)
	if err != nil {
		if errors.Is(err, ErrTrustPinSetAbsent) {
			return err
		}
		return fmt.Errorf("%w: load external pin set: %w", ErrTrustPinSetAbsent, err)
	}
	if pins == nil {
		return ErrTrustPinSetAbsent
	}
	for i := range anchors {
		if !pins.Contains(anchors[i].Fingerprint) {
			return &TrustAnchorPinError{Index: i, ID: anchors[i].ID, Err: ErrTrustAnchorNotPinned}
		}
	}
	return nil
}
