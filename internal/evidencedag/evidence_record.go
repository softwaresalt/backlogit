package evidencedag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// RecordSchemaVersion is the evidence record envelope schema version.
const RecordSchemaVersion = 1

// validateShipment checks the shipment identity and attribute bounds.
func validateShipment(s Shipment) error {
	if !validTaskID(s.ID) {
		return fmt.Errorf("%w: shipment id is invalid", ErrInvalidInput)
	}
	if len(s.HeadCommit) > 64 || !faultline.ValidCommitSHA(s.HeadCommit) {
		return fmt.Errorf("%w: head commit must be a full lowercase commit SHA", ErrInvalidInput)
	}
	if len(s.Attributes) > MaxAttributes {
		return fmt.Errorf("%w: more than %d shipment attributes", ErrInvalidInput, MaxAttributes)
	}
	for k, v := range s.Attributes {
		if k == "" || len(k) > MaxIDLength*2 || len(v) > MaxIDLength*2 {
			return fmt.Errorf("%w: shipment attribute key or value out of bounds", ErrInvalidInput)
		}
	}
	return nil
}

// artifactDigest is the hex SHA-256 of the artifact's canonical form.
func artifactDigest(a faultline.EvidenceArtifact) (string, error) {
	c, err := a.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

// NewEvidenceRecord wraps a conformant S4 U4 artifact with its shipment, node
// and head-commit binding and a digest. It refuses artifacts that do not
// validate against the shared contract.
func NewEvidenceRecord(s Shipment, nodeID string, a faultline.EvidenceArtifact) (EvidenceRecord, error) {
	var zero EvidenceRecord
	if err := validateShipment(s); err != nil {
		return zero, err
	}
	if !validID(nodeID) {
		return zero, fmt.Errorf("%w: node id is invalid", ErrInvalidInput)
	}
	if err := faultline.Validate(a); err != nil {
		return zero, fmt.Errorf("evidencedag: artifact violates the shared evidence contract: %w", err)
	}
	digest, err := artifactDigest(a)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: canonicalize artifact: %w", err)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: marshal artifact: %w", err)
	}
	if err := rejectDuplicateKeys(a.VerifiedEvidence); err != nil {
		return zero, fmt.Errorf("evidencedag: artifact violates the shared evidence contract: %s", faultline.SanitizeDiagnostic(err.Error()))
	}
	return EvidenceRecord{
		SchemaVersion: RecordSchemaVersion, ShipmentID: s.ID, NodeID: nodeID,
		HeadCommit: s.HeadCommit, Digest: digest, Artifact: raw,
	}, nil
}
