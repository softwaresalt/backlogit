---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stashes B85DAEE8 + 1AEA2B0E: close the two remaining empty-head fail-open holes in internal/core/shipment_gate.go. Under enforcement in a real git worktree, an empty shipment head (1AEA2B0E) OR an empty member head_sha (B85DAEE8) must fail closed; the no-repo / non-enforcement / non-autoharness skip is preserved via a bounded, fail-closed repo-presence probe. One feature, one test-first task, three ordered subtasks (repo-presence helper + fixtures; empty-shipment-head wiring; empty-member-head wiring). Mirrors the 084-F ancestor-aware bounded/fail-closed discipline. F3844849 (malformed-JSONL) is out of scope.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-07-shipment-gate-empty-head-fail-closed-plan.md
title: 'Shipment-gate empty-head fail-closed hardening'
---

# Shipment-gate empty-head fail-closed hardening

**Source deliberation:** `docs/decisions/2026-07-07-shipment-gate-empty-head-fail-closed-deliberation.md`
**Stashes:** `B85DAEE8` (low/bug — empty member head skips staleness), `1AEA2B0E` (low/bug — empty shipment head under enforcement skips member-lineage + drift guard)
**Prior art:**
`docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` (names these two seams as deferred 084 scope),
`docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` (the `Enforced` contract),
`docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` (bounded git helper cap),
`docs/compound/2026-07-06-external-process-timeout-before-probe.md` (bound lock-holding external calls),
`docs/closure/2026-07-06-083-S-feature-pr-adversarial-review.md` (origin of both advisory items)

## Source

- **B85DAEE8** (low/bug): "Gate member-evidence: empty head_sha skips the staleness comparison, so evidence authored without a head SHA cannot be detected as stale." Advisory hardening candidate from 083-S adversarial reviewer B (non-blocking).
- **1AEA2B0E** (low/bug): "Shipment gate enforced empty-shipment-head fail-open: in `gateShipmentCompletion`, when the gate is ENFORCED but `ws.headSHA/headSHABounded` resolves HEAD to empty for a NON-context reason (git rev-parse fails, e.g. not a repo), the member-lineage staleness check is skipped entirely rather than failing closed." Pre-existing; discovered/FLAGGED during 084-F plan-harden.
- Both are the two seams 084-F explicitly deferred (see prior-art compound doc, lines 56–59). The bounded-read timeout/cancel path added by 084-F already fails closed (`headResolveError`); only the **legacy non-context empty** cases remain.

## Problem Frame

`internal/core/shipment_gate.go` gates the `shipment ship` path via
`gateShipmentCompletion` → `validateMemberGateEvidence`. Two empty-head cases still
**fail open** under enforcement:

**Hole 1 — empty member head (B85DAEE8), `shipment_gate.go:351-352`:**

```go
if shipmentHead != "" {
    if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {
        // h != "" preserves the empty-member-head bypass (B85DAEE8, out of scope) ...
    }
}
```

When a terminal gated member's recorded evidence `head_sha` is empty, the `h != ""`
short-circuit skips the entire lineage/staleness comparison → the member ships with
no proof its gated commit is contained in the shipment history.

**Hole 2 — empty shipment head under enforcement (1AEA2B0E), `shipment_gate.go:198, 229, 293`:**

`shipmentHead, headErr := ws.headSHABounded(ctx)` can return a **legacy non-context
empty** `("", nil)` when `git rev-parse HEAD` fails for a non-timeout reason. Under
enforcement (`ev.Enforced == true`, `headErr == nil`) that empty `shipmentHead`:
- is passed into `validateMemberGateEvidence`, which then skips the whole member
  scan via `if shipmentHead != ""` (351), and
- skips the head-drift stable-head assertion via `if ev.Enforced && shipmentHead != ""` (293).

Net: the enforced gate cannot verify lineage but ships anyway.

**The invariant to establish:** under enforcement in a real git worktree, a resolved
shipment head is mandatory and every gated member must carry a verifiable
(non-empty, well-formed, ancestor-or-equal) head. The discriminator is **repo
presence**, because `ev.Enforced` does NOT track it — the test broker fakes the git
probe (`injectBroker` → `Git: fakeGitAllOK{}`), so `Enforced == true` even in a
no-repo temp dir (`broker.go:91`).

### Key supporting facts (verified against post-084 `main`)

- Evidence authoring writes `delta["head_sha"]` **only when non-empty**
  (`gate_transition.go:407-409`), so an **empty member `head_sha` ⟺ the member's gate
  was authored without a resolvable HEAD** (no-repo / resolution failure at task time),
  whether `Passed(ran)` or `Forced`. In a real repo a `Forced` gate records a
  non-empty head, so forced members never hit the empty-head branch there — no forced
  exception is needed.
- `TestShipmentGate_AllMembersHaveEvidence_Ships` (`shipment_gate_test.go:55`) runs
  `EnabledTrue` in a **no-repo** temp dir and asserts the ship **succeeds** — the
  no-repo skip MUST be preserved.
- **R7** inside `TestValidateMemberGateEvidence_StaleRefused`
  (`shipment_gate_test.go:184-186`) runs in a **real repo** with a non-empty
  `shipmentHead` and currently asserts an empty member head is **accepted**. A
  repo-wide grep confirms this is the ONLY test asserting empty-member-head-accepted
  in a real repo; its expectation flips to fail-closed here.

## Requirements Trace

| # | Source requirement | Implementation action | Unit |
|---|---|---|---|
| R-1 | Discriminate real repo from no-repo, fail-closed on probe error | Add bounded repo-presence probe `inGitWorktreeBounded` reusing 084 exec/timeout discipline | ST1 |
| R-2 | Add a real-repo-with-empty-HEAD fixture (unborn branch) for tests | Add `initGitRepoNoCommits` test fixture | ST1 |
| R-3 | 1AEA2B0E: enforced + real worktree + empty shipment head → fail closed | Wire the probe into `gateShipmentCompletion` after the `headErr` check | ST2 |
| R-4 | Preserve no-repo / non-enforcement skip for empty shipment head | `!inRepo` keeps `shipmentHead == ""`; member scan + drift guard stay inert | ST2 |
| R-5 | B85DAEE8: enforced + real worktree + empty member head → fail closed | Inside `if shipmentHead != ""`, empty member head → typed refusal | ST3 |
| R-6 | Preserve no-repo skip for empty member head | Empty member head under `shipmentHead == ""` is still skipped (block not entered) | ST3 |
| R-7 | Red-then-green tests for each hole; regression tests for legitimate-empty | Test scenarios distributed across ST1–ST3 (each < 4 scenarios) | ST1–ST3 |

## Implementation Units

One **feature** → one **test-first task** → three **ordered subtasks**. Every subtask
is Go core code+tests under the test-first posture (the established repo convention,
mirroring the 084-F plan): the red test lands first, then the minimal green change,
producing a passing `go build ./... && go test ./internal/core/...` as its atomic
milestone. Each subtask touches < 3 files, modifies < 5 functions, and adds < 4 test
scenarios.

### Task T1 — Fail closed on empty shipment/member heads under enforcement in a real worktree (test-first)

Covering task for the feature; its acceptance is the union of ST1–ST3 acceptance
plus the full **quality-gate quartet** run once at the end of the task:
`gofmt -l internal/core` (empty output), `go vet ./internal/core/...` (clean),
`golangci-lint run ./internal/core/...` (clean, matching CI), and a full
`go test ./...` (not just `./internal/core/...`) to catch any cross-package regression
from the gate behavior change. All four must pass before the task is considered done.

#### Subtask ST1 — Bounded, fail-closed repo-presence probe + fixtures (test-first)

- **Changes:**
  - Add `func (ws *Workspace) inGitWorktreeBounded(ctx context.Context) (bool, error)`
    to `internal/core/shipment_gate.go`, landed **first as a stub** (`return false, nil`)
    so ST1 test (a) is observed to fail **red** rather than producing a compile error
    (Principle II). Then implement the body: run `git rev-parse --is-inside-work-tree`
    under `ws.boundedHelperTimeout()` with the 084 exec trust boundary
    (`exec.CommandContext`, argv-array, `cmd.Dir = ws.RootPath`,
    `cmd.Env = gate.MinimalEnv()`). Capture stdout **and** stderr into `bytes.Buffer`s
    via `cmd.Stdout`/`cmd.Stderr` + `cmd.Run()` (mirror `isAncestor`, not `cmd.Output()`),
    and parse with `bytes.Equal(bytes.TrimSpace(stdout.Bytes()), []byte("true"))` so no
    new `strings` import is added. Exit-code map:
    - `runCtx.Err() != nil` (checked **first**, Windows gotcha) → `(false, ctxErr)` — fail closed.
    - `runErr == nil` + stdout `true` → `(true, nil)` — real worktree.
    - `runErr == nil` + stdout not `true` (bare repo / inside `.git`) → `(false, nil)` — not-a-worktree skip.
    - `ExitError` code 128 **whose stderr matches** the not-a-repository signal
      (`/not a git repository/`, case-insensitive) → `(false, nil)` — no repo, legacy skip.
    - `ExitError` code 128 with **any other** stderr (corrupt `.git`, unreadable objects,
      permission, bad HEAD) → `(false, err)` — **fail closed** (a present-but-broken repo
      is NOT a no-repo skip; wrap git's diagnostic like `isAncestor` L103-104).
    - any other non-ExitError / git-missing → `(false, err)` — fail closed.
    Rationale for `--is-inside-work-tree` over `--git-dir`: `--git-dir` returns exit 0
    inside a bare repo and inside `.git`, so it cannot distinguish "can resolve a
    work-tree HEAD" from "is under some git dir".
  - Add `initGitRepoNoCommits(t *testing.T, dir string)` fixture to
    `internal/core/shipment_gate_ancestry_test.go` — `git init` with **no commit**
    (unborn branch): `--is-inside-work-tree` → true, `rev-parse HEAD` → empty. Skips
    when git is not on PATH (mirrors `initGitRepoWithCommits`).
- **Files:** `internal/core/shipment_gate.go`, `internal/core/shipment_gate_ancestry_test.go`.
- **Functions:** 1 new helper (+1 test fixture).
- **Tests (3 scenarios):** (a) real repo (`initGitRepoWithCommits`) → `(true, nil)`;
  (b) no-repo temp dir (`newGateTestWorkspace`) → `(false, nil)`; (c) already-expired
  bounded context → `(false, ctxErr)` fail closed.
- **Execution posture:** test-first.
- **Acceptance:** helper tests pass; `go build ./...` green; no change to existing gate behavior yet.

#### Subtask ST2 — Empty shipment-head fail-closed under enforcement (1AEA2B0E) (test-first)

- **Changes:** in `gateShipmentCompletion`, after the existing
  `if headErr != nil { return headResolveError(...) }` (223-225) and before the member
  scan, insert: when `shipmentHead == ""`, call `ws.inGitWorktreeBounded(ctx)`:
  - probe error → fail closed via `headResolveError(shipmentID, probeErr)` (a real cause
    exists); the probe error wrap uses a "cannot determine repository presence" phrasing
    for operator triage.
  - `inRepo == true` → fail closed via a **dedicated** constructor
    `shipmentHeadUnresolvedInRepoError(shipmentID)` returning `*GateBlockedError`
    (fields mirror `shipmentMemberEvidenceError`: `OldStatus`/`NewStatus`=`StatusActive`,
    `StateChanged=false`, `Outcome="blocked"`), message "cannot resolve shipment head in
    repository". A dedicated constructor is used rather than shoe-horning a synthetic
    cause into `headResolveError` (whose `%v: %w` shape expects a real cause), giving a
    clean, assertable message distinct from the bounded-read timeout.
  - `inRepo == false` → preserve the legacy skip (`shipmentHead` stays `""`; member
    scan + drift guard remain inert).
  - **Observability (Principle V):** both fail-closed branches emit an
    `EventGateBlocked` evidence event (mirror the shipment-diff refusal, `shipment_gate.go`
    266-275) with `"outcome":"blocked"`, `"level":"shipment"`, and a
    `"reason":"empty-shipment-head"` field, AND a `slog.WarnContext`, so the monitoring
    signal in the closure section is real (the sibling malformed/lineage member branches
    at 360-372 already `WarnContext`; the empty-head branch must not be the only silent
    refusal).
- **Files:** `internal/core/shipment_gate.go`, `internal/core/shipment_gate_test.go`
  (or `shipment_gate_headdrift_test.go`).
- **Functions:** modify `gateShipmentCompletion`; add `shipmentHeadUnresolvedInRepoError`
  constructor.
- **Tests (3 scenarios):** (a) RED-first: `initGitRepoNoCommits` real repo (unborn HEAD)
  + `EnabledTrue` → `ShipShipment`/`gateShipmentCompletion` **refuses** with a typed
  `*GateBlockedError`, shipment state unchanged, and an `EventGateBlocked` evidence event
  recorded; (b) regression: no-repo temp dir + `EnabledTrue` still ships (explicit
  no-repo empty-shipment-head-skip test, peer to `TestShipmentGate_AllMembersHaveEvidence_Ships`);
  (c) `shipmentHeadUnresolvedInRepoError` message/type unit assertion (mirrors
  `TestHeadResolveError`).
- **Execution posture:** test-first.
- **Depends on:** ST1.
- **Acceptance:** both scenarios pass; `TestShipmentGate_AllMembersHaveEvidence_Ships`
  and all existing no-repo gate tests stay green.

#### Subtask ST3 — Empty member-head fail-closed under enforcement + real repo (B85DAEE8) (test-first)

- **Changes:** in `validateMemberGateEvidence`, inside the `if shipmentHead != ""`
  block, split the member-head handling: read `h`; when `h == ""` → emit a
  `slog.WarnContext` (mirroring the sibling malformed/lineage branches at 360-372) **and**
  append an `EventGateBlocked` evidence event with `"outcome":"blocked"`,
  `"level":"member"`, `"reason":"empty-member-head"`, and the member id (mirroring the
  ST2 shipment-level emission and the existing shipment-diff refusal at 266-275), then
  return
  `shipmentMemberEvidenceError(id, "gate evidence has no recorded head_sha (cannot verify lineage under enforcement)")`
  (fail closed). The evidence event — not just the returned typed error — is what makes
  the `"reason":"empty-member-head"` monitoring signal in the closure section real
  (Constitution Principle V). When `h != ""` keep the existing malformed-shape guard +
  `isAncestor` lineage check unchanged. Update the function doc-comment to (1) record that
  the empty-member-head bypass is now closed (supersedes the B85DAEE8 note) **and**
  (2) state the *accurate* caller invariant: *"This function's empty-`head_sha`
  fail-closed executes only inside the `shipmentHead != ""` block. A non-empty
  `shipmentHead` is produced solely by `headSHABounded` resolving a real `HEAD`, which
  itself proves a real worktree with a committed HEAD — so the empty-member-head refusal
  can never fire in a no-repo/unresolved-head context. `inGitWorktreeBounded` is the
  discriminator on the *empty*-`shipmentHead` branch (in `gateShipmentCompletion`), not a
  precondition of this function. A future caller that passes a non-empty `shipmentHead`
  NOT obtained from a resolved `HEAD` would break this invariant."* (This corrects the
  earlier draft that mis-attributed the non-empty guarantee to `inGitWorktreeBounded`.)
- **Files:** `internal/core/shipment_gate.go`, `internal/core/shipment_gate_test.go`.
- **Functions:** modify `validateMemberGateEvidence`.
- **Tests (3 scenarios):** (a) **flip R7** in `TestValidateMemberGateEvidence_StaleRefused`:
  real repo + empty member head → **refuse** (was accept), assert typed
  `*GateBlockedError` + message **and** assert an `EventGateBlocked` evidence event with
  `"reason":"empty-member-head"` was recorded (grounds the closure monitoring signal);
  (b) add: no-repo / `shipmentHead == ""` + empty member
  head → skip preserved (`validateMemberGateEvidence(ctx, ws, []string{emptyMember}, "")`
  → `NoError`); (c) confirm the non-empty ancestor/equal/divergent/malformed cases
  (R1/R2/R3/R6/R4) are unchanged.
- **Execution posture:** test-first.
- **Depends on:** ST1 (fixtures) and follows ST2 to serialize edits to
  `shipment_gate.go`. Note: the invariant that "`shipmentHead != ""` ⟹ real worktree with
  resolved HEAD" is provided by `headSHABounded` (pre-existing), **not** newly established
  by ST2; ST2 handles the complementary empty-`shipmentHead` branch. The empty-member-head
  refusal's no-repo safety rests on the pre-existing `if shipmentHead != ""` guard (member
  checks stay inert when the head is empty).
- **Acceptance:** R7 flipped and green; no-repo member-skip test green; the full
  `internal/core` gate suite green; `go vet ./internal/core/...` clean.

## Dependency Graph

```
T1 (covering task)
└─ ST1 (repo-presence helper + fixtures)
   └─ ST2 (empty shipment-head fail-closed; depends ST1)
      └─ ST3 (empty member-head fail-closed; depends ST2)
```

Linear chain, no cycles. ST1 → ST2 → ST3. Rationale: ST1 provides the repo-presence
discriminator ST2 needs. ST2 → ST3 ordering serializes the edits to `shipment_gate.go`
(avoiding merge churn) and lands the observability/evidence pattern in ST2 that ST3
reuses. Correctness note: the "`shipmentHead != ""` ⟹ real worktree with resolved HEAD"
invariant that keeps ST3's empty-member-head refusal from firing in no-repo is provided
by `headSHABounded` (pre-existing), not established by ST2 — ST2 handles the complementary
empty-`shipmentHead` branch. The dependency edge is therefore an edit-ordering + pattern-
reuse edge, not a semantic prerequisite.

## Decisions and Rationale

1. **Repo-presence probe as the discriminator (Option A).** `ev.Enforced` cannot
   distinguish no-repo from real-repo-empty-head (broker fakes the probe). A separate
   bounded `git rev-parse --is-inside-work-tree` is the minimal, precise signal.
   Rejected: flag-only fail-closed (breaks the no-repo suite) and converting the whole
   test harness to real repos (high churn, no production benefit). See deliberation.
2. **No forced-evidence exception; corrected empty-member-head provenance.** An empty
   member `head_sha` is written whenever `outcome.HeadSHA == ""` at task-completion time
   (`gate_transition.go:407-409`), i.e. whenever the best-effort `ws.headSHA(ctx)` read
   returned empty. That happens in **no-repo authoring** OR — less commonly — a
   **transient/degraded `git rev-parse HEAD` failure in a real repo** even under strict
   completion (Evaluate can succeed via base-ref resolution while the separate HEAD read
   fails). Either way, at ship time in a real worktree the gate **cannot prove the
   member's gated commit is contained in the shipment history**, so fail-closed is the
   correct security outcome regardless of provenance — no forced exception is needed
   (a forced gate in a real repo normally records a head; a forced member with an empty
   head is exactly the unverifiable case). **Accepted consequence:** a strict real-repo
   member whose head recording transiently failed at completion will be refused at ship
   time; the supported recovery is the escape valve in Decision 6. An **upstream**
   hardening (strict task completion/force should itself fail closed when `HeadSHA`
   cannot be recorded in a real repo) is a **separate, out-of-scope follow-up** captured
   as a new stash (see Risks R6); it is not required for this fix to be correct.
3. **Dedicated refusal constructor for empty-shipment-head-in-repo.** A small
   `shipmentHeadUnresolvedInRepoError(shipmentID)` returns `*GateBlockedError` with a
   distinct "cannot resolve shipment head in repository" message, so operators can tell
   it from a bounded-read timeout (`headResolveError`) and from a member refusal. Avoids
   shoe-horning a synthetic cause into `headResolveError`'s `%v: %w` shape.
4. **Fail-closed on probe error.** If we cannot even determine repo presence under the
   deadline, we cannot prove lineage → refuse. Consistent with `isAncestor`.
5. **Minimal error-taxonomy growth.** Reuse `*GateBlockedError` and the existing
   `shipmentMemberEvidenceError` shape; add exactly one dedicated constructor
   (`shipmentHeadUnresolvedInRepoError`) for message/observability clarity — no new error
   *types*, only a typed-error constructor.
6. **Escape valve (accurate).** There is **no** shipment-level force override
   (`ShipShipment`/`gateShipmentCompletion` take no `opts.Force`), and an already-terminal
   member will **not** re-run the per-task gate (`gateApplies` short-circuits terminal
   artifacts), so "re-run the gate" is **not** a reliable recovery for a terminal member.
   The **supported** recoveries under strict enforcement are: (a) for empty-shipment-head,
   fix the underlying git state so HEAD resolves (commit / repair HEAD) and re-ship; (b)
   for empty-member-head, lower `pre_task_completion_gate.enabled` to `auto`/`false` for
   the ship. A dedicated re-evidence workflow for terminal members is a possible future
   enhancement, tracked with the Risks-R6 follow-up. This is consistent with strict
   mode's opt-in "refuse if unverifiable" contract.

## Risks and Caveats

- **R1 — Test expectation flip (R7).** Intended. Mitigated by splitting R7 into
  real-repo-refuse + adding a no-repo-skip test; `TestShipmentGate_AllMembersHaveEvidence_Ships`
  stays green.
- **R2 — Git-exec portability** (Windows exit codes, unborn branch, bare repo, inside
  `.git`, corrupt repo). Mitigated by the 084 bounded exec discipline and the finalized
  exit-code matrix in `## Plan Hardening` (exit 128 is disambiguated by stderr so a
  corrupt-but-present repo fails closed instead of skipping).
- **R3 — Lock-holding DoS via hung probe.** Mitigated by `boundedHelperTimeout` cap
  (`ancestryCheckTimeout`).
- **R4 — Cross-environment / transient false refusal** (member gated in no-repo, or a
  strict real-repo member whose HEAD read transiently failed at completion, shipped in a
  real repo). Accepted: strict mode correctly refuses an unverifiable lineage; recovery
  is the Decision 6 escape valve (fix git state / lower enforcement). Frequency is low
  and bounded to the empty-`head_sha` population.
- **R5 — Extra subprocess cost.** Only on the rare empty-shipment-head path;
  equality/ancestor fast-paths untouched.
- **R6 — Upstream provenance gap (out of scope, follow-up).** Task evidence records
  `head_sha` best-effort (`gate_transition.go:407`), so a strict real-repo completion can
  persist evidence with no `head_sha` when the HEAD read fails. This fix correctly
  refuses such a member at ship time, but the *root* would be better closed upstream by
  failing strict task completion closed when `HeadSHA` cannot be recorded. Captured as a
  new stash for a future phase; **not** implemented here to keep scope to the two named
  ship-gate holes.

## Plan Hardening Signals (REQUIRED)

- **public API / schema / contract change:** ABSENT — internal `internal/core`
  functions only; no exported signature or on-disk schema change. (The *behavioral*
  contract of the enforced shipment gate tightens; no wire/API change.)
- **security / auth / permission / compliance-sensitive behavior:** **PRESENT** — this
  is a fail-open → fail-closed security hardening of the shipment-completion gate, the
  reconciliation guarantee that release finalization cannot bypass gating. Getting the
  boundary wrong either re-opens the hole or over-refuses legitimate ships.
- **migration / backfill / destructive / irreversible step:** ABSENT.
- **external integration / operator checkpoint / external dependency:** **PRESENT
  (narrow)** — spawns a `git` subprocess (external process) on the ship path under the
  workspace lock; must be bounded and fail-closed.
- **high runtime / rollout / rollback risk:** **PRESENT (moderate)** — changes the
  behavior of an enforced ship gate; a regression could block legitimate shipments
  (`ShipShipment`). Rollback is a code revert (no state migration).

**Requires plan hardening: yes**

## Runtime Verification and Closure

- **Runtime surface changed:** the `backlogit shipment ship` path (CLI + MCP
  `backlogit_ship_shipment`) under `pre_task_completion_gate.enabled: true`. No new
  flags or output schema.
- **Runtime verification (seed for plan-harden / Ship):**
  1. Real repo + `enabled:true` + a member with empty recorded `head_sha` → `shipment
     ship` refuses (typed blocked error, shipment stays active). *(B85DAEE8 proof)*
  2. Real repo (unborn HEAD) + `enabled:true` → `shipment ship` refuses with the
     distinct shipment-head message. *(1AEA2B0E proof)*
  3. No-repo workspace + `enabled:true` → `shipment ship` still succeeds. *(no
     regression)*
  4. Real repo + `enabled:true` + all member heads ancestors of shipment head →
     `shipment ship` succeeds. *(084 behavior intact)*
  5. `enabled:auto` non-autoharness → skip preserved.
- **Operational closure:**
  - **Owner:** Ship agent for the 08x shipment-gate stream.
  - **Monitoring signal (made real by ST2/ST3):** both empty-head fail-closed branches
    emit an `EventGateBlocked` evidence event carrying `"reason":"empty-shipment-head"` /
    `"reason":"empty-member-head"` **and** a `slog.WarnContext` line, so a spike in
    shipment-gate `blocked` evidence events or these warning logs is an observable
    over-refusal signal. (Before this revision the refusal branches were silent, so the
    signal would not have fired — Constitution Principle V.)
  - **Rollback trigger:** any legitimate `enabled:true` real-repo shipment refused with
    an empty-head message that is not a genuine unverifiable-lineage case.
  - **Rollback procedure:** revert the ST1–ST3 commit(s); no data/state migration.
  - **Validation window:** first two `enabled:true` real-repo shipments after merge.
  - **Knowledge graduation (post-merge, Ship):** update
    `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` scope note
    (lines 56–59) to record that the empty-member-head and empty-shipment-head seams
    are now closed; add a compound learning for the repo-presence discriminator if the
    review surfaces a reusable pattern.

## Plan Hardening

**Hardening required: YES.** This is a fail-open → fail-closed security change to the
enforced shipment-completion gate that spawns a `git` subprocess on the lock-holding
ship path. Hardening focus (per operator): **git-exec edge cases** and
**enforcement-mode detection**.

### Learnings and instructions consulted

- `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — the new probe MUST
  derive its deadline from `ws.boundedHelperTimeout()` (min(configured, 5s
  `ancestryCheckTimeout`)), never a raw `GateBroker.TimeoutSeconds` (600s default), or
  a hung `git` pins the workspace lock (DoS on the unbounded ship path).
- `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md` — the fail-closed
  git exit-code discipline and the Windows context-kill gotcha (check `runCtx.Err()`
  before reading an ExitError code).
- `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — bound the first
  lock-holding external call.
- `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` — exec
  trust boundary: argv-array, no shell, `gate.MinimalEnv()`, `cmd.Dir = ws.RootPath`.
- `.github/instructions/constitution.instructions.md` — Principle I (Safety-First Go),
  III (Workspace Isolation / exec trust boundary), V (Structured Observability), VIII
  (Explicit Safety Modes).

### Protected invariants

- **INV-1** — `TestShipmentGate_AllMembersHaveEvidence_Ships` and every existing
  no-repo gate test stay green (the `!inRepo` skip branch preserves them).
- **INV-2** — 084-F non-empty behavior (equality fast-path, `--is-ancestor` accept,
  divergent/malformed/absent reject, bounded-read timeout fail-closed, head-drift
  guard) is unchanged.
- **INV-3** — the repo-presence probe runs ONLY on the enforced path (after the
  `!ev.Enforced` early return), so a non-enforcing environment is never blocked by a
  probe error and pays zero added subprocess cost.
- **INV-4** — no exported signature, MCP tool, CLI flag, or on-disk schema change.
- **INV-5** — the probe is bounded by `ancestryCheckTimeout`; no lock-holding DoS.

### Git-exec edge-case matrix (finalizes the ST1 exit-code map)

Probe: `git rev-parse --is-inside-work-tree`, argv-array, `cmd.Dir = ws.RootPath`,
`cmd.Env = gate.MinimalEnv()`, stdout captured for the true/false parse, stderr
captured for diagnostics (mirror `isAncestor`). Deadline = `boundedHelperTimeout()`.
**Check `runCtx.Err()` first**, before any exit-code inspection.

| Environment | `--is-inside-work-tree` result | Probe returns | Gate outcome (enforced, empty shipmentHead) |
|---|---|---|---|
| Normal work tree | exit 0, stdout `true` | `(true, nil)` | **FAIL CLOSED** (real repo, HEAD unresolved) |
| Unborn branch (`git init`, no commit) | exit 0, stdout `true`; `rev-parse HEAD` → empty | `(true, nil)` | **FAIL CLOSED** — the load-bearing 1AEA2B0E fixture |
| Bare repo | exit 0, stdout `false` | `(false, nil)` | skip (not a work tree; ship-in-bare is not a real scenario) |
| Inside the `.git` dir | exit 0, stdout `false` | `(false, nil)` | skip |
| Not a repo | exit 128, stderr matches `/not a git repository/`, empty stdout | `(false, nil)` | **skip (legacy preserved)** — no-repo tests + non-autoharness |
| Corrupt/broken repo | exit 128, stderr does NOT match not-a-repository (bad HEAD, unreadable objects, perms) | `(false, err)` | **FAIL CLOSED** — a present-but-broken repo is not a no-repo skip |
| Context timeout / cancel | non-zero, `runCtx.Err() != nil` | `(false, ctxErr)` | **FAIL CLOSED** (Windows: killed git may exit 1 — must not misread) |
| `git` missing / non-ExitError | exec error | `(false, err)` | **FAIL CLOSED** |
| Other non-{0,128} exit | ExitError, other code | `(false, err)` | **FAIL CLOSED** |

Parse rule (no `strings` import — `shipment_gate.go` already imports `bytes`):
`inRepo := runErr == nil && bytes.Equal(bytes.TrimSpace(stdout.Bytes()), []byte("true"))`.
Any exit 0 with stdout ≠ `true` → `(false, nil)` skip. For non-nil `runErr`: check
`runCtx.Err()` first → fail closed; else if it is an `*exec.ExitError` with code 128
**and** `bytes.Contains(bytes.ToLower(stderr.Bytes()), []byte("not a git repository"))`
→ `(false, nil)` skip; **any other** 128 (corrupt repo) or any other code → fail closed.
Rationale for `--is-inside-work-tree` over `--git-dir`: `--git-dir` returns exit 0 in
a bare repo and inside `.git`, so it cannot distinguish "can resolve a work-tree HEAD"
from "is under some git dir"; `--is-inside-work-tree` is the precise signal.

### Enforcement-mode detection (production vs. test-harness reachability)

- `ev.Enforced` derivation (`broker.go:60-114`): true only when the version/binary
  probe passes AND base-ref resolution succeeds. It does **not** track work-tree
  presence.
- **Production `enabled:true` + genuinely no repo** already fails closed *upstream*:
  `ResolveBaseRef` auto-discovery fails → under `EnabledTrue` the broker returns the
  error (`broker.go:107-108`) → `gateShipmentCompletion` fails closed at the
  `Evaluate` error branch (`shipment_gate.go:208-214`) **before** the empty-head code
  runs. So the empty-`shipmentHead`-under-`Enforced` state is reached in production
  only for a **real repo** whose `rev-parse HEAD` fails for a non-context reason
  (unborn branch, transient failure) — exactly the case the probe fails closed on.
- The `!inRepo` **skip** branch therefore primarily keeps the **test harness** green
  (the harness fakes `Enforced=true` in a no-repo temp dir via `fakeGitAllOK`), and
  the same skip covers the residual non-autoharness edge. This is a critical point for
  the plan-review Security lens: the change **does not weaken production** — production
  strict+no-repo already fails closed at `Evaluate`; the probe *adds* fail-closed for
  strict+real-repo+empty-head and *preserves* the no-repo skip that the suite and
  non-autoharness environments depend on.
- The probe call site MUST remain after the `!ev.Enforced` early return
  (`shipment_gate.go:215-219`) and after the `headErr` fail-closed check (223-225), so
  neither a non-enforcing environment nor a bounded-read timeout is re-routed through
  the new branch.

### Risky actions (strict-safety classification)

- **ProposedAction PA-1** — spawn `git rev-parse --is-inside-work-tree` on the
  lock-holding ship path.
  - **ActionRisk:** MEDIUM (external process under the workspace lock; read-only).
  - **Guard:** `boundedHelperTimeout()` hard cap; `MinimalEnv()`; argv-array (no
    shell); fail-closed on error/timeout.
  - **ActionResult (expected):** near-instant `(bool, nil)` in the common case; a
    bounded error only on a hung/absent git.
  - **Approval:** none required (bounded, read-only, no state change).
- **ProposedAction PA-2** — change the enforced shipment gate from fail-open to
  fail-closed on empty shipment/member heads.
  - **ActionRisk:** HIGH blast-radius (behavioral): a boundary error could block
    legitimate `ShipShipment` calls under `enabled:true`.
  - **Guard:** repo-presence discriminator + INV-1/INV-2 regression tests + the
    validation window and rollback trigger below.
  - **ActionResult (expected):** legitimate real-repo shipments with resolvable heads
    ship unchanged; only unverifiable-lineage ships are refused.
  - **Approval:** satisfied by this planning gate + plan-review; no runtime destructive
    action; rollback = code revert (no data/state migration).

### Reinforced rollback / monitoring / validation

- **Rollback trigger:** any legitimate `enabled:true` real-repo shipment refused with
  an empty-head message that is not a genuine unverifiable-lineage case.
- **Rollback procedure:** revert the ST1–ST3 commit(s). No state or schema migration;
  no evidence-format change (refusals reuse existing `EventGateBlocked` evidence).
- **Monitoring signal:** a spike in shipment-gate `blocked` evidence carrying the
  empty-head refusal messages indicates over-refusal.
- **Validation window:** the first two `enabled:true` real-repo shipments after merge
  (Ship owns runtime verification per the section above).
- **Operator checkpoint:** none blocking — operator granted full downstream autonomy.
  Accurate escape valve for a strict-mode refusal: fix the underlying git state so HEAD
  resolves (for empty-shipment-head) or lower `pre_task_completion_gate.enabled` to
  `auto`/`false` (for empty-member-head). Note: there is **no** shipment-level force
  override and a terminal member will **not** re-run the per-task gate (`gateApplies`
  short-circuits terminal), so "re-run the gate" is not a recovery for terminal members
  (see Decision 6).

### Broker-contract note (surface tension, resolved)

`.github/instructions/autoharness-gate-broker-integration-contract.md` advises core code
to route gate concerns through the `GateBroker` rather than reaching for `exec` directly.
This plan's direct `git rev-parse` in `shipment_gate.go` is **consistent with the 084-S
precedent already merged in this same file** (`isAncestor`, `headSHABounded` all invoke
git directly under the bounded-helper trust boundary): these are *local lineage-verification
helpers on the ship path*, not gate *evaluation* (which remains the broker's job via
`Evaluate`). The broker contract governs gate decisioning and environment probing for the
per-task gate; the shipment-completion lineage checks are a distinct, already-established
direct-exec surface. No broker API is bypassed — `ev.Enforced`/base-ref still come from
the broker; only the work-tree presence probe is local, mirroring the existing helpers.
**Contract-gap acknowledgment (Architecture review):** an alternative is to expose a
`WorktreePresent` signal on `gate.Evaluation` (derivable inside `Broker.Evaluate` from the
`ResolveBaseRef` outcome), eliminating the extra subprocess and keeping environment probing
inside the broker boundary. That is deliberately **deferred as out of scope** here: widening
the broker↔core `Evaluation` contract inside a fail-open→fail-closed *security* fix would
enlarge the blast radius and couple two changes that should land independently. This plan
takes the lower-risk, precedent-consistent local-probe path (084-S) and records the
`Evaluation.WorktreePresent` option as a future consolidation (tracked with R6's follow-up
family) so the next helper author does not re-derive the signal a third time.

### Unresolved decisions carried to plan-review — RESOLVED

- Probe command: **`git rev-parse --is-inside-work-tree`** confirmed (precise work-tree
  signal; bare-repo/`.git`-dir → skip). Corrupt-repo exit 128 disambiguated by stderr
  (fail closed unless stderr matches "not a git repository").
- Empty-shipment-head-in-repo refusal: **dedicated `shipmentHeadUnresolvedInRepoError`
  constructor** with a distinct message, not `headResolveError` reuse (Decision 3).

## Constitution Check

Mapping of the change to `.github/instructions/constitution.instructions.md`. This section
is mandatory (Governance) and records one justified deviation.

| Principle | Compliance |
|---|---|
| **I — Safety-First Go** | ST1 probe uses argv-array `exec.CommandContext` (no shell), captures stdout+stderr into `bytes.Buffer`s, checks `runCtx.Err()` before exit codes, and returns typed errors. No `panic`, no unchecked type assertions on the git path. Probe deadline = `ws.boundedHelperTimeout()` (≤ `ancestryCheckTimeout` 5s) — no unbounded external call on the lock-holding path. |
| **II — Test-First Development (NON-NEGOTIABLE)** | Every subtask lands its red test first; ST1 helper ships as a stub so its test is observed failing before the body exists; R7 flip is an explicit red→green transition; T1 acceptance runs the full `go test ./...`. |
| **III — Workspace Isolation and Security Boundaries** | Probe runs with `cmd.Dir = ws.RootPath` and `cmd.Env = gate.MinimalEnv()`; read-only; no writes outside the workspace. |
| **IV — CLI Workspace Containment (NON-NEGOTIABLE)** | No new CLI command/flag; the git subprocess is contained to the workspace via `cmd.Dir = ws.RootPath` + `MinimalEnv()` (no ambient env, no path escape). INV-4: no exported signature, MCP tool, CLI flag, or on-disk schema change. |
| **V — Structured Observability** | Both new fail-closed branches (ST2 shipment-level, ST3 member-level) emit an `EventGateBlocked` evidence event (with a `reason` field) **and** a `slog.WarnContext`, so the plan's own monitoring signal is real rather than a silent refusal. |
| **VI — Single Responsibility** | Two named holes only; the upstream provenance root-cause is explicitly deferred (R6) rather than pulled in. One new constructor, no new error types, no import growth (`bytes` reused, `strings` avoided). |
| **VII — Destructive Command Approval (NON-NEGOTIABLE)** | No destructive command introduced; the probe is read-only. Rollback = pure code revert of ST1–ST3 with no data/state migration. PA-1/PA-2 classified in `## Plan Hardening`. |
| **VIII — Explicit Safety Modes for Elevated Risk** | Fail-closed applies **only** under `ev.Enforced` (strict `enabled:true`); `auto`/`false` and non-enforcing environments keep the legacy skip — the strict-vs-permissive boundary is explicit and test-pinned (INV-1/INV-3). |
| **IX — Git-Friendly Persistence** | N/A — no new persisted state format; refusals reuse the existing `EventGateBlocked` evidence record. The plan/decision/memory artifacts are line-oriented Markdown + YAML frontmatter. |
| **X — Agent Context Efficiency** | N/A — no change to context-window data access; the change is a localized gate behavior fix. |
| **XI — Merge Commit History Preservation (NON-NEGOTIABLE)** | N/A — no merge-strategy or history-rewrite behavior touched; Ship owns the eventual merge and is bound by this principle independently. |

**Justified deviation — the `!inRepo` skip preserves a fail-open path.** Refusing *all*
empty heads (including no-repo) would satisfy a naive "fail closed everywhere" reading but
would break the no-repo test suite and legitimately-non-git / non-autoharness environments,
and provides **no production security benefit**: production strict + genuinely-no-repo
already fails closed *upstream* at `ResolveBaseRef`/`Evaluate` (`broker.go:107-108` →
`shipment_gate.go:208-214`) before the empty-head code runs. The `!inRepo` skip therefore
only preserves the test harness + non-autoharness edge, while the `inRepo` branch closes
the real production residual (strict + real-repo + empty head). The deviation is scoped,
test-pinned (INV-1), and non-weakening — documented here per Governance.

## Plan Review

<!-- plan-review-attempt: 1 -->

### Attempt 1 — verdict: RE-ENTRY (material P2s; no P0/P1)

Six-persona parallel review. Positive: Scope Auditor = TIGHT (no findings); Learnings =
WELL-ALIGNED (all four required 084 patterns reused); Go Reviewer = design sound;
Constitution = substantively sound. Architecture Strategist produced no response (model
non-response, not a substantive finding) — re-run in attempt 2 with a reliable model.

Material findings and their resolutions (all folded into this revision):

| ID | Finding | Resolution |
|---|---|---|
| P2-C1 | Empty-head refusal branches were silent → the plan's own monitoring signal could not fire (Principle V). | ST2/ST3 now emit `EventGateBlocked` (with a `reason` field) **and** `slog.WarnContext`; Runtime closure monitoring signal rewritten to depend on them. |
| P2-C2 | Missing mandatory `## Constitution Check` (Governance). | Added `## Constitution Check` with all eight principles mapped + the `!inRepo` justified deviation. |
| P2-G1 | Exit 128 is git's generic fatal code; all-128→skip left a corrupt-repo fail-open residual. | Exit 128 now disambiguated by stderr (`/not a git repository/` → skip; any other 128 → fail closed). Matrix + ST1 map updated. |
| P2-G2 | `validateMemberGateEvidence` empty-member fail-closed relies on a caller-invisible invariant. | ST3 doc-comment now states the *accurate* invariant explicitly (non-empty `shipmentHead` ⟹ `headSHABounded` resolved a real HEAD ⟹ real worktree; `inGitWorktreeBounded` is the discriminator on the *empty* branch, not a precondition of this function — corrected in attempt 2 per Architecture review). |
| P2-L1 | Broker-contract surface tension (direct exec in core). | Added `### Broker-contract note` distinguishing local lineage helpers (084-S precedent) from broker gate evaluation. |
| P2-S1 | "Empty member head only from no-repo" rationale was incomplete (can be a transient real-repo HEAD-read failure). | Decision 2 rewritten with corrected provenance; fail-closed shown correct regardless of cause; upstream root-cause deferred as R6. |
| P2-S2 | Escape-valve claim inaccurate — no shipment-level force override; terminal members don't re-run the gate. | Decision 6 + R4 + Operator-checkpoint rewritten with the accurate recovery (fix git state / lower enforcement). |

Valuable P3s folded: dedicated `shipmentHeadUnresolvedInRepoError` constructor;
`bytes.Equal`/`bytes.Buffer` capture instead of a new `strings` import; ST1 helper landed
as a stub for a true red phase; quality-gate quartet (gofmt, go vet, golangci-lint, full
`go test ./...`) added to T1 acceptance; hung-git bounded-return covered by the reused
`boundedHelperTimeout` cap.

<!-- plan-review-attempt: 2 -->

### Attempt 2 — verdict: PASS (advisories resolved in-place)

Four-persona re-review of the attempt-1-revised plan on reliable models (Security lens +
Go on `gpt-5.3-codex`; Constitution + Architecture on `claude-sonnet-4.6`, replacing the
attempt-1 non-responding model). No P0/P1 at any point.

| Persona | Verdict | Outcome |
|---|---|---|
| **Security lens** | **PASS** | Boundary **explicitly resolved**: the repo-presence discriminator is sound given `ev.Enforced` does not track repo presence; P2-S1/P2-S2 + exit-128 residual confirmed closed; no remaining enforced-real-worktree fail-open residual. Zero findings. |
| **Go** | **PASS** | P2-G1 (stderr-disambiguated exit 128, no `strings` import) and P2-G2 (doc-comment precondition) confirmed fixed; exec/error pattern Go-correct; stub-first red phase valid. Zero findings. |
| **Constitution** | ADVISORY → resolved | P2-C1 and P2-C2 confirmed resolved for ST2; flagged one residual **P2 (F-1)**: ST3's Changes bullet + test omitted the `EventGateBlocked` emission that the closure/Constitution-Check claimed (internal inconsistency). Plus P3 advisories: Principle IV mislabeled; principles IX/X/XI unmapped. |
| **Architecture** | ADVISORY → resolved | Sound coupling/decomposition; no cycles/layering violation. Flagged one residual **P2**: the ST3 doc-comment invariant *mis-attributed* the non-empty-`shipmentHead` guarantee to `inGitWorktreeBounded` (which only runs on the empty branch) — the real guarantor is `headSHABounded`. Plus P3s: imprecise ST2→ST3 dependency rationale; git-helper file-accumulation advisory; suggested (deferred) `Evaluation.WorktreePresent` consolidation. |

**Attempt-2 residual resolutions (applied in-place; no code, plan-only):**

| ID | Finding | Resolution |
|---|---|---|
| C-F1 (P2) | ST3 evidence-event emission absent from Changes + test; monitoring signal for `empty-member-head` ungrounded. | ST3 Changes now mandates appending `EventGateBlocked` (`reason:"empty-member-head"`, `level:"member"`); ST3 test (a) now asserts the event was recorded. |
| A-P2 (P2) | ST3 doc-comment invariant causally inaccurate (credited `inGitWorktreeBounded` for the non-empty path). | Doc-comment reworded to the accurate invariant (non-empty `shipmentHead` ⟹ `headSHABounded` resolved a real HEAD; probe is the *empty*-branch discriminator, not a precondition). |
| A-P3 | ST2→ST3 dependency rationale imprecise (ST2 does not establish the non-empty invariant). | Dependency Graph + ST3 "Depends on" reworded: edge is edit-ordering + pattern-reuse; the invariant is pre-existing via `headSHABounded`. |
| C-P3 | Constitution Check: Principle IV mislabeled; IX/X/XI unmapped. | Table corrected to authoritative principle names (verified against `constitution.instructions.md`) and extended with IX/X/XI (N/A rows). |
| A-P2b (defer) | Consider `Evaluation.WorktreePresent` to avoid the extra subprocess. | Acknowledged in the Broker-contract note as an explicitly **out-of-scope** future consolidation (security fix kept narrow); tracked with the R6 follow-up family. |

**Gate outcome: PASS.** Both blocking requirements met — the Security lens PASSED with an
explicit boundary-resolution verdict, and the mandatory `## Constitution Check` is present
and complete. The two attempt-2 P2 advisories were internal-consistency/accuracy defects
(not new security holes), and both were corrected in-place. The review trail is convergent
(attempt 1: 7 P2s → all fixed; attempt 2: 2 P2s → both fixed; no P0/P1 at any point). No
third cycle required; the 3-attempt convergent-cap was not reached. Proceeding to harvest.
