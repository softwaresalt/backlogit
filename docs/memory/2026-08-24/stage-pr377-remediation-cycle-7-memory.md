---
agent: stage
session_id: stage-d3ce9e81-2026-08-24
cycle: pr377-review-remediation-cycle-7
scope: D3CE9E81
shipment: 130-S
feature: 147-F
branch: chore/stage-130-s
reviewed_head: 122cdf30723196f8ebdeb9e1ce9ae2a04e5bdf69
worktree: C:\Source\GitHub\backlogit\.copilot\session-state\337f2436-0fad-4797-be93-b72985d25d56\files\stage-130s-worktree
created_at: 2026-08-24T23:55:00Z
---

# Stage — PR #377 Copilot review remediation cycle 7

A seventh Copilot review against head `122cdf30723196f8ebdeb9e1ce9ae2a04e5bdf69` (cycle-6 tip)
raised six comments grouped by four root causes. Cycle 7 is an **operator-authorized extension**
past the three-cycle limit, recorded as such rather than treated as a silent counter reset.

## Root causes and remediation

### 1. Canonical checkpoint stale at cycle 4

`.backlogit/checkpoints/checkpoint-20260824-191617.json` recorded cycle-4 state, listed obsolete
push/review flags, and did not name the reviewed HEAD versus the local unreviewed commits
generated in cycles 5, 6, and 7.

**Fix.** Regenerated with cycle-7 `context` metadata. Schema-1 top-level namespace stayed closed
— no new top-level or `progress` keys were introduced, matching the very invariant this feature
enforces. New `context` fields:

- `reviewed_head`: `122cdf30` (last commit Copilot has reviewed).
- `local_unreviewed_state`: recorded in `push_state` and `pr` — the cycle-7 remediation commit
  is local-only until Ship pushes and secures fresh CI + fresh review.
- `review_remediation`: appended cycle-5, cycle-6, and cycle-7 entries.

Validated by `backlogit checkpoint get checkpoint-20260824-191617.json` returning
`"valid": true`.

### 2. Canonical memory stale at cycle 3

`.backlogit/memories.json` key `stage-d3ce9e81-checkpoint-toplevel-keys` stopped at cycle 3 (26
tasks / 40 edges) and did not describe cycles 4–7.

**Fix.** Regenerated through cycle-7 final state via `backlogit memory save`. The updated entry
records 27 tasks / 43 edges / 28 shipment members (shape stable since cycle 4), the cycle-4 U8c
addition, the cycles-5/6 backlog-only remediation, and the cycle-7 shell-contract decision.

### 3. Wrong repair-task references in 147.018-T (comment 3)

`147.018-T` line 36 cited `(147.010-T / U6b, 147.011-T / U6c, 147.023-T / U8c)` as the get-result
producer and its projections. The actual mapping is:

| Wrong pairing | Correct pairing | What that task actually is |
|---|---|---|
| `147.010-T / U6b` | `147.012-T / U6b` | 147.010-T = U5b state-dimension classification (I3). 147.012-T = U6b `GetCheckpointResult` declaration — the producer. |
| `147.011-T / U6c` | `147.022-T / U6c` | 147.011-T = U6 `ListCheckpoints` (list surface). 147.022-T = U6c MCP `get_checkpoint` projection. |
| `147.023-T / U8c` | `147.027-T / U8c` | 147.023-T = U6d filter exemption. 147.027-T = U8c CLI `checkpoint get` projection. |

**Fix.** Edited the sentence to name the correct producer/projection triple with an inline
description of each task's role, removing the ambiguity that let the review reviewer spot the
error.

### 4. Cross-platform shell contract conflict on 147.011-T and in plan (comments 4 and 5)

The plan and 147.011-T / 147.015-T / 147.019-T claimed `RemediationCommand` was
**PowerShell-safe**. The shipped implementation in
`internal/events/checkpoint_lifecycle.go:274-290` uses `shellQuoteSingle`, which emits the POSIX
close-escape-reopen idiom `'\''`. That idiom is not a valid PowerShell literal (PowerShell
escapes an interior single quote as doubled `''`). No single command string can be paste-runnable
in both shells for an accepted filename containing a single quote, so the "PowerShell-safe"
claim over arbitrary accepted filenames was empirically false.

**Design decision — POSIX-safe contract.** Preserves shipped behavior. Grounded in existing code,
validation rules, call sites, projections, and tests:

- Shipped `remediationQuarantineCommand` emits POSIX-safe output via `shellQuoteSingle`.
- Shipped `TestListCheckpoints_RemediationCommandIsShellSafe`
  (`internal/events/checkpoint_lifecycle_test.go:430-455`) comments "safe to run verbatim in a
  POSIX shell" and asserts POSIX-safe escape output.
- `validateCheckpointFilename` (`:254-273`) rejects only empty, path-separator-bearing, or
  non-`checkpoint-*.json` names; the accepted filename grammar today already includes shell
  metacharacters, which the POSIX escape correctly handles and any PowerShell claim cannot.
- MCP `get_checkpoint` (U6c) and CLI `checkpoint get` (U8c) projections reproject the shipped
  `RemediationCommand` string verbatim — no projection layer re-quotes or re-renders. The read
  surface therefore inherits whatever the events layer emits, which is POSIX-safe.
- The natural filename generator (`internal/events/memory.go:59`) only writes
  `checkpoint-YYYYMMDD-HHMMSS.json`, so the natural corpus contains no shell-special characters
  and the emitted command is literally paste-safe in `pwsh` for every backlogit-generated file.

**Alternatives rejected.**

- **Separate shell-specific renderings** (add `remediation_command_posix` and
  `remediation_command_powershell` on the read surface): doubles the API, forces new MCP/CLI
  projection changes on U6b / U6c / U8c / U8b, adds new test scenarios, and grows the width of
  at least four tasks. Justified only if PowerShell-native support is a hard requirement — which
  the Windows-first note does not actually establish, because the generator produces safe
  filenames that are literally paste-safe in `pwsh`.
- **Filename grammar restriction** to `[A-Za-z0-9._-]+`: requires product-code change to
  `validateCheckpointFilename`, adds a new refusal path guarding against nothing on disk today,
  and needs a new width-isolated task at minimum. Justified only if the "PowerShell-safe" claim
  cannot be honestly reframed, which it can.

**Fix (documentation only, no code change).** Rewrote:

- `.backlogit/queue/147.011-T.md` — body and acceptance criterion now say POSIX-safe with the
  Windows-first-workspace generator-shape rationale reproduced in full.
- `.backlogit/queue/147.015-T.md` — body and acceptance criterion say POSIX-safe with a
  cross-reference to 147.011-T for rationale.
- `.backlogit/queue/147.019-T.md` — read-verdict row says POSIX-safe with the same cross-reference.
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — U6, U8, and U10
  relabelled to POSIX-safe with the rationale reproduced once in U6 and cross-referenced from
  U8 / U10 / U10's `Rows` block. Runtime Verification row for `U6, U6b` says "POSIX-runnable"
  with the same cross-reference. Plan-hardening remediation register (line 1467) updated to
  record the cycle-7 relabelling. Added a full cycle-7 remediation section documenting the
  decision, grounds, and rejected alternatives.

### 5. Same shell conflict in plan (comment 5)

Merged into fix #4 above — the plan units U6, U8, and U10 and their runtime verification row
all relabelled in the same pass.

### 6. U9 generated-doc ownership drift in 147.017-T (comment 6)

Plan U9 (line 748–770) assigns to 147.017-T both `docs/design-docs/checkpoint-administrative-disposition.md`
AND regenerated `docs/cli-reference/backlogit_checkpoint_*.md`, and the acceptance line "CLI
Reference Drift check is clean". The task file only listed the design-doc file and had no CLI
Reference Drift acceptance criterion.

**Fix.** Rewrote 147.017-T:

- Files list now includes regenerated `docs/cli-reference/backlogit_checkpoint_*.md`.
- Body describes the regeneration obligation and its trigger (any U6 / U6b / U8 / U8c help-text
  or output-projection change).
- Acceptance now includes "CLI Reference Drift check clean — the committed `docs/cli-reference/`
  output matches a fresh `gen-docs` run byte-for-byte".
- Added an explicit acceptance criterion that `.github/instructions/backlogit.instructions.md`
  is **not** touched by this unit — that ownership stays on **147.018-T / U9b** and must not be
  moved here.

## Files changed (this cycle)

- `.backlogit/queue/147.011-T.md`
- `.backlogit/queue/147.015-T.md`
- `.backlogit/queue/147.017-T.md`
- `.backlogit/queue/147.018-T.md`
- `.backlogit/queue/147.019-T.md`
- `.backlogit/checkpoints/checkpoint-20260824-191617.json`
- `.backlogit/memories.json` (canonical `stage-d3ce9e81-checkpoint-toplevel-keys` entry)
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
- `docs/memory/2026-08-24/stage-pr377-remediation-cycle-7-memory.md` (this file)

Untouched (deliberate): all production Go under `internal/`, `.github/instructions/`,
`.github/agents/`, `.autoharness/`, `.backlogit/queue/147-F.md`, `.backlogit/queue/130-S.md`.

## Six comments → edits mapping

| Copilot comment | Edit(s) applied |
|---|---|
| 1. checkpoint handoff stale | `.backlogit/checkpoints/checkpoint-20260824-191617.json` regenerated with cycle-7 context. |
| 2. canonical memory stale | `stage-d3ce9e81-checkpoint-toplevel-keys` in `.backlogit/memories.json` regenerated through cycle-7. |
| 3. repair task IDs wrong | `.backlogit/queue/147.018-T.md` line 36 corrected. |
| 4. shell contract conflict on 147.011-T | `.backlogit/queue/147.011-T.md` relabelled to POSIX-safe; 147.015-T and 147.019-T carry the same correction. |
| 5. same shell conflict in plan | Plan U6, U8, U10 and the Runtime Verification row for `U6, U6b` relabelled; Plan Hardening register updated; cycle-7 remediation section added. |
| 6. U9 generated-doc ownership drift | `.backlogit/queue/147.017-T.md` gains `docs/cli-reference/backlogit_checkpoint_*.md` in Files and "CLI Reference Drift check clean" plus "instruction file not touched" in acceptance. U9b ownership on 147.018-T preserved. |

## Validation evidence

- `backlogit sync` — `Indexed 1186 artifacts`.
- Task count: `SELECT COUNT(*) … artifact_type='task' AND parent_id='147-F'` returns 27.
- Edge count: `SELECT COUNT(*) FROM item_deps d JOIN items t ON d.item_id=t.id WHERE t.parent_id='147-F'` returns 43.
- Shipment members: `130-S` manifest still lists 28 items (1 feature + 27 tasks).
- All 27 task IDs `147.001-T` … `147.027-T` present.
- `backlogit doctor` — the only issues reported are pre-existing `106.*` orphans unrelated to 147-F.
- `backlogit docs lint` — `valid: true, violation_count: 0`.
- `backlogit checkpoint get checkpoint-20260824-191617.json` — `valid: true` (this build predates the U6b conformance projection, so the field is absent; validation is positive).
- Ready set: `SELECT id FROM items WHERE parent_id='147-F' AND status='queued' AND …` returns exactly `147.001-T`.
- Grep for remaining positive "PowerShell-safe" or "PowerShell-runnable" claims: only negative citations (`not claimed PowerShell-safe`, `relabelled from PowerShell-safe`, `PowerShell-safety … not claimed`) remain; no unqualified positive claim survives.

## Blockers

None. Intercom remained unavailable throughout, so remote operator visibility stays degraded — a
documented deviation carried from prior cycles.

## Next safe action

Ship or the operator:

1. Push `chore/stage-130-s` to origin.
2. Wait for CI to complete on the cycle-7 head.
3. Confirm the fresh Copilot review covers the new head.
4. Post replies to the six cycle-7 threads.
5. Resolve the threads (Stage cannot).
6. Secure explicit P-014 operator merge approval.
7. Merge with merge-commit strategy (P-009).
