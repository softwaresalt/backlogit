package evidencedag

import (
	"errors"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

var errNotImplemented = errors.New("evidencedag: not implemented")

// Load decodes, bounds-checks and validates a DAG declaration (JSON or YAML).
func Load(data []byte) (*Declaration, error) { return nil, errNotImplemented }

// Validate checks the declaration deterministically and fails closed.
func (d *Declaration) Validate() error { return errNotImplemented }

// NewEvidenceRecord wraps an S4 U4 artifact with its shipment/node/commit binding.
func NewEvidenceRecord(s Shipment, nodeID string, a faultline.EvidenceArtifact) (EvidenceRecord, error) {
	return EvidenceRecord{}, errNotImplemented
}

// Evaluator evaluates a declaration incrementally.
type Evaluator struct{}

// NewEvaluator validates d and returns an Evaluator.
func NewEvaluator(d *Declaration) (*Evaluator, error) { return nil, errNotImplemented }

// Evaluate evaluates every node in topological order.
func (e *Evaluator) Evaluate(in Input) (Report, error) { return Report{}, errNotImplemented }

// Rollup computes the shipment roll-up over applicable terminal nodes.
func (r Report) Rollup() Rollup { return Rollup{} }

// Node returns the result for a node ID.
func (r Report) Node(id string) (NodeResult, bool) { return NodeResult{}, false }

// ResolveApplicability resolves a node's declared predicate for a shipment.
func ResolveApplicability(n Node, s Shipment) (bool, error) { return false, errNotImplemented }
