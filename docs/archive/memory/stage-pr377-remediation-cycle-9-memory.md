---
agent: stage
session_id: stage-d3ce9e81-2026-08-24
cycle: pr377-review-remediation-cycle-9
scope: D3CE9E81
shipment: 130-S
feature: 147-F
branch: chore/stage-130-s
worktree: C:\Source\GitHub\backlogit\.copilot\session-state\337f2436-0fad-4797-be93-b72985d25d56\files\stage-130s-worktree
created_at: 2026-08-25T22:30:00Z
---

# Stage — PR #377 Copilot review remediation cycle 9

A ninth Copilot review against the current PR head raised one comment on
`docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`. Cycle 9 is an
**operator-authorized extension** past the three-cycle limit, recorded as such rather than
treated as a silent counter reset.

GitHub PR #377 is authoritative for the current PR head, CI, review coverage, push status, and
unresolved threads. This memory intentionally does **not** SHA-pin the reviewed head, because
every review-remediation cycle advances the branch tip and any recorded SHA goes stale the
moment the branch advances further. Consumers must query the live PR before treating any
merge-readiness claim as valid.

## Reviewer's finding

> This scope boundary contradicts the reviewed plan: requirement R8 and units U6/U6d explicitly
> modify `ListCheckpoints` and its filter contract. Leaving it listed as out of scope gives Ship
> conflicting authoritative instructions. Reconcile the deliberation with the widened
> read-surface scope.

The deliberation's Scope boundary section carried a single bullet —
`` `CleanupCheckpoints`, `ListCheckpoints`, and hook checkpoints under
`.backlogit/runtime/hooks/`. `` — that named `ListCheckpoints` as wholly out of scope. The
implementation plan already documents the correct, opposite decision: **Q3** states plainly that
"The source document's Scope boundary listed `ListCheckpoints` as out of scope. **This plan
explicitly overrides that exclusion** ... That is R8, and it is plan-originated rather than
inherited." **R8** ("Every checkpoint read surface agrees with the mutation verbs about which
files are rewrite-safe") is implemented by **U6** (`147.011-T`, conformance-verdict projection —
`NeedsQuarantine`/`RemediationCommand` populated from the same `CheckConformingTopLevelNamespace`
check the mutation verbs use) and **U6d** (`147.023-T`, quarantine-candidate filter exemption
with a published `ListCheckpoints` doc-comment contract). Both units land in
`internal/events/checkpoint_lifecycle.go`, which the deliberation's own Scope boundary section
already lists as in scope for the mutation-verb changes — the same file, contradictory scope
verdicts for two functions living in it.

The reviewer is correct: the plan resolved this tension on its own authority (a plan may widen a
deliberation's scope when it names a concrete reason, which Q3 does), but the deliberation itself
was never updated to say so, leaving Ship with two authoritative documents in direct conflict.

## Root cause and remediation

### The deliberation's out-of-scope bullet named `ListCheckpoints` broadly instead of precisely

**Fix.** Split the single bullet into two:

- `CleanupCheckpoints` and hook checkpoints under `.backlogit/runtime/hooks/` remain named,
  unqualified, out of scope — unchanged from before.
- `ListCheckpoints` is now named **partially in scope**, with the exact carve-in stated: the
  conformance-verdict projection (U6) and the quarantine-candidate filter exemption with its
  published doc-comment contract (U6d), both attributed to the plan's R8 widening of this
  document's own Unresolved Question 3 (previously "deferred to planning," now "yes, and widened
  by plan review"). What remains out of scope within `ListCheckpoints` is named precisely rather
  than left as a residual blanket exclusion: its filter semantics for conforming documents, sort
  order, pagination, and any read path unrelated to the quarantine verdict.

No other section of the deliberation asserted a conflicting claim. The Findings section's mention
of `ListCheckpoints` (F2: "`ListCheckpoints` is read-only and already surfaces these files with
`needs_quarantine: true`...") describes pre-existing behavior for schema-invalid files and does
not conflict with the widened scope. Unresolved Question 3 already correctly deferred the
conformance-flagging question to planning rather than asserting an answer, so it required no
edit — the plan's Q3 answer is the resolution of that deferral, not a contradiction of it.

## Net effect

- **No task added.**
- **No dep edge changed.**
- **No shipment member added.**
- **Backlog shape stays 27 tasks / 43 edges / 28 shipment members** (confirmed by direct query
  against the worktree's synced index: 27 tasks under `147-F`, 43 `item_deps` edges rooted in
  those tasks, 28 members in shipment `130-S`).
- Reviewed decision, implementation plan, feature/task structure, shell contract, checkpoint
  repair semantics, hard merge gate, dependencies, and shipment membership are unchanged — this
  cycle is a docs-only wording reconciliation of one deliberation, not a scope decision change.
- `docs lint` (scoped to this worktree via `--cwd`) reports zero violations across the corpus,
  including the edited deliberation file.

## Files changed

- `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md` — Scope
  boundary's out-of-scope bullet list split so `CleanupCheckpoints` and hook checkpoints stay
  named out of scope while `ListCheckpoints` is corrected to partially in scope, naming the exact
  U6/U6d carve-in and precisely what remains out of scope within that surface.
- `.backlogit/checkpoints/checkpoint-20260824-191617.json` — cycle-9 entry appended to
  `context.review_remediation`, `context.pr`, and `resume_hint`; `context.memory_path` gains the
  cycle-9 reference; `updated_at` bumped. No `ci_state`, `push_state`, `resume_ref`, or head/push/
  CI claim was added or changed — those fields stay live-PR-authoritative per cycle 8's framing.
- `.backlogit/memories.json` — cycle-9 addendum appended to
  `stage-d3ce9e81-checkpoint-toplevel-keys`.
- `docs/memory/2026-08-24/stage-pr377-remediation-cycle-9-memory.md` — this file.

## Next safe action

Query the live PR #377 head, CI, and Copilot review coverage. If the current head lacks green CI
or fresh Copilot coverage against that head, obtain them. Stage stopped at its Role Boundary; Ship
or the operator advances the remote, posts the reply for the cycle-9 thread, resolves it, and
merges under the hard merge gate: `147.018-T` (U9b) must land in the same merge commit as
`147.007-T`, `147.008-T`, and `147.009-T`. If the `147.009-T` paired accept/refuse assertion
cannot pass, halt rather than weaken it.
