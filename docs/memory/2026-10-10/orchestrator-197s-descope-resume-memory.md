---
schema_version: "1.0"
doc_type: memory
title: Orchestrator 197-S operator ruling, U15 descoping and resume record
description: Records the operator ruling that descoped U15, the review verdicts, the deviation from the Orchestrator role boundary when committing the operator config edit, and the instructions for the resumed Ship run.
timestamp: "2026-10-10T06:15:00Z"
---

# Orchestrator 197-S operator ruling, U15 descoping and resume record

## Operator ruling

At 2026-10-09T22:47-07:00 the operator ruled, verbatim: "Descope U15 as recommended." and directed that
`.autoharness/config.yaml` be committed on the current branch because its changes are intentional. I read "as
recommended" as adopting option 1 of `orchestrator-197s-wave2-halt-memory.md` in full, including the explicit
re-freeze of M at 18 tasks, the archive of 197.016-T, the removal of the 197.018-T -> 197.016-T edge, and the
single-key `green_maker_closes_wave` correction on 197.017-T (3 -> 2) that the new wave partition requires.

## Executed (Stage, no git writes)

- Follow-up stash `76553D8D` holds the R10 "fixed" remainder (real ShipShipment path profile).
- 197-S is now 197-F + 18 tasks (M = 18). Partition: W1 = 001-008, 017 (done); W2 = 009, 011-015, 018; W3 = 010;
  W4 = 019.
- Plan Erratum E2, 197-F and 197.018-T amendments, and the Stage note are the durable record.
- Two adversarial reviewers (Correctness, Constitution) returned APPROVE_WITH_NOTES. Their findings were fixed (stale
  197-F and 197.018-T text) or are recorded below.

## Deviation record: config commit (P-010)

The Orchestrator committed the operator's own `.autoharness/config.yaml` edit (`45dea2f8`) on the shipment branch at
the operator's explicit instruction. The file is outside the Orchestrator's Step 1.5 carve-out, so this is a recorded,
operator-directed deviation, not a precedent. I authored no content: I staged and committed the existing edit
byte-for-byte (sha256 `6D5E5614...661F5`). It passed `autoharness verify-workspace` strict schema. It is unrelated to
the CX plan, so the shipment PR and Copilot may flag it as out of scope. Ship answers any such comment as
operator-directed and intentional and cites this note; it is not a P-021 deferred-scope item.

## Instructions for the resumed Ship run

- Do not restore any checkpoint (`053830`, `053048`, `024921`, or the stale 153-S one). Their `M` is the old 19-task
  set, their open-red set and 197.017-T's close wave are stale. Resume from repository state at Step 3 and re-freeze M
  at 18 tasks.
- The working tree carries a staged rename (197.016-T archived) plus unstaged amendments. Commit them with explicit
  paths before the Step 4.1a baseline.
- Closure artifacts and the PR body must not describe R10 as fully delivered: it is profiled + progress reporting;
  the fix is follow-up stash `76553D8D`.
- Session usage measured from `session.usage_record` events at about 2,700 AIC (all subagents), below the 10000 limit.
