---
agent: stage
session_id: stage-d3ce9e81-2026-08-24
cycle: pr377-review-remediation-cycle-8
scope: D3CE9E81
shipment: 130-S
feature: 147-F
branch: chore/stage-130-s
worktree: C:\Source\GitHub\backlogit\.copilot\session-state\337f2436-0fad-4797-be93-b72985d25d56\files\stage-130s-worktree
created_at: 2026-08-25T20:35:00Z
---

# Stage — PR #377 Copilot review remediation cycle 8

An eighth Copilot review against the current PR head raised three comments — one each on
`147.005-T` (U2d), `147.010-T` (U5b), and `147.016-T` (U8b). Each comment identified the same
underlying failure: a task body declared a "regression guard — green on landing, no red phase"
exemption from P-004, and no such exemption exists in
`.github/policies/workflow-policies.md` P-004. Cycle 8 is an **operator-authorized extension**
past the three-cycle limit, recorded as such rather than treated as a silent counter reset.

GitHub PR #377 is authoritative for the current PR head, CI, review coverage, push status, and
unresolved threads. This memory intentionally does **not** SHA-pin the reviewed head, because
every review-remediation cycle advances the branch tip and any recorded SHA goes stale the
moment the branch advances further. Consumers must query the live PR before treating any
merge-readiness claim as valid.

## Reviewer's finding

P-004 (`.github/policies/workflow-policies.md` lines 76–90) defines the harness-ready label as:

> the harness compiles **and** fails RED before implementation

with the precondition:

> `go test -run=^$ -count=1 ./...` exits 0 **and** `go test ./...` exits non-zero with expected
> failure markers

There is no "regression guard" carve-out and no "green on landing" exemption. Cycles 1–7 allowed
three tasks (U2d, U5b, U8b) to declare themselves exempt on the grounds that they pin
pre-existing shipped behaviour. Cycle 8 retires that framing entirely.

## Root causes and remediation

### 1. All-guards exemption text on three tasks and in the plan preamble

The Test-First posture section of the plan, Constitution Check II, and the bodies of `147.005-T`,
`147.010-T`, and `147.016-T` all carried some form of "declared regression guard / green on
landing, no red phase" exemption. P-004 does not permit this.

**Fix.** Removed the all-guards exemption language everywhere it appeared. Each of the three
tasks now carries a concrete red load against the pre-implementation state, described below.
Test-First posture text now permits a unit to **contain** guards, but requires at least one case
to fail against the pre-delta state; Constitution Check II now states the concrete red load for
each of U2d, U5b, and U8b instead of claiming an exemption.

### 2. `147.005-T` (U2d) — rebalance to own a real production delta

Under cycles 1–7, U2 introduced the `checkpointV1AllTopLevelKeys` derived set and refactored
`CheckConformingTopLevelNamespace` to consult it. That left U2d with only invariant assertions
and no red load.

**Fix — rebalance across the U2 / U2d boundary.** The derived-set introduction and the
conformance-check refactor **move from U2 to U2d**:

- **U2 (147.002-T)** keeps its conformance-helper skeleton and unknown-key rule but wires the
  read-boundary legal-key set as an **inline two-set check** —
  `isFoldKeyIn(k, checkpointV1TopLevelKeys) || isFoldKeyIn(k, checkpointV1ReservedKeys)` —
  against the two already-existing sets. Case 3 (four `disposition*` keys with
  `status: "abandoned"` → nil) is preserved as the reserved-key admission guard so U2 does not
  lose behavioural coverage.
- **U2d (147.005-T)** now owns the production delta: land
  `var checkpointV1AllTopLevelKeys = map[string]struct{}{}` as an empty declaration stub, land
  three test cases (case 1 asserts set equality against
  `checkpointV1TopLevelKeys ∪ checkpointV1ReservedKeys`; cases 2 and 3 are declared regression
  guards for the pre-existing `json:"-"` absence and the tag-coverage invariant), then fill in
  the derivation and refactor `CheckConformingTopLevelNamespace` to consult
  `checkpointV1AllTopLevelKeys` instead of the two-set inline check.

Case 1's set-equality assertion is RED against the empty declaration stub. Cases 2 and 3 are
guards for state that already holds and pass at landing. P-004 is satisfied by case 1 as the
single red assertion.

Files list gains `internal/events/checkpoint_conformance.go` (previously listed only the
_test.go). No task added, no dep edge changed.

### 3. `147.010-T` (U5b) — rebalance to own a real production delta

Under cycles 1–7, U5b claimed "no production change" and existed only to pin the state-scoping
of invariant I3. An initial cycle-8 iteration proposed wrapping `QuarantineCheckpoint`'s
`ErrCheckpointUseAbandon` return with the target `baseName` for observability parity — that
proposal has been **retracted as out-of-scope refusal-observability** (the frozen feature
explicitly excludes refusal observability such as counters, logs, telemetry, and error-message
enrichment whose end-goal is operator observability rather than a semantic classification
change).

**Fix — extend U5b with a state-scoped classification delta.** In `QuarantineCheckpoint`, after
the existing `validTarget` classification block, add a status check: when a target parses,
validates, and is conforming, but `cp.Status != "active"`, refuse with
`ErrCheckpointNotActive` wrapped as `%w: status=%s` rather than the bare
`ErrCheckpointUseAbandon`. Only a valid+conforming **active** target continues to be refused
with `ErrCheckpointUseAbandon`. Change form, inserted between the `validTarget` block and the
archive/audit path:

```go
if validTarget {
    if cp.Status != "active" {
        return fmt.Errorf("%w: status=%s", blerrors.ErrCheckpointNotActive, cp.Status)
    }
    return blerrors.ErrCheckpointUseAbandon
}
```

This corrects a pre-existing misleading redirect: today, a `status:"resolved"` conforming
document hitting quarantine returns `ErrCheckpointUseAbandon` ("use abandon instead"), but the
same document then hits `AbandonCheckpoint`'s existing `status != "active"` gate and is
refused with `ErrCheckpointNotActive` — the operator is bounced between verbs with neither
surface naming the actual state-class problem. After U5b, both mutation verbs converge on the
same truthful state-class sentinel for the non-active conforming case, so the pre-existing
double-refusal state-conflict class named in the plan's Decisions and Rationale (Row 3 of the
I3 discussion) is disclosed at first contact.

Semantic constraints preserved:

- Quarantine is **not** widened to accept the state-conflict class; it still refuses every
  non-active conforming target (only the sentinel it returns changes).
- U5's active-scope hard gate (Row 2 of the I3 table) is **unchanged**: a fully conforming
  `status:"active"` document refused by quarantine still returns `ErrCheckpointUseAbandon`.
- `errors.Is` domain behaviour is honoured for both cases:
  `errors.Is(err, ErrCheckpointUseAbandon)` for the active row (U5's row-2 guard) and
  `errors.Is(err, ErrCheckpointNotActive)` for the non-active row (now cross-surface with
  abandon).
- No new sentinel is introduced. `ErrCheckpointNotActive` already exists in
  `internal/errors/checkpoint_errors.go` and is already routed through the MCP disposition
  error response (`internal/mcp/errors.go:329` maps it to `checkpoint_not_active`), so no
  MCP code changes and no U7/U7d follow-up.
- Error-message enrichment is not the end-goal — the `%w: status=%s` form matches
  `AbandonCheckpoint`'s pre-existing wrap for the same sentinel so the two surfaces produce
  the same shape; the classification correction is the delta.
- Existing shipped test `TestQuarantineCheckpoint_ValidRefusesNamingAbandon` uses a fixture
  from `validDispositionTestCheckpoint()` whose `Status` is `"active"`, so it stays green:
  it exercises U5's row-2 branch, which still returns `ErrCheckpointUseAbandon`.

U5b's three test rows: case 1 (RED) asserts a `status:"resolved"` conforming document refused
by `QuarantineCheckpoint` returns an error where `errors.Is(err, ErrCheckpointNotActive)`
holds and the message names the status; case 2 (guard) asserts the same document refused by
`AbandonCheckpoint` returns `errors.Is(err, ErrCheckpointNotActive)` naming the status
(pre-existing shipped behaviour, colocated so a future edit cannot decouple the two surfaces'
state-class agreement); case 3 (guard) asserts a `status:"active"` conforming document
refused by `QuarantineCheckpoint` returns `errors.Is(err, ErrCheckpointUseAbandon)` — U5's
row-2 hard gate, unchanged. Case 1 is RED against U5's landing state, where
`QuarantineCheckpoint` returns bare `ErrCheckpointUseAbandon` for the resolved conforming
target and `errors.Is(ErrCheckpointUseAbandon, ErrCheckpointNotActive)` is false.

Files list gains `internal/core/checkpoint_disposition.go`. No task added, no dep edge changed.

### 4. `147.016-T` (U8b) — restructure Expected Red for the batch-harness moment

Under cycles 1–7, U8b claimed "Expected red: none — the assertions codify the agreement earlier
commits land". That framing implicitly assumed harnesses land after each dep's implementation.
The reviewer noted this contradicts the mandatory batch harness: at batch-harness generation
time, deps have declaration stubs but not full implementations. Against declaration stubs, U8b's
parity assertions must actually fail before the deps' implementations turn them green.

**Fix — restate Expected Red as a per-fixture-row enumeration of specific assertion failures
against declaration stubs and current handlers at batch-harness time**:

- `legacy-shaped` row: `events.GetCheckpoint` currently returns success for a schema-invalid
  document because the U6b/`147.012-T` validity gate is not yet in its declaration stub;
  `errors.Is(err, ErrCheckpointInvalid)` **fails**. MCP `get_checkpoint` — pre-U6c/`147.022-T`,
  `code == "validation_failed"` **fails**. CLI `checkpoint get` — pre-U8c/`147.027-T` reprojection
  of `newCheckpointGetCmd` (hardcoded `"valid": true`), `exit_code != 0` **fails**. `resolve` —
  pre-U3/pre-U7d, resolve succeeds and rewrites; refusal with `checkpoint_use_quarantine`
  **fails**.
- `valid-but-non-conforming` row: `result.Conforming == false` **fails** against the U6b/U6c/U8c
  declaration stubs; refusals with `checkpoint_non_conforming` on resolve and abandon **fail**
  against pre-U3b and pre-U4 handlers. The byte-identity postcondition **fails** against the
  current `ResolveCheckpoint`'s rewrite.
- `conforming-active` row: every surface accepts `abandon`; `get` reports `conforming: true`.
  Pre-existing shipped behaviour — declared regression guard.

Rows 1 and 2 carry the aggregate red load; row 3 is a declared regression guard for the accept
path. This unit no longer claims an all-guards exemption. U8b remains test-only in the sense
that it produces no new production code; once all deps implement in order, every assertion turns
green and U8b takes on its intended parity-contract / regression-guard role: pin the agreement
so a future regression in any one surface (a stale MCP projection, a CLI hardcode, an
events-layer default) surfaces as a failing test instead of a silent drift.

No task added, no dep edge changed.

## Framing correction: live-PR-authoritative semantics

Cycles 5, 6, and 7 recorded the checkpoint and memories in a `reviewed_head` vs
`local_unreviewed_state` framing that captured the SHA Copilot had reviewed against and the
local tip that was pending push and re-review. That framing is retired in cycle 8. The
operator's directive is:

> Update the durable canonical checkpoint and `stage-d3ce9e81-checkpoint-toplevel-keys` memory
> to record cycle 8 and any intentional shape/edge changes, using live-PR-authoritative
> semantics (no local-only/unpushed/current-remote claims).

Applied here as: the canonical checkpoint's `ci_state`, `pr`, `push_state`, `resume_ref`, and
`resume_hint` no longer name a reviewed SHA or claim any push status. They defer to live PR
#377 state and require consumers to query it. This memory file follows the same rule.

## Net effect

- **No task added.**
- **No dep edge changed.**
- **No shipment member added.**
- **Backlog shape stays 27 tasks / 43 edges / 28 shipment members.**
- Reviewed decision, scope, data-loss safety posture, fail-closed refusal, checkpoint-safety
  design, shell contract, repair mapping, 147.018-T same-merge requirement, and the 147.009-T
  paired-assertion halt condition are unchanged.
- Three affected tasks now carry P-004-compliant harnesses with concrete pre-implementation-state
  red assertions.

## Files changed

- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — Test-First posture
  section reworded, Constitution Check II reworded, U2 / U2d / U5b / U8b sections rebalanced,
  cycle-8 plan-hardening remediation register entry appended.
- `.backlogit/queue/147.002-T.md` — description reflects the inline two-set check ownership;
  acceptance criterion 3 reworded to preserve reserved-key admission guard.
- `.backlogit/queue/147.005-T.md` — description rewrites U2d as an events code+tests unit owning
  the derived-set introduction and the conformance-check refactor; acceptance criteria include
  the two-step red posture and the single red case; all-guards exemption text removed.
- `.backlogit/queue/147.010-T.md` — description gains the state-scoped classification delta
  on `QuarantineCheckpoint` (state-truthful refusal via `ErrCheckpointNotActive` for
  non-active conforming targets); acceptance criteria include the `errors.Is(err,
  ErrCheckpointNotActive)` assertion on the quarantine surface as the single red case, plus
  the non-widening guarantee, the U5 row-2 hard-gate preservation, the shipped-test
  preservation, and the retraction of the prior cycle-8 observability-wrap proposal as
  out-of-scope refusal-observability; "no production change" and "green on landing" claims
  removed.
- `.backlogit/queue/147.016-T.md` — Expected Red narrative restructured to enumerate specific
  assertion failures against declaration stubs and current handlers at batch-harness time;
  "Expected red: none" and posture-exemption text removed.
- `.backlogit/checkpoints/checkpoint-20260824-191617.json` — cycle-8 entry appended to
  `context.review_remediation`; `ci_state`, `pr`, `push_state`, `resume_ref`, `resume_hint`
  reframed to live-PR-authoritative semantics; `updated_at` bumped; `memory_path` updated to
  include the cycle-8 memory reference.
- `.backlogit/memories.json` — cycle-8 addendum appended to
  `stage-d3ce9e81-checkpoint-toplevel-keys`.
- `docs/memory/2026-08-24/stage-pr377-remediation-cycle-8-memory.md` — this file.

## Next safe action

Query the live PR #377 head, CI, and Copilot review coverage. If the current head lacks green
CI or fresh Copilot coverage against that head, obtain them. Stage stopped at its Role
Boundary; Ship or the operator advances the remote, posts replies for the three cycle-8
threads, resolves them, and merges under the hard merge gate: `147.018-T` (U9b) must land in
the same merge commit as `147.007-T`, `147.008-T`, and `147.009-T`. If the `147.009-T`
paired accept/refuse assertion cannot pass, halt rather than weaken it.
