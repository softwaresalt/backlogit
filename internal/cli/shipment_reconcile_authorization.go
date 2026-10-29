package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/softwaresalt/backlogit/internal/core"
)

// maxReconcileAuthorizationTokenBytes bounds how much of an
// --authorization-token file is read.
const maxReconcileAuthorizationTokenBytes = 1 << 20

// loadReconcileAuthorization reads and parses the authorization token file,
// resolves the token-issuer anchor through the trust-anchor primitives and
// binds it to the external pin set (170.005-T). It performs no workspace
// writes and fails closed at every step. Verification, request-identity
// binding and nonce consumption happen later, inside the core locked section.
func loadReconcileAuthorization(ctx context.Context, cwd, path string) (*core.ReconcileAuthorization, error) {
	data, err := readReconcileAuthorizationFile(path)
	if err != nil {
		return nil, fmt.Errorf("read authorization token %q: %w", path, err)
	}
	token, err := core.ParseAuthorizationToken(data)
	if err != nil {
		return nil, fmt.Errorf("parse authorization token %q: %w", path, err)
	}
	anchors, err := loadTrustAnchors(ctx, cwd)
	if err != nil {
		return nil, fmt.Errorf("load trust anchors for authorization token: %w", err)
	}
	pins := loadPins(ctx)
	selector := core.KeySelector{ID: token.Header.KeyID, Purpose: core.TokenIssuerRole}
	key, err := core.ResolveVerificationKeyAuthoritative(ctx, anchors, selector, time.Now().UTC(), pins, trustRevocationAuthority())
	if err != nil {
		return nil, fmt.Errorf("resolve authorization token issuer key: %w", errors.Join(core.ErrTokenUntrustedIssuer, err))
	}
	public, err := core.LoadVerificationKey(cwd, key.Anchor)
	if err != nil {
		return nil, fmt.Errorf("load authorization token issuer key: %w", err)
	}
	if err := core.VerifyVerificationKeyPin(public, key.Anchor, pins); err != nil {
		return nil, fmt.Errorf("bind authorization token issuer key to external pin: %w", err)
	}
	return &core.ReconcileAuthorization{Token: token, Key: key, PublicKey: public}, nil
}

// reconcileNonceConsumer returns the atomic nonce consumer for a live
// authenticated run. The nonce authority is an external, rollback-resistant
// compare-and-consume provider (core.NonceConsumeProvider); none ships with
// the CLI, so a live authenticated run has no consumer and core fails closed
// with core.ErrNonceProviderUnavailable before any write. A dry run never
// consumes a nonce.
func reconcileNonceConsumer() *core.NonceConsumer { return nil }

func readReconcileAuthorizationFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // operator-supplied token path; read-only and size-bounded
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(f, maxReconcileAuthorizationTokenBytes+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(data) > maxReconcileAuthorizationTokenBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxReconcileAuthorizationTokenBytes)
	}
	return data, nil
}
