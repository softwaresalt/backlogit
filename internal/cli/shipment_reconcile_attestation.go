package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/core"
)

// maxReconcileAttestationBytes bounds how much of an --attestation file is read.
const maxReconcileAttestationBytes = 1 << 20

const (
	reconcileAttestationNotEvaluated = "not_evaluated"
	reconcileAttestationVerified     = "verified"
)

// reconcileAttestation is the CLI-side state of an opt-in --attestation run
// (169.004-T). The claim is derived from the parsed envelope and the resolved
// anchor only; it is recorded and compared on replay, and the verifier proves
// it against the reconcile-computed evidence before any write.
type reconcileAttestation struct {
	claim   core.ShipmentReconciledShippedAttestation
	parsed  core.ParsedAttestation
	key     core.ResolvedKey
	public  *core.VerificationKey
	outcome string
}

// reconcileAttestationLoadError reports an attestation that could not be
// loaded or trusted. KeyID is the signing key the envelope named, empty when
// the failure happened before a key was identified.
type reconcileAttestationLoadError struct {
	KeyID string
	Err   error
}

func (e *reconcileAttestationLoadError) Error() string { return e.Err.Error() }

func (e *reconcileAttestationLoadError) Unwrap() error { return e.Err }

// loadReconcileAttestation reads and parses the attestation file, resolves the
// signing key through the trust-anchor primitives and binds it to the external
// pin set. It performs no workspace writes and fails closed at every step with
// a *reconcileAttestationLoadError.
func loadReconcileAttestation(ctx context.Context, cwd, path string) (*reconcileAttestation, error) {
	keyID := ""
	fail := func(err error) (*reconcileAttestation, error) {
		return nil, &reconcileAttestationLoadError{KeyID: keyID, Err: err}
	}
	data, err := readReconcileAttestationFile(path)
	if err != nil {
		return fail(fmt.Errorf("read attestation %q: %w", path, err))
	}
	parsed, err := core.ParseAttestation(data)
	if err != nil {
		return fail(fmt.Errorf("parse attestation %q: %w", path, err))
	}
	if len(parsed.Envelope.Signatures) == 0 {
		return fail(fmt.Errorf("attestation %q carries no signature: %w", path, core.ErrAttestationSignatureInvalid))
	}
	keyID = parsed.Envelope.Signatures[0].KeyID
	anchors, err := loadTrustAnchors(ctx, cwd)
	if err != nil {
		return fail(fmt.Errorf("load trust anchors for attestation: %w", err))
	}
	pins := loadPins(ctx)
	selector := core.KeySelector{ID: keyID, Purpose: core.AttestationSignerRole}
	key, err := core.ResolveVerificationKeyAuthoritative(ctx, anchors, selector, time.Now().UTC(), pins, trustRevocationAuthority())
	if err != nil {
		return fail(fmt.Errorf("resolve attestation signing key: %w", err))
	}
	public, err := core.LoadVerificationKey(cwd, key.Anchor)
	if err != nil {
		return fail(fmt.Errorf("load attestation signing key: %w", err))
	}
	if err := core.VerifyVerificationKeyPin(public, key.Anchor, pins); err != nil {
		return fail(fmt.Errorf("bind attestation signing key to external pin: %w", err))
	}
	return &reconcileAttestation{
		claim: core.ShipmentReconciledShippedAttestation{
			KeyID:           key.Anchor.ID,
			SignerIdentity:  parsed.Statement.Predicate.SignerIdentity,
			SigAlgo:         key.Algo,
			StatementDigest: canonical.HashBytes(parsed.PayloadBytes),
			Envelope:        string(data),
		},
		parsed:  parsed,
		key:     key,
		public:  public,
		outcome: reconcileAttestationNotEvaluated,
	}, nil
}

// auditReconcileAttestationLoadFailure persists an attestation_rejected audit
// event for a live run whose attestation failed to load, before any reconcile
// work. The original load error is always returned; an audit failure is joined.
func auditReconcileAttestationLoadFailure(ctx context.Context, cwd, shipmentID string, loadErr error) (retErr error) {
	keyID := ""
	var le *reconcileAttestationLoadError
	if errors.As(loadErr, &le) {
		keyID = le.KeyID
	}
	ws, err := core.NewWorkspace(ctx, cwd)
	if err != nil {
		return errors.Join(loadErr, fmt.Errorf("audit attestation rejection: open workspace: %w", err))
	}
	defer func() {
		if cerr := ws.Close(); cerr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close workspace: %w", cerr))
		}
	}()
	return core.AuditAttestationRejection(ctx, ws, shipmentID, keyID, loadErr)
}

func readReconcileAttestationFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // operator-supplied attestation path; read-only and size-bounded
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(f, maxReconcileAttestationBytes+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(data) > maxReconcileAttestationBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxReconcileAttestationBytes)
	}
	return data, nil
}

// verifier returns the core.ShipmentShippedReconcileRequest.AttestationVerifier
// hook. It only verifies: a live run's rejection audit is persisted by
// rejectionAuditor, which core invokes with the lock-holding context.
func (a *reconcileAttestation) verifier() func(core.AttestationExpectedEvidence) error {
	return func(expected core.AttestationExpectedEvidence) error {
		in := core.AttestationVerifyInput{
			Parsed:    a.parsed,
			Key:       a.key,
			PublicKey: a.public,
			Expected:  expected,
			At:        time.Now().UTC(),
		}
		verified, err := core.VerifyAttestation(in)
		if err != nil {
			return fmt.Errorf("attestation verification failed: %w", err)
		}
		if verified.KeyID != a.claim.KeyID || verified.SignerIdentity != a.claim.SignerIdentity ||
			verified.SigAlgo != a.claim.SigAlgo || verified.StatementDigest != a.claim.StatementDigest {
			return fmt.Errorf("attestation verification failed: verified attestation differs from the recorded claim: %w", core.ErrAttestationEvidenceMismatch)
		}
		a.outcome = reconcileAttestationVerified
		return nil
	}
}

// summary is the JSON object printed alongside the reconcile result.
func (a *reconcileAttestation) summary() map[string]any {
	return map[string]any{
		"outcome":          a.outcome,
		"key_id":           a.claim.KeyID,
		"signer_identity":  a.claim.SignerIdentity,
		"sig_algo":         a.claim.SigAlgo,
		"statement_digest": a.claim.StatementDigest,
	}
}

// rejectionAuditor returns the core.ShipmentShippedReconcileRequest.AttestationRejectionAuditor
// hook, persisting an attestation_rejected event through the context core
// supplies. ws is resolved lazily because the workspace opens after the
// attestation is loaded.
func (a *reconcileAttestation) rejectionAuditor(ws func() *core.Workspace, shipmentID string) func(context.Context, error) error {
	return func(ctx context.Context, rejection error) error {
		return core.AuditAttestationRejection(ctx, ws(), shipmentID, a.claim.KeyID, rejection)
	}
}
