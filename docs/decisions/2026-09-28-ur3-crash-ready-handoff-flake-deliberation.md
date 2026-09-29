---
chunk_strategy: h1-h2-h3
description: "Deliberation for deferred stash BDA56ED8 (duplicate 46A898B8): the UR3 crash-child readiness handoff can be read half-written, which flakes TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage; fix it in the test harness only, with a reader that tolerates a partial marker and a reproduce-first RED"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-28-ur3-crash-ready-handoff-flake-deliberation.md
title: "Deliberation: UR3 crash-child readiness handoff flake (BDA56ED8)"
docline:
    stash_id: BDA56ED8
    status: decided
    created_at: 2026-09-29T05:40:00Z
---

## Deliberation: UR3 crash-child readiness handoff flake

**Depth**: focused.
**Route**: `deliberate`. Both entries carry the `DEFERRED SCOPE EXPANSION`
marker, so P-021 C6 applies.
**Operator direction** (2026-09-28T22:22 -07:00): "Probably should fix a flaky
test before a next task that could depend on it." Fix it before any `173-F`
task whose gate runs `./internal/core/...`.

## Triage and duplicate reconciliation

| Entry | Captured | Source | Disposition |
|---|---|---|---|
| `BDA56ED8` | 2026-09-26 (earliest) | Ship, task `174.078-T`, feature `174-F`, shipment `155-S`, PR N/A (pre-PR), thread N/A | **Survivor** |
| `46A898B8` | 2026-09-28 | v1.11.0 release gate (Windows, `131577c1`) | **Duplicate**: same root cause, same test and file. Its detail (line refs, fix options) is merged into the survivor. It is archived (non-destructively) with a `duplicate of BDA56ED8` note. |

* **(A) Duplicate scan.** The two entries above are duplicates of each other.
  No other active stash entry or queued item covers this harness. A search
  for the test name, "crash-ready" and "readiness handoff" found only these
  two.
* **(B) Late-ID reconciliation.** `BDA56ED8` has PR and thread = N/A. No Ship
  residual-risk record cites either stash ID: tracked docs, `.backlogit/reconcile/*`,
  checkpoints, and the bodies and comments of PR #450 and #451 were all
  searched. No late identifier was found, so N/A stands.

## Problem frame

In `internal/core/shipment_blocked_recovery_harness_test.go`:

* The crash child publishes its crash point by writing JSON to `ReadyPath`
  with plain `os.WriteFile` (in `TestShipmentBlockedRecoverySubprocessHelper`,
  about lines 576 and 610).
* The parent (`runUR3CrashSubprocess`, about lines 353-358) polls with
  `os.ReadFile`. As soon as the file exists, it runs
  `require.NoError(t, json.Unmarshal(...))`.

A read that lands between file creation and the end of the write sees empty
or truncated JSON and fails the test with "unexpected end of JSON input".
This was observed once in a full `go test ./...` run on Windows. Isolated
reruns pass 10 out of 10 times, and CI is green on the same tree. The
product has no impact; this is test harness synchronization only.

## Options

1. **Writer: atomic publish.** Write to a temp file, then `os.Rename` it to
   `ReadyPath`. This fixes the root cause at the source, but:
   * it needs a new helper plus two call-site edits in the child;
   * it cannot be reproduced deterministically as RED.

   The Windows rename concern originally listed here was withdrawn in review:
   `ReadyPath` does not exist before the publish, so no reader holds a handle
   on the destination.
2. **Reader: tolerate a partial marker.** Extract a small reader helper. The
   parent treats an unmarshal failure as "not ready yet" and keeps polling
   until the existing deadline. At the deadline it reports the last parse
   error. This allows a deterministic RED: stage a truncated marker, then
   complete it. It touches one file and three functions.
3. **Both.** This is more robust, but it touches five functions in one file,
   which exceeds the "fewer than 5 functions" task budget.

## Chosen direction: option 2 (reader-side tolerance)

The operator pre-authorized either fix ("atomic temp+rename write of the
marker, or treat unmarshal failure as not-ready and keep polling").

Option 2 is chosen because:

* it is the one that allows a reproduce-first RED;
* it fully closes the race, since the child writes the marker exactly once
  and then parks until it is killed;
* it stays within the task budget.

Option 1 is recorded as optional defense in depth and is not scheduled.

**Scope limits.** Test-harness only; no production code.

* The UR10 and UR1b readiness markers are out of scope. They check only
  whether the file exists (`waitUR10Marker`, `os.Stat`) or write the fixed
  text `"ready"`, and never parse JSON.

## Traceability

* Plan: `docs/exec-plans/2026-09-28-ur3-crash-ready-handoff-flake-plan.md`.
* Harvested as feature `181-F` and task `181.001-T`, in single-member
  shipment `182-S`.
* Stash `BDA56ED8` was promoted and archived. `46A898B8` was archived as its
  duplicate.
* Consumer gate: `173-F` / `154-S` (see
  `docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`
  `## Verification`).
