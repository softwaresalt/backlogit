package evidencedag

import (
	"context"
	"errors"
	"fmt"
)

// NewGate builds the single authoritative review-ready gate over d. The mode
// must be explicit. ws may be nil, in which case every waiver token is denied.
func NewGate(d *Declaration, mode Mode, ws *WaiverService) (*Gate, error) {
	if mode != ModeReportOnly && mode != ModeEnforce {
		return nil, fmt.Errorf("%w: gate mode must be %q or %q", ErrInvalidInput, ModeReportOnly, ModeEnforce)
	}
	ev, err := NewEvaluator(d)
	if err != nil {
		return nil, err
	}
	return &Gate{decl: cloneDeclaration(d), eval: ev, mode: mode, waivers: ws}, nil
}

func cloneDeclaration(d *Declaration) *Declaration {
	c := &Declaration{SchemaVersion: d.SchemaVersion, Nodes: make([]Node, len(d.Nodes)), Edges: append([]Edge(nil), d.Edges...)}
	for i, n := range d.Nodes {
		c.Nodes[i] = cloneNode(n)
	}
	return c
}

// Declaration returns a copy of the gate's DAG declaration.
func (g *Gate) Declaration() *Declaration { return cloneDeclaration(g.decl) }

// Check evaluates the shipment. A shipment is review-ready only when at least
// one terminal node is applicable and every applicable terminal node is
// verified or validly waived. Waiver tokens are verified here, one waiver per
// node per check; a denied token never satisfies a node. In report-only mode
// the truthful verdict is preserved in ReviewReady and only Allowed relaxes.
func (g *Gate) Check(ctx context.Context, in GateInput) (Decision, error) {
	var zero Decision
	if g == nil || g.eval == nil {
		return zero, errors.New("evidencedag: nil gate")
	}
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("evidencedag: gate check interrupted: %w", err)
	}
	if len(in.WaiverTokens) > MaxNodes {
		return zero, fmt.Errorf("%w: more than %d waiver tokens", ErrInvalidInput, MaxNodes)
	}
	if err := validateShipment(in.Shipment); err != nil {
		return zero, err
	}
	lookup := func(id string) (Node, bool) { n, ok := g.eval.nodes[id]; return n, ok }
	waivers := map[string]Waiver{}
	var denials []WaiverDenial
	for i, tok := range in.WaiverTokens {
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("evidencedag: gate check interrupted: %w", err)
		}
		w, err := g.waivers.Accept(tok, in.Shipment, lookup)
		if err != nil {
			denials = append(denials, WaiverDenial{Index: i, Reason: truncate(err.Error())})
			continue
		}
		if _, dup := waivers[w.NodeID]; dup {
			denials = append(denials, WaiverDenial{Index: i, Reason: "more than one waiver for the same node in one check"})
			continue
		}
		waivers[w.NodeID] = w
	}
	rep, err := g.eval.Evaluate(Input{Shipment: in.Shipment, Evidence: in.Evidence, Waivers: waivers, Failures: in.Failures})
	if err != nil {
		return zero, err
	}
	ru := rep.Rollup()
	dec := Decision{
		ReviewReady: ru.State == RollupReady, Mode: g.mode, Rollup: ru, Report: rep,
		Denials: denials, Reason: ru.Reason,
	}
	dec.Allowed = dec.ReviewReady || g.mode == ModeReportOnly
	if !dec.ReviewReady && g.mode == ModeReportOnly {
		dec.Reason += " (report-only: not enforced)"
	}
	return dec, nil
}
