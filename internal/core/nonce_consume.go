package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for the atomic nonce consume (170.008-T).
var (
	// ErrNonceProviderUnavailable reports a missing or failing external
	// compare-and-consume provider. The consume fails closed.
	ErrNonceProviderUnavailable = errors.New("nonce consume provider unavailable (fail closed)")
	// ErrNonceConsumeIndeterminate reports a consume that definitively
	// succeeded at the provider but whose audit or projection write failed.
	ErrNonceConsumeIndeterminate = errors.New("nonce consumed but audit missing (indeterminate)")
)

// NonceConsumeProvider is the injected external, rollback-resistant
// compare-and-swap authority (for example a monotonic counter service). It is
// not a workspace file: implementations live outside the workspace and the
// workspace never writes outside its root on their behalf.
type NonceConsumeProvider interface {
	// CompareAndConsumeNonce atomically checks and consumes nonce. It returns
	// an error wrapping ErrNonceReplayed when nonce was already consumed and
	// any other error when the authority cannot decide.
	CompareAndConsumeNonce(ctx context.Context, nonce string, notAfter time.Time) error
}

// NonceConsumeEvent is the audit record of a successful consume.
type NonceConsumeEvent struct {
	// Nonce is the consumed nonce.
	Nonce string
	// NotAfter is the token expiry the nonce belongs to.
	NotAfter time.Time
	// ConsumedAt is the consume time.
	ConsumedAt time.Time
	// RequestIdentityDigest is the request-identity digest the nonce was
	// consumed against; empty for an unbound consume.
	RequestIdentityDigest string
}

// NonceConsumeAuditor persists consume audit events.
type NonceConsumeAuditor interface {
	// RecordNonceConsume persists ev or returns an error.
	RecordNonceConsume(ev NonceConsumeEvent) error
}

// NonceConsumeIndeterminateError is the typed indeterminate outcome: the nonce
// is consumed and stays consumed, but the audit trail has an operator-visible
// gap.
type NonceConsumeIndeterminateError struct {
	// Nonce is the consumed nonce.
	Nonce string
	// Cause is the audit or projection write failure.
	Cause error
}

// Error implements error.
func (e *NonceConsumeIndeterminateError) Error() string {
	return fmt.Sprintf("nonce %q consumed but audit missing: %v", e.Nonce, e.Cause)
}

// Unwrap exposes the category and the cause.
func (e *NonceConsumeIndeterminateError) Unwrap() []error {
	return []error{ErrNonceConsumeIndeterminate, e.Cause}
}

// NonceConsumer performs the single authoritative atomic nonce consume. The
// presence check and the consume are one provider compare-and-swap, so
// in-workspace projection deletion or truncation cannot re-enable replay and
// concurrent consumers cannot both succeed.
type NonceConsumer struct {
	provider   NonceConsumeProvider
	projection *NonceLedger
	auditor    NonceConsumeAuditor
	now        func() time.Time
}

// NewNonceConsumer builds a consumer. A nil provider fails closed. The
// projection is the contained in-workspace record and the auditor persists the
// consume audit; both are required.
func NewNonceConsumer(provider NonceConsumeProvider, projection *NonceLedger, auditor NonceConsumeAuditor) (*NonceConsumer, error) {
	if provider == nil {
		return nil, fmt.Errorf("new nonce consumer: %w", ErrNonceProviderUnavailable)
	}
	if projection == nil || auditor == nil {
		return nil, fmt.Errorf("new nonce consumer: projection and auditor are required: %w", ErrNonceConsumeIndeterminate)
	}
	return &NonceConsumer{provider: provider, projection: projection, auditor: auditor, now: time.Now}, nil
}

// Consume atomically consumes nonce. It returns a NonceLedgerError wrapping
// ErrNonceReplayed for an already-consumed nonce, ErrNonceProviderUnavailable
// when the provider cannot decide, and NonceConsumeIndeterminateError when the
// consume succeeded but the audit or projection write failed. A successful
// consume is never rolled back.
func (c *NonceConsumer) Consume(ctx context.Context, nonce string, notAfter time.Time) error {
	return c.ConsumeBound(ctx, nonce, notAfter, "")
}

// ConsumedBinding returns the request-identity digest the projection records
// for nonce. An absent record or an unbound consume yields an empty digest,
// which callers must treat as unprovable (fail closed).
func (c *NonceConsumer) ConsumedBinding(nonce string) (string, error) {
	digest, found, err := c.projection.BoundDigest(nonce)
	if err != nil {
		return "", fmt.Errorf("read nonce binding: %w", err)
	}
	if !found {
		return "", nil
	}
	return digest, nil
}

// ConsumeBound is Consume that records requestIdentityDigest, the digest the
// nonce is consumed against, in the projection and the consume audit.
func (c *NonceConsumer) ConsumeBound(ctx context.Context, nonce string, notAfter time.Time, requestIdentityDigest string) error {
	if nonce == "" {
		return &NonceLedgerError{Err: ErrNonceInvalid}
	}
	if err := c.provider.CompareAndConsumeNonce(ctx, nonce, notAfter); err != nil {
		if errors.Is(err, ErrNonceReplayed) {
			return &NonceLedgerError{Nonce: nonce, Err: ErrNonceReplayed}
		}
		return &NonceLedgerError{Nonce: nonce, Err: fmt.Errorf("%w: %w", ErrNonceProviderUnavailable, err)}
	}
	// From here the nonce is definitively consumed; failures never undo it.
	if err := c.projection.RecordBound(nonce, notAfter, requestIdentityDigest); err != nil && !errors.Is(err, ErrNonceReplayed) {
		return &NonceConsumeIndeterminateError{Nonce: nonce, Cause: fmt.Errorf("projection: %w", err)}
	}
	ev := NonceConsumeEvent{Nonce: nonce, NotAfter: notAfter.UTC(), ConsumedAt: c.now().UTC(), RequestIdentityDigest: requestIdentityDigest}
	if err := c.auditor.RecordNonceConsume(ev); err != nil {
		return &NonceConsumeIndeterminateError{Nonce: nonce, Cause: fmt.Errorf("audit: %w", err)}
	}
	return nil
}
