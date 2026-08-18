---
title: "Adversarial multi-model review: shipment shipped-event audit-log durability plan"
description: "Consensus-weighted findings from two adversarial panels, each with three independent reviewers on three model families, against the third and fourth revisions of the shipped-event audit-log plan, with remediation queue and rejected-finding rationale"
source: "docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md"
doc_type: review
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

* Artifact reviewed: `docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md`
* Panel 1 reviewed the third revision (11 units); panel 2 reviewed the fourth revision (11 units,
  post-Copilot-review rewrite) and its findings produced revision 5 (12 units)
* Baseline: clean worktree at `origin/main` `3ec95ee3e7c6787762beb15c0e4b226746e50a89`
* Escalation trigger: the standard plan-review gate surfaced more than three P0/P1 findings in each
  of two prior cycles, which meets the adversarial-review escalation threshold
* Dispatch mode: `multi-model-adversarial`, three parallel reviewers per panel, no shared context

## Reviewer panel

| Reviewer | Model family | Emphasis assigned | Raw verdict |
|---|---|---|---|
| A | Anthropic (Tier 3) | Correctness of the rollback design and the defer swap; whether the RED harnesses can genuinely fail then pass; unimplementable units; enforceability of the report-only contract; Baseline Verification accuracy | PASS - P0=0 P1=0 P2=3 P3=2 |
| B | OpenAI (Tier 3) | Testability and executability; error-model coherence with `MutationEnvelope` and `MutationPartialError`; observability measurability; unenumerated failure modes; broken intermediate builds | FAIL - P0=0 P1=5 P2=5 P3=0 |
| C | Google (Tier 3) | Risk and operability; whether the fix is worse than the bug; scope proportionality; internal consistency; honesty of the Baseline Verification table | FAIL - P0=2 P1=1 P2=1 P3=0 |

All three independently spot-checked the Baseline Verification table. Reviewer C verified eight
citations and reported all eight accurate. Reviewer A confirmed the high-risk citations
(`shipment.go`, `shipment_lifecycle.go`, `mutation_envelope.go`, `gate_evidence.go`) accurate.
Reviewer B found no nonexistent symbol. One minor drift was reported (`models.Artifact.ArchivedStatus`
cited at `:70`, actually `:68`) and is within the plan's stated tolerance.

## Consensus assembly

Confidence is assigned by how many independent reviewers surfaced the same substantive finding, per
`.github/instructions/adversarial-review.instructions.md`.

### HIGH confidence (all three reviewers)

**None.** No finding was surfaced by all three reviewers. The adversarial gate is therefore not
blocked: HIGH-confidence P0/P1 findings are the gate-blocking class.

### MEDIUM confidence (two of three reviewers)

| ID | Finding | Reviewers | Severity | Disposition |
|---|---|---|---|---|
| M1 | `internal/mcp/errors.go:155` sets `Retryable: err.Class == "not-applied"`, so the new `not-applied` plus `partially-compensated` result would advertise itself as safe to retry while release-scope items remain un-restored | B (P1), A (P2) | P1 | **FIXED.** Unit 10 now gates `Retryable` on class **and** compensation state, with a contract assertion. A Plan Hardening guardrail forbids a partially-compensated result ever being reported retryable. |
| M2 | The success-ordering scenario already passes on the tree its harness lands on, so the plan's claim that all three scenarios fail is inaccurate | B (P1), A (P3) | P2 | **FIXED.** The durability harness now states that only its not-applied and indeterminate scenarios must fail, and that the success-ordering scenario characterizes existing correct behavior to lock the ordering contract against regression. The same honesty correction was applied at PR review cycle 1 to the detection harness's non-mutation scenario. |

### LOW confidence (single reviewer)

| ID | Finding | Reviewer | Severity | Disposition |
|---|---|---|---|---|
| L1 | Unit 4 could not capture the append boundary within its declared files: the wrap must happen at `internal/core/shipment.go:205`, which Unit 4 does not touch | B | P1 | **FIXED.** The `shipmentEventAppendError` type is declared with the appender (now Unit 2) and wrapped at the call site (Unit 3); Unit 4 only classifies it. |
| L2 | The "bounded at 3 seconds" claim is false for in-process contention: `internal/events/stream.go:117-125` performs an uncancellable `mutex.Lock()` before the bounded file-lock deadline at `:172-177` | B | P1 | **FIXED, then corrected again at PR review cycle 1.** The Risks table stated the bound honestly, but the Baseline Verification row still asserted a flat 3-second bound. Both now distinguish the bounded cross-process sidecar lock (`:40`, `:177`, reached through `LockItemLogCrossProcess`) from the unbounded in-process `LockItemLog` `mutex.Lock()` at `:125`. |
| L3 | The parallel dependency graph permits unverifiable task states, because the first unit on each track required `go test ./internal/core/...` green while the harness units deliberately leave that package red | B | P1 | **FIXED.** Every unit on both core tracks now verifies with named `-run` selectors and states explicitly that the sibling track's harness may legitimately be red at the same time. |
| L4 | Untagged classification as `not-applied` is unsafe for a short or partial write, because `appendFast` uses `fmt.Fprintf`, which can return an untagged error after bytes reached the file | C | P0 | **FIXED, then strengthened at PR review cycle 1.** The first fix required tagging a short or partial write `ErrWriteIndeterminate`, which is not implementable: `AppendEvent` returns only `error` and `appendFast` discards the byte count (`internal/events/stream.go:243`, `:290-291`). The plan now classifies conservatively instead - only a proven pre-write failure compensates, and every other append error, untagged included, is `indeterminate`. No writer API change. |
| L5 | Unit 7's preferred `refs`-direct approach cannot read `archived_status`, so the audit would false-positive on shipments archived from other statuses; widening is mandatory, not conditional | A | P2 | **FIXED.** The `missing_shipped_event` unit now states the widening is mandatory and names both `artifactRef` and the doctor-local struct; the detection harness gains a negative subtest for an archived non-shipped shipment. |
| L6 | The indeterminate branch halts the whole archive set, but the finding and the recovery procedure covered only the shipment | A | P2 | **FIXED.** The `shipped_unarchived_residue` unit enumerates the stranded release-scope items in the finding detail, and the named-limitation procedure archives them too. |
| L7 | Structured partial-compensation handling covered only lock failure; `restoreShipArtifacts` can also fail at read, file restore, log restore, event replay, reindex, and upsert | B | P2 | **FIXED.** Unit 4.4 now applies the promotion to every per-item rollback stage, with the exact line anchors. |
| L8 | The claimed CLI-versus-MCP parity assertion does not exist in `internal/cli/registry_parity_test.go`, and the MCP unit did not list that file | B | P2 | **FIXED.** Unit 9 adds the file and states the fixture is new rather than an extension. |
| L9 | The detection harness cannot verify a doctor exit code from an `internal/core` test, because `core.Doctor` returns a report and an error | B | P2 | **FIXED.** The exit-code assertion moved to Unit 8's CLI tests. |
| L10 | SLI 5 is not measurable from returned error text, because `MutationPartialError.Error()` omits `CompensationState` | B | P2 | **FIXED.** SLI 5 now measures MCP JSON and the `slog` record, and Unit 4.8 requires `compensation_state` and `unrestored_ids` in that record. |
| L11 | Crash and malformed-tail behavior was absent from the failure matrix | B | P2 | **FIXED.** Both are declared in the named-limitation section, with the note that only SLI 2 detects crash residue. |
| L12 | Unit 3 silently suppresses the `HookMoveShipmentStatus` post-hook on the fail-closed path | A | P3 | **FIXED.** Unit 3.4 records the suppression and its rationale in the doc comment. |
| L13 | Unit 11's dependency list omitted the CLI and MCP units, whose surfaces its documentation references | C | P2 | **FIXED.** `143.011-T` now depends on `143.004-T`, `143.007-T`, `143.008-T`, `143.009-T`, and `143.010-T`. |

### LOW confidence, REJECTED with rationale

| ID | Finding | Reviewer | Severity claimed | Rejection rationale |
|---|---|---|---|---|
| R1 | "Do not block the transition or archival on an audit-log append failure. Keep the append best-effort and emit a high-visibility warning instead." | C | P0 | This is the status quo the bug report is about, and it directly contradicts the originating stash requirement, which states verbatim: "integrate the shipped-event append into the shipment mutation/rollback envelope using an error-returning writer; **archival must not continue without that durable event**". Deliberation `059-DL` evaluated exactly this as the do-nothing baseline and chose Option B. Reviewers A and B both examined the fail-closed design and neither judged it unsafe; A specifically confirmed "no unrecoverable state beyond the explicitly-declared named limitation". Accepting R1 would silently reverse an operator-stated requirement under the guise of a review finding. The operability concern it raises is legitimate and is answered by the declared named limitation, the documented manual recovery procedure, SLI 2, SLI 3, SLI 5, and the rollback triggers. |
| R2 | "Cut Units 4 through 11; implement only Units 1-3 with logging." | C | P1 | Contradicted by both other panels. The Architecture Strategist judged the decomposition "not over-engineered relative to the bug", with Units 1-4 the minimum honest fix and Unit 11 required rather than optional. The Scope Boundary Auditor independently returned KEEP for every one of Units 8-11 with per-unit reasoning. Cutting Unit 4 in particular would leave the fail-closed transition with no rollback contract at all - strictly more dangerous than either the bug or the full plan. The genuine scope reductions the panel did identify (the transient `cli_only_flags` entry, the pre-existing registry drift, the repo-wide parity invariant) were adopted and are recorded as DEFERRED in the plan's Requirements Trace. |

## Remediation queue

Ordered by confidence multiplied by severity. All entries are dispositioned; none is left open.

| Rank | ID | Confidence | Severity | Action class | Status |
|---|---|---|---|---|---|
| 1 | M1 | MEDIUM | P1 | manual | Fixed in Unit 10 |
| 2 | L4 | LOW | P0 | manual | Fixed in the classification contract, Unit 2.1, Unit 4.2, and Plan Hardening |
| 3 | L1 | LOW | P1 | manual | Fixed across Units 2.3, 3.2, 4.1 |
| 4 | L3 | LOW | P1 | manual | Fixed in the per-unit verification selectors on both core tracks |
| 5 | L2 | LOW | P1 | advisory | Fixed in the Risks table and the Baseline Verification row |
| 6 | M2 | MEDIUM | P2 | manual | Fixed in both harness verifications |
| 7 | L5, L6, L7 | LOW | P2 | manual | Fixed in Units 4.4, 5, 6, 7 |
| 8 | L8, L9, L10, L11, L13 | LOW | P2 | manual | Fixed in Units 5, 8, 9, 11, SLI 5, Dependency Graph |
| 9 | L12 | LOW | P3 | advisory | Fixed in Unit 3.4 |
| 10 | R1, R2 | LOW | P0 / P1 claimed | rejected | Rationale recorded above |

No entry requires a new backlog item. Every accepted panel-1 finding was remediated inside the
eleven-unit decomposition that existed at revision 3, without adding a unit. Panel 2 later did add
one - `143.012-T` - for a granularity breach, recorded below.

## Gate outcome (panel 1, revision 3)

* HIGH-confidence P0/P1 findings: **0**
* MEDIUM-confidence findings: **2**, both fixed
* LOW-confidence findings: **15** (L1 through L13 plus the two rejected entries R1 and R2), of
  which **13 fixed** and **2 rejected** with recorded rationale
* Adversarial gate: **PASS**

## Panel 2 (revision 4)

Panel 2 ran against the revision produced in response to the nine GitHub PR #366 Copilot comments,
in the same clean worktree, alongside five plan-review personas whose findings are consensus-weighted
together with the adversarial ones per
`.github/instructions/adversarial-review.instructions.md`.

| Reviewer | Model family | Emphasis assigned | Raw verdict |
|---|---|---|---|
| A | Anthropic (Tier 3) | Correctness and implementability: file-list containment, whether both harnesses compile and fail for the intended reason, whether the classification contract maps onto verified primitives | PASS - P0=0 P1=0 P2=0 P3=2 |
| B | OpenAI (Tier 3) | Testability, error-model coherence with `MutationEnvelope` and `MutationPartialError`, observability measurability, unenumerated failure modes | FAIL - P0=0 P1=8 P2=5 |
| C | Google (Tier 3) | Operational risk, cross-artifact consistency, honesty of the Baseline Verification table | PASS - P0=0 P1=0 |

Reviewer A independently confirmed sixteen anchors byte-exact and confirmed that the two
"unimplementable" claims driving revision 4 are correct. Reviewer C spot-checked ten baseline rows
and found none false. Reviewer B produced every blocking finding in this panel.

### HIGH confidence (all three reviewers)

**None.** The adversarial gate is therefore not blocked.

### MEDIUM confidence (two or more independent reviewers, counting the plan-review personas)

| ID | Finding | Reviewers | Severity | Disposition |
|---|---|---|---|---|
| M3 | `appendShipmentEventErr` was never required to pass its locked context to `AppendEvent`; `LockItemLog` is a non-reentrant uncancellable `mutex.Lock()` (`internal/events/stream.go:125`) and `AppendEvent` re-locks when the marker is absent (`:254-269`), so a wrong-context implementation deadlocks the ship goroutine permanently while holding the membership lock and every artifact lock | Architecture Strategist, Concurrency Reviewer | P1 | **FIXED.** Unit 2.2 requires the locked context, a Plan Hardening guardrail repeats it, and a hang rather than a failure is now a stop condition. |
| M4 | The defer-swap regression was non-discriminating: "a post-closure `archiveItems` failure still restores the covering feature" passes identically before and after the swap | Constitution Reviewer, Reviewer B | P1 | **FIXED.** Unit 12 requires an ordering-discriminating assertion, observed red before the swap. |
| M5 | The fail-closed gate sat on the shared transition function, so the exported `MoveShipmentStatus` would fail closed with no compensating half, exceeding the declared path-scoped guarantee | Scope Boundary Auditor, Architecture Strategist | P1 | **FIXED.** Unit 3.1 gates on `newStatus == ShipmentShipped && !topLevel`; the caller enumeration is now a Baseline Verification row and the exported-path tests are the boundary check. |
| M6 | Harness tasks shipped a scenario that passes, which P-002, P-004, and `harness-architect` Step 5.2 reject for the `harness-ready` label | Coupling Reviewer, Constitution Reviewer | P1 | **FIXED.** Both always-green scenarios moved into the first implementation unit of their track; every harness scenario is now red. |
| M7 | The appender's own contract - lock tagging, sentinel passthrough, untagged passthrough - had no test anywhere, because every matrix row injects through the seam and bypasses it | Constitution Reviewer, Reviewer B | P1 | **FIXED.** Unit 2 gains colocated appender tests, written first and observed failing. |

### LOW confidence (single reviewer)

| ID | Finding | Reviewer | Severity | Disposition |
|---|---|---|---|---|
| L14 | Unit 4 breached the 2-hour rule: nine numbered changes across more than five functions on the riskiest change in the plan, with the deviations list silent about it | Constitution Reviewer | P1 | **FIXED.** Split into Unit 4 (classify and halt) and Unit 12 (`143.012-T`: compensation, defer swap), with deviation 7 recording the split. |
| L15 | An early `return` from the new promotion path would leak the process-global item-log mutex, because `unlockItemLog()` is a plain statement at `internal/core/shipment_lifecycle.go:227` and `LockItemLogCrossProcess` returns a nil unlock on error | Concurrency Reviewer | P1 | **FIXED.** Unit 12.2 forbids the early return and requires a nil-guarded deferred unlock inside a per-item closure. |
| L16 | The partial-compensation scenario could not be injected through the seam, and the obvious alternative (taking the lock in the test) would hang rather than fail | Concurrency Reviewer | P1 | **FIXED.** Unit 1 now specifies arming a directory at the lock sidecar path from inside the seam callback, forbids the harness from taking the lock, and requires a watchdog. |
| L17 | `143.003-T` without its classifier compensates over an unproven append, and the rollback table prescribed exactly that revert | Architecture Strategist | P1 | **FIXED.** A stop condition forbids merging `143.003-T` without `143.012-T`, and both rollback triggers now revert `143.003-T`, `143.004-T`, and `143.012-T` together. |
| L18 | Unit 11 did not amend the contract doc's classification-precedence and failure-branch sections, which state the opposite of the new rule, nor its closed `CompensationState` enumeration | Coupling Reviewer | P1 | **FIXED.** Unit 11 now names both sections and the enumeration. |
| L19 | Classifying `IsWriteNotApplied` first would compensate an error carrying both sentinels, inverting `MutationEnvelope`'s indeterminate-dominates invariant | Reviewer B | P1 | **FIXED.** Unit 4.2 fixes the precedence and Unit 1 scenario 2 adds a both-sentinel subtest. |
| L20 | A fully compensated failure returned a bare error, so MCP rendered it as an unclassified internal error and SLI 4 had no structured surface; a joined restore failure outside `Cause` would also be dropped by `internal/mcp/errors.go:116-118` | Reviewer B | P1 | **FIXED.** Unit 4.5 returns a `MutationPartialError` on the compensated branch and Unit 4.4 puts the join inside `Cause`. |
| L21 | Recovery guidance could not distinguish compensation states, and `Retryable` was still class-only | Reviewer B | P1 | **FIXED.** Unit 10 passes `CompensationState`, differentiates the guidance, and gates `Retryable` on `not-applied` plus `compensated`. |
| L22 | The appender omitted the SQLite projection parity the existing appender has (`internal/core/shipment.go:688-700`) | Reviewer B | P2 | **FIXED.** Unit 2.4 preserves best-effort indexing. |
| L23 | The per-item rollback-stage inventory was incomplete, and unreadable event logs had no defined doctor outcome | Reviewer B | P2 | **FIXED.** Unit 12.1 enumerates the stages by construction from the loop body; Units 6 and 7 define the unreadable-log rule. |
| L24 | The recovery procedure and the residue finding enumerated release-scope items only, while `collectArchiveCandidateIDs` also archives linked deliberations | Reviewer B | P2 | **FIXED.** Unit 7 enumerates the full archive-candidate set. |
| L25 | Six source anchors had drifted: `lockArtifactMutation`, `persistArtifactWriteFn`, `lockAdoptionEventLogs`, `attachCommitToItems`, `archiveItems`, `setArtifactStatus`, and `models.Artifact.ArchivedStatus` | Coupling Reviewer, Concurrency Reviewer, Reviewer A | P2/P3 | **FIXED.** All corrected against the worktree. |
| L26 | Internal-consistency nits: deviation-list file counts, the cycle-3 record still crediting deleted scaffold units, the `.backlogit/reconcile/` rationale claiming a path the reconcile skill does write, the review report's revision label, and the decision record's "two lessons" heading over four items | Coupling Reviewer | P3 | **FIXED.** All corrected. |

### LOW confidence, ACCEPTED AS DECLARED RISK

| ID | Finding | Reviewer | Disposition |
|---|---|---|---|
| A1 | The retry budget bounds attempts, not wall-clock, because `LockItemLogCrossProcess` begins with an unbounded in-process `mutex.Lock()`; the reviewer recommended revising the `internal/events` freeze to add a context-aware primitive | Reviewer B | **ACCEPTED, NOT ADOPTED.** The freeze on `internal/events` is the constraint that keeps this change proportionate. Unit 12.3 adds a wall-clock bound to the retry loop and records honestly that a single acquisition is not bounded; widening the writer/lock contract is a named closure follow-up. |
| A2 | SLI 4 and SLI 5 will correlate, because compensation re-acquires the same item-log locks whose failure produced the not-applied class | Architecture Strategist, Concurrency Reviewer | **ACCEPTED AND DOCUMENTED.** Recorded as a Risks-table row and in the SLI 4 threshold note rather than engineered away. |

## Gate outcome (panel 2, revision 4)

* HIGH-confidence P0/P1 findings: **0**
* MEDIUM-confidence findings: **5**, all fixed in revision 5
* LOW-confidence findings: **15** - 13 fixed, 2 accepted as declared risk
* Adversarial gate: **PASS**

The plan may proceed to harvest.
