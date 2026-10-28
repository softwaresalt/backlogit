package config

import "time"

// TrustAnchorStatus is the lifecycle status of a trust anchor.
type TrustAnchorStatus string

// Trust anchor lifecycle statuses.
const (
	// TrustAnchorStatusActive marks an anchor that may verify signatures.
	TrustAnchorStatusActive TrustAnchorStatus = "active"
	// TrustAnchorStatusRevoked marks an anchor that must no longer verify.
	TrustAnchorStatusRevoked TrustAnchorStatus = "revoked"
)

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
