package evidencedag

import (
	"context"
	"errors"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// LoadEvidenceFile reads one evidence record from rel under root.
func LoadEvidenceFile(root, rel string) (EvidenceRecord, error) {
	return EvidenceRecord{}, errNotImplemented
}

var errNotImplemented = errors.New("evidencedag: not implemented")

// RunExecutors runs the bound executors for applicable nodes.
func RunExecutors(ctx context.Context, d *Declaration, s Shipment, bindings map[string]NodeExecutor) (ExecutionResult, error) {
	return ExecutionResult{}, errNotImplemented
}

// NewFuncExecutor wraps fn as a NodeExecutor with the given identity.
func NewFuncExecutor(identity string, fn ExecFunc) (NodeExecutor, error) {
	return nil, errNotImplemented
}

// Identity implements NodeExecutor.
func (x *RedHonestyExecutor) Identity() string { return "" }

// Execute implements NodeExecutor.
func (x *RedHonestyExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, errNotImplemented
}

// Identity implements NodeExecutor.
func (x *ParityExecutor) Identity() string { return "" }

// Execute implements NodeExecutor.
func (x *ParityExecutor) Execute(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, errNotImplemented
}
