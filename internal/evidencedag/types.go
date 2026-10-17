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
// Its fields are unexported and sealed at acceptance: it can only be produced
// by WaiverService, and the evaluator rejects any copy altered afterwards, so
// an evaluation input can never carry a forged or retargeted waiver.
type Waiver struct {
	id         string
	shipmentID string
	nodeID     string
	headCommit string
	principal  string
	// family is the authorized waiver family and nodeDigest the digest of the
	// node definition the waiver was authorized for; both are sealed.
	family     string
	nodeDigest string
	seal       string
}

// Input is one evaluation request.
type Input struct {
	Shipment Shipment
	Evidence map[string]EvidenceRecord
	Waivers  map[string]Waiver
	// Failures maps a node ID to a reason when its executor could not produce
	// evidence; such an applicable node evaluates as unverifiable.
	Failures map[string]string
}

// NodeResult is the evaluated outcome of one node.
type NodeResult struct {
	ID       string `json:"id"`
	Family   Family `json:"family"`
	Terminal bool   `json:"terminal"`
	// OnUnmet is the effective blocking semantics: the node's declared value
	// strengthened to block when an unmet upstream node blocks.
	OnUnmet OnUnmet `json:"on_unmet"`
	Status  Status  `json:"status"`
	Reason  string  `json:"reason"`
	// Waiver is the principal that waived the node, when Status is waived.
	Waiver string `json:"waiver,omitempty"`
	// MaskedUpstreams lists the direct upstream nodes that were unmet or
	// unverifiable but not consulted because this node is waived: a waiver
	// covers its node as a whole.
	MaskedUpstreams []string `json:"masked_upstreams,omitempty"`
	// Key is the content digest of the node and its transitive inputs.
	Key string `json:"key"`
}

// Report is the deterministic evaluation result, ordered by node ID.
type Report struct {
	ShipmentID string       `json:"shipment_id"`
	HeadCommit string       `json:"head_commit"`
	Nodes      []NodeResult `json:"nodes"`
	// Computed and CacheHits describe incremental evaluation effort.
	Computed  int `json:"computed"`
	CacheHits int `json:"cache_hits"`
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
	// RollupUnproven means no applicable terminal node is blocked or
	// unverifiable, but none is verified or waived either (every applicable
	// terminal is an advisory, unmet report-mode node). Nothing proves the
	// shipment, so it is never ready.
	RollupUnproven RollupState = "unproven"
)

// Rollup is the shipment roll-up over applicable terminal nodes.
type Rollup struct {
	State               RollupState `json:"state"`
	ApplicableTerminals int         `json:"applicable_terminals"`
	// ProvenTerminals counts applicable terminal nodes that are verified or
	// waived; ready requires at least one.
	ProvenTerminals int      `json:"proven_terminals"`
	Unmet           []string `json:"unmet"`
	Unverifiable    []string `json:"unverifiable"`
	Advisory        []string `json:"advisory"`
	Reason          string   `json:"reason"`
}
