package policyengine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Surface identifies the entry point that invoked the engine.
type Surface string

// The three enforcement surfaces. All of them call Engine.Check; none has a
// separate evaluation path.
const (
	// SurfaceCLI is the backlogit command line.
	SurfaceCLI Surface = "cli"
	// SurfaceMCP is the MCP server.
	SurfaceMCP Surface = "mcp"
	// SurfaceAgent is agent skill dispatch.
	SurfaceAgent Surface = "agent"
)

// Surfaces returns every known surface.
func Surfaces() []Surface { return []Surface{SurfaceCLI, SurfaceMCP, SurfaceAgent} }

func knownSurface(s Surface) bool {
	for _, k := range Surfaces() {
		if k == s {
			return true
		}
	}
	return false
}

// Disposition is the engine's verdict.
type Disposition string

// Dispositions.
const (
	// DispositionPass means every applicable gate is satisfied.
	DispositionPass Disposition = "pass"
	// DispositionBlock means the transition is blocked.
	DispositionBlock Disposition = "block"
	// DispositionReportOnly means gates failed but report-only mode allows it.
	DispositionReportOnly Disposition = "report-only"
)

// Structured error codes. A Violation carries one of these.
const (
	CodeEvidenceAbsent        = "evidence_absent"
	CodeEvidenceAmbiguous     = "evidence_ambiguous"
	CodeEvidenceInvalid       = "evidence_invalid"
	CodeDigestMismatch        = "digest_mismatch"
	CodeBindingMismatch       = "binding_mismatch"
	CodeSHAUnverifiable       = "sha_unverifiable"
	CodeSHANotInHistory       = "sha_not_in_history"
	CodeTimestampFuture       = "timestamp_in_future"
	CodeTimestampRetroactive  = "timestamp_before_commit"
	CodeOutOfOrder            = "out_of_order"
	CodeOutcomeNotPass        = "outcome_not_pass"
	CodeArtifactInvalid       = "artifact_invalid"
	CodeSelfAsserted          = "self_asserted_derived_evidence"
	CodeDerivationUnavailable = "derivation_unavailable"
	CodeDerivationDegraded    = "derivation_degraded"
	CodeDerivationMismatch    = "derivation_head_mismatch"
	CodeReviewIncomplete      = "review_incomplete"
	CodeCIFailing             = "ci_failing"
	CodeUpstreamUnmet         = "upstream_unmet"
	CodeUpstreamUnverifiable  = "upstream_unverifiable"
	CodeHeadUnpublished       = "head_unpublished"
	CodeNoProtectedRef        = "no_protected_ref"
	CodeNotReady              = "gate_not_ready"
	CodeInvalidRequest        = "invalid_request"
)

const (
	// MaxErrors caps the structured errors in one decision.
	MaxErrors = 64
	// maxPR bounds a pull request number.
	maxPR = 1<<31 - 1
	// defaultClockSkew is the tolerated future skew of a record timestamp.
	defaultClockSkew = 5 * time.Minute
)

// ErrInvalidRequest classifies malformed evaluation requests.
var ErrInvalidRequest = errors.New("policyengine: invalid request")

// Derivation derives review and CI evidence from the single GitHub/git
// derivation source (internal/evidence, S7). *evidence.GitHub implements it.
type Derivation interface {
	ReviewEvidence(ctx context.Context, pr int) evidence.ReviewResult
	CIEvidence(ctx context.Context, pr int) evidence.CIResult
}

// Violation is one structured gate failure.
type Violation struct {
	Code   string `json:"code"`
	Node   string `json:"node,omitempty"`
	Reason string `json:"reason"`
}

// GateResult is the evaluated state of one gate.
type GateResult struct {
	Node   NodeType `json:"node"`
	Status string   `json:"status"`
	Code   string   `json:"code,omitempty"`
	Reason string   `json:"reason"`
	Waiver string   `json:"waiver,omitempty"`
}

// OverrideDenial records a refused override token.
type OverrideDenial struct {
	Index  int    `json:"index"`
	Reason string `json:"reason"`
}

// Request is one evaluation request. It deliberately has no operator-identity
// field: identity only ever comes from a verified out-of-band token.
type Request struct {
	Surface    Surface
	TaskID     string
	HeadCommit string
	// PR is the pull request review-complete and ci-pass are derived from; 0
	// means none.
	PR      int
	Records []Record
	// Interactive is set only by the surface adapter that observed a human at
	// a terminal; evidence can never supply it.
	Interactive    bool
	OverrideTokens [][]byte
}

// Decision is the engine's structured result.
type Decision struct {
	Surface         Surface          `json:"surface"`
	TaskID          string           `json:"task_id"`
	HeadCommit      string           `json:"head_commit"`
	Mode            Mode             `json:"mode"`
	Disposition     Disposition      `json:"disposition"`
	Allowed         bool             `json:"allowed"`
	ReviewReady     bool             `json:"review_ready"`
	Gates           []GateResult     `json:"gates"`
	Errors          []Violation      `json:"errors"`
	OverrideDenials []OverrideDenial `json:"override_denials,omitempty"`
	Reason          string           `json:"reason"`
}

// Options configures an Engine.
type Options struct {
	// Git verifies commit evidence. Required.
	Git Git
	// Derivation is the single source of review and CI evidence. Optional:
	// without it the derived gates block.
	Derivation Derivation
	// ProtectedRefs are the remote-tracking refs (refs/remotes/...) the head
	// commit must be reachable from. At least one is required; a writable
	// local branch is never accepted.
	ProtectedRefs []string
	// Waivers verifies signed operator override tokens. Optional: without it
	// every override is denied.
	Waivers *evidencedag.WaiverService
	// Mode may tighten the registry mode but never relax it.
	Mode Mode
	// Now supplies the clock; time.Now when nil.
	Now func() time.Time
	// MaxClockSkew is the tolerated future skew of record timestamps.
	MaxClockSkew time.Duration
	// AllowNonInteractiveOverrides permits overrides from non-interactive
	// surfaces. It is off by default.
	AllowNonInteractiveOverrides bool
}

// Engine is the one shared evaluation entry point. It is safe for concurrent
// use.
type Engine struct {
	reg  *Registry
	opts Options
	mode Mode
	refs []string
	now  func() time.Time
	skew time.Duration
}

// NewEngine validates the options and builds an Engine.
func NewEngine(reg *Registry, opts Options) (*Engine, error) {
	if reg == nil {
		return nil, errors.New("policyengine: a registry is required")
	}
	if opts.Git == nil {
		return nil, errors.New("policyengine: a git source is required")
	}
	if len(opts.ProtectedRefs) == 0 || len(opts.ProtectedRefs) > maxProtectedRefs {
		return nil, fmt.Errorf("policyengine: between 1 and %d protected refs are required", maxProtectedRefs)
	}
	seen := map[string]struct{}{}
	refs := make([]string, 0, len(opts.ProtectedRefs))
	for _, ref := range opts.ProtectedRefs {
		if !validProtectedRef(ref) {
			return nil, fmt.Errorf("policyengine: protected ref %q must be a remote-tracking ref (refs/remotes/...)", clip(ref, 64))
		}
		if _, dup := seen[ref]; !dup {
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	mode := reg.Mode()
	if opts.Mode != "" {
		if opts.Mode != ModeReportOnly && opts.Mode != ModeEnforce {
			return nil, fmt.Errorf("policyengine: unknown mode %q", clip(string(opts.Mode), 32))
		}
		if mode == ModeEnforce && opts.Mode != ModeEnforce {
			return nil, errors.New("policyengine: the mode option would relax the registry's enforce mode; it may only tighten")
		}
		mode = opts.Mode
	}
	if opts.MaxClockSkew < 0 {
		return nil, errors.New("policyengine: clock skew must not be negative")
	}
	e := &Engine{reg: reg, opts: opts, mode: mode, refs: refs, now: opts.Now, skew: opts.MaxClockSkew}
	if e.now == nil {
		e.now = time.Now
	}
	if e.skew == 0 {
		e.skew = defaultClockSkew
	}
	return e, nil
}

// fact is one gate's own evidence outcome, before upstream propagation.
type fact struct {
	status evidencedag.Status
	code   string
	reason string
	commit string
	at     time.Time
	hasAt  bool
	// ok means the gate's own evidence is well-formed, verified against git
	// and carries a usable commit for ordering checks.
	ok bool
}

func (f fact) verdict() evidencedag.Verdict {
	return evidencedag.Verdict{Status: f.status, Reason: f.reason}
}

func failed(status evidencedag.Status, code, format string, args ...any) fact {
	return fact{status: status, code: code, reason: clip(fmt.Sprintf(format, args...), MaxErrorBytes)}
}

func blocked(req Request, mode Mode, v Violation, err error) (Decision, error) {
	d := Decision{
		Surface: req.Surface, TaskID: clip(req.TaskID, 96), HeadCommit: clip(req.HeadCommit, 64), Mode: mode,
		Disposition: DispositionBlock, Gates: []GateResult{}, Errors: []Violation{v}, Reason: v.Reason,
	}
	return d, err
}

func invalidRequest(req Request, mode Mode, format string, args ...any) (Decision, error) {
	reason := clip(fmt.Sprintf(format, args...), MaxErrorBytes)
	return blocked(req, mode, Violation{Code: CodeInvalidRequest, Reason: reason}, fmt.Errorf("%w: %s", ErrInvalidRequest, reason))
}

// Check evaluates one task at one head commit. It is the single evaluation
// entry point for every surface: the verdict depends only on the registry,
// the evidence, git, the derivation source and verified override tokens, never
// on the surface. A malformed request is an error and a blocked decision in
// every mode; evidence defects are blocked decisions with nil error (or
// report-only decisions when the mode is report-only).
func (e *Engine) Check(ctx context.Context, req Request) (Decision, error) {
	if e == nil {
		return blocked(req, "", Violation{Code: CodeInvalidRequest, Reason: "engine is not configured"}, errors.New("policyengine: nil engine"))
	}
	if ctx == nil {
		return invalidRequest(req, e.mode, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: "evaluation was cancelled"}, fmt.Errorf("policyengine: %w", err))
	}
	switch {
	case !knownSurface(req.Surface):
		return invalidRequest(req, e.mode, "surface %q is not a known surface", clip(string(req.Surface), 32))
	case !evidencedag.ValidTaskID(req.TaskID):
		return invalidRequest(req, e.mode, "task id is not a canonical backlog task ID")
	case !faultline.ValidCommitSHA(req.HeadCommit):
		return invalidRequest(req, e.mode, "head commit must be a full lowercase commit SHA")
	case req.PR < 0 || req.PR > maxPR:
		return invalidRequest(req, e.mode, "pull request number is out of range")
	case len(req.Records) > MaxRecords:
		return invalidRequest(req, e.mode, "more than %d evidence records", MaxRecords)
	case len(req.OverrideTokens) > maxOverrideTokens:
		return invalidRequest(req, e.mode, "more than %d override tokens", maxOverrideTokens)
	}
	byNode := map[NodeType][]Record{}
	for _, r := range req.Records {
		if !knownNode(r.Node) {
			return invalidRequest(req, e.mode, "evidence names unknown node %q", clip(string(r.Node), 64))
		}
		byNode[r.Node] = append(byNode[r.Node], r)
	}

	if v, err := e.verifyPublished(ctx, req.HeadCommit); err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: "evaluation was cancelled"}, fmt.Errorf("policyengine: %w", err))
	} else if v != nil {
		return e.unverifiableHead(req, *v), nil
	}

	now := e.now()
	gates := e.reg.Gates()
	facts := make(map[NodeType]fact, len(gates))
	for _, g := range gates {
		if !g.Enabled {
			continue
		}
		if err := ctx.Err(); err != nil {
			return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: "evaluation was cancelled"}, fmt.Errorf("policyengine: %w", err))
		}
		switch g.ID {
		case NodeReviewComplete, NodeCIPass:
			facts[g.ID] = e.derivedFact(ctx, g.ID, byNode[g.ID], req)
		default:
			facts[g.ID] = e.recordedFact(ctx, g.ID, byNode[g.ID], req, now)
		}
	}
	if err := ctx.Err(); err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: "evaluation was cancelled"}, fmt.Errorf("policyengine: %w", err))
	}
	e.checkOrdering(ctx, facts)
	if err := ctx.Err(); err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: "evaluation was cancelled"}, fmt.Errorf("policyengine: %w", err))
	}

	verdicts := make(map[string]evidencedag.Verdict, len(facts))
	for id, f := range facts {
		verdicts[string(id)] = f.verdict()
	}
	plan := e.prepareOverrides(req)
	decl, err := e.reg.Declaration(req.TaskID)
	if err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: clip(err.Error(), MaxErrorBytes)}, err)
	}
	gate, err := evidencedag.NewGate(decl, e.mode, e.opts.Waivers)
	if err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: clip(err.Error(), MaxErrorBytes)}, fmt.Errorf("policyengine: build gate: %w", err))
	}
	dec, err := gate.Check(ctx, evidencedag.GateInput{
		Shipment:     evidencedag.Shipment{ID: req.TaskID, HeadCommit: req.HeadCommit, Attributes: e.reg.gateAttributes()},
		Verdicts:     verdicts,
		WaiverTokens: plan.tokens,
	})
	if err != nil {
		return blocked(req, e.mode, Violation{Code: CodeInvalidRequest, Reason: clip(err.Error(), MaxErrorBytes)}, fmt.Errorf("policyengine: evaluate gates: %w", err))
	}
	return e.decide(req, gates, facts, dec, plan), nil
}

// decide maps the core gate decision to the engine decision.
func (e *Engine) decide(req Request, gates []GateDecl, facts map[NodeType]fact, dec evidencedag.Decision, plan overridePlan) Decision {
	out := Decision{
		Surface: req.Surface, TaskID: req.TaskID, HeadCommit: req.HeadCommit, Mode: e.mode,
		ReviewReady: dec.ReviewReady, Allowed: dec.Allowed, Gates: make([]GateResult, 0, len(gates)),
		Errors: []Violation{}, OverrideDenials: plan.denials,
	}
	for _, d := range dec.Denials {
		out.OverrideDenials = append(out.OverrideDenials, OverrideDenial{Index: plan.original(d.Index), Reason: clip(d.Reason, MaxErrorBytes)})
	}
	for _, g := range gates {
		nr, _ := dec.Report.Node(string(g.ID))
		res := GateResult{Node: g.ID, Status: string(nr.Status), Reason: clip(nr.Reason, MaxErrorBytes), Waiver: clip(nr.Waiver, 96)}
		f, has := facts[g.ID]
		switch {
		case nr.Status == evidencedag.StatusVerified, nr.Status == evidencedag.StatusNotApplicable && !g.Enabled:
		case nr.Status == evidencedag.StatusWaived:
			if !g.Overridable {
				// Defense in depth: peeking already refuses such tokens.
				res.Code = CodeOverrideNotPermitted
				out.Errors = append(out.Errors, Violation{Code: CodeOverrideNotPermitted, Node: string(g.ID), Reason: "a waiver was accepted for a gate that is not overridable"})
			}
		default:
			// Anything that is not positively verified, waived or disabled (an
			// unmet or unverifiable gate, but also an unexpected or missing
			// status) is a violation: nothing fails open.
			code, reason := CodeUpstreamUnmet, res.Reason
			if nr.Status != evidencedag.StatusUnmet {
				code = CodeUpstreamUnverifiable
			}
			if has && f.status != evidencedag.StatusVerified {
				code, reason = f.code, f.reason
			}
			res.Code = code
			if len(out.Errors) < MaxErrors {
				out.Errors = append(out.Errors, Violation{Code: code, Node: string(g.ID), Reason: clip(reason, MaxErrorBytes)})
			}
		}
		out.Gates = append(out.Gates, res)
	}
	sort.SliceStable(out.OverrideDenials, func(i, j int) bool { return out.OverrideDenials[i].Index < out.OverrideDenials[j].Index })
	if dec.ReviewReady && len(out.Errors) > 0 {
		// The core rolls up terminal nodes only, and a disabled (not-applicable)
		// gate does not bridge propagation, so an enabled gate that failed
		// upstream of it would otherwise be invisible to the roll-up.
		out.ReviewReady = false
		out.Allowed = e.mode == ModeReportOnly
	}
	if !out.ReviewReady && len(out.Errors) == 0 {
		// The core refused without any gate-level cause the engine can name;
		// never report a blocked decision with no violation.
		reason := dec.Reason
		if reason == "" {
			reason = "the gate roll-up is not review-ready"
		}
		out.Errors = append(out.Errors, Violation{Code: CodeNotReady, Reason: clip(reason, MaxErrorBytes)})
	}
	switch {
	case out.ReviewReady:
		out.Disposition, out.Reason = DispositionPass, "every applicable gate is satisfied"
	case out.Allowed:
		out.Disposition, out.Reason = DispositionReportOnly, fmt.Sprintf("%d gate violation(s) reported; report-only mode does not block", len(out.Errors))
	default:
		out.Disposition, out.Reason = DispositionBlock, fmt.Sprintf("%d gate violation(s) block this transition", len(out.Errors))
	}
	return out
}

// unverifiableHead builds the decision for a head commit that cannot be tied
// to a protected ref: every enabled gate is unverifiable.
func (e *Engine) unverifiableHead(req Request, v Violation) Decision {
	out := Decision{
		Surface: req.Surface, TaskID: req.TaskID, HeadCommit: req.HeadCommit, Mode: e.mode,
		Gates: []GateResult{}, Errors: []Violation{v},
	}
	for _, g := range e.reg.Gates() {
		if g.Enabled {
			out.Gates = append(out.Gates, GateResult{Node: g.ID, Status: string(evidencedag.StatusError), Code: v.Code, Reason: v.Reason})
		} else {
			out.Gates = append(out.Gates, GateResult{Node: g.ID, Status: string(evidencedag.StatusNotApplicable), Reason: "gate is disabled"})
		}
	}
	if e.mode == ModeReportOnly {
		out.Disposition, out.Allowed = DispositionReportOnly, true
		out.Reason = v.Reason + " (report-only: not enforced)"
	} else {
		out.Disposition, out.Reason = DispositionBlock, v.Reason
	}
	return out
}

// verifyPublished proves the head commit is reachable from a protected
// remote-tracking ref. A nil violation with a nil error means it is. A
// cancelled context is returned as an error, never as a violation, so even
// report-only mode cannot turn a cancelled check into an allowed one. A git
// execution failure is reported with its (clipped) error rather than as a
// generic unpublished head.
func (e *Engine) verifyPublished(ctx context.Context, head string) (*Violation, error) {
	var tips []string
	var gitErr error
	note := func(err error) {
		if gitErr == nil {
			gitErr = err
		}
	}
	for _, ref := range e.refs {
		tip, err := e.opts.Git.ResolveCommit(ctx, ref)
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		switch {
		case errors.Is(err, ErrRefNotFound):
			continue
		case err != nil:
			note(err)
			continue
		case !faultline.ValidCommitSHA(tip):
			note(errors.New("git returned an invalid commit for a protected ref"))
			continue
		}
		tips = append(tips, tip)
	}
	if len(tips) == 0 {
		if gitErr != nil {
			return &Violation{Code: CodeSHAUnverifiable, Reason: clip("the protected remote refs could not be resolved: "+gitErr.Error(), MaxErrorBytes)}, nil
		}
		return &Violation{Code: CodeNoProtectedRef, Reason: "none of the protected remote refs resolves; the head commit cannot be verified"}, nil
	}
	for _, tip := range tips {
		ok, err := e.opts.Git.IsAncestor(ctx, head, tip)
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if err != nil {
			note(err)
			continue
		}
		if ok {
			return nil, nil
		}
	}
	if gitErr != nil {
		return &Violation{Code: CodeSHAUnverifiable, Reason: clip("the head commit's publication could not be verified: "+gitErr.Error(), MaxErrorBytes)}, nil
	}
	return &Violation{Code: CodeHeadUnpublished, Reason: "the head commit is not reachable from any protected remote ref (a local branch is not trusted)"}, nil
}

func (e *Engine) derivedFact(ctx context.Context, node NodeType, recs []Record, req Request) (f fact) {
	if len(recs) > 0 {
		return failed(evidencedag.StatusError, CodeSelfAsserted, "%s evidence is derived from the review/CI source; a self-asserted record is refused", node)
	}
	if e.opts.Derivation == nil || req.PR <= 0 {
		return failed(evidencedag.StatusError, CodeDerivationUnavailable, "no derivation source or pull request to derive %s from", node)
	}
	defer func() {
		if r := recover(); r != nil {
			f = failed(evidencedag.StatusError, CodeDerivationDegraded, "the derivation source failed for %s: %s", node, clip(fmt.Sprint(r), 120))
		}
	}()
	var status evidence.Status
	var reason string
	var pr int
	var head string
	complete, incompleteReason, incompleteCode := false, "", ""
	switch node {
	case NodeReviewComplete:
		res := e.opts.Derivation.ReviewEvidence(ctx, req.PR)
		status, reason, pr, head = res.Status, res.Reason, res.PR, res.HeadSHA
		complete = res.ReviewedCurrent && res.UnresolvedThreads == 0
		incompleteCode, incompleteReason = CodeReviewIncomplete, "no completed review of the head commit with every thread resolved"
	default:
		res := e.opts.Derivation.CIEvidence(ctx, req.PR)
		status, reason, pr, head = res.Status, res.Reason, res.PR, res.HeadSHA
		complete = res.AllPassed
		incompleteCode, incompleteReason = CodeCIFailing, "CI checks on the head commit have not all passed"
	}
	if status != evidence.StatusVerified {
		return failed(evidencedag.StatusError, CodeDerivationDegraded, "%s could not be derived: %s", node, reason)
	}
	if pr != req.PR || head != req.HeadCommit {
		return failed(evidencedag.StatusError, CodeDerivationMismatch, "derived %s evidence is not bound to this pull request and head commit", node)
	}
	if !complete {
		return failed(evidencedag.StatusUnmet, incompleteCode, "%s", incompleteReason)
	}
	return fact{status: evidencedag.StatusVerified, reason: "derived evidence is complete", commit: req.HeadCommit, ok: true}
}

// recordedFact verifies the single commit-SHA-bound record of a gate.
func (e *Engine) recordedFact(ctx context.Context, node NodeType, recs []Record, req Request, now time.Time) fact {
	switch len(recs) {
	case 0:
		return failed(evidencedag.StatusUnmet, CodeEvidenceAbsent, "no commit-SHA-bound evidence was supplied for %s", node)
	case 1:
	default:
		return failed(evidencedag.StatusError, CodeEvidenceAmbiguous, "%d records were supplied for %s", len(recs), node)
	}
	r := recs[0]
	if err := r.checkShape(); err != nil {
		return failed(evidencedag.StatusError, CodeEvidenceInvalid, "%s", err.Error())
	}
	if r.TaskID != req.TaskID || r.Node != node {
		return failed(evidencedag.StatusError, CodeBindingMismatch, "the record is bound to a different task or node")
	}
	if want, err := r.computeDigest(); err != nil || want != r.Digest {
		return failed(evidencedag.StatusError, CodeDigestMismatch, "the record digest does not match its content")
	}
	if len(r.Artifact) > 0 && node != NodeRedPhaseVerify {
		return failed(evidencedag.StatusError, CodeEvidenceInvalid, "%s evidence carries no artifact", node)
	}
	anc, err := e.opts.Git.IsAncestor(ctx, r.CommitSHA, req.HeadCommit)
	if err != nil {
		return failed(evidencedag.StatusError, CodeSHAUnverifiable, "the evidence commit cannot be verified against git history")
	}
	if !anc {
		return failed(evidencedag.StatusError, CodeSHANotInHistory, "the evidence commit is not in the history of the head commit")
	}
	ct, err := e.opts.Git.CommitTime(ctx, r.CommitSHA)
	if err != nil {
		return failed(evidencedag.StatusError, CodeSHAUnverifiable, "the evidence commit time cannot be read from git")
	}
	if r.RecordedAt.After(now.Add(e.skew)) {
		return failed(evidencedag.StatusError, CodeTimestampFuture, "the evidence timestamp is in the future")
	}
	if r.RecordedAt.Before(ct) {
		return failed(evidencedag.StatusError, CodeTimestampRetroactive, "the evidence is timestamped before the commit it claims to cover")
	}
	f := fact{commit: r.CommitSHA, at: r.RecordedAt, hasAt: true}
	outcome := r.Outcome
	if node == NodeRedPhaseVerify {
		derived, err := redOutcome(r)
		if err != nil {
			return failed(evidencedag.StatusError, CodeArtifactInvalid, "%s", err.Error())
		}
		if derived != r.Outcome {
			return failed(evidencedag.StatusError, CodeArtifactInvalid, "the record outcome does not match the artifact status")
		}
		outcome = derived
	}
	if outcome != OutcomePass {
		f.status, f.code, f.reason = evidencedag.StatusUnmet, CodeOutcomeNotPass, clip(fmt.Sprintf("%s evidence records a failing outcome", node), MaxErrorBytes)
		return f
	}
	f.status, f.reason, f.ok = evidencedag.StatusVerified, "commit-bound evidence verified", true
	return f
}

// redOutcome validates the S9 red/green artifact and returns the outcome it
// proves.
func redOutcome(r Record) (string, error) {
	if len(r.Artifact) == 0 {
		return "", errors.New("red-phase-verify requires the S9 red-test honesty artifact")
	}
	a, payload, err := faultline.DecodeAndValidate(r.Artifact)
	if err != nil {
		return "", fmt.Errorf("the red-phase artifact does not conform to the shared evidence contract: %s", clip(faultline.SanitizeDiagnostic(err.Error()), 120))
	}
	rg, ok := payload.(*faultline.RedGreenEvidence)
	if !ok || a.NodeFamily != faultline.NodeFamilyRedGreen || a.SchemaVersion != faultline.EvidenceSchemaVersionV2 {
		return "", errors.New("the red-phase artifact is not a red_green artifact")
	}
	if a.ProducingTask != r.TaskID || a.ProducingCommit != r.CommitSHA || rg.HeadSHA != r.CommitSHA {
		return "", errors.New("the red-phase artifact is not bound to this task and commit")
	}
	if a.Status == faultline.StatusPass {
		return OutcomePass, nil
	}
	return OutcomeFail, nil
}
