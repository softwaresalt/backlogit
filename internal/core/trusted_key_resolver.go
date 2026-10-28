package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// Sentinel errors for the trusted-key resolver (168.003-T).
var (
	// ErrKeySelectorInvalid reports a selector with neither id nor role.
	ErrKeySelectorInvalid = errors.New("key selector requires an id or a role")
	// ErrTrustAnchorNotFound reports that no anchor matches the selector.
	ErrTrustAnchorNotFound = errors.New("no trust anchor matches selector")
	// ErrTrustAnchorNotYetValid reports an anchor whose not_before is after
	// the resolution time.
	ErrTrustAnchorNotYetValid = errors.New("trust anchor not yet valid")
	// ErrTrustAnchorExpired reports an anchor whose not_after is before the
	// resolution time.
	ErrTrustAnchorExpired = errors.New("trust anchor expired")
	// ErrTrustAnchorRevoked reports an anchor whose status is revoked.
	ErrTrustAnchorRevoked = errors.New("trust anchor revoked")
	// ErrTrustAnchorAmbiguous reports a role selector matching more than one
	// non-revoked anchor inside its validity window.
	ErrTrustAnchorAmbiguous = errors.New("trust anchor role is ambiguous: multiple active matches")
)

// TrustAnchorResolveError is the typed error returned by the resolver.
type TrustAnchorResolveError struct {
	// ID is the anchor id the error concerns (may be empty).
	ID string
	// Role is the requested role (may be empty).
	Role string
	// At is the resolution time.
	At time.Time
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *TrustAnchorResolveError) Error() string {
	return fmt.Sprintf("resolve trust anchor (id %q, role %q) at %s: %v", e.ID, e.Role, e.At.UTC().Format(time.RFC3339), e.Err)
}

// Unwrap returns the sentinel category.
func (e *TrustAnchorResolveError) Unwrap() error { return e.Err }

// KeySelector selects a trust anchor by id and/or role.
type KeySelector struct {
	ID   string
	Role string
}

// ResolvedKey is the result of resolving a trust anchor. Key material is
// loaded separately (LoadVerificationKey) so resolution stays pure.
type ResolvedKey struct {
	// Anchor is a copy of the resolved anchor declaration.
	Anchor config.TrustAnchor
	// Algo is the anchor's pinned algorithm.
	Algo string
}

// ResolveVerificationKey resolves the verification key anchor selected by sel
// at time at. It performs no I/O. A selector id takes precedence over a role;
// a role-only selector returns the first anchor with that role whose
// not_before/not_after window contains at (bounds inclusive). When no anchor
// of the role is inside its window the first window error is returned.
func ResolveVerificationKey(anchors []config.TrustAnchor, sel KeySelector, at time.Time) (ResolvedKey, error) {
	fail := func(id string, err error) (ResolvedKey, error) {
		return ResolvedKey{}, &TrustAnchorResolveError{ID: id, Role: sel.Role, At: at, Err: err}
	}
	switch {
	case sel.ID != "":
		for i := range anchors {
			if anchors[i].ID != sel.ID {
				continue
			}
			if err := checkAnchorUsable(&anchors[i], at); err != nil {
				return fail(sel.ID, err)
			}
			return newResolvedKey(&anchors[i]), nil
		}
		return fail(sel.ID, ErrTrustAnchorNotFound)
	case sel.Role != "":
		var firstErr error
		var match *config.TrustAnchor
		matches := 0
		for i := range anchors {
			if anchors[i].Role != sel.Role {
				continue
			}
			err := checkAnchorUsable(&anchors[i], at)
			if err == nil {
				if match == nil {
					match = &anchors[i]
				}
				matches++
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if matches > 1 {
			return fail("", ErrTrustAnchorAmbiguous)
		}
		if match != nil {
			return newResolvedKey(match), nil
		}
		if firstErr == nil {
			firstErr = ErrTrustAnchorNotFound
		}
		return fail("", firstErr)
	default:
		return fail("", ErrKeySelectorInvalid)
	}
}
func newResolvedKey(a *config.TrustAnchor) ResolvedKey {
	return ResolvedKey{Anchor: *a, Algo: a.Algo}
}

// checkAnchorWindow checks the not_before/not_after validity window.
func checkAnchorWindow(a *config.TrustAnchor, at time.Time) error {
	if a.NotBefore != nil && at.Before(*a.NotBefore) {
		return ErrTrustAnchorNotYetValid
	}
	if a.NotAfter != nil && at.After(*a.NotAfter) {
		return ErrTrustAnchorExpired
	}
	return nil
}

// checkAnchorUsable rejects revoked anchors, then applies the validity window.
func checkAnchorUsable(a *config.TrustAnchor, at time.Time) error {
	if a.Status == config.TrustAnchorStatusRevoked {
		return ErrTrustAnchorRevoked
	}
	return checkAnchorWindow(a, at)
}
