// Package gateproof provides an HMAC-authenticated, domain-separated proof
// envelope for backlogit's formal gate evidence (106-F F1).
//
// The envelope binds every field that matters for trust and replay resistance
// inside a single MAC computed over the canonical serialization
// (internal/canonical.Canonicalize) of the envelope: a protocol magic constant,
// a purpose discriminator (task vs shipment) so a proof cannot be replayed
// across purposes, a schema version so unknown payload shapes are rejected
// rather than partially trusted, the MAC algorithm identifier, the key
// identifier (so a key cannot be silently swapped), the workspace identity,
// the item/shipment identity, event-pinning fields, the validated report
// digest, a monotonic per-item counter for rollback/duplicate detection, and
// (for shipment proofs only) a manifest digest.
//
// Sign and Verify each return a single error — never a dual-return outcome
// enum — so callers cannot accidentally ignore a "maybe invalid" result.
// Verify distinguishes ErrProofInvalid (definitively wrong: structural
// violation, malformed MAC, or HMAC mismatch) from ErrProofUnverifiable
// (the proof could not be evaluated at all, e.g. canonicalization failed).
package gateproof

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/canonical"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Protocol constants. Magic prevents cross-protocol reuse of a MAC computed
// for an unrelated purpose; Schema is the current supported envelope version.
const (
	Magic           = "backlogit.gate-evidence.v1"
	PurposeTask     = "task"
	PurposeShipment = "shipment"
	Schema          = 1
	AlgHMACSHA256   = "HMAC-SHA256"

	// minKeyBytes mirrors config.ResolveFormalGateKey's minimum so a key
	// resolved outside that helper (e.g. in a future rotation path) still
	// cannot silently weaken the MAC.
	minKeyBytes = 32
)

// Envelope is the fixed structure signed and verified as a single unit. Every
// field is bound inside the MAC; only the MAC bytes themselves are ever
// persisted outside the envelope.
type Envelope struct {
	Magic       string `json:"magic"`
	Purpose     string `json:"purpose"` // PurposeTask | PurposeShipment
	Schema      int    `json:"schema"`
	Alg         string `json:"alg"`
	KeyID       string `json:"key_id"`
	WorkspaceID string `json:"workspace_id"`
	ItemID      string `json:"item_id"`
	EventType   string `json:"event_type"`
	Ran         bool   `json:"ran"`
	Actor       string `json:"actor"`
	// TimestampUTC is audit data only. It is bound inside the MAC for
	// tamper-evidence but is NEVER used for ordering or replay decisions —
	// the monotonic Counter is the sole ordering signal.
	TimestampUTC string `json:"timestamp_utc"`
	HeadSHA      string `json:"head_sha"`
	ReportDigest string `json:"report_digest"`
	Counter      int64  `json:"counter"`
	// ManifestDigest is REQUIRED when Purpose == PurposeShipment and FORBIDDEN
	// when Purpose == PurposeTask.
	ManifestDigest string `json:"manifest_digest,omitempty"`
}

// validate enforces the envelope's structural contract: known magic, known
// purpose, known schema, and the purpose-conditional manifest_digest rule.
// A violation here means the envelope is definitively not a valid proof
// shape, distinct from an HMAC mismatch.
func (e Envelope) validate() error {
	if e.Magic != Magic {
		return fmt.Errorf("%w: unknown magic %q", bkerrors.ErrProofInvalid, e.Magic)
	}
	if e.Schema != Schema {
		return fmt.Errorf("%w: unknown schema %d", bkerrors.ErrProofInvalid, e.Schema)
	}
	switch e.Purpose {
	case PurposeTask:
		if e.ManifestDigest != "" {
			return fmt.Errorf("%w: manifest_digest must not be set for purpose=task", bkerrors.ErrProofInvalid)
		}
	case PurposeShipment:
		if e.ManifestDigest == "" {
			return fmt.Errorf("%w: manifest_digest is required for purpose=shipment", bkerrors.ErrProofInvalid)
		}
	default:
		return fmt.Errorf("%w: unknown purpose %q", bkerrors.ErrProofInvalid, e.Purpose)
	}
	return nil
}

// canonicalMap converts the envelope into the map[string]any shape
// internal/canonical.Canonicalize accepts. Field presence mirrors validate's
// purpose-conditional manifest_digest rule so the signed bytes never carry a
// field the purpose forbids.
func (e Envelope) canonicalMap() map[string]any {
	m := map[string]any{
		"magic":         e.Magic,
		"purpose":       e.Purpose,
		"schema":        e.Schema,
		"alg":           e.Alg,
		"key_id":        e.KeyID,
		"workspace_id":  e.WorkspaceID,
		"item_id":       e.ItemID,
		"event_type":    e.EventType,
		"ran":           e.Ran,
		"actor":         e.Actor,
		"timestamp_utc": e.TimestampUTC,
		"head_sha":      e.HeadSHA,
		"report_digest": e.ReportDigest,
		"counter":       e.Counter,
	}
	if e.Purpose == PurposeShipment {
		m["manifest_digest"] = e.ManifestDigest
	}
	return m
}

// Sign computes the hex-encoded HMAC-SHA256 MAC over the canonical
// serialization of env, keyed by key. It returns ErrProofInvalid if env fails
// structural validation (unknown magic/schema/purpose, or a
// manifest_digest/purpose mismatch) and ErrProofUnverifiable if the envelope
// cannot be canonicalized. The key must be at least minKeyBytes; a shorter key
// is rejected rather than silently accepted with reduced security margin.
func Sign(env Envelope, key []byte) (string, error) {
	if err := env.validate(); err != nil {
		return "", err
	}
	if len(key) < minKeyBytes {
		return "", fmt.Errorf("%w: key must be at least %d bytes, got %d", bkerrors.ErrProofUnverifiable, minKeyBytes, len(key))
	}
	payload, err := canonical.Canonicalize(env.canonicalMap())
	if err != nil {
		return "", fmt.Errorf("%w: canonicalize envelope: %v", bkerrors.ErrProofUnverifiable, err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify reports whether macHex is the correct HMAC-SHA256 MAC for env under
// key. It returns nil only when the envelope is structurally valid, the
// provided MAC decodes to the correct fixed size, and hmac.Equal confirms an
// exact match. Any other outcome returns ErrProofInvalid (definitively wrong)
// or ErrProofUnverifiable (could not be evaluated).
func Verify(env Envelope, macHex string, key []byte) error {
	if err := env.validate(); err != nil {
		return err
	}
	expectedHex, err := Sign(env, key)
	if err != nil {
		return err
	}
	got, err := hex.DecodeString(macHex)
	if err != nil {
		return fmt.Errorf("%w: mac is not valid hex", bkerrors.ErrProofInvalid)
	}
	want, err := hex.DecodeString(expectedHex)
	if err != nil {
		// Sign() always returns valid hex; a failure here would be a bug in
		// this package rather than a caller error, but still fail closed.
		return fmt.Errorf("%w: internal encoding error", bkerrors.ErrProofUnverifiable)
	}
	if len(got) != sha256.Size {
		return fmt.Errorf("%w: mac has wrong length %d, want %d", bkerrors.ErrProofInvalid, len(got), sha256.Size)
	}
	if !hmac.Equal(got, want) {
		return fmt.Errorf("%w: mac mismatch", bkerrors.ErrProofInvalid)
	}
	return nil
}
