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
	// ErrTrustAnchorRoleMismatch reports an anchor whose role differs from the
	// purpose the key was requested for (cross-protocol key reuse).
	ErrTrustAnchorRoleMismatch = errors.New("trust anchor role does not match requested purpose")
	// ErrTrustAnchorAlgoMismatch reports a payload-declared algorithm that
	// differs from the anchor pinned algorithm (including "none").
	ErrTrustAnchorAlgoMismatch = errors.New("declared algorithm does not match trust anchor pinned algorithm")
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
	// Purpose, when set, is the role the caller needs the key for (for
	// example attestation-signer or token-issuer). The resolved anchor role
	// must equal it.
	Purpose string
}

// VerifyDeclaredAlgo enforces the pinned-algorithm binding. A payload- or
// caller-declared algorithm must equal the anchor pinned algorithm; anything
// else, including "none", is rejected. An empty declaration means the caller
// takes the algorithm from the key.
func (k ResolvedKey) VerifyDeclaredAlgo(declared string) error {
	if declared == "" || declared == k.Algo {
		return nil
	}
	return &TrustAnchorResolveError{ID: k.Anchor.ID, Role: k.Anchor.Role, Err: fmt.Errorf("%w: declared %q, pinned %q", ErrTrustAnchorAlgoMismatch, declared, k.Algo)}
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
// at time at against the injected external pin set pins (168.007-T). The pin
// set is authoritative: a nil pin set fails closed with ErrTrustPinSetAbsent
// and an unpinned fingerprint is never trusted. Workspace status is advisory
// (it can revoke, never re-activate). It performs no I/O. A selector id takes precedence over a role;
// a role-only selector returns the first anchor with that role whose
// not_before/not_after window contains at (bounds inclusive). When no anchor
// of the role is inside its window the first window error is returned.
func ResolveVerificationKey(anchors []config.TrustAnchor, sel KeySelector, at time.Time, pins *ExternalPinSet) (ResolvedKey, error) {
	fail := func(id string, err error) (ResolvedKey, error) {
		return ResolvedKey{}, &TrustAnchorResolveError{ID: id, Role: sel.Role, At: at, Err: err}
	}
	if pins == nil {
		// Fail closed: without an external pin set nothing can be trusted.
		return fail(sel.ID, ErrTrustPinSetAbsent)
	}
	switch {
	case sel.ID != "":
		for i := range anchors {
			if anchors[i].ID != sel.ID {
				continue
			}
			if sel.Purpose != "" && anchors[i].Role != sel.Purpose {
				return fail(sel.ID, ErrTrustAnchorRoleMismatch)
			}
			if err := checkAnchorUsable(&anchors[i], at, pins); err != nil {
				return fail(sel.ID, err)
			}
			return newResolvedKey(&anchors[i]), nil
		}
		return fail(sel.ID, ErrTrustAnchorNotFound)
	case sel.Role != "":
		if sel.Purpose != "" && sel.Role != sel.Purpose {
			return fail("", ErrTrustAnchorRoleMismatch)
		}
		var firstErr error
		var match *config.TrustAnchor
		matches := 0
		for i := range anchors {
			if anchors[i].Role != sel.Role {
				continue
			}
			err := checkAnchorUsable(&anchors[i], at, pins)
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

// checkAnchorUsable applies the trust decision for one anchor. The external
// pin set is authoritative: an anchor whose fingerprint is not pinned is never
// trusted, whatever its in-workspace status says. The in-workspace status is
// advisory and can only restrict (revoked is honored), never grant trust, so
// revocation is monotonic. The validity window is applied last.
func checkAnchorUsable(a *config.TrustAnchor, at time.Time, pins *ExternalPinSet) error {
	if a.Status == config.TrustAnchorStatusRevoked {
		return ErrTrustAnchorRevoked
	}
	if !pins.Contains(a.Fingerprint) {
		return ErrTrustAnchorNotPinned
	}
	if err := checkAnchorBinding(a, pins); err != nil {
		return err
	}
	return checkAnchorWindow(a, at)
}

// checkAnchorBinding binds the in-workspace role and validity window to the
// external pin (168.008-T). It fails closed: a pin without a binding cannot
// verify them, a differing role is rejected, and a window that is wider than
// the external window (later not_after, earlier not_before, or an absent
// bound where the external authority sets one) is rejected. The workspace may
// only narrow the window.
func checkAnchorBinding(a *config.TrustAnchor, pins *ExternalPinSet) error {
	b, ok := pins.Binding(a.Fingerprint)
	if !ok {
		return ErrTrustAnchorBindingUnverifiable
	}
	if a.Role != b.Role {
		return fmt.Errorf("%w: role %q, external %q", ErrTrustAnchorBindingMismatch, a.Role, b.Role)
	}
	if b.NotAfter != nil && (a.NotAfter == nil || a.NotAfter.After(*b.NotAfter)) {
		return fmt.Errorf("%w: not_after extends external window", ErrTrustAnchorBindingMismatch)
	}
	if b.NotBefore != nil && (a.NotBefore == nil || a.NotBefore.Before(*b.NotBefore)) {
		return fmt.Errorf("%w: not_before widens external window", ErrTrustAnchorBindingMismatch)
	}
	return nil
}
