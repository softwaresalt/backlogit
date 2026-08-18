---
title: "Adversarial multi-model review: shipment shipped-event audit-log durability plan"
description: "Consensus-weighted findings from three independent reviewers on three model families against the third revision of the shipped-event audit-log plan, with remediation queue and rejected-finding rationale"
source: "docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md"
doc_type: review
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

* Artifact reviewed: `docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md`,
  third revision, 11 units
* Baseline: clean worktree at `origin/main` `3ec95ee3e7c6787762beb15c0e4b226746e50a89`
* Escalation trigger: the standard plan-review gate surfaced more than three P0/P1 findings in each
  of two prior cycles, which meets the adversarial-review escalation threshold
* Dispatch mode: `multi-model-adversarial`, three parallel reviewers, no shared context

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
| M2 | Unit 2 scenario 1 (success ordering) already passes on the Unit 1 tree, so the plan's claim that all three scenarios fail is inaccurate | B (P1), A (P3) | P2 | **FIXED.** Unit 2's verification now states that scenarios 2 and 3 must fail and that scenario 1 characterizes existing correct behavior to lock the ordering contract against regression. |

### LOW confidence (single reviewer)

| ID | Finding | Reviewer | Severity | Disposition |
|---|---|---|---|---|
| L1 | Unit 4 could not capture the append boundary within its declared files: the wrap must happen at `internal/core/shipment.go:205`, which Unit 4 does not touch | B | P1 | **FIXED.** The `shipmentEventAppendError` type is declared in Unit 1.3 and wrapped in Unit 3.2; Unit 4.1 only classifies it. |
| L2 | The "bounded at 3 seconds" claim is false for in-process contention: `internal/events/stream.go:117-125` performs an uncancellable `mutex.Lock()` before the bounded file-lock deadline at `:172-177` | B | P1 | **FIXED.** The Risks table now states the bound honestly: bounded for cross-process contention, serialized for in-process contention. |
| L3 | The parallel dependency graph permits unverifiable task states, because Units 1 and 5 required `go test ./internal/core/...` green while Units 2 and 6 deliberately leave that package red | B | P1 | **FIXED.** Units 1 and 5 now verify with named `-run` selectors and state explicitly that the sibling track's harness may legitimately be red at the same time. |
| L4 | Untagged classification as `not-applied` is unsafe for a short or partial write, because `appendFast` uses `fmt.Fprintf`, which can return an untagged error after bytes reached the file | C | P0 | **FIXED.** Unit 1.2 now requires a short or partial write to be tagged `blerrors.ErrWriteIndeterminate`, and a Plan Hardening guardrail forbids compensating over a partially written append-only log. |
| L5 | Unit 7's preferred `refs`-direct approach cannot read `archived_status`, so the audit would false-positive on shipments archived from other statuses; widening is mandatory, not conditional | A | P2 | **FIXED.** Unit 7 now states the widening is mandatory and names both `artifactRef` and the doctor-local struct; Unit 6 scenario 1 gains a negative subtest for an archived non-shipped shipment. |
| L6 | The indeterminate branch halts the whole archive set, but the finding and the recovery procedure covered only the shipment | A | P2 | **FIXED.** Unit 7 enumerates the stranded release-scope items in the finding detail, and the named-limitation procedure archives them too. |
| L7 | Structured partial-compensation handling covered only lock failure; `restoreShipArtifacts` can also fail at read, file restore, log restore, event replay, reindex, and upsert | B | P2 | **FIXED.** Unit 4.4 now applies the promotion to every per-item rollback stage, with the exact line anchors. |
| L8 | The claimed CLI-versus-MCP parity assertion does not exist in `internal/cli/registry_parity_test.go`, and Unit 9 did not list that file | B | P2 | **FIXED.** Unit 9 adds the file and states the fixture is new rather than an extension. |
| L9 | Unit 6 cannot verify a doctor exit code from an `internal/core` test, because `core.Doctor` returns a report and an error | B | P2 | **FIXED.** The exit-code assertion moved to Unit 8's CLI tests. |
| L10 | SLI 5 is not measurable from returned error text, because `MutationPartialError.Error()` omits `CompensationState` | B | P2 | **FIXED.** SLI 5 now measures MCP JSON and the `slog` record, and Unit 4.8 requires `compensation_state` and `unrestored_ids` in that record. |
| L11 | Crash and malformed-tail behavior was absent from the failure matrix | B | P2 | **FIXED.** Both are declared in the named-limitation section, with the note that only SLI 2 detects crash residue. |
| L12 | Unit 3 silently suppresses the `HookMoveShipmentStatus` post-hook on the fail-closed path | A | P3 | **FIXED.** Unit 3.4 records the suppression and its rationale in the doc comment. |
| L13 | Unit 11's dependency list omitted Units 8 and 9, whose surfaces its documentation references | C | P2 | **FIXED.** `143.011-T` now depends on `143.004-T`, `143.007-T`, `143.008-T`, `143.009-T`, and `143.010-T`. |

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
| 2 | L4 | LOW | P0 | manual | Fixed in Unit 1.2 and Plan Hardening |
| 3 | L1 | LOW | P1 | manual | Fixed across Units 1.3, 3.2, 4.1 |
| 4 | L3 | LOW | P1 | manual | Fixed in Units 1 and 5 verification |
| 5 | L2 | LOW | P1 | advisory | Fixed in the Risks table |
| 6 | M2 | MEDIUM | P2 | manual | Fixed in Unit 2 verification |
| 7 | L5, L6, L7 | LOW | P2 | manual | Fixed in Units 4.4, 6, 7 |
| 8 | L8, L9, L10, L11, L13 | LOW | P2 | manual | Fixed in Units 6, 8, 9, 11, SLI 5, Dependency Graph |
| 9 | L12 | LOW | P3 | advisory | Fixed in Unit 3.4 |
| 10 | R1, R2 | LOW | P0 / P1 claimed | rejected | Rationale recorded above |

No entry requires a new backlog item. Every accepted finding was remediated inside the existing
eleven-unit decomposition without adding a unit.

## Gate outcome

* HIGH-confidence P0/P1 findings: **0**
* MEDIUM-confidence findings: **2**, both fixed
* LOW-confidence findings: **13**, of which 11 fixed and 2 rejected with recorded rationale
* Adversarial gate: **PASS**

The plan may proceed to harvest.
