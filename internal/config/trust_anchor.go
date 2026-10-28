package config

import (
	"encoding/pem"
	"path"
	"regexp"
	"strings"
	"time"
)

// TrustAnchorStatus is the lifecycle status of a trust anchor.
type TrustAnchorStatus string

// Trust anchor lifecycle statuses.
const (
	// TrustAnchorStatusActive marks an anchor that may verify signatures.
	TrustAnchorStatusActive TrustAnchorStatus = "active"
	// TrustAnchorStatusRevoked marks an anchor that must no longer verify.
	TrustAnchorStatusRevoked TrustAnchorStatus = "revoked"
)

// Allowed trust anchor signature algorithms.
const (
	// TrustAnchorAlgoEd25519 is the Ed25519 signature algorithm.
	TrustAnchorAlgoEd25519 = "ed25519"
	// TrustAnchorAlgoECDSAP256 is the ECDSA over NIST P-256 algorithm.
	TrustAnchorAlgoECDSAP256 = "ecdsa-p256"
)

// ValidateTrustAnchors validates a trust_anchors list: id uniqueness and the
// algo allowlist. It returns the first violation as a
// *TrustAnchorValidationError wrapping a sentinel category. An empty list is
// valid. Filesystem-material and pin policy live in internal/core.
func ValidateTrustAnchors(anchors []TrustAnchor) error {
	seen := make(map[string]struct{}, len(anchors))
	for i := range anchors {
		a := &anchors[i]
		if _, dup := seen[a.ID]; dup {
			return &TrustAnchorValidationError{Index: i, ID: a.ID, Field: "id", Err: ErrTrustAnchorDuplicateID}
		}
		seen[a.ID] = struct{}{}
		if a.Algo != TrustAnchorAlgoEd25519 && a.Algo != TrustAnchorAlgoECDSAP256 {
			return &TrustAnchorValidationError{Index: i, ID: a.ID, Field: "algo", Err: ErrTrustAnchorDisallowedAlgo}
		}
		if err := validatePublicKeyRef(a.PublicKeyRef); err != nil {
			return &TrustAnchorValidationError{Index: i, ID: a.ID, Field: "public_key_ref", Err: err}
		}
	}
	return nil
}

var refSchemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*:`)

// validatePublicKeyRef enforces the STRUCTURAL public_key_ref contract (168.018-T):
// exactly an inline PEM "PUBLIC KEY" block or a workspace-contained relative
// path. It performs no filesystem access; existence, readability, and pin/trust
// policy belong to internal/core (168.003-T/168.005-T).
func validatePublicKeyRef(ref string) error {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ErrTrustAnchorRefEmpty
	}
	if strings.HasPrefix(trimmed, "-----BEGIN") {
		block, rest := pem.Decode([]byte(trimmed))
		if block == nil || block.Type != "PUBLIC KEY" || len(block.Bytes) == 0 || strings.TrimSpace(string(rest)) != "" {
			return ErrTrustAnchorRefMalformed
		}
		return nil
	}
	if strings.ContainsAny(trimmed, "\r\n\x00") {
		return ErrTrustAnchorRefMalformed
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) || refSchemeRe.MatchString(trimmed) {
		return ErrTrustAnchorRefDisallowedScheme
	}
	for _, seg := range strings.FieldsFunc(trimmed, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return ErrTrustAnchorRefDisallowedScheme
		}
	}
	if strings.HasSuffix(trimmed, "/") || strings.HasSuffix(trimmed, `\`) || path.Clean(strings.ReplaceAll(trimmed, `\`, "/")) == "." {
		return ErrTrustAnchorRefMalformed
	}
	return nil
}

// TrustAnchor declares one verification-key trust anchor
// (workspace config key trust_anchors). This is a declaration only: parsing
// validation lives in the trust_anchors loader (168.011-T/168.018-T) and
// external-pin policy lives in internal/core (168.012-T). The YAML tags on
// every field are owned by this declaration (168.001-T).
type TrustAnchor struct {
	ID           string            `yaml:"id"`
	Role         string            `yaml:"role"`
	Algo         string            `yaml:"algo"`
	PublicKeyRef string            `yaml:"public_key_ref"`
	Fingerprint  string            `yaml:"fingerprint"`
	Status       TrustAnchorStatus `yaml:"status"`
	NotBefore    *time.Time        `yaml:"not_before,omitempty"`
	NotAfter     *time.Time        `yaml:"not_after,omitempty"`
}
