package core

import (
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// taskAwareRunner returns a passing result for per-task gate checks (args contain
// --task) and a caller-configured result for the shipment-level aggregate check
// (no --task). This lets a test complete member tasks cleanly while forcing the
// shipment-diff check to block.
type taskAwareRunner struct {
	taskRes     gate.GateResult
	shipmentRes gate.GateResult
	shipmentErr error
	lastCmd     []string
}

func (r *taskAwareRunner) Run(_ context.Context, args []string, _ string, _ []string) (gate.GateResult, error) {
	r.lastCmd = args
	for _, a := range args {
		if a == "--task" {
			return r.taskRes, nil
		}
	}
	return r.shipmentRes, r.shipmentErr
}

// newGatedFeatureTask creates a feature+task, wraps them in a claimed shipment,
// and returns the ids. The task is left active (claimed), ready to be completed
// through the gated path by the caller.
func newGatedShipment(t *testing.T, ws *Workspace) (featureID, taskID, shipmentID string) {
	t.Helper()
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Ship gate feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Ship gate task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Ship gate shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return feat.ID, task.ID, shipment.ID
}

func TestShipmentGate_AllMembersHaveEvidence_Ships(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Complete the member task through the gated path -> records passing evidence.
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.Equal(t, "done", statusOf(t, ws, taskID))

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "ship must succeed when members carry evidence and the shipment gate passes")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}

func TestShipmentGate_MemberMissingEvidence_Refused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Leave the task active (never completed through the gate).
	require.Equal(t, "active", statusOf(t, ws, taskID))

	_, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse a member that was never completed through the gate")
	assert.Contains(t, err.Error(), taskID)

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "want *GateBlockedError, got %T", err)

	// Reconciliation guarantee: the ungated member was NOT auto-completed, and the
	// shipment remains active.
	assert.Equal(t, "active", statusOf(t, ws, taskID), "ungated member must not be auto-completed")
	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state must be unchanged on refusal")
}

func TestShipmentGate_ShipmentDiffBlocked_Refused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// Per-task checks pass (exit 0); the shipment-level aggregate check blocks.
	report := `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
		shipmentRes: gate.GateResult{ExitCode: 1, Stdout: []byte(report)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse when the shipment-level gate check blocks")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
	assert.Equal(t, shipmentID, blocked.ItemID)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state unchanged after a shipment-diff block")
}

func TestShipmentGate_FailOpenAuto_ShipsWithoutEvidence(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// enabled:auto + unresolvable binary -> not enforced -> shipment gate fails open.
	injectBroker(ws, gate.EnabledAuto, &fakeGateRunner{}, fakeVersion{err: bkerrors.ErrGateBinaryNotFound})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// The task is still active with NO gate evidence, yet a fail-open ship proceeds
	// (completeReleaseScope completes it) — enforcement is off in this environment.
	require.Equal(t, "active", statusOf(t, ws, taskID))

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "auto fail-open must not enforce shipment gating")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}

// hasBlockedReason reports whether the events contain an EventGateBlocked whose
// delta carries the given "reason" — the monitoring signal the empty-head
// fail-closed branches must emit (Constitution Principle V).
func hasBlockedReason(evs []events.Event, reason string) bool {
	for _, e := range evs {
		if e.EventType != EventGateBlocked {
			continue
		}
		if r, _ := e.Delta["reason"].(string); r == reason {
			return true
		}
	}
	return false
}

// TestShipmentGate_EmptyShipmentHeadInRepo_Refused pins 1AEA2B0E: under
// enforcement, an empty shipment head resolved INSIDE a real work tree (an unborn
// branch: --is-inside-work-tree=true, rev-parse HEAD empty) must FAIL CLOSED with
// the dedicated in-repo message, leave shipment state unchanged, and record an
// EventGateBlocked monitoring signal.
func TestShipmentGate_EmptyShipmentHeadInRepo_Refused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	// Make ws.RootPath a real work tree with an UNBORN HEAD (git init, no commit).
	initGitRepoNoCommits(t, ws.RootPath)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "an empty shipment head in a real work tree must fail closed under enforcement")
	assert.Contains(t, err.Error(), "cannot resolve shipment head in repository")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "want *GateBlockedError, got %T", err)

	// Shipment state unchanged on refusal.
	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state must be unchanged on refusal")

	// The refusal recorded an EventGateBlocked monitoring signal (Principle V).
	evs, rerr := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), shipmentID)
	require.NoError(t, rerr)
	assert.True(t, hasBlockedReason(evs, "empty-shipment-head"),
		"an EventGateBlocked with reason=empty-shipment-head must be recorded")
}

// TestShipmentGate_EmptyShipmentHeadNoRepo_Skips is the regression peer to the
// in-repo refusal: a GENUINE no-repo empty shipment head under enforcement
// preserves the legacy skip and ships (no-repo test harness + non-autoharness
// environments must not regress).
func TestShipmentGate_EmptyShipmentHeadNoRepo_Skips(t *testing.T) {
	// This test exercises the git-PRESENT no-repo path: inGitWorktreeBounded must
	// run `git rev-parse` and return (false, nil) for the legacy skip. With git
	// absent from PATH the probe fails closed (refuse) by design, so the ship
	// expectation below would not hold — skip, mirroring the other git-dependent
	// tests in this package (initGitRepoWithCommits / initGitRepoNoCommits / the
	// broken-repo cases in TestInGitWorktreeBounded).
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	ws := newGateTestWorkspace(t) // temp dir, NOT a git repo
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "a genuine no-repo empty shipment head must preserve the legacy skip and ship")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}

// TestValidateMemberGateEvidence_StaleRefused exercises the ancestor-aware
// staleness dimension directly against a real git repo. A member whose recorded
// evidence head is an ANCESTOR of the shipment head is accepted (the post-merge
// false-staleness case strict equality wrongly rejected); an equal head is
// accepted; an empty head now FAILS CLOSED (B85DAEE8); and a genuinely divergent
// head, a malformed head_sha, and an unverifiable (absent-object) lineage are each
// refused with a fail-closed, branch-specific message.
func TestValidateMemberGateEvidence_StaleRefused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	// base (A) is an ancestor of head (B); divergent (D) is a real sibling of A
	// that is NOT an ancestor of B.
	base, head, divergent := initGitRepoWithCommits(t, ws.RootPath)

	// mkMember creates a terminal (done) gated member carrying a single passing
	// evidence event with the given recorded head_sha (omitted when empty).
	mkMember := func(headSHA string) string {
		t.Helper()
		id := newActiveTask(t, ws)
		_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
		require.NoError(t, err)
		delta := map[string]any{"outcome": "passed", "ran": true}
		if headSHA != "" {
			delta["head_sha"] = headSHA
		}
		require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, delta))
		return id
	}

	var blocked *bkerrors.GateBlockedError

	// R1 — ancestor accepted (the false-staleness case strict equality rejected).
	ancestorMember := mkMember(base)
	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{ancestorMember}, head),
		"a member head that is an ancestor of the shipment head must be accepted")

	// R3 — exact equality still accepted (fast-path, no repo access needed).
	equalMember := mkMember(head)
	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{equalMember}, head))

	// R7 (flipped, B85DAEE8) — an empty member head now FAILS CLOSED under
	// enforcement. A non-empty shipmentHead proves headSHABounded resolved a real
	// HEAD (real work tree), so a member with no recorded head_sha cannot prove its
	// gated commit is contained in the shipment history. The refusal is a typed
	// *GateBlockedError AND records an EventGateBlocked monitoring signal.
	emptyMember := mkMember("")
	r7err := validateMemberGateEvidence(ctx, ws, []string{emptyMember}, head)
	require.Error(t, r7err, "an empty member head must fail closed under enforcement in a real repo")
	assert.Contains(t, r7err.Error(), "no recorded head_sha")
	require.True(t, stderrors.As(r7err, &blocked), "empty-member-head refusal must be a *GateBlockedError")
	r7evs, r7rerr := events.ReadAllEvents(ctx, WorkspaceLogsRoot(ws.RootPath), emptyMember)
	require.NoError(t, r7rerr)
	assert.True(t, hasBlockedReason(r7evs, "empty-member-head"),
		"an EventGateBlocked with reason=empty-member-head must be recorded")

	// R2 — genuinely divergent (non-ancestor) head refused, specific message.
	divergentMember := mkMember(divergent)
	derr := validateMemberGateEvidence(ctx, ws, []string{divergentMember}, head)
	require.Error(t, derr)
	assert.Contains(t, derr.Error(), "divergent")
	require.True(t, stderrors.As(derr, &blocked), "divergent refusal must be a *GateBlockedError")

	// R6 — malformed recorded head_sha refused before any git exec (fail closed).
	malformedMember := mkMember("oldsha0000")
	merr := validateMemberGateEvidence(ctx, ws, []string{malformedMember}, head)
	require.Error(t, merr)
	assert.Contains(t, merr.Error(), "malformed")
	require.True(t, stderrors.As(merr, &blocked), "malformed refusal must be a *GateBlockedError")

	// R4 — valid-shape but absent object: unverifiable lineage refused (fail closed).
	absentMember := mkMember("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	aerr := validateMemberGateEvidence(ctx, ws, []string{absentMember}, head)
	require.Error(t, aerr)
	assert.Contains(t, aerr.Error(), "lineage")
	require.True(t, stderrors.As(aerr, &blocked), "lineage-error refusal must be a *GateBlockedError")
}

// TestValidateMemberGateEvidence_EmptyMemberHeadNoRepoSkipped is the regression
// peer to the R7 flip: when the shipment head is empty (no-repo — the member scan
// runs only inside `if shipmentHead != ""`), an empty member head stays SKIPPED so
// no-repo test harnesses and non-autoharness environments do not regress. The
// empty-member-head fail-closed fires only when a non-empty shipmentHead proves a
// real resolved HEAD.
func TestValidateMemberGateEvidence_EmptyMemberHeadNoRepoSkipped(t *testing.T) {
	ws := newGateTestWorkspace(t) // temp dir, NOT a git repo
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome": "passed", "ran": true, // no head_sha recorded (no-repo).
	}))

	// shipmentHead == "" -> the member scan block is not entered; empty member head
	// skip preserved.
	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, ""),
		"an empty member head under an empty (no-repo) shipment head must stay skipped")
}

// TestValidateMemberGateEvidence_DescopedArchivedMemberExempt pins the
// descoped-member exemption: a feature descendant that was scaffolded then
// removed from the release (archived directly from a non-terminal status, so it
// never went through the completion gate and carries NO gate evidence) MUST NOT
// block the shipment. releaseScopeItemIDs expands a feature to ALL descendants
// (IncludeArchived: true), so such a descoped task lands in the release scope even
// when the shipment manifest excludes it; demanding per-member gate evidence for
// it would permanently block the parent feature's ship with no operator recourse
// (archived is a terminal sink with no allowed transitions, so it cannot be
// force-gated). The exemption is narrow and safe: archived-WITH-evidence members
// still get full lineage validation, non-archived members are unchanged, and the
// shipment-level aggregate diff gate still covers the full shipment diff.
func TestValidateMemberGateEvidence_DescopedArchivedMemberExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	// A member scaffolded then descoped: archived directly from a non-terminal
	// status, so it never went through the gate and carries no gate evidence.
	descoped := newActiveTask(t, ws)
	_, err := ArchiveItem(ctx, ws.DB, ws, descoped)
	require.NoError(t, err)
	require.Equal(t, "archived", statusOf(t, ws, descoped),
		"the descoped member must be archived for this scenario")

	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{descoped}, ""),
		"a descoped (archived, ungated) member must be exempt from the per-member evidence requirement")

	// Safety guard 1: a NON-archived, non-terminal member that was never gated
	// still refuses — the exemption must not weaken the live-member guarantee.
	live := newActiveTask(t, ws)
	lerr := validateMemberGateEvidence(ctx, ws, []string{live}, "")
	require.Error(t, lerr, "a non-archived ungated member must still refuse")
	assert.Contains(t, lerr.Error(), "not completed through the gate")

	// Safety guard 2: a DONE (terminal, non-archived) member with no evidence
	// still refuses — only archived (descoped) members are exempt, not
	// completed-status members that skipped the gate.
	doneID := newActiveTask(t, ws)
	_, derr := updateArtifactUngated(ctx, ws, doneID, map[string]any{"status": "done"})
	require.NoError(t, derr)
	d2 := validateMemberGateEvidence(ctx, ws, []string{doneID}, "")
	require.Error(t, d2, "a done member with no gate evidence must still refuse")
	assert.Contains(t, d2.Error(), "missing passing gate evidence")
}

// TestValidateMemberGateEvidence_ArchivedFromDoneNotExempt pins the boundary of
// the descoped-member exemption: the exemption must apply ONLY to members archived
// from a non-terminal (in-flight) status, NOT to any archived member. ArchiveItem
// accepts terminal items and preserves the pre-archive status in archived_status,
// so a member driven to done with NO valid gate evidence (only a fail-open
// EventGatePassed{ran:false}, rejected by the F4 predicate) and THEN archived must
// still refuse. Exempting it on the bare archived status would bypass the F4
// evidence predicate pinned by TestValidateMemberGateEvidence_FailOpenNoRunRejected.
func TestValidateMemberGateEvidence_ArchivedFromDoneNotExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	// Drive a member to done WITHOUT valid evidence: its only "pass" is a fail-open
	// no-run (ran=false), which the F4 predicate rejects.
	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome": "passed", "ran": false,
	}))

	// Now archive it. ArchiveItem preserves the pre-archive status ("done") in
	// archived_status, so this is a completed-then-archived member, NOT a descope.
	_, aerr := ArchiveItem(ctx, ws.DB, ws, id)
	require.NoError(t, aerr)
	require.Equal(t, "archived", statusOf(t, ws, id),
		"the member must be archived for this scenario")

	verr := validateMemberGateEvidence(ctx, ws, []string{id}, "")
	require.Error(t, verr,
		"an archived-from-done member with only fail-open evidence must NOT be exempt")
	assert.Contains(t, verr.Error(), "missing passing gate evidence")
}

// TestValidateMemberGateEvidence_ArchivedFromUnknownStatusNotExempt pins the
// fail-closed boundary for MALFORMED archive provenance. archivedFromDescopeEligibleStatus
// exempts a member only when archived_status is a RECOGNIZED descope-eligible status. An
// unrecognized/typo value (e.g. "dne") must NOT be treated as a proven descope: because
// isDescopeEligibleStatus returns false for every unknown value, and the explicit
// isRecognizedReleaseStatus guard rejects garbage provenance, malformed provenance cannot
// bypass the per-member F4 evidence requirement. Unknown provenance must fail closed.
func TestValidateMemberGateEvidence_ArchivedFromUnknownStatusNotExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	// Archive a member from active (archived_status becomes "active"), then corrupt
	// the persisted archived_status to a malformed value on disk to simulate a typo
	// or a future serializer bug emitting unrecognized provenance. archived_status is
	// read from the Markdown source, so patching the file exercises the real path.
	id := newActiveTask(t, ws)
	_, err := ArchiveItem(ctx, ws.DB, ws, id)
	require.NoError(t, err)
	require.Equal(t, "archived", statusOf(t, ws, id),
		"the member must be archived for this scenario")

	path, ferr := FindArtifactPath(ctx, ws, id)
	require.NoError(t, ferr)
	raw, rerr := os.ReadFile(path)
	require.NoError(t, rerr)
	require.Contains(t, string(raw), "archived_status: active",
		"precondition: a member archived from active carries archived_status: active")
	corrupted := strings.Replace(string(raw), "archived_status: active", "archived_status: dne", 1)
	require.NoError(t, os.WriteFile(path, []byte(corrupted), 0o644))

	verr := validateMemberGateEvidence(ctx, ws, []string{id}, "")
	require.Error(t, verr,
		"an archived member with unrecognized archived_status must fail closed, not be exempted")
	assert.Contains(t, verr.Error(), "missing passing gate evidence")
}

// TestLatestGatePassEvidence_ComposedPredicate pins the F4 (083.002-T) composed
// member-evidence predicate: a member gate-pass event counts as valid evidence
// only when it is an EventGateForced (unconditional break-glass) OR an
// EventGatePassed with ran==true. A fail-open EventGatePassed{ran:false} no-run
// must NOT be selected as the latest passing evidence.
func TestLatestGatePassEvidence_ComposedPredicate(t *testing.T) {
	passedRan := events.Event{EventType: EventGatePassed, Delta: map[string]any{"ran": true, "head_sha": "p1"}}
	passedNoRun := events.Event{EventType: EventGatePassed, Delta: map[string]any{"ran": false, "head_sha": "pn"}}
	passedMissingRan := events.Event{EventType: EventGatePassed, Delta: map[string]any{"head_sha": "pm"}}
	forcedRan := events.Event{EventType: EventGateForced, Delta: map[string]any{"ran": true, "head_sha": "f1"}}
	forcedNoRun := events.Event{EventType: EventGateForced, Delta: map[string]any{"ran": false, "head_sha": "fn"}}
	blocked := events.Event{EventType: EventGateBlocked, Delta: map[string]any{"ran": true}}

	tests := []struct {
		name     string
		evs      []events.Event
		wantNil  bool
		wantHead string // head_sha of the expected latest event when non-nil
	}{
		{name: "passed_ran_true", evs: []events.Event{passedRan}, wantHead: "p1"},
		{name: "passed_ran_false_rejected", evs: []events.Event{passedNoRun}, wantNil: true},
		{name: "passed_missing_ran_rejected", evs: []events.Event{passedMissingRan}, wantNil: true},
		{name: "forced_ran_false_still_valid", evs: []events.Event{forcedNoRun}, wantHead: "fn"},
		{name: "forced_ran_true_valid", evs: []events.Event{forcedRan}, wantHead: "f1"},
		{name: "interleaved_forced_then_passed_norun", evs: []events.Event{forcedNoRun, passedNoRun}, wantHead: "fn"},
		{name: "passed_ran_true_then_passed_norun_keeps_ran", evs: []events.Event{passedRan, passedNoRun}, wantHead: "p1"},
		{name: "only_blocked_missing", evs: []events.Event{blocked}, wantNil: true},
		{name: "empty_missing", evs: nil, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := latestGatePassEvidence(tt.evs)
			if tt.wantNil {
				assert.Nil(t, got, "ran=false EventGatePassed / non-pass events must not count as evidence")
				return
			}
			require.NotNil(t, got)
			h, _ := got.Delta["head_sha"].(string)
			assert.Equal(t, tt.wantHead, h)
		})
	}
}

// TestValidateMemberGateEvidence_FailOpenNoRunRejected exercises the F4 predicate
// through the shipment member scan: a terminal member whose only pass evidence is
// a fail-open EventGatePassed{ran:false} is refused as missing evidence.
func TestValidateMemberGateEvidence_FailOpenNoRunRejected(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	// A fail-open no-run: EventGatePassed with ran=false must be treated as missing.
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome": "passed", "ran": false,
	}))

	err = validateMemberGateEvidence(ctx, ws, []string{id}, "")
	require.Error(t, err, "a ran=false EventGatePassed no-run must be treated as missing gate evidence")
	assert.Contains(t, err.Error(), "missing")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
}

// TestValidateMemberGateEvidence_ForcedNoRunAccepted confirms the audited
// break-glass force is unconditional: an EventGateForced counts even with ran=false.
func TestValidateMemberGateEvidence_ForcedNoRunAccepted(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGateForced, map[string]any{
		"outcome": "passed", "ran": false, "forced": true,
	}))

	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, ""),
		"a forced break-glass event must satisfy member evidence regardless of ran")
}

// TestShipmentGate_DecisionErrorConfig_TypedGateError pins F5 (083.003-T): a
// shipment-level DecisionError{config} (autoharness exit 2) preserves exit-7
// class fidelity — it returns a typed *GateError (config class), NOT a
// *GateBlockedError that would collapse to exit 6.
func TestShipmentGate_DecisionErrorConfig_TypedGateError(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
		shipmentRes: gate.GateResult{ExitCode: 2, Stdout: []byte(`{}`), Stderr: []byte("bad config")},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "a shipment DecisionError{config} must refuse the ship")

	var ge *bkerrors.GateError
	require.True(t, stderrors.As(err, &ge), "want *GateError, got %T", err)
	assert.Equal(t, "config", ge.Class)
	assert.False(t, ge.Retryable(), "config class is non-retryable (exit 7)")

	var blocked *bkerrors.GateBlockedError
	assert.False(t, stderrors.As(err, &blocked),
		"a DecisionError must NOT collapse to *GateBlockedError (exit 6)")

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state unchanged on a config error")
}

// TestShipmentGate_DecisionErrorTimeout_RetryableGateError pins F5: a
// shipment-level DecisionError{timeout} returns a retryable *GateError
// (exit-8 class), not a *GateBlockedError.
func TestShipmentGate_DecisionErrorTimeout_RetryableGateError(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
		shipmentErr: bkerrors.ErrGateTimeout,
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err)

	var ge *bkerrors.GateError
	require.True(t, stderrors.As(err, &ge), "want *GateError, got %T", err)
	assert.Equal(t, "timeout", ge.Class)
	assert.True(t, ge.Retryable(), "timeout class is retryable (exit 8)")
}

// TestShipmentGate_ExitOneBlock_StillBlockedError confirms F5 does not regress
// the plain block path: a genuine below-threshold exit-1 block still returns a
// *GateBlockedError (exit 6).
func TestShipmentGate_ExitOneBlock_StillBlockedError(t *testing.T) {
	ws := newGateTestWorkspace(t)
	report := `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
		shipmentRes: gate.GateResult{ExitCode: 1, Stdout: []byte(report)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err)
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "an exit-1 block must remain *GateBlockedError")
	var ge *bkerrors.GateError
	assert.False(t, stderrors.As(err, &ge), "a plain block must not be a *GateError")
}
