package evidencedag

import (
	"context"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

// ExecRequest is the context a NodeExecutor runs a node in.
type ExecRequest struct {
	Shipment Shipment
	Node     Node
}

// NodeExecutor binds a detector to a DAG node. The single authoritative
// pattern: an executor returns an S4 U4 evidence artifact; the DAG wraps it in
// a bound EvidenceRecord and verifies it. Executors never set node statuses.
type NodeExecutor interface {
	// Identity is the producer identity; it must equal the node's validator
	// reference and the artifact's validator_identity.
	Identity() string
	// Execute runs the detector and returns its S4 U4 artifact.
	Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error)
}

// ExecFunc is a function-valued detector adapter.
type ExecFunc func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error)

// ExecutionResult is the bounded outcome of RunExecutors.
type ExecutionResult struct {
	// Evidence holds the bound records for nodes whose executor produced an
	// artifact that satisfies the shared contract.
	Evidence map[string]EvidenceRecord
	// Failures maps node IDs to a bounded reason when an executor could not
	// produce conformant evidence.
	Failures map[string]string
}

// RedGreenChecker is the S9 red-test honesty detector surface; *redhonesty.Gate
// satisfies it.
type RedGreenChecker interface {
	Check(ctx context.Context, req redhonesty.Request) (redhonesty.Report, error)
}

// RedHonestyExecutor adapts the S9 red-test honesty gate as a node executor.
type RedHonestyExecutor struct {
	Gate    RedGreenChecker
	BaseRef string
}

// ParityValidatorIdentity is the producer identity of ParityExecutor artifacts.
const ParityValidatorIdentity = "backlogit/parity@v1"

// ParityResult is a parity detector outcome for one scenario dimension.
type ParityResult struct {
	Evidence faultline.ParityEvidence
	// Status is the envelope status (pass, report_only or fail).
	Status string
}

// ParitySource runs the S4 parity detectors for a node.
type ParitySource func(ctx context.Context, req ExecRequest) (ParityResult, error)

// ParityExecutor adapts the S4 parity detectors as a node executor.
type ParityExecutor struct {
	Source ParitySource
}
