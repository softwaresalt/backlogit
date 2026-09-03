---
chunk_strategy: h1-h2-h3
description: "P2/P3 deferred risk stash-ID reconciliation for shipment 131-S / 148-F"
doc_type: incident
schema_version: "1.0"
source: ship-audit-p002
title: "P2/P3 Stash Reconciliation — 131-S / 148-F"
---

# P2/P3 Deferred Risk Stash ID Reconciliation — 131-S / 148-F

**Audit date**: 2026-08-28 (post-merge closure)
**Audited by**: Ship agent (read-only audit session)

## Prior Completion Report Claims vs. Actuals

The prior completion report (session memory `131s-148f-ship-session.md`) listed
P2/P3 follow-up items. This document audits whether governed stash IDs exist for each.

### FINDING-3 / P1 — syncWriteFileAtomic pre-Remove Windows data-loss window

**Claimed in session memory**: "syncWriteFileAtomic pre-Remove Windows data-loss (deferred stash)"
**Claimed disposition (closure doc)**: "P1 FINDING-3 (MEDIUM): Documented — syncWriteFileAtomic pre-Remove data-loss window, deferred follow-up"
**Actual stash ID created**: **NONE**

Evidence: `git diff 323c5247 5c374bce -- .backlogit/stash.jsonl` shows NO new stash
entries added during the Ship phase for 131-S. The phrase "deferred stash" in the
session memory expressed intent, but no `backlogit stash add` operation was executed.

The active stash on `origin/main` (commit `5c374bce`) does NOT contain an entry
describing the syncWriteFileAtomic pre-Remove data-loss window.

**Gap**: The "deferred stash" was never created. This is an incomplete follow-up
reporting item. The finding remains untracked in the governed stash.

---

### "stash 35A27CD0 extended" claim

**Claimed in session memory**: "Directory symlink protection in O_NOFOLLOW chain (stash 35A27CD0 extended)"
**Actual**: Stash 35A27CD0 was HARVESTED into 148.003-T (U4) during the Stage
phase (commit `00e69aab`). It was NOT "extended" — its text was not modified.
The entry exists in `.backlogit/archive/stash.jsonl` with:
  - `reason: "harvested"`
  - `harvested_artifact_id: "148.003-T"`
  - `archived_at: "2026-08-29T00:42:55Z"`

The session memory description is inaccurate. 35A27CD0 represents the SCOPE
of U4 that was implemented; additional O_NOFOLLOW coverage concerns (directory
symlink protection beyond the implemented read paths) are not separately stashed.

**Gap**: If directory symlink protection in O_NOFOLLOW chain is genuinely deferred
scope beyond what 148.003-T implemented, it has NO governed stash entry.
The only related active stash entry is `302EFF07` (Reject symlinked checkpoint
targets in read/mutate-lite verbs), which was created on 2026-08-28 and covers
the read/mutate-lite verb scope — distinct from the O_NOFOLLOW create path.

---

### AR-P2-1 — Windows O_NOFOLLOW portability

**Claimed**: "documented in-code (AR-P2-1 documented)"
**Actual stash ID**: NONE (in-code documentation in `checkpoint_nofollow_windows.go`)
**Assessment**: In-code documentation is the appropriate disposition for
implementation notes. No stash needed per closure doc. CONFIRMED CORRECT.

---

### AR-P2-2 — MkdirAll boundary not U4 scope

**Claimed**: "confirmed non-scope, no action needed"
**Actual stash ID**: NONE
**Assessment**: Confirmed non-scope. No stash needed. CONFIRMED CORRECT.

---

### AR-P2-3 — No slog security-rejection logging

**Claimed**: "deferred to monitoring plan"
**Actual stash ID**: NONE
**Assessment**: The monitoring plan covers observability via CLI error counting
(non-slog path). The disposition is a design decision (CLI errors don't use slog).
No stash created. The closure doc notes this explicitly.
**Gap assessment**: If AR-P2-3 was genuinely deferred (meaning: future work to add
slog), a stash entry should have been created. If it was closed (design decision
to not add slog), no stash needed. The closure doc is ambiguous.

---

### FINDING-4 — Predictable .tmp name

**Claimed**: "pre-existing, advisory"
**Actual stash ID**: NONE
**Assessment**: Pre-existing issue in `atomicfile` package. Advisory. No stash required
unless a security case is made. CONFIRMED CORRECT.

---

### FINDING-5 — Multiple fold-variant context top-level entries

**Claimed**: "advisory"
**Actual stash ID**: NONE
**Assessment**: Advisory, single-reviewer LOW confidence. No stash required. CONFIRMED CORRECT.

---

### FINDING-6 — reportedDuplicate uses ToLower vs EqualFold

**Claimed**: "advisory, safe"
**Actual stash ID**: NONE
**Assessment**: Single-reviewer LOW confidence, demonstrated safe. No stash required. CONFIRMED CORRECT.

---

### FILE_FLAG_OPEN_REPARSE_POINT for Windows

**Claimed**: "tracked in checkpoint_nofollow_windows.go"
**Actual stash ID**: NONE
**Assessment**: Code comment in the implementation. This is an in-code note for
future work. No governed stash entry. If this is genuinely deferred work (not just
a code comment), it lacks a governed stash entry.

## Summary Table

| Finding | Description | Claimed Stash | Actual Stash ID | Gap? |
|---|---|---|---|---|
| FINDING-3 | syncWriteFileAtomic pre-Remove data-loss | "deferred stash" | **NONE** | **YES — missing stash entry** |
| 35A27CD0 | O_NOFOLLOW directory symlink chain | "stash extended" | Harvested (not extended) | **YES — description inaccurate** |
| AR-P2-1 | Windows O_NOFOLLOW portability | in-code docs | In-code ✓ | No |
| AR-P2-2 | MkdirAll scope exclusion | confirmed non-scope | None needed | No |
| AR-P2-3 | slog security-rejection logging | "deferred to monitoring" | **NONE** (ambiguous) | Possible |
| FINDING-4 | Predictable .tmp name | pre-existing advisory | None needed | No |
| FINDING-5 | Fold-variant context entries | advisory | None needed | No |
| FINDING-6 | reportedDuplicate ToLower | safe, advisory | None needed | No |
| FILE_FLAG_OPEN_REPARSE_POINT | Windows reparse | in-code note | None (code comment) | Possible |

## Required Operator Actions

1. **FINDING-3 (syncWriteFileAtomic pre-Remove data-loss)**: A governed stash entry
   was NOT created for this deferred P1/MEDIUM finding. This is an untracked risk.
   The orchestrator or Stage agent must decide: create a stash entry for this finding
   or formally close it with a rationale note.

2. **35A27CD0 description inaccuracy**: The session memory's "stash 35A27CD0 extended"
   is factually incorrect. 35A27CD0 was harvested into 148.003-T. Any additional
   directory-level symlink protection scope beyond what 148.003-T covered is untracked.

3. **AR-P2-3 disposition**: Ambiguous between "design decision" and "deferred work."
   Orchestrator should clarify whether a stash entry is needed.

4. This audit is READ-ONLY. No new stash entries are created here per the audit scope.
   Stash creation for FINDING-3 must be done by an authorized Stage/Ship session.
