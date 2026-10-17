package evidencedag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	if ctx == nil {
		return faultline.EvidenceArtifact{}, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
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
	if ctx == nil {
		return faultline.EvidenceArtifact{}, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
	if req.Node.ProducingCommit != CommitHead {
		// The S9 gate always judges the shipment head commit, so evidence for a
		// pinned commit cannot be produced honestly; refuse rather than mislabel.
		return faultline.EvidenceArtifact{}, errors.New("evidencedag: red-test honesty executor does not support a pinned producing_commit")
	}
	rep, err := x.Gate.Check(ctx, redhonesty.Request{TaskID: req.Node.ProducingTask, BaseRef: x.BaseRef, HeadRef: req.Shipment.HeadCommit})
	if err != nil {
		return faultline.EvidenceArtifact{}, fmt.Errorf("evidencedag: red-test honesty gate: %w", err)
	}
	return rep.Artifact, nil
}

// Identity implements NodeExecutor.
func (x *ParityExecutor) Identity() string { return ParityValidatorIdentity }

// Execute calls the caller-supplied parity source and wraps its result in the shared S4
// envelope, bound to the node's producing task and commit.
func (x *ParityExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	var zero faultline.EvidenceArtifact
	if x == nil || x.Source == nil {
		return zero, errors.New("evidencedag: parity source is not configured")
	}
	if ctx == nil {
		return zero, fmt.Errorf("%w: nil context", ErrInvalidInput)
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
