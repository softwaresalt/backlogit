package evidencedag

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
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

// waiverSeal is a per-process secret that binds a Waiver's fields to the
// moment WaiverService accepted it. Waiver fields are unexported, so code
// outside this package cannot forge or mutate one; the seal additionally makes
// the evaluator reject a copy whose fields were altered after acceptance.
var waiverSeal = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("evidencedag: system entropy unavailable: " + err.Error())
	}
	return b
}()

func sealWaiver(id, shipmentID, nodeID, headCommit, principal string) string {
	m := hmac.New(sha256.New, waiverSeal)
	for _, f := range []string{id, shipmentID, nodeID, headCommit, principal} {
		writeField(m, f)
	}
	return hex.EncodeToString(m.Sum(nil))
}

func newWaiver(id, shipmentID, nodeID, headCommit, principal string) Waiver {
	return Waiver{
		id: id, shipmentID: shipmentID, nodeID: nodeID, headCommit: headCommit, principal: principal,
		seal: sealWaiver(id, shipmentID, nodeID, headCommit, principal),
	}
}

// valid reports whether the waiver was produced by WaiverService and has not
// been altered since.
func (w Waiver) valid() bool {
	if w.seal == "" {
		return false
	}
	want := sealWaiver(w.id, w.shipmentID, w.nodeID, w.headCommit, w.principal)
	return hmac.Equal([]byte(want), []byte(w.seal))
}

// ID is the consumed waiver nonce.
func (w Waiver) ID() string { return w.id }

// ShipmentID is the shipment the waiver is bound to.
func (w Waiver) ShipmentID() string { return w.shipmentID }

// NodeID is the node the waiver is bound to.
func (w Waiver) NodeID() string { return w.nodeID }

// HeadCommit is the head commit the waiver is bound to.
func (w Waiver) HeadCommit() string { return w.headCommit }

// Principal is the human principal that authorized the waiver.
func (w Waiver) Principal() string { return w.principal }

// NewWaiverService builds a WaiverService over the out-of-band trust anchor,
// the append-only ledger and the external ledger anchor store. The anchor
// store is mandatory: without an anchor outside the ledger, deletion, rollback
// and forged tails cannot be detected. A nil clock uses time.Now. There is
// deliberately no issuance API here: tokens are minted out of band by a
// human-held key.
func NewWaiverService(t *Trust, l *Ledger, anchors AnchorStore, clock func() time.Time) (*WaiverService, error) {
	if t == nil || l == nil {
		return nil, errors.New("evidencedag: waiver service requires a trust anchor and a ledger")
	}
	if anchors == nil {
		return nil, ErrAnchorRequired
	}
	if clock == nil {
		clock = time.Now
	}
	return &WaiverService{trust: t, ledger: l, anchors: anchors, now: clock}, nil
}

func deny(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrWaiverDenied, fmt.Sprintf(format, args...))
}

// preparedWaiver is a token that passed every check that needs no ledger
// access. Nothing is consumed until commit.
type preparedWaiver struct {
	claims WaiverClaims
	key    TrustedKey
	node   Node
	ship   Shipment
	digest string
}

// Accept verifies a waiver token for the shipment and, only when every check
// passes, records it in the ledger and returns a verified Waiver. Every failure
// is a denial (errors.Is(err, ErrWaiverDenied)); the default is deny. Checks:
// strict bounded decoding, a trusted human-held key, a valid signature over
// the exact claims bytes, a principal equal to the key's principal (never
// self-asserted), binding to this shipment, node and head commit, a bounded
// short-lived validity window, an explicit authorization policy entry for the
// node's waiver family, and a never-before-consumed nonce. Only the "waiver"
// kind is implemented; "not-applicable" tokens are denied.
func (s *WaiverService) Accept(raw []byte, ship Shipment, lookup func(string) (Node, bool)) (Waiver, error) {
	p, err := s.prepare(raw, ship, lookup)
	if err != nil {
		return Waiver{}, err
	}
	return s.commit(p)
}

func (s *WaiverService) prepare(raw []byte, ship Shipment, lookup func(string) (Node, bool)) (preparedWaiver, error) {
	var zero preparedWaiver
	if s == nil || s.trust == nil || s.ledger == nil || s.anchors == nil || s.now == nil || lookup == nil {
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
	if !validNonce(claims.Nonce) {
		return zero, deny("nonce must be 16-64 lowercase hex characters")
	}
	switch claims.Kind {
	case claimKindWaiver:
	case claimKindNotApplicable:
		return zero, deny("the not-applicable waiver kind is not implemented")
	default:
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
	if c := claims; c.NotBefore < 0 || c.ExpiresAt > maxWaiverTimestamp || c.NotBefore >= c.ExpiresAt ||
		c.ExpiresAt-c.NotBefore > int64(MaxWaiverLifetime/time.Second) {
		return zero, deny("validity window is out of range, inverted or longer than %s", MaxWaiverLifetime)
	}
	if err := claims.checkWindow(s.now().Unix()); err != nil {
		return zero, err
	}
	family := node.WaiverAuthorityFamily()
	if !s.trust.authorized(key.Principal, family) {
		return zero, deny("principal is not authorized to waive family %s", family)
	}
	sum := sha256.Sum256(append(append([]byte{}, payload...), sig...))
	return preparedWaiver{claims: claims, key: key, node: node, ship: ship, digest: hex.EncodeToString(sum[:])}, nil
}

func (c WaiverClaims) checkWindow(now int64) error {
	if now < c.NotBefore {
		return deny("token is not yet valid")
	}
	if now >= c.ExpiresAt {
		return deny("token has expired")
	}
	return nil
}

// commit consumes the nonce in the anchored ledger. The clock is read again
// under the ledger lock, so a token that expired while waiting for the lock is
// denied.
func (s *WaiverService) commit(p preparedWaiver) (Waiver, error) {
	_, _, err := s.ledger.appendUnique(LedgerEntry{
		ShipmentID: p.ship.ID, NodeID: p.node.ID, HeadCommit: p.ship.HeadCommit,
		Principal: p.key.Principal, KeyID: p.key.KeyID, Family: string(p.node.WaiverAuthorityFamily()), Kind: p.claims.Kind,
		Nonce: p.claims.Nonce, Reason: p.claims.Reason, TokenDigest: p.digest,
	}, appendOpts{
		unique: true,
		store:  s.anchors,
		stamp: func() (int64, error) {
			now := s.now().Unix()
			return now, p.claims.checkWindow(now)
		},
	})
	if err != nil && !committedAndUsable(err) {
		if errors.Is(err, ErrNonceReplayed) || errors.Is(err, ErrWaiverDenied) {
			return Waiver{}, err
		}
		return Waiver{}, fmt.Errorf("%w: ledger: %w", ErrWaiverDenied, err)
	}
	return newWaiver(p.claims.Nonce, p.ship.ID, p.node.ID, p.ship.HeadCommit, p.key.Principal), nil
}

// committedAndUsable reports whether err is a durable commit whose only
// warnings are cosmetic (close, sidecar, lock release): the record is anchored,
// so the waiver stands. A stale anchor or a lost lock denies it.
func committedAndUsable(err error) bool {
	return errors.Is(err, ErrCommitted) && !errors.Is(err, ErrAnchorStale) && !errors.Is(err, ErrLockLost)
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
