---
chunk_strategy: h1-h2-h3
description: "Implementation plan for BDA56ED8 — make the UR3 crash-child readiness handoff tolerate a partially written JSON marker (test-harness only), with a reproduce-first RED"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-28-ur3-crash-ready-handoff-flake-plan.md
title: "Implementation Plan: UR3 crash-child readiness handoff flake fix"
docline:
    stash_id: BDA56ED8
    status: reviewed
    created_at: 2026-09-29T05:40:00Z
---

## Objective

Remove the intermittent "unexpected end of JSON input" failure from
`TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage` and its sibling
`TestUR3_*` tests. The cause is the parent reading a crash-point marker
(`ReadyPath`) that the crash child has not finished writing.

The fix is test-harness only, in
`internal/core/shipment_blocked_recovery_harness_test.go`.

It is a prerequisite for `154-S` / `173-F`: every `173-F` unit whose gate runs
`./internal/core/...` depends on it.

**Sources:**

* `docs/decisions/2026-09-28-ur3-crash-ready-handoff-flake-deliberation.md`.
* Stash `BDA56ED8` (survivor) and `46A898B8` (duplicate).

## Requires plan hardening: no

Test code only. Nothing touches a production path, schema, contract,
migration or security surface. The rollback is a single revert.

## Implementation Units

* **U1: tolerant UR3 crash-ready reader (one task).**
  * Domain: tests. File: `internal/core/shipment_blocked_recovery_harness_test.go`
    only.
  * Functions:
    * `readUR3CrashReady`, a new helper;
    * `runUR3CrashSubprocess`, whose poll loop is edited;
    * one new RED test.
  * **Pinned helper signature** (review attempt 1, P2):

    ```go
    readUR3CrashReady(path string) (crash ur3CrashPoint, ready bool, decodeErr error, err error)
    ```

    | Input | Result |
    |---|---|
    | Missing file | `(zero, false, nil, nil)` |
    | Complete JSON | `(crash, true, nil, nil)` |
    | Partial or malformed JSON | `(zero, false, <json error>, nil)`. This is not ready, and the decode error is kept for diagnostics. |
    | Any other read error | `(zero, false, nil, err)`. This is fatal, as it is today. |

    The helper holds no package-global state.

    Implementation rules (attempt-2 review, P3):

    * Decode into a local variable, and return `ur3CrashPoint{}` on any
      decode error.
    * Wrap errors with context and `%w`: `fmt.Errorf("decode ur3 crash ready %s: %w", ...)`
      and `fmt.Errorf("read ur3 crash ready %s: %w", ...)`.
    * Document the contract in a doc comment.
  * Execution posture: **reproduce-first** (characterization, then RED, then
    GREEN).
    1. **Extract, keeping today's behavior.**
       * Move the existing read-and-unmarshal step out of the
         `runUR3CrashSubprocess` poll loop into `readUR3CrashReady`, with the
         pinned signature.
       * For now, keep today's behavior: a decode failure is returned in
         `err`, which is fatal.
       * Confirm the existing `^TestUR3` tests stay green after the
         extraction (review P3).
    2. **Add the RED test, `TestUR3CrashReady_PartialMarkerIsNotReady`.**
       * Write an empty (0-byte) file to a `t.TempDir()` path. This is the
         most likely state: `os.WriteFile` creates or truncates the file
         before writing. Then write a truncated JSON prefix of a valid
         `ur3CrashPoint`. For each, assert `ready == false`, `err == nil`
         and `decodeErr != nil`.
       * Complete the file. Assert `ready == true` and that the decoded crash
         point matches.
       * As an independent `t.Run` subtest, write a permanently malformed
         marker. Assert `ready == false` and `decodeErr != nil`. This covers
         the AC5 diagnostic at helper level without adding a scenario.
       * Run the test and **observe it fail** against the extracted,
         unchanged helper. That failure is the reproduction.
    3. **Fix the helper** so that decode failures go to `decodeErr` and are
       not fatal.
       * In the poll loop, remember the last `decodeErr` and keep polling
         until the existing 5s deadline.
       * On timeout, the failure message includes the last `decodeErr`.
       * The existing `stopChild` / `t.Cleanup` path still kills and reaps
         the child.
  * Out of scope:
    * The child's writers. Writing to a temp file and renaming it is an
      optional follow-up, not scheduled.
    * The UR10 and UR1b markers, which only check that the file exists.
    * Transient Windows read errors, other than a missing file. They stay
      fatal, as today (review P3, considered and not adopted).
    * Production code.
    * The sibling `failurePath` marker in the same poll loop. It is only
      used for diagnostics, so a torn read only shortens a failure message
      the parent is already emitting.
  * Acceptance criteria:
    1. **Reproduce-first (primary evidence).** Ship records the new test
       failing against the extracted, unchanged helper, and passing after the
       fix. This deterministic RED/GREEN proves the helper **classifies**
       empty or partial markers as not-ready. The poll-loop wiring is
       covered by AC5's diff-review check.
    2. **Stress/regression (secondary evidence).** This alone does not prove
       the race is closed.
       * Run `go test ./internal/core/ -run '^TestUR3' -count=25 -race -timeout 20m -v`.
       * It passes, with no JSON decode failure.
       * The `-v` output shows that
         `TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage` and
         `TestUR3CrashReady_PartialMarkerIsNotReady` ran. This is the
         non-vacuity check.
    3. `go test -race ./internal/core/...` passes.
    4. `git diff --name-only` for the task shows no code file other than
       `internal/core/shipment_blocked_recovery_harness_test.go`. Plan and
       backlog bookkeeping files are excepted.
    5. A permanently malformed marker is reported with its decode error, so
       the fix does not hide a malformed marker.
       * **Helper level:** asserted in step 2.
       * **Poll-loop level:** verified by diff review. The loop never passes
         a non-nil `decodeErr` to `require.NoError`, or treats it as fatal,
         before the deadline. It formats the last `decodeErr` into the
         timeout `FailNow` message.
       * No subprocess timeout test is added.

## Constitution Check

* **Test-first:** the reproduce-first RED comes before the fix, inside the
  same task. The task is test-only, so there is no separate production unit.
  Pass.
* **Single domain:** tests. Pass.
* **2-hour rule:** 1 file and 3 functions. There are 3 new test scenarios:
  * empty or partial, then complete;
  * permanently malformed;
  * the existing `^TestUR3` regression and stress rerun.

  Pass (limit < 4).
* **Error handling (I):** read and decode errors are wrapped with context and
  `%w`. No error is ignored. Decode errors are returned in `decodeErr` and
  surfaced at the timeout. Pass.
* **Dependencies (VI):** none added. Pass.
* **Backward compatibility:** no production change. Pass.
* **Workspace containment (P-017):** in-repo. Pass.

Constitution Check: pass

## Verification

* The ACs above.
* CI green on the shipment PR.
* **Harvest IDs.**
  * Feature `181-F`, task `181.001-T`, shipment `182-S` (single member,
    task only).
  * Consumer edges:
    * `154-S` blocks-depends on `182-S`;
    * `173.001-T`, `173.002-T`, `173.003-T`, `173.004-T` and `173.006-T`
      each blocks-depend on `181.001-T`.
  * These edges are NOT a sufficient release condition on their own. Queue
    dependency resolution stops blocking on any of the six cascade terminal
    statuses, including `abandoned` and `rejected`, and `ClaimShipment` does
    no dependency validation. The edges alone would therefore release if
    `182-S` or `181.001-T` went terminal without shipping.
  * **Shipped-provenance guard (manual policy until `AF1E5075`).** The
    consumers proceed only when both of these hold, read from the Markdown
    source (the index does not project `archived_status`):
    * `182-S` has shipped provenance: live `status: shipped`, or
      `status: archived` with `archived_status: shipped`;
    * `181.001-T` is archived with `archived_status: done` or `shipped`, and
      reached that state through `182-S`.

    `154-S` stays held, and the `173-F` tasks above stay blocked, under any
    other terminal outcome. The `154-S` hold label and `C29EBEE5` are
    separate gates.

## Closure (post-merge, Ship)

`182-S` is single-member (`[181.001-T]`), so covering feature `181-F` is not
closed by shipping it. This is an Orchestrator decision (2026-09-28, PR #459
review cycle 1): adding `181-F` would make `182-S` multi-member, which
condition (b) forbids claiming before the `154-S` marker exists. That would
deadlock, because `154-S` depends on `182-S`. Following the 157-S precedent,
Ship's post-merge closure for `182-S` includes a governed close step:

1. Complete the normal closure, and confirm from Markdown source that
   `181.001-T` is archived with shipped provenance.
2. Only then associate the `182-S` merge SHA with `181-F`, BEFORE archival:
   `backlogit update 181-F --commit <merge-sha>` (MCP:
   `backlogit_track_commit`). Both route through `core.AssociateCommit`,
   which writes the frontmatter `commit` field, a `commit_links` row and a
   `commit_tracked` event.
3. Then run `backlogit move 181-F --status done` and
   `backlogit archive 181-F`.
4. Optional narrative: `backlogit comment add 181-F --actor ship --comment
   "..."`. This writes only a `comment` event and does NOT associate the
   commit, so it cannot replace step 2.

If step 1 is not satisfied, leave `181-F` `queued` and report the gap.

This section and the guard above are post-PASS bookkeeping from PR #459
review. They add no scope, units or acceptance criteria to `181.001-T`.

## Follow-ups

* Optional: atomic temp-file-then-rename publish in the crash child, as
  defense in depth. Not scheduled.
  * On Windows, do not call `os.Remove` before `os.Rename`. See
    `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY

Attempt 1 was run by Stage on 2026-09-28, in the same dispatch as `173-F`
attempt 6. Six personas returned:

| Persona | Verdict |
|---|---|
| Constitution Reviewer | ADVISORY (1 P2) |
| Scope Boundary Auditor | ADVISORY (1 P2, the same issue) |
| Go Reviewer | PASS |
| Learnings Researcher | PASS |
| Architecture Strategist | PASS |
| Agent-Native Parity Reviewer | PASS (no parity surface) |

**Gate rationale.** The only P2 finding means ADVISORY. Stage chose to
revise the plan and re-review it (the ADVISORY/FAIL option (a)) rather than
ask the operator to authorize proceeding with an open P2.

* Plan hardening was not required. The plan justifies this: it is test
  code only.
* `Constitution Check: pass` was confirmed.

**P2-1 (Constitution, Scope).** The pinned signature
`(ur3CrashPoint, bool, error)`, with `err == nil` on a decode failure, had no
channel to carry "the last decode error" that AC5 requires.

* Remediation: pin
  `(crash, ready, decodeErr, err)`, and assert `decodeErr` in the RED test,
  including a permanently malformed case.

**P3s, all applied:**

* Check that the extraction step keeps `^TestUR3` green.
* Label AC2 as secondary evidence, and add `-timeout 20m -v` for
  non-vacuity.
* The timeout path still reaps the child.
* The helper has no package-global state.
* Correct the Windows rename rationale in the deliberation.
* Add a follow-up note: no `os.Remove` before `os.Rename`.
* Set frontmatter status to `draft` until the review passes.

Considered and not adopted (P3): treating transient Windows read errors as
not-ready. They stay fatal, as today, and that is now explicitly out of
scope.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2 was run by Stage on 2026-09-28. Five personas re-reviewed the
revised body.

| Persona | Verdict |
|---|---|
| Constitution Reviewer | PASS (4 P3) |
| Go Reviewer | PASS (4 P3) |
| Scope Boundary Auditor | PASS (2 P3) |
| Learnings Researcher | PASS (1 optional P3) |
| Architecture Strategist | PASS (3 P3) |

The Agent-Native Parity Reviewer was not triggered: there is no
agent-facing surface.

**Attempt-1 P2-1 is verified fixed by all five personas.** The 4-value
signature carries `decodeErr`, and the RED test asserts it.

**Merged P3s applied after the review.** These are wording and precision
changes only. No scope or unit changes, so no re-review is needed.

* **AC1** now claims helper classification, not "race closed". The
  poll-loop wiring is moved to an AC5 diff-review check: no non-nil
  `decodeErr` reaches `require.NoError`, and the last `decodeErr` goes into
  `FailNow`. Raised by Constitution, Go and Architecture.
* **Empty (0-byte) marker case** added to the RED test, because
  `os.WriteFile` truncates first. Raised by Go.
* **Helper internals:** decode into a local and return the zero value on a
  decode error, wrap errors with `%w`, and add a contract doc comment.
  Raised by Go and Constitution.
* **Malformed case** runs as an independent `t.Run` subtest using
  `t.TempDir()`. Raised by Constitution.
* **Constitution Check:** scenario count corrected to 3, and lines for I
  and VI added. Raised by Constitution.
* **AC4** allows plan and backlog bookkeeping. Raised by Scope.
* **Out of scope:** now names the `failurePath` diagnostic marker. Raised by
  Architecture.
* **Verification** names this plan's shipment and task IDs, and states the
  release condition for the `154-S` edges. Raised by Architecture;
  back-filled at harvest.

**Noted, not adopted:**

* The 5s readiness deadline under `-race -count=25` (Learnings, optional).
  If AC2 times out with no `decodeErr`, look at the deadline; the 4-tuple
  already distinguishes a slow child from a malformed marker.
* Two error returns are unusual Go (Go P3). Kept, because the signature is
  pinned.

<!-- plan-review-attempt: 2 -->
