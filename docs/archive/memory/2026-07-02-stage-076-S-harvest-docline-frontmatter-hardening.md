# Memory Checkpoint — Stage 076-S: harvest docline frontmatter hardening

- **Date**: 2026-07-02
- **Agent**: Stage
- **Entry point**: single targeted stash `A9D74372` (kind=task, priority=medium), routed by the Orchestrator.
- **Outcome**: covering feature `076-F` + 2 tasks harvested → queued shipment `076-S`. Ready for Ship.

## Root cause (grounded, confirmed)

075-S PR #164 was blocked by the CI "Docline frontmatter gate" because a Stage-authored exec-plan
had `doc_type: exec-plan` (outside the closed docline vocabulary) and no top-level `title`/`source`,
and that Stage harvest commit rode into the Ship feature branch (branch cut off un-pushed local main).

Two harness gaps (confirmed by reading the files):
- `.github/skills/impl-plan/SKILL.md` specifies plan **body** sections but gives **zero** docline
  frontmatter guidance → author improvised the invalid frontmatter.
- `.github/skills/harvest/SKILL.md` has **no pre-commit lint gate** → invalid plan reached CI.

Contract source of truth: `internal/docline/policy.go` (authoring-profile required fields =
`title,source,doc_type`; closed vocabulary includes `plan`, not `exec-plan`; `docs/exec-plans/**` →
`plan`). CI gate: `.github/workflows/ci.yml` job `docs-lint` → `make docs-lint` →
`go run ./cmd/backlogit docs lint`. `.github/` and `docs/memory/` are scope-excluded from linting.

## Pipeline decisions

- **Deliberate: SKIPPED** (folded into plan). Investigation confirmed the stash hypothesis exactly
  (no divergence); operator authorized lean plan. Option analysis (directions 1/2/3) captured in the
  plan's Decisions section, mirroring the green reference which also folded its decision in.
- **Plan**: `docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md`.
  Authored with **valid** docline frontmatter and self-verified: `backlogit docs lint` →
  `valid: true, 0 violations`; repo-wide corpus stays green. Self-demonstrates the fix target state.
- **Plan-harden: SKIPPED** — `Requires plan hardening: no` (all 5 signals absent; docs-only,
  git-reversible). P-006 satisfied.
- **Plan-review: ADVISORY (PASS)**. Personas: Scope Boundary Auditor + Constitution Reviewer.
  0 P0/P1; 4 P2 (all resolved pre-harvest); 5 P3 (folded in). Key P2 = **version-skew**: pin the
  lint gate/self-lint to the CI entrypoint (`make docs-lint`), not a stale installed binary.
- **Scope**: directions (1) born-compliant frontmatter guidance + (2) pre-harvest lint gate IN;
  direction (3) harvest-delivery topology + upstream `.tmpl` drift DEFERRED → follow-up stash
  `EED25928`. **No Go code change** (`backlogit docs` surface already exists).

## Backlog produced

- Feature `076-F` "Harden Stage harvest: born-compliant plan frontmatter + docline lint gate" (queued).
  - Task `076.001-T` "Add docline frontmatter contract + self-lint to impl-plan skill" (queued, single-file).
  - Task `076.002-T` "Add pre-harvest docline lint gate to harvest skill" (queued, single-file).
  - Tasks are **independent** (no dependency edge); either order / parallel.
- Shipment `076-S` (queued): items `[076-F, 076.001-T, 076.002-T]` — parent-first, item_count=3,
  matches harvest scope. Scope guard applied (unrelated stash 21E17BFC/9140F65C/17D29DDC excluded).
- Stash `A9D74372` archived with forward-link marker → 076-F / 076-S / plan / EED25928.

## Deferred (still active in stash)

- `21E17BFC` (low, feature) — singleton MCP server (contingency; untouched).
- `9140F65C` (low, task) — npm publish in release workflow (untouched).
- `17D29DDC` (low, task) — consolidate shipment-items normalization (untouched).
- `EED25928` (low, task) — NEW follow-up: direction 3 + upstream `.tmpl` drift.

## Handoff to Ship

`shipment_id = 076-S` (queued). Implementation = two `.github/skills/**` harness edits (Ship's
"Source code / harness" scope). Ship should run the lint gate via the CI entrypoint; CI Docline gate
remains the non-bypassable backstop.

## Working-tree note

Committed only Stage artifacts (plan, `.backlogit/queue/076-*`, `.backlogit/stash.jsonl`,
`.backlogit/archive/stash.jsonl`, this memory file). Operator in-flux files intentionally NOT
committed: `.backlogit/hooks_queue.jsonl`, `.github/agents/*`, `.gitignore`, `.cursor/`,
`.github/copilot/`. Built binary `bin/backlogit-stage.exe` is gitignored.
