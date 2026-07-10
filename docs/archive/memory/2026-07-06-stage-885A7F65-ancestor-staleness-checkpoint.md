# Stage session checkpoint — 885A7F65 (ancestor-aware shipment-gate staleness)

**Status:** COMPLETE — plan-review PASS (attempt 4, operator-authorized confirmation cycle);
harvested + queued shipment 084-S assembled; stash 885A7F65 archived. Planning-only (P-010); no
application code written.

## Outcome (final)

- **Gate:** plan-review PASS at attempt 4 (Security Lens PASS — H1 resolved, no new P0/P1; Go PASS).
  Cycle cap (2 re-entries) was reached at attempt 3; operator authorized ONE confirmation cycle.
- **Harvest:** Feature `084-F` → Task `084.001-T` → Subtasks `084.001.001-ST` (git helper+guard),
  `084.001.002-ST` (wire+rework tests), `084.001.003-ST` (head-binding/drift+bounded-read).
- **Dependency edges (blocks):** 084.001.002-ST → 084.001.001-ST; 084.001.003-ST → 084.001.002-ST
  (ST1 → ST2 → ST3). Persisted in item frontmatter `dependencies:`.
- **Shipment:** `084-S` (status **queued**) — items [084-F, 084.001-T, 084.001.001-ST,
  084.001.002-ST, 084.001.003-ST], parent-first. Left queued for Ship.
- **Stash:** `885A7F65` archived (promoted → 084-F / 084-S). New follow-up stash `1AEA2B0E`
  (low/bug) created for the FLAGGED empty-shipment-head enforced fail-open. B85DAEE8, F3844849
  untouched.
- **Commit:** path-scoped — decision + plan + this memory + `.backlogit/queue/084*.md` +
  `.backlogit/stash.jsonl` + `.backlogit/archive/stash.jsonl`. EXCLUDED operator WIP
  (`.backlogit/hooks_queue.jsonl`, `.github/agents/*`, `.gitignore`, `start.ps1`) and
  `docs/cli-reference/*.md` CRLF noise. Never `git add -A`. `.backlogit/backlogit.db` gitignored.

## Original halt trail (retained for history)

**Date:** 2026-07-06
**Stash:** `885A7F65` (medium/bug) — shipment-gate member-evidence staleness false-rejection.
**Registry mode:** backlogit CLI (`DEGRADED_MODE: MCP→CLI`); `features.shipments: true`.

## Pipeline progress

- [x] Step 0.0 tool gate (CLI mode) / 0.1 index sync (INDEX_SYNC_OK, 757 artifacts)
- [x] Step 0 operator visibility (intercom broadcasts surfaced inline; no MCP broadcast tool)
- [x] Step 1 triage (task-shaped, solo group) / 1.5 grouping (single-entry) / 1.8 learnings (3 compound docs)
- [x] Step 2 deliberation → Option A (direct-exec `*Workspace` git helper)
- [x] Step 3.1 impl-plan / 3.2 plan-harden (`Requires plan hardening: yes`)
- [~] Step 4 plan-review — **3 attempts, all FAIL; cycle cap reached → HALT**
- [ ] Step 5 harvest / 5.5 shipment / 5.6 archive / 6 summary — BLOCKED on gate PASS

## Artifacts

- Deliberation: `docs/decisions/2026-07-06-shipment-gate-ancestor-aware-staleness-deliberation.md` (lint clean)
- Plan: `docs/exec-plans/2026-07-06-shipment-gate-ancestor-aware-staleness-plan.md` (lint clean, 0 violations)
  - Contains full Plan Hardening + Constitution Check + Plan Review (attempts 1-3) sections.
  - Cycle markers present: `plan-review-attempt: 1`, `2`, `3` (all FAIL).

## Plan-review convergence trail

- **Attempt 1 FAIL** (2×P1): Security — checks #1/#2 not bound to same HEAD (fail-open on headSHA "");
  Go — isAncestor unbounded on unbounded ship path + timeout-before-ExitError ordering.
  → Added Unit 3 head-drift guard; mandatory self-bounded isAncestor timeout; test rework; Constitution Check.
- **Attempt 2 FAIL** (1×P1): Security — drift guard placed only between Evaluate and member scan =
  residual TOCTOU window. Go = ADVISORY (P2: bound the drift reads' timeout). Constitution = ADVISORY (2×P3).
  → Re-placed drift guard as LAST read before success (brackets whole eval); added `headSHABounded`
  bounded helper; capability-overlay rows; slog on malformed branch.
- **Attempt 3 FAIL** (1×P1, conf 0.84): **Go = PASS** (all Go items resolved). Security **confirms
  attempt-2 TOCTOU P1 RESOLVED**, surfaces NEW narrow P1 (H1): `headSHABounded` returning "" on its
  OWN timeout collapses a hang into a silent skip = new fail-open introduced by this change.
  → **Fix applied to plan (ready)**: `headSHABounded` now returns `(string, error)`; distinguishes
  bounded-context failure (timeout/cancel → `headResolveError` fail closed) from legacy non-context
  empty (→ preserve pre-existing skip; no-repo tests stay green). H1 resolution documented in plan.

## Design decisions (locked)

- Option A: direct-exec `func (ws *Workspace) isAncestor(ctx, ancestor, descendant) (bool, error)`
  running `git merge-base --is-ancestor` via argv-array + `cmd.Dir=ws.RootPath` + `gate.MinimalEnv()`.
  Own `context.WithTimeout` (GateBroker.TimeoutSeconds else ~5s); timeout checked before `*exec.ExitError`.
  Trichotomy: 0→included; 1→divergent→block; other/exec-error/timeout→fail closed. stderr buffer + `%w`.
- `isGitObjectName` regex `^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$` on untrusted on-disk head_sha.
- Wire behind preserved empty-*member*-head bypass (`h != ""` = B85DAEE8 scope) + equality fast-path.
- Unit 3: resolve `shipmentHead` once (bounded) before Evaluate; thread single value into
  validateMemberGateEvidence; drift re-resolve as LAST read before success → fail closed on drift;
  `headResolveError` fail-closed on bounded-read timeout/cancel; legacy non-context "" → skip.
- Decomposition: 1 feature + 1 task + 3 ordered subtasks (ST1 git helper+guard → ST2 wire+rework
  tests → ST3 head-binding/drift+bounded-read). Each single-domain, <3 files, <5 funcs, ≤4 scenarios.

## FLAGGED (do NOT fix in this item)

- Empty-*shipment*-head enforced fail-open (non-context "" → skip). Pre-existing; would break many
  no-repo tests; new follow-up stash (sibling to B85DAEE8). Documented in plan "Discovered adjacent
  issue (FLAGGED)".
- Empty-*member*-head bypass = `B85DAEE8` (untouched). `F3844849` malformed-JSONL (untouched).

## Operator decision needed (resume point)

Cycle cap (2 re-entries) exhausted. Options presented to operator:
(a) authorize attempt-4 re-review (override) — H1 fix already applied, expected PASS;
(b) accept plan as-is and proceed to harvest with H1 as an implementation-time hardening;
(c) halt fully / hand to a fresh planning session.

**On resume:** if (a), re-run Security Lens (+ optional Go/Constitution) on the current plan; append
`plan-review-attempt: 4`; on PASS proceed to Step 5 harvest → 5.5 queued shipment → 5.6 archive
`885A7F65` → 6 summary + `backlogit sync` + final memory. Commit path-scoped (decision/plan/memory +
backlog state ONLY); never `git add -A`; exclude `.github/agents/*.agent.md`, `.gitignore`,
`start.ps1`, `.backlogit/hooks_queue.jsonl`, `docs/cli-reference/*.md` CRLF noise, `internal/*.go`.
