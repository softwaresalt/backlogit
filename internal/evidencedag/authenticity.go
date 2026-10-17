package evidencedag

import (
	"errors"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// expectedCommit resolves the commit a node's evidence must be produced at.
func expectedCommit(n Node, s Shipment) string {
	if n.ProducingCommit == CommitHead {
		return s.HeadCommit
	}
	return n.ProducingCommit
}

// authenticate proves that rec is evidence for exactly this node of this
// shipment at this head commit, produced by the declared validator and task,
// and that its digest matches. It runs before applicability is considered, so
// forged or replayed evidence is an integrity failure even on a node that the
// shipment would otherwise treat as not applicable.
func authenticate(n Node, s Shipment, rec EvidenceRecord) error {
	if rec.SchemaVersion != RecordSchemaVersion {
		return fmt.Errorf("unsupported evidence record version %d", rec.SchemaVersion)
	}
	if rec.ShipmentID != s.ID || rec.NodeID != n.ID || rec.HeadCommit != s.HeadCommit {
		return errors.New("evidence is bound to a different shipment, node or head commit")
	}
	if len(rec.Artifact) == 0 {
		return errors.New("evidence record carries no artifact")
	}
	a, _, err := faultline.DecodeAndValidate(rec.Artifact)
	if err != nil {
		return fmt.Errorf("evidence does not conform to the S4 contract: %s", faultline.SanitizeDiagnostic(err.Error()))
	}
	digest, err := artifactDigest(a)
	if err != nil || digest != rec.Digest {
		return errors.New("evidence digest does not match the artifact")
	}
	if a.ProducingTask != n.ProducingTask {
		return errors.New("evidence producing task does not match the node declaration")
	}
	if a.ProducingCommit != expectedCommit(n, s) {
		return errors.New("evidence producing commit does not match the node declaration")
	}
	if a.ValidatorIdentity != n.Validator {
		return errors.New("evidence validator identity does not match the node declaration")
	}
	return nil
}

// duplicateDigests returns the node IDs whose presented evidence digest is
// also presented for another node (cross-node replay).
func duplicateDigests(ev map[string]EvidenceRecord) map[string]struct{} {
	byDigest := map[string][]string{}
	for id, r := range ev {
		if r.Digest != "" {
			byDigest[r.Digest] = append(byDigest[r.Digest], id)
		}
	}
	dup := map[string]struct{}{}
	for _, ids := range byDigest {
		if len(ids) > 1 {
			for _, id := range ids {
				dup[id] = struct{}{}
			}
		}
	}
	return dup
}
