---
description: "Ship post-merge closure for shipment 093-S — frontmatter hygiene backfill (feature 104-F, tasks 104.001-T/104.002-T/104.003-T): PR #237 merged 647263c, shipment archived, 3 stash follow-ups created, compound learning captured. Last queued shipment; queue empty after."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-14T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-14-093-S-ship-closure.md
title: "093-S frontmatter hygiene backfill — Ship closure"
---

## Outcome

`ship next` executed queued shipment **093-S** (frontmatter hygiene backfill)
end-to-end and closed it. Merged via **PR #237**, merge commit **`647263c`**
(true merge commit — parents `832e682` + `ef03a3c`, P-009 satisfied). Shipment
`093-S` shipped/archived; feature `104-F` and tasks
`104.001-T`/`104.002-T`/`104.003-T` archived recording `647263c`.

This was the **LAST queued shipment** — the shipment queue is empty after this
closure.

## Members completed

| ID | Type | Work |
|---|---|---|
| `104.001-T` | task | Added missing top-level `name: spike` key to `.github/skills/spike/SKILL.md` to match the authoritative `plugin/skills/spike/SKILL.md`. Genuine TDD red phase via a NEW targeted parity test. |
| `104.002-T` | task | Backfilled `chunk_strategy: h1-h2-h3` + `schema_version: "1.0"` on 2 of the four 091-S docline docs. |
| `104.003-T` | task | Backfilled the same two soft-convention keys on the other 2 docs. |
| `104-F` | feature | Covering feature — done/archived. |
| `093-S` | shipment | Shipped (`shipment ship 093-S --sha 647263c`). |

## Files changed (PR #237, merged 647263c)

* `.github/skills/spike/SKILL.md` — added `name: spike` as first frontmatter key.
* `tests/integration/github_skill_parity_test.go` — **NEW**:
  `TestGitHubSpikeSkillFrontmatterMatchesPluginCopy` + helper
  `parseFrontmatterDoc`. Enforces full `.github`↔`plugin` frontmatter map parity
  (every key AND value) via `require.Equal`. RED before the edit
  (`[]string{"description"} does not contain "name"`), GREEN after.
* `docs/compound/2026-07-13-copilot-review-loop-convergence.md` — +`chunk_strategy`/`schema_version`.
* `docs/closure/2026-07-13-091-S-spike-docline-closure.md` — +2 soft keys.
* `docs/closure/2026-07-13-091-S-compound-refresh.md` — +2 soft keys.
* `docs/memory/2026-07-13/091-S-spike-docline-ship-memory.md` — +2 soft keys.
* `.backlogit/` — claim + completion lifecycle artifacts (queue↔archive).

Scope confirmed: one small Go test + docs/frontmatter only; no schema or CLI
behavior changed.

## Quality gates (full Constitution sequence — 104.001-T added Go test code)

`go test ./...` → `go vet ./...` → `golangci-lint run` → `gofmt -l .` all passed;
`backlogit docs lint` stayed 0-violations on each edited doc. Feature-PR Copilot
review converged: one thread raised on push (test compared only the `name` value,
not full parity) — fixed by rewriting to full-map `require.Equal`, committed
`ef03a3c`, replied + resolved; the post-push fresh review on `ef03a3c` raised zero
new threads. §1.9 readiness gate passed before HALT.

## Post-merge closure actions

1. **Fresh binary** — built `bin/backlogit-closure.exe` from merged HEAD
   `647263c`; verified embedded `vcs.revision=647263c…`. (The `+dirty`/modified
   marker is solely the retained untracked scratch file, below.) Avoided the
   092-S stale-binary trap.
2. **shipment-reconcile (pre)** — `093-S` active; `104-F` + 3 tasks done. Doctor:
   1 pre-existing, unrelated orphan (`016.001-R`, old review artifact) — out of
   scope, not introduced here, destructive `--fix-orphans` NOT run.
3. **`shipment ship 093-S --sha 647263c`** — archived all 5 members with
   `commit: 647263c`.
4. **shipment-reconcile (post)** — all 5 members `status: archived` /
   `archived_status: shipped`; none remain in queue; doctor unchanged (same lone
   pre-existing orphan). **UTC verification:** all 5 archived members carry
   `updated_at` in canonical UTC `Z` (e.g. `2026-07-14T16:12:19.6422255Z`). 093-S
   is the FIRST shipment shipped with the merged 092-S UTC-normalized writer —
   confirmed working end-to-end. (`created_at` retains its original local offset;
   expected — only mutations are UTC-normalized.)
5. **compound-refresh** (apply) — created new learning
   `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md`; the four
   recent `2026-07-13` entries reviewed and kept (reinforced/unaffected). Report:
   `docs/closure/2026-07-14-093-S-compound-refresh.md`.
6. **compact-context** (`target: all`) — applied the FULL Phase-2 candidate
   criteria (`.github/skills/compact-context/SKILL.md:59-65`), not only the
   age/count/size and appended-plan thresholds:
   * **Memory — "part of a completed feature or chore":** the new
     `docs/memory/2026-07-14/093-S-frontmatter-hygiene-ship-memory.md` DOES match
     this candidate rule (feature 104-F is done). Governing action per Phase 3 +
     the behavioral constraints "**Preserve the most recent checkpoint for each
     completed task**" and "never compact active/most-recent checkpoints":
     memory compaction consolidates a *group* of superseded/verbose checkpoints
     into one summary and archives the originals. This file is the **sole,
     newest** checkpoint for 104-F (only file in its `2026-07-14` date-group,
     with no superseded siblings), so it is **preserved**, not consolidated —
     there is no multi-file group to collapse. The older date-groups
     (`2026-07-10`/`-12`/`-13`) belong to prior *already-closed* shipments; each
     is likewise the most-recent checkpoint for its own unit and is out of THIS
     closure's scope.
   * **Memory — age/superseded rules:** none qualify (oldest 4 days < 14-day
     threshold; no superseded duplicates).
   * **Plans:** the 093-S plan has no appended plan-review verbosity to
     consolidate into a decided-plan (its "Plan Review" is an inline
     single-agent self-assessment) and is 1 day old.
   * **Closure records:** recent (1 day old, < 14-day threshold).
   * **Global thresholds:** memory 8 files / 28.2 KB (« 40 files / 500 KB).

   **Result: no compaction performed** — the only completed-feature candidate is
   the just-written 093-S checkpoint, which the preserve-most-recent constraint
   requires be kept intact; nothing archived. This is the correct governed
   outcome, not an omission of the completed-feature rule.
7. **Backlog index resync** (`.ship.agent.md:553`, Step 9) — ran
   `backlogit sync` after all archival + stash mutations: **`Indexed 834
   artifacts`, exit 0 → `CLOSURE_INDEX_SYNC_OK`**. Verified the index reflects
   the closure (`093-S`/`104-F`/tasks now `status: archived` via
   `backlogit query`). Sync did not modify any tracked files (SQLite index is
   gitignored).

## Stash follow-ups created (operator-requested)

| Stash ID | Kind | Priority | Summary |
|---|---|---|---|
| `8CD8F46A` | task | medium | Enable (or formally waive) the formal multi-persona plan-review skill gate — this env cannot dispatch reviewer persona sub-agents, so 091-S–093-S ran an honest inline self-assessment; establish a real persona-dispatch path OR a documented operator waiver so the pre-harvest gate is satisfiable. |
| `CA877CD1` | task | low | Add a labeled "Constitution Check" section to the impl-plan skill output — recent 091-S/092-S/093-S plans omitted an explicitly-labeled Constitution Check. |
| `A4BE2FAD` | task | low | Add a persistent regression guard for soft-convention docline keys (`chunk_strategy`, `schema_version`) — `docs lint` passes with or without them, so presence can silently drift. |

## Retained scratch artifact

`docs/decisions/2026-07-13-scratch-spike.md` remains **untracked** in the working
tree. Per Principle VII it is retained and NOT committed or deleted here;
deletion still requires explicit operator approval.

## Closure PR

Closure work (this artifact, the Ship session memory, the compound learning +
refresh report, and the `.backlogit/` ship/stash artifacts) committed on branch
`post-merge/093-S`; closure PR opened per §1.10 with Copilot review surveillance
and §1.9 pre-merge readiness gate. HALTED at "closure PR ready, awaiting operator
merge approval" (P-014) — NOT self-merged.
