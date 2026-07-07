---
chunk_strategy: h1-h2-h3
description: 'Post-merge closure of shipment 083-S (Gate-Broker Phase-2 Hardening) — RESOLVED. 083-S was initially blocked (see the companion BLOCKED record and PR #181) because the 082-F shipment gate used strict head_sha equality for member-evidence staleness, which falsely rejected member evidence recorded at feature-branch commits that are ancestors of the merge commit ac41bb1. The ancestor-aware gate fix (084-F, PR #182/#183, merged to main at 941062f) replaced strict equality with a fail-closed git merge-base --is-ancestor lineage test. Re-running backlogit shipment ship 083-S --sha ac41bb1 with the rebuilt ancestor-aware binary PASSED the completion gate (exit 0): shipment_status shipped, 11 items archived (083-F + 5 tasks + 4 subtasks + the 083-S shipment), returned_ids empty. Post-gate reconcile PASS: all 11 manifest items present in .backlogit/archive, no queue residue, P-007 archive-integrity clean (no archive deletions). 083-S is now archived. This closure required the new binary — the strict-equality binary refused the identical command with exit 6 — providing independent second-shipment confirmation that the ancestor-aware admission is real and correct.'
doc_type: closure
docline:
    ms.date: 2026-07-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-07-083-S-post-merge-closure-RESOLVED.md
title: 083-S Gate-Broker Phase-2 Hardening — Post-Merge Closure RESOLVED (ancestor-aware gate fix)
---

# Post-Merge Closure — 083-S (RESOLVED)

**Date:** 2026-07-07
**Shipment:** 083-S — Gate-Broker Phase-2 Hardening
**Feature merge (PR #180):** `ac41bb1d2611fadd0fae6ccc49b3a8233468622d` (2-parent)
**Closure branch:** `post-merge/083-S-reclose`
**Closure status:** ✅ **RESOLVED** — 083-S `archived`

---

## Summary

Shipment 083-S was **initially blocked** at post-merge closure (documented in
`docs/closure/2026-07-06-083-S-post-merge-closure-BLOCKED.md`, landed on `main` via
**PR #181**). The 082-F shipment gate's `validateMemberGateEvidence` used strict
`head_sha` **equality**, which falsely rejected 083-S's own members: their recorded
build heads (feature-branch commits) are **ancestors** of the merge commit `ac41bb1`,
never equal to it. Closure was correctly **halted** rather than forced, and the
ancestor-aware fix was stashed (`885A7F65`).

That fix graduated through **084-F** (PR #182 → post-merge closure PR #183), now live on
`main` at `941062f`: strict equality was replaced with a fail-closed
`git merge-base --is-ancestor` lineage test. Re-running the **identical** closure command
with the rebuilt ancestor-aware binary now **passes** where the old binary refused.

---

## Re-closure execution (the payoff)

Rebuilt binary from `main@941062f` (`go build -o backlogit.exe ./cmd/backlogit`, OK), then:

```
$ backlogit shipment ship 083-S --sha ac41bb1d2611fadd0fae6ccc49b3a8233468622d
shipment status changed  shipment_id=083-S new_status=shipped
{
  "shipment_id": "083-S",
  "shipment_status": "shipped",
  "archived_ids": [
    "083.005.001-ST","083.005.002-ST","083.005.003-ST","083.005.004-ST",
    "083.001-T","083.002-T","083.003-T","083.004-T","083.005-T",
    "083-F","083-S"
  ],
  "returned_ids": [],
  "commit_sha": "ac41bb1d2611fadd0fae6ccc49b3a8233468622d"
}
exit 0
```

**Contrast:** at the original blocked attempt (prior phase, shipment head `ac41bb1`) the
strict-equality binary refused this same command with **exit 6** ("member 083.001-T gate
evidence is stale (recorded at a prior head)"). The rerun's shipment head has since
advanced to `941062f` (a descendant of `ac41bb1`). Crucially, every member head is an
ancestor of **both** `ac41bb1` and `941062f` but equal to **neither** — so the refusal was
caused by equality-vs-ancestry semantics, not by head position: the strict-equality binary
would refuse at `941062f` too, and the ancestor-aware binary would have passed at `ac41bb1`
too. The operative change that unblocks closure is the gate semantics (the binary), not the
head advance. This is an independent, second-shipment confirmation of the fix.

### Why it now passes

Shipment head (`git rev-parse HEAD`) = `941062f`. Every member's recorded head is an
ancestor of it (via `ac41bb1`):

| Member | Recorded head | `--is-ancestor` of `941062f` |
|---|---|---|
| 083.001-T | `be1bf1e` | exit 0 → accept |
| 083.002-T | `9cc241c` | exit 0 → accept |
| 083.003-T | `bcc5fba` | exit 0 → accept |
| 083.004-T | `6ed1fd3` | exit 0 → accept |
| 083.005-T | `c93080d` | exit 0 → accept |
| 083.005.001-ST | `ffb67c8` | exit 0 → accept |
| 083.005.002-ST | `e375956` | exit 0 → accept |
| 083.005.003-ST | `5d2bc31` | exit 0 → accept |
| 083.005.004-ST | `c93080d` | exit 0 → accept |

---

## Post-gate reconciliation — PASS

- **083-S status:** `archived`.
- **Archive presence:** all 11 manifest items (`083-F`, `083.001-T`..`083.005-T`,
  `083.005.001-ST`..`083.005.004-ST`, `083-S`) present in `.backlogit/archive/`.
- **Queue residue:** none (`.backlogit/queue/083-S.md` relocated to archive).
- **P-007 archive integrity:** clean — no `.backlogit/archive/` files show as working-tree
  deletions. (The `D .backlogit/queue/083-S.md` is the expected queue→archive relocation,
  not an archive deletion.)

---

## Delivery lineage (for traceability)

- **083-S feature** — PR #180, merged `ac41bb1` (9 executable items F4/F1/F5/F7 + Q3.0–Q3.3).
- **083-S blocked record** — PR #181, merged `57722ba`; carried the 3 follow-up stashes
  (`885A7F65` blocker fix, `B85DAEE8`, `F3844849`) onto `main`.
- **084-F ancestor-aware fix** — PR #182 (feature) + PR #183 (closure), live at `941062f`;
  learning: `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md`.
- **083-S re-closure** — this artifact; `shipment ship 083-S` exit 0, 11 items archived.

## Follow-ups (unchanged; still stashed for Stage)

- `B85DAEE8` (bug) — empty `head_sha` bypasses the staleness comparison (advisory).
- `F3844849` (task) — unify malformed-JSONL-line handling (advisory).

083-S is fully closed. No further action required for this shipment.
