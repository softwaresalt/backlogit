# Stage session memory — 2026-07-04 — CI cost-gating + closure compaction

Status: complete. Agent: Stage. Mode: AFK autonomous (operator pre-authorized, pre-Ship half of Stage→Ship).
Branch: main. Role isolation: planning/backlog only (P-010) — no code/build/PR.

## Scope of this session (operator mandate)

- Actionable: `D760E508` (CI cost-gating, medium), `2EF8B7AD` (docs/closure compaction, low).
- Defer/exclude (do not promote): `34F11E5A` (npm publish, operator BLOCKED), `21E17BFC` (singleton MCP, contingency not triggered), `EED25928` (part-a Ship-owned topology, part-b out-of-tree Principle IV).
- Risk isolation directive: keep the two actionable items as SEPARATE shipments.

## Pipeline outcome

| Step | Result |
|---|---|
| 0.0 Tool gate | CLI fallback mode (backlogit v1.3.0 on PATH); TOOL_OK for stash/shipment; hooks/checkpoint CLI probed. DEGRADED: agent-intercom has no reachable tool surface → milestones captured here instead. |
| 0.1 Index sync | INDEX_SYNC_OK (717 artifacts at start). |
| 1 Triage | Both actionable items task-shaped. |
| 1.5 Grouping | Two solo groups → two shipments (risk isolation per mandate). |
| 1.8 Learnings | Retrieved (CI gating + closure compaction) — medium/high confidence prior art folded into deliberations. |
| 2 Deliberation | Two artifacts written + linted clean. |
| 3.1 impl-plan | Two plans written + linted clean. |
| 3.2 plan-harden | Plan 1 (CI) hardened (Requires plan hardening: yes). Plan 2 (closure): no. |
| 4 plan-review | Plan 2: PASS. **Plan 1: DEFERRED after 3 consecutive FAILs (cycle budget exhausted).** |
| 5 Harvest | Plan 2 only → 081-F + 081.001-T. Plan 1 NOT harvested (deferred). |
| 5.5 Shipment | 081-S (queued): [081-F, 081.001-T]. Verified 2 items. |
| 5.6 Archive | 2EF8B7AD archived (consumed → 081-F/081-S). D760E508 left ACTIVE (deferred). |

## Artifacts created

- `docs/decisions/2026-07-04-ci-cost-gating-deliberation.md` — D760E508 (Option C chosen).
- `docs/decisions/2026-07-04-closure-docs-compaction-deliberation.md` — 2EF8B7AD (lightweight).
- `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` — D760E508 plan + Plan Hardening + full Plan Review trail (rounds 1–3 FAIL, round-4 corrected design). **Marked DEFERRED / UNREVIEWED.**
- `docs/exec-plans/2026-07-04-closure-docs-compaction-plan.md` — 2EF8B7AD plan + Constitution Check + Plan Review (PASS).
- This memory file.

## Backlog created (this session)

- Feature `081-F` "Compact docs/closure archive (housekeeping)" (chore-labeled, priority low, status queued).
- Task `081.001-T` "Consolidate and archive stale docs/closure records" (docs domain, 5 ACs, priority low, queued).
- Shipment `081-S` (queued) — items [081-F, 081.001-T], parent-first.

## Recommended Ship order

1. **081-S** (closure compaction) — low risk, docs-only, good first Ship / gate-warmup. Ready now.
2. (CI gating — only after D760E508 is re-planned and passes a fresh plan-review; see below.)

## D760E508 DEFERRAL — resumption dossier (IMPORTANT)

**Why deferred**: plan-review FAILed 3 consecutive rounds, all on the CORE merge-safety mechanism (the dorny/paths-filter change-detection gate). Cycle budget (2 re-entry cycles) exhausted per Step-4 contract. Operator AFK guidance explicitly authorized DEFER over risking broken required checks. Self-certifying a 4th unreviewed revision = P-005 violation.

**Finding trail**:
- Round 1 FAIL: (P1-A) positive `code` allowlist under default `some` = FAIL-OPEN (unenumerated paths like `schemas/**`, `scripts/**`, `plugin/**`, `npm/**`, `.mcp.json`, non-workflow `.github/**` skip tests). (P1-B) adding `Needs` to the SHARED `ciJob` struct as string/[]string breaks yaml.v3 parsing of `release.yml` (dual scalar `needs: test` + sequence `needs: [a,b]`) → regresses 4 existing tests.
- Round 2 FAIL: (P1-C) the round-1 fix used a trailing in-list negation `!docs/cli-reference/**` assuming gitignore last-match-wins — wrong; dorny evaluates per-file, so it both skipped cli-reference manual edits AND skipped drift on pure code PRs. Plus P2: `needs: changes` + skipped-conclusion treated as passing by branch protection = fail-open.
- Round 3 FAIL: (P1-D) the round-2 `docs_only` positive allowlist under `predicate-quantifier: every` is a CONSTANT-FALSE NO-OP. **Source-verified** against dorny/paths-filter `README.md` + `src/filter.ts`: under `every`, `isMatch(file) = patterns.every(rule => rule.isMatch(file))` (each pattern an independent picomatch) → filter true iff SOME file matches ALL patterns; three disjoint positive patterns can never all match one file. (Author + round-1/2 reviewers all held the wrong mental model; caught only by verifying source.)

**Source-verified CORRECT design (round 4, UNREVIEWED — must pass a fresh plan-review before harvest)**:
- Detect existence of an out-of-allowlist file with ALL-NEGATED patterns under `every`:
  `unsafe: ['!docs/**','!**/*.md','!.backlogit/**']`, `predicate-quantifier: 'every'` → `unsafe='true'` iff any changed file is outside the allowlist; `'false'` iff docs-only.
- Gate heavy `test` steps: `if: needs.changes.outputs.unsafe != 'false'` (use `!= 'false'`, NOT `== 'true'`, so an empty output from an infra failure runs heavy work = fail-safe).
- `docs-lint` (Docline gate): always-run, ungated (cheap; genuine required context every PR).
- Drift workflow: add single-pattern `cli_ref_touched: ['docs/cli-reference/**']` (quantifier-invariant); gate `if: needs.changes.outputs.unsafe != 'false' || needs.changes.outputs.cli_ref_touched == 'true'`.
- Job-level `if: ${{ !cancelled() }}` on `test` + `drift` so a failed/absent `changes` job still runs heavy verification (fail-safe), not a silently-passing skipped context.
- Truth table (no fail-open): code PR → runs; cli-reference edit → drift runs; docs-only → skips (savings); infra failure → runs.
- Unit C: type any `Needs` as `any`/custom (or omit); add `ciJob.Name` + `Strategy.Matrix` (use `map[string][]any` forward-safe); raw-content scan for `paths:`/`paths-ignore:` anchored to the `on:` block + positive parse canary (`On.PullRequest.Branches` contains `main`; sound under yaml.v3); assert the fail-safe gate DIRECTION (`unsafe != 'false'`); add a **required-SKIP canary** (a docs-only PR must be observed to actually SKIP heavy steps — a gate that can never skip is as much a defect as one that skips on code); cross-file `unsafe` allowlist consistency; extend SHA-pin + persist-credentials coverage to cli-reference-drift.yml. Mandate test-first red phase.
- Hard constraint preserved: 4 required contexts (`test (1.23)`, `test (1.24)`, `Docline frontmatter gate`, `CLI Reference Drift`) keep reporting genuine `success` on every PR type (no workflow-level `paths`/`paths-ignore`; always-run jobs, step-level gating). AC-1 required-check-satisfiability criterion carried.

**Next session**: re-open `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` (body already reflects the round-4 corrected design), run a FRESH plan-review cycle (fresh budget), and if PASS, harvest into feature + Unit A (config/ci.yml) + Unit B (config/cli-reference-drift.yml) + Unit C (tests) with dep C→A,B, then a separate queued shipment. Ideally operator-attended given the 3-round difficulty.

## Deferred / excluded / out-of-mandate stash (left active)

- `D760E508` (medium, task) — DEFERRED (see dossier above). Active.
- `34F11E5A` (low, task) — EXCLUDED: npm publish; operator BLOCKED (npmjs.com login broken, cannot provision @backlogit org). Human-only. Active.
- `21E17BFC` (low, feature) — DEFERRED: singleton MCP server contingency; trigger condition not met (fixes shipped in 031-F). Active.
- `EED25928` (low, task) — DEFERRED: part-a branch/push topology (larger blast radius, partly Ship-owned); part-b targets external autoharness `.tmpl` sources = out-of-tree, Principle IV NOT ACTIONABLE from this workspace. Active.
- `D23DFA0B` (medium, feature) — OUT OF MANDATE: newly present this session (Pre-Task-Completion Gate Broker; references its own design doc + 5-unit plan). Substantial feature warranting its own dedicated deliberation/planning cycle. Not in the operator's actionable list for this run. Left active for a future session.

## Staging discipline (do at commit time)

Stage ONLY: `docs/decisions/2026-07-04-*`, `docs/exec-plans/2026-07-04-*` (the 4 new artifacts), this memory file, and legitimate backlog state (`.backlogit/queue/**` new items 081-*, `.backlogit/archive/**`, `.backlogit/stash.jsonl`). NEVER stage operator WIP: `.github/agents/*.agent.md`, `.gitignore`, `start.ps1`, `.backlogit/hooks_queue.jsonl`. No `git add -A`. Conventional commit.
