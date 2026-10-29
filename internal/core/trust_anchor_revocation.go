package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// ErrRevocationAuthorityRequired reports a revoke that cannot be recorded
// authoritatively: no external revocation authority is injected. Workspace
// status is advisory, so a status-only flip never counts as a revocation
// (168.009-T). The caller must perform the surfaced external operation.
var ErrRevocationAuthorityRequired = errors.New("authoritative revocation requires an external revocation authority (fail closed)")

// RevocationRecord is one authoritative revocation request.
type RevocationRecord struct {
	AnchorID    string
	Fingerprint string
	Approver    string
	At          time.Time
}

// RevocationAuthority is the injected, non-filesystem external authority that
// durably records revocations as monotonic tombstones. Implementations must
// never allow a tombstone to be removed through this interface. The core never
// writes revocation state to any filesystem path itself.
type RevocationAuthority interface {
	// RecordRevocation durably records the tombstone; nil means it is recorded.
	RecordRevocation(ctx context.Context, rec RevocationRecord) error
	// Tombstones returns every tombstoned fingerprint the authority holds.
	Tombstones(ctx context.Context) ([]string, error)
}

// WithTombstones returns a copy of the pin set in which the given fingerprints
// are tombstoned. A tombstone is monotonic: it overrides any pin and binding,
// so the resolver denies the key as revoked regardless of workspace status.
func (p *ExternalPinSet) WithTombstones(fingerprints ...string) *ExternalPinSet {
	if p == nil {
		return nil
	}
	out := MergePinSets(p, nil)
	out.tombstones = make(map[string]struct{}, len(p.tombstones)+len(fingerprints))
	for fp := range p.tombstones {
		out.tombstones[fp] = struct{}{}
	}
	for _, fp := range fingerprints {
		if fp != "" {
			out.tombstones[fp] = struct{}{}
		}
	}
	return out
}

// Tombstoned reports whether fingerprint has been revoked externally.
func (p *ExternalPinSet) Tombstoned(fingerprint string) bool {
	if p == nil || fingerprint == "" {
		return false
	}
	_, ok := p.tombstones[fingerprint]
	return ok
}

// ResolveVerificationKeyAuthoritative resolves like ResolveVerificationKey but
// always applies the external tombstones held by authority first, so a
// tombstoned key is untrusted even when workspace status says active. It fails
// closed on an authority error. A nil authority is allowed only to mean that
// no tombstone source is configured (the shipped CLI wires none); callers that
// have an authority must pass it.
func ResolveVerificationKeyAuthoritative(ctx context.Context, anchors []config.TrustAnchor, sel KeySelector, at time.Time, pins *ExternalPinSet, authority RevocationAuthority) (ResolvedKey, error) {
	applied, err := ApplyRevocations(ctx, pins, authority)
	if err != nil {
		return ResolvedKey{}, &TrustAnchorResolveError{ID: sel.ID, Role: sel.Role, At: at, Err: err}
	}
	return ResolveVerificationKey(anchors, sel, at, applied)
}

// ApplyRevocations returns pins with every tombstone held by auth applied, so
// the resolver consults the same authority revoke writes to. It fails closed:
// an authority error is returned and no pin set is produced.
func ApplyRevocations(ctx context.Context, pins *ExternalPinSet, auth RevocationAuthority) (*ExternalPinSet, error) {
	if pins == nil || auth == nil {
		return pins, nil
	}
	fps, err := auth.Tombstones(ctx)
	if err != nil {
		return nil, fmt.Errorf("load external revocations: %w", err)
	}
	return pins.WithTombstones(fps...), nil
}
