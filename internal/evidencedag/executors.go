package evidencedag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

type funcExecutor struct {
	identity string
	fn       ExecFunc
}

// NewFuncExecutor wraps fn as a NodeExecutor with the given producer identity.
// It is the generic adapter for detectors that already emit S4 U4 artifacts.
func NewFuncExecutor(identity string, fn ExecFunc) (NodeExecutor, error) {
	if identity == "" || len(identity) > MaxIDLength {
		return nil, errors.New("evidencedag: executor identity is required and bounded")
	}
	if fn == nil {
		return nil, errors.New("evidencedag: executor function is required")
	}
	return &funcExecutor{identity: identity, fn: fn}, nil
}

func (f *funcExecutor) Identity() string { return f.identity }

func (f *funcExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	return f.fn(ctx, req)
}

// Identity implements NodeExecutor.
func (x *RedHonestyExecutor) Identity() string { return redhonesty.ValidatorIdentity }

// Execute runs the S9 gate for the node's producing task against the
// shipment head commit and returns the gate's S4 U4 red_green artifact.
func (x *RedHonestyExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	if x == nil || x.Gate == nil {
		return faultline.EvidenceArtifact{}, errors.New("evidencedag: red-test honesty gate is not configured")
	}
	rep, err := x.Gate.Check(ctx, redhonesty.Request{TaskID: req.Node.ProducingTask, BaseRef: x.BaseRef, HeadRef: req.Shipment.HeadCommit})
	if err != nil {
		return faultline.EvidenceArtifact{}, fmt.Errorf("evidencedag: red-test honesty gate: %w", err)
	}
	return rep.Artifact, nil
}

// Identity implements NodeExecutor.
func (x *ParityExecutor) Identity() string { return ParityValidatorIdentity }

// Execute runs the parity detectors and wraps the result in the shared S4
// envelope, bound to the node's producing task and commit.
func (x *ParityExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	if x == nil || x.Source == nil {
		return zero, errors.New("evidencedag: parity source is not configured")
	}
	res, err := x.Source(ctx, req)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: parity detector: %w", err)
	}
	raw, err := json.Marshal(res.Evidence)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: marshal parity evidence: %w", err)
	}
	a := faultline.EvidenceArtifact{
		SchemaVersion:     faultline.EvidenceSchemaVersion,
		ProducingTask:     req.Node.ProducingTask,
		ProducingCommit:   expectedCommit(req.Node, req.Shipment),
		NodeFamily:        faultline.NodeFamilyParity,
		Applicability:     append([]string(nil), res.Evidence.Surfaces...),
		ValidatorIdentity: ParityValidatorIdentity,
		VerifiedEvidence:  raw,
		Status:            res.Status,
	}
	if err := faultline.Validate(a); err != nil {
		return zero, fmt.Errorf("evidencedag: parity evidence violates the shared contract: %w", err)
	}
	return a, nil
}

// RunExecutors runs the bound executor of every applicable node, in sorted
// node order, and returns bound evidence records plus bounded per-node failure
// reasons. Executors only produce artifacts; verification happens in the
// evaluator. A panic, error, identity mismatch or non-conformant artifact is a
// failure for that node and never evidence.
func RunExecutors(ctx context.Context, d *Declaration, s Shipment, bindings map[string]NodeExecutor) (ExecutionResult, error) {
	res := ExecutionResult{Evidence: map[string]EvidenceRecord{}, Failures: map[string]string{}}
	if d == nil {
		return res, fmt.Errorf("%w: nil declaration", ErrInvalidDeclaration)
	}
	if err := d.Validate(); err != nil {
		return res, err
	}
	if err := validateShipment(s); err != nil {
		return res, err
	}
	nodes := make(map[string]Node, len(d.Nodes))
	for _, n := range d.Nodes {
		nodes[n.ID] = n
	}
	ids := make([]string, 0, len(bindings))
	for id := range bindings {
		if _, ok := nodes[id]; !ok {
			return res, fmt.Errorf("%w: executor bound to undeclared node %q", ErrInvalidInput, truncate(id))
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return res, fmt.Errorf("evidencedag: executors interrupted: %w", err)
		}
		n, x := nodes[id], bindings[id]
		applicable, err := ResolveApplicability(n, s)
		if err != nil {
			res.Failures[id] = truncate(err.Error())
			continue
		}
		if !applicable {
			continue
		}
		if x == nil {
			res.Failures[id] = "executor binding is nil"
			continue
		}
		if x.Identity() != n.Validator {
			res.Failures[id] = "executor identity does not match the node validator"
			continue
		}
		rec, err := runOne(ctx, x, ExecRequest{Shipment: s, Node: n})
		if err != nil {
			res.Failures[id] = truncate(err.Error())
			continue
		}
		res.Evidence[id] = rec
	}
	return res, nil
}

func runOne(ctx context.Context, x NodeExecutor, req ExecRequest) (rec EvidenceRecord, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("executor panicked: %v", p)
		}
	}()
	a, err := x.Execute(ctx, req)
	if err != nil {
		return EvidenceRecord{}, err
	}
	return NewEvidenceRecord(req.Shipment, req.Node.ID, a)
}
