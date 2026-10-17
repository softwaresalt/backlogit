package evidencedag

import (
	"context"
	"errors"
	"time"
)

// ParseTrust decodes a trust configuration document.
func ParseTrust(data []byte) (*Trust, error) { return nil, errNotImplemented }

// LoadTrust reads a trust configuration located outside forbiddenRoots.
func LoadTrust(path string, forbiddenRoots ...string) (*Trust, error) {
	return nil, errNotImplemented
}

// OpenLedger opens (without creating) a ledger at path.
func OpenLedger(path string) (*Ledger, error) { return nil, errNotImplemented }

// Append adds one record under the single-writer lock after re-verifying the chain.
func (l *Ledger) Append(e LedgerEntry) (LedgerRecord, error) {
	return LedgerRecord{}, errNotImplemented
}

// Records verifies and returns every record.
func (l *Ledger) Records(anchor *Anchor) ([]LedgerRecord, error) { return nil, errNotImplemented }

// Head returns the verified chain head.
func (l *Ledger) Head() (Anchor, error) { return Anchor{}, errNotImplemented }

// NewWaiverService builds a WaiverService; a nil clock uses time.Now.
func NewWaiverService(t *Trust, l *Ledger, clock func() time.Time) (*WaiverService, error) {
	return nil, errNotImplemented
}

// Accept verifies a waiver token for the shipment and returns a verified Waiver.
func (s *WaiverService) Accept(raw []byte, ship Shipment, lookup func(string) (Node, bool)) (Waiver, error) {
	return Waiver{}, errNotImplemented
}

// NewGate builds the authoritative gate; waivers may be nil (waivers disabled).
func NewGate(d *Declaration, mode Mode, ws *WaiverService) (*Gate, error) {
	return nil, errNotImplemented
}

// Check evaluates the shipment against the gate.
func (g *Gate) Check(ctx context.Context, in GateInput) (Decision, error) {
	return Decision{}, errNotImplemented
}

// Compose merges policy-node contributions into base.
func Compose(base *Declaration, contribs ...Contribution) (*Declaration, error) {
	return nil, errNotImplemented
}

// Text renders a deterministic, bounded status report.
func (r Report) Text() string { return "" }

// DOT renders a deterministic, bounded Graphviz graph; r may be nil.
func (d *Declaration) DOT(r *Report) string { return "" }

func boundOutput(s string) string { return s }

var errNotImplemented = errors.New("evidencedag: not implemented")

func (t *Trust) authorized(principal string, f Family) bool { return false }
