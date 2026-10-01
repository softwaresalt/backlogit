# Stage memory: 184-S / 185-S shipment decomposition

* Date: 2026-09-30 (local), timestamps in UTC 2026-10-01
* Agent: Stage, invoked by the Orchestrator for a user-authorized staging
  correction. Route: claude-opus-5.5 / anthropic / high.
* Branch: `stage/condition-b-enforcement-staging`, single worktree. No
  checkout, no second branch, no PR, no Ship work.
* Request: "Between 184-S and 185-S, the two combined have far too many
  tasks assigned per shipment. These need to be further decomposed based on
  size and complexity."
* Result: COMPLETE. The 18-task and 26-task manifests became 10 release
  units of 1 to 8 tasks each.

## Decisions

* Each release unit has its own root feature, because a shipment releases
  only its explicit manifest and lists its covering feature first. A broad
  root feature in an early shipment would mix scopes.
* `184-S` and `184-F` keep the A4 closing chunk. `185-S` and `185-F` keep
  the B6 closing chunk. Nothing was retired, abandoned, or archived.
* The other 38 tasks were moved with `backlogit_adopt_item`, which keeps
  `origin_feature`. No task was cloned or split.
* B3 (7 tasks) and B5 (8 tasks) are above the 4-to-6 target. They are
  inseparable green boundaries; the plan gives the rationale.
* Packaging refinements PR-1 to PR-5 changed only task bodies, titles, task
  edges, and closure text. They added no new requirements.

## Review

* A new packaging-only review surface was used: shipment decomposition
  review attempts 1 to 3. The original plan-review attempts 1 to 4 are
  unchanged.
* Attempt 1: FAIL (7 P1). Attempt 2: FAIL (1 P1, the R12 pin timing).
* Attempt 3: ADVISORY (P0 0, P1 0, P2 5, P3 15), seven personas.
  * `operator_authorization: approved` was recorded as delegated through
    the user's end-to-end correction request, following the attempt-4
    precedent. The operator may object.
  * Mandatory conditions MDC-1 to MDC-5 were applied.

## Artifacts

* Plan addendum, review records, and Applied Mapping:
  `docs/exec-plans/2026-09-30-6434a4d7-shipment-predecessor-readiness-guard-plan.md`.
* Checkpoint commit `8725f50b` (addendum and reviews only).
* New features `187-F` to `194-F` and new shipments `187-S` to `194-S`.

| Shipment | Unit | Tasks | Effort (h) | Prerequisites |
|---|---|---|---|---|
| `187-S` | A1 | 4 | 2.5 to 4.25 | `154-S` |
| `188-S` | A2 | 6 | 5.5 to 8.75 | `154-S`, `187-S` |
| `189-S` | A3 | 5 | 4 to 7 | `154-S`, `187-S`, `188-S` |
| `184-S` | A4 | 3 | 2 to 3 | `154-S`, `188-S`, `189-S` |
| `190-S` | B1 | 5 | 4.25 to 7.5 | `154-S`, `188-S`, `189-S` |
| `191-S` | B2 | 2 | 3 to 4 | `154-S`, `190-S` |
| `192-S` | B3 | 7 | 7 to 10.75 | `154-S`, `187-S`, `191-S` |
| `193-S` | B4 | 1 | 0.25 to 0.5 | `154-S`, `186-S` |
| `194-S` | B5 | 8 | 6 to 8.75 | `154-S`, `186-S`, `189-S`, `192-S`, `193-S` |
| `185-S` | B6 | 3 | 2.25 to 3.25 | `154-S`, `184-S`, `186-S`, `194-S` |

## Execution notes

* One MCP `backlogit_get_shipment` read after the manifest sync was stale.
  The second MCP sync was correct before any adoption.
* The `184-S` to `189-S` edge call timed out. A read-back showed it was
  absent, and it was added once.
* The first `188-S` hold comment paraphrased R14 wrongly. A second comment
  supersedes that sentence.
* The trailing blank lines at the end of the `184-S` and `185-S` manifests
  were trimmed for `git diff --check`.

## Verification

* 44 tasks, each in exactly one manifest. Feature first, parents correct,
  all `queued`, no status changes.
* Shipment edges exact. Kahn order: `187-S`, `193-S`, `188-S`, `189-S`,
  `184-S`, `190-S`, `191-S`, `192-S`, `194-S`, `185-S`. No cycle.
* There are no cross-shipment task edges outside the prerequisite closure,
  no missing targets, and no old-ID references.
* The 17 guarded tasks keep their label and their guard text hashes. The
  `183` and `186` files are unchanged.
* `backlogit docs lint`: valid, 0 violations. `backlogit_doctor`: only 23
  pre-existing orphan findings (`016.001-R`, `106.012-T` to `106.033-T`).
* Not run: builds, tests, P-008 gates (Ship owns these), and the full
  autoharness verifier.

## Holds and follow-ups

* Condition B scheduler-consumption attestation C for `154-S` is absent, so
  no unit is Ship-eligible.
* Follow-ups (not acted on):
  * The plugin bundle `plugin/agents/ship.agent.md` grants `backlogit/*`
    with no disposition ban.
  * There is no backlog-integration instructions row for dispose.
  * The adopt instruction header is stale.
  * `docs/memory` holds 103 files (402 KB), above the compact-context
    trigger. Compacting it was out of scope.

## Next step

No Stage work remains for this correction. Ship must not claim any unit
until attestation C exists. The order is the Kahn order above.
