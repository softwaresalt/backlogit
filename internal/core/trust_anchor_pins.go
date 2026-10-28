package core

import (
	"context"
	"errors"
	"fmt"

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
)

// ExternalPinSet is an immutable set of trusted anchor fingerprints that is
// provisioned from outside the workspace (never read from workspace config).
type ExternalPinSet struct {
	fingerprints map[string]struct{}
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
	return &ExternalPinSet{fingerprints: set}
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
