package policyengine

import (
	"context"
	"fmt"
	"sort"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

// MaxShipmentTasks caps the constituent tasks of one shipment evaluation.
const MaxShipmentTasks = 64

// ShipmentPolicyNodeID is the terminal node the shipment roll-up contributes to
// the S10 evidence DAG.
const ShipmentPolicyNodeID = "workflow-policy"

// CodeTaskMissing marks a manifest task for which no evidence was supplied.
const CodeTaskMissing = "task_missing"

// ShipmentRequest evaluates every constituent task of a shipment.
type ShipmentRequest struct {
	Surface    Surface
	ShipmentID string
	// Manifest is the closed set of constituent task IDs. It must be
	// non-empty: a shipment with no tasks proves nothing.
	Manifest []string
	// Tasks carries one request per manifest task. The shipment surface
	// overrides each task's Surface.
	Tasks []Request
}

// ShipmentDecision is the shipment-level roll-up result.
type ShipmentDecision struct {
	Surface     Surface            `json:"surface"`
	ShipmentID  string             `json:"shipment_id"`
	Mode        Mode               `json:"mode"`
	Disposition Disposition        `json:"disposition"`
	Allowed     bool               `json:"allowed"`
	ReviewReady bool               `json:"review_ready"`
	Rollup      evidencedag.Rollup `json:"rollup"`
	Tasks       []Decision         `json:"tasks"`
	Errors      []Violation        `json:"errors"`
	Reason      string             `json:"reason"`
}

func shipmentBlocked(req ShipmentRequest, mode Mode, format string, args ...any) (ShipmentDecision, error) {
	reason := clip(fmt.Sprintf(format, args...), MaxErrorBytes)
	return ShipmentDecision{
		Surface: req.Surface, ShipmentID: clip(req.ShipmentID, 96), Mode: mode, Disposition: DispositionBlock,
		Tasks: []Decision{}, Errors: []Violation{{Code: CodeInvalidRequest, Reason: reason}}, Reason: reason,
		Rollup: evidencedag.Rollup{State: evidencedag.RollupUnverifiable, Unmet: []string{}, Unverifiable: []string{}, Advisory: []string{}, Reason: reason},
	}, fmt.Errorf("%w: %s", ErrInvalidRequest, reason)
}

// CheckShipment evaluates every constituent task with Check and rolls the
// results up through the shared evidencedag roll-up: the shipment is
// review-ready only when every manifest task satisfied every applicable gate.
// A missing, malformed or unverifiable task keeps the shipment from being
// ready; in report-only mode the truthful verdict is preserved in ReviewReady
// and only Allowed relaxes.
func (e *Engine) CheckShipment(ctx context.Context, req ShipmentRequest) (ShipmentDecision, error) {
	if e == nil {
		return shipmentBlocked(req, "", "engine is not configured")
	}
	if ctx == nil {
		return shipmentBlocked(req, e.mode, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return shipmentBlocked(req, e.mode, "evaluation was cancelled: %v", err)
	}
	switch {
	case !knownSurface(req.Surface):
		return shipmentBlocked(req, e.mode, "surface %q is not a known surface", clip(string(req.Surface), 32))
	case !validTaskID(req.ShipmentID):
		return shipmentBlocked(req, e.mode, "shipment id is not a canonical backlog ID")
	case len(req.Manifest) == 0:
		return shipmentBlocked(req, e.mode, "the shipment manifest is empty: nothing can be verified")
	case len(req.Manifest) > MaxShipmentTasks || len(req.Tasks) > MaxShipmentTasks:
		return shipmentBlocked(req, e.mode, "more than %d constituent tasks", MaxShipmentTasks)
	}
	manifest := make(map[string]struct{}, len(req.Manifest))
	for _, id := range req.Manifest {
		if !validTaskID(id) {
			return shipmentBlocked(req, e.mode, "manifest task id %q is not canonical", clip(id, 64))
		}
		if _, dup := manifest[id]; dup {
			return shipmentBlocked(req, e.mode, "manifest task %s is listed more than once", id)
		}
		manifest[id] = struct{}{}
	}
	byTask := make(map[string]Request, len(req.Tasks))
	for _, t := range req.Tasks {
		if _, ok := manifest[t.TaskID]; !ok {
			return shipmentBlocked(req, e.mode, "task %q is not in the shipment manifest", clip(t.TaskID, 64))
		}
		if _, dup := byTask[t.TaskID]; dup {
			return shipmentBlocked(req, e.mode, "task %s is supplied more than once", t.TaskID)
		}
		t.Surface = req.Surface
		byTask[t.TaskID] = t
	}

	ids := append([]string(nil), req.Manifest...)
	sort.Strings(ids)
	out := ShipmentDecision{
		Surface: req.Surface, ShipmentID: req.ShipmentID, Mode: e.mode,
		Tasks: make([]Decision, 0, len(ids)), Errors: []Violation{},
	}
	report := evidencedag.Report{ShipmentID: req.ShipmentID, Nodes: make([]evidencedag.NodeResult, 0, len(ids))}
	for _, id := range ids {
		node := evidencedag.NodeResult{
			ID: id, Family: evidencedag.FamilyWorkflowPolicy, Terminal: true, OnUnmet: evidencedag.OnUnmetBlock,
		}
		treq, ok := byTask[id]
		if !ok {
			node.Status, node.Reason = evidencedag.StatusUnmet, "no evidence was supplied for this task"
			out.Errors = append(out.Errors, Violation{Code: CodeTaskMissing, Node: id, Reason: node.Reason})
			report.Nodes = append(report.Nodes, node)
			continue
		}
		td, err := e.Check(ctx, treq)
		out.Tasks = append(out.Tasks, td)
		if err != nil {
			node.Status, node.Reason = evidencedag.StatusError, clip(err.Error(), MaxErrorBytes)
			out.Errors = append(out.Errors, Violation{Code: CodeInvalidRequest, Node: id, Reason: node.Reason})
			report.Nodes = append(report.Nodes, node)
			out.Rollup = report.Rollup()
			return e.finishShipment(out, report, err)
		}
		node.Status, node.Reason = taskStatus(td)
		report.Nodes = append(report.Nodes, node)
	}
	return e.finishShipment(out, report, nil)
}

// taskStatus maps a task decision to a roll-up node status: ready is verified,
// any unverifiable gate is an error, anything else is unmet.
func taskStatus(d Decision) (evidencedag.Status, string) {
	if d.ReviewReady {
		return evidencedag.StatusVerified, "every applicable gate is satisfied"
	}
	for _, g := range d.Gates {
		if g.Status == string(evidencedag.StatusError) {
			return evidencedag.StatusError, "a gate of this task is unverifiable"
		}
	}
	return evidencedag.StatusUnmet, "a gate of this task is unmet"
}

func (e *Engine) finishShipment(out ShipmentDecision, report evidencedag.Report, cause error) (ShipmentDecision, error) {
	sort.Slice(report.Nodes, func(i, j int) bool { return report.Nodes[i].ID < report.Nodes[j].ID })
	out.Rollup = report.Rollup()
	out.ReviewReady = out.Rollup.State == evidencedag.RollupReady && cause == nil
	switch {
	case out.ReviewReady:
		out.Allowed, out.Disposition = true, DispositionPass
		out.Reason = "every constituent task satisfied every applicable gate"
	case e.mode == ModeReportOnly && cause == nil:
		out.Allowed, out.Disposition = true, DispositionReportOnly
		out.Reason = fmt.Sprintf("shipment roll-up is %s; report-only mode does not block", out.Rollup.State)
	default:
		out.Disposition = DispositionBlock
		out.Reason = fmt.Sprintf("shipment roll-up is %s: %s", out.Rollup.State, out.Rollup.Reason)
	}
	out.Reason = clip(out.Reason, MaxErrorBytes)
	if cause != nil {
		return out, cause
	}
	return out, nil
}

// Verdict converts the roll-up into the verdict of the shipment policy node in
// the S10 evidence DAG, so the one authoritative review-ready gate sees the
// policy roll-up as a terminal node. Only a ready roll-up verifies.
func (d ShipmentDecision) Verdict() evidencedag.Verdict {
	switch {
	case d.Rollup.State == evidencedag.RollupReady && d.ReviewReady:
		return evidencedag.Verdict{Status: evidencedag.StatusVerified, Reason: "every constituent task satisfied every applicable gate"}
	case d.Rollup.State == evidencedag.RollupBlocked || d.Rollup.State == evidencedag.RollupUnproven || d.Rollup.State == evidencedag.RollupReady:
		return evidencedag.Verdict{Status: evidencedag.StatusUnmet, Reason: clip("shipment policy roll-up is not satisfied: "+d.Reason, MaxErrorBytes)}
	default:
		return evidencedag.Verdict{Status: evidencedag.StatusError, Reason: clip("shipment policy roll-up is unverifiable: "+d.Reason, MaxErrorBytes)}
	}
}

// Contribution returns the S10 contribution that adds the shipment policy
// terminal node to a base declaration through evidencedag.Compose. Supply the
// node's verdict from ShipmentDecision.Verdict in the gate input.
func Contribution(producingTask string) evidencedag.Contribution {
	return evidencedag.Contribution{
		Source: ShipmentPolicyNodeID,
		Nodes: []evidencedag.Node{{
			ID: ShipmentPolicyNodeID, Family: evidencedag.FamilyWorkflowPolicy,
			Applicability: &evidencedag.Applicability{Always: true},
			Validator:     ValidatorIdentity, ProducingTask: producingTask, ProducingCommit: evidencedag.CommitHead,
			OnUnmet: evidencedag.OnUnmetBlock, Terminal: true,
		}},
	}
}
