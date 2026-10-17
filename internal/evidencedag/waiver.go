package evidencedag

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	claimKindWaiver        = "waiver"
	claimKindNotApplicable = "not-applicable"
	maxClaimsBytes         = 8 << 10
)

// NewWaiverService builds a WaiverService over the out-of-band trust anchor
// and the append-only ledger. A nil clock uses time.Now. There is deliberately
// no issuance API here: tokens are minted out of band by a human-held key.
func NewWaiverService(t *Trust, l *Ledger, clock func() time.Time) (*WaiverService, error) {
	if t == nil || l == nil {
		return nil, errors.New("evidencedag: waiver service requires a trust anchor and a ledger")
	}
	if clock == nil {
		clock = time.Now
	}
	return &WaiverService{trust: t, ledger: l, now: clock}, nil
}

func deny(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrWaiverDenied, fmt.Sprintf(format, args...))
}

// Accept verifies a waiver token for the shipment and, only when every check
// passes, records it in the ledger and returns a verified Waiver. Every failure
// is a denial (errors.Is(err, ErrWaiverDenied)); the default is deny. Checks:
// strict bounded decoding, a trusted human-held key, a valid signature over
// the exact claims bytes, a principal equal to the key's principal (never
// self-asserted), binding to this shipment, node and head commit, a bounded
// short-lived validity window, an explicit authorization policy entry for the
// node's waiver family, and a never-before-consumed nonce.
func (s *WaiverService) Accept(raw []byte, ship Shipment, lookup func(string) (Node, bool)) (Waiver, error) {
	var zero Waiver
	if s == nil || s.trust == nil || s.ledger == nil || s.now == nil || lookup == nil {
		return zero, deny("waivers are not enabled")
	}
	if err := validateShipment(ship); err != nil {
		return zero, deny("invalid shipment")
	}
	if len(raw) == 0 || len(raw) > MaxWaiverTokenBytes {
		return zero, deny("token is empty or exceeds %d bytes", MaxWaiverTokenBytes)
	}
	tok, err := decodeToken(raw)
	if err != nil {
		return zero, deny("malformed token: %s", truncate(err.Error()))
	}
	if tok.SchemaVersion != TokenSchemaVersion {
		return zero, deny("unsupported token version %d", tok.SchemaVersion)
	}
	key, ok := s.trust.keys[tok.KeyID]
	if !ok {
		return zero, deny("signing key is not trusted")
	}
	if key.Kind != KeyKindHuman {
		return zero, deny("agent or non-interactive keys cannot authorize waivers")
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(tok.Payload)
	if err != nil || len(payload) == 0 || len(payload) > maxClaimsBytes {
		return zero, deny("payload is malformed")
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(tok.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return zero, deny("signature is missing or malformed")
	}
	if !ed25519.Verify(key.PublicKey, append([]byte(waiverDomain), payload...), sig) {
		return zero, deny("signature verification failed")
	}
	claims, err := decodeClaims(payload)
	if err != nil {
		return zero, deny("malformed claims: %s", truncate(err.Error()))
	}
	if claims.Principal != key.Principal {
		return zero, deny("claimed principal does not match the signing key's principal")
	}
	if !noncePattern.MatchString(claims.Nonce) {
		return zero, deny("nonce must be 16-64 lowercase hex characters")
	}
	if claims.Kind != claimKindWaiver && claims.Kind != claimKindNotApplicable {
		return zero, deny("unknown waiver kind")
	}
	if claims.Reason == "" || len(claims.Reason) > MaxReasonBytes {
		return zero, deny("a bounded reason is required")
	}
	if claims.ShipmentID != ship.ID || claims.HeadCommit != ship.HeadCommit {
		return zero, deny("token is bound to a different shipment or head commit")
	}
	node, ok := lookup(claims.NodeID)
	if !ok {
		return zero, deny("token names a node that is not declared")
	}
	if claims.NotBefore >= claims.ExpiresAt || time.Duration(claims.ExpiresAt-claims.NotBefore)*time.Second > MaxWaiverLifetime {
		return zero, deny("validity window is inverted or longer than %s", MaxWaiverLifetime)
	}
	now := s.now().Unix()
	if now < claims.NotBefore {
		return zero, deny("token is not yet valid")
	}
	if now >= claims.ExpiresAt {
		return zero, deny("token has expired")
	}
	family := node.WaiverAuthorityFamily()
	if !s.trust.authorized(key.Principal, family) {
		return zero, deny("principal is not authorized to waive family %s", family)
	}
	sum := sha256.Sum256(append(append([]byte{}, payload...), sig...))
	_, _, err = s.ledger.appendUnique(LedgerEntry{
		AcceptedAt: now, ShipmentID: ship.ID, NodeID: node.ID, HeadCommit: ship.HeadCommit,
		Principal: key.Principal, KeyID: key.KeyID, Family: string(family), Kind: claims.Kind,
		Nonce: claims.Nonce, Reason: claims.Reason, TokenDigest: hex.EncodeToString(sum[:]),
	}, true)
	if err != nil {
		if errors.Is(err, ErrNonceReplayed) {
			return zero, err
		}
		return zero, fmt.Errorf("%w: ledger: %w", ErrWaiverDenied, err)
	}
	return Waiver{
		ID: claims.Nonce, ShipmentID: ship.ID, NodeID: node.ID, HeadCommit: ship.HeadCommit,
		Principal: key.Principal, verified: true,
	}, nil
}

func decodeToken(raw []byte) (waiverToken, error) {
	var tok waiverToken
	if err := rejectDuplicateKeys(raw); err != nil {
		return tok, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tok); err != nil {
		return tok, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return tok, errors.New("trailing data after token")
	}
	return tok, nil
}

func decodeClaims(payload []byte) (WaiverClaims, error) {
	var c WaiverClaims
	if err := rejectDuplicateKeys(payload); err != nil {
		return c, err
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return c, errors.New("trailing data after claims")
	}
	return c, nil
}
