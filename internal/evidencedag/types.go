package evidencedag

import "encoding/json"

// EvidenceRecord binds one S4 U4 evidence artifact to the shipment, node and
// head commit it was produced for. The binding and digest give anti-replay:
// a record presented for any other shipment, node or commit is rejected.
type EvidenceRecord struct {
	SchemaVersion int             `json:"schema_version"`
	ShipmentID    string          `json:"shipment_id"`
	NodeID        string          `json:"node_id"`
	HeadCommit    string          `json:"head_commit"`
	Digest        string          `json:"digest"`
	Artifact      json.RawMessage `json:"artifact"`
}

// Waiver is an authorized, authenticated waiver accepted by a WaiverService.
// It can only be constructed inside this package, so an evaluation input can
// never carry a forged waiver.
type Waiver struct {
	ID         string
	ShipmentID string
	NodeID     string
	HeadCommit string
	Principal  string
	verified   bool
}

// Input is one evaluation request.
type Input struct {
	Shipment Shipment
	Evidence map[string]EvidenceRecord
	Waivers  map[string]Waiver
}

// NodeResult is the evaluated outcome of one node.
type NodeResult struct {
	ID       string
	Family   Family
	Terminal bool
	OnUnmet  OnUnmet
	Status   Status
	Reason   string
	// Waiver is the principal that waived the node, when Status is waived.
	Waiver string
	// Key is the content digest of the node and its transitive inputs.
	Key string
}

// Report is the deterministic evaluation result, ordered by node ID.
type Report struct {
	ShipmentID string
	HeadCommit string
	Nodes      []NodeResult
	// Computed and CacheHits describe incremental evaluation effort.
	Computed  int
	CacheHits int
}

// RollupState is the shipment-level roll-up verdict.
type RollupState string

// Roll-up states.
const (
	// RollupReady means every applicable terminal node is verified or waived.
	RollupReady RollupState = "ready"
	// RollupBlocked means an applicable terminal node is unmet.
	RollupBlocked RollupState = "blocked"
	// RollupUnverifiable means the verdict cannot be established.
	RollupUnverifiable RollupState = "unverifiable"
)

// Rollup is the shipment roll-up over applicable terminal nodes.
type Rollup struct {
	State               RollupState
	ApplicableTerminals int
	Unmet               []string
	Unverifiable        []string
	Advisory            []string
	Reason              string
}
