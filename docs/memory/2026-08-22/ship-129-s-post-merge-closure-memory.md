---
chunk_strategy: h1-h2-h3
description: "Ship agent post-merge closure session memory for PR #373 merge and shipment 129-S / feature 146-F closure."
doc_type: memory
docline:
    date: 2026-08-22T00:00:00Z
    status: accepted
    tags:
        - ship
        - post-merge-closure
        - 146-F
        - 129-S
        - pr-373
schema_version: "1.0"
source: docs/memory/2026-08-22/ship-129-s-post-merge-closure-memory.md
title: "Ship — PR #373 merge and 129-S / 146-F post-merge closure"
---

# Ship session memory — PR #373 merge and post-merge closure

## Task IDs Completed

* PR #373 merged (merge-commit strategy, no admin fallback) — SHA `15ab30a2a394439f52e5338fc94d1c50e3f395ae`.
* Feature `146-F` moved to `status: done` (all 23 tasks were already `done`).
* Runtime verification of both shipped defect fixes: **PASS**.
* Post-merge closure branch created: `post-merge/146-f-success-shaped-evidence-loss`.
* Shipment `129-S` pre-ship reconciliation run (`mode: pre`, `expected_status: done`):
  **PROCEED** — report at `.backlogit/reconcile/129-S-pre-20260822-192054.md`.
* Follow-up stash `DD957688` created for the shipment-level ship-time gate blocker.
* Compound-refresh check: no stale entries found relevant to this shipment.

## Task IDs BLOCKED

* Shipment `129-S` `backlogit_ship_shipment` call: **BLOCKED** — task `146.006-T`'s
  gate-pass evidence head_sha is dangling (not an ancestor of merged `main`), almost
  certainly due to a history rewrite during the PR #373 review cycle. Verified the
  underlying code is genuinely present at HEAD; this is stale evidence, not missing work.
  No supported non-destructive repair tool exists (see closure artifact for full detail).
  Shipment lock was acquired and cleanly released; no partial mutation occurred (the
  `gate_blocked` refusal happens in `validateMemberGateEvidence`, before any archival
  mutation).

## Files Modified / Created

* `.backlogit/queue/146-F.md` → `.backlogit/archive/146-F.md` (status: active → done;
  auto-archived on terminal-status transition, per workspace convention).
* `.backlogit/hooks_queue.jsonl` — append-only event log entry for the 146-F status change.
* `.backlogit/reconcile/129-S-pre-20260822-192054.md` — pre-ship reconciliation report.
* `docs/closure/2026-08-22-146-f-129-s-runtime-verification.md` — runtime verification
  (PASS) for both defect fixes.
* `docs/closure/2026-08-22-146-f-129-s-closure.md` — full operational closure artifact,
  including the shipment-gate blocker analysis and READY WITH CONDITIONS verdict.
* This memory file.

## Decisions and Rationale

1. **Moved 146-F to `done` before pre-ship reconciliation.** All 23 child tasks were
   already `done`; the covering feature itself was still `active`. Prior archived
   features (e.g. `144-F`) show `archived_status: done`, confirming the feature must
   reach `done` before shipment archival. This is a normal, expected Ship-step action, not
   a workaround.
2. **Did not attempt to repair the 146.006-T stale gate-evidence via hand-editing or
   force-gates.** `--force-gates` is documented as inapplicable to archived items, and no
   `UnarchiveItem`-equivalent CLI/MCP tool exists. Hand-editing `.backlogit/archive/` or
   the JSONL evidence log would defeat the security guard the code intentionally enforces
   and violates the explicit "do not hand-edit generated cache state" instruction. Failed
   closed per operator instruction to fail closed on any reconciliation ambiguity.
3. **Verified the narrow scope of the blocker** by checking gate-evidence ancestry for
   all 23 task members of the 129-S manifest against current `main` HEAD (the ship-time
   member-evidence gate only checks `task`/`subtask` artifacts, not the covering feature)
   — 22 tasks pass cleanly and only `146.006-T` fails; the underlying code change is
   independently verified present at HEAD.
4. **Stashed a follow-up (`DD957688`)** for Stage triage recommending a supported repair
   path (an audited "force ship-time member evidence" operation), rather than leaving the
   gap undocumented.
5. **Created the post-merge closure branch BEFORE committing** the 146-F status change and
   reconciliation report, per Step 6.0 (post-merge commits must not land on `main`
   directly). The `backlogit_move_item` mutation happened while still checked out on
   `main` but was not committed until after `git checkout -b post-merge/...`.

## Failed Approaches

* Attempted `backlogit_ship_shipment(129-S, sha=15ab30a2...)` directly — refused with
  `gate_blocked` on `146.006-T`. Root-caused via source inspection
  (`internal/core/shipment_gate.go`, `validateMemberGateEvidence`) rather than retried
  blindly (would have failed identically every time; the block is a git-ancestry fact, not
  a transient condition).

## Open Questions / Next Steps

* Operator decision needed on how to resolve stash `DD957688` — either add tooling support
  for this repair class, or manually decide an acceptable path to complete
  `backlogit_ship_shipment(129-S)`.
* Post-merge closure PR still needs to be created, reviewed (Copilot), CI-monitored, and
  presented for **separate** operator approval before merge (not covered by the PR #373
  approval). See closure PR section of the final handoff.
