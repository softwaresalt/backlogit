---
chunk_strategy: h1-h2-h3
description: "Ship post-merge closure memory for shipment 106-S ('Provision markdownlint (P-008) tooling'): PR #300 merged (59269785), shipment + 126-F + 126.001-005-T archived, compound learning graduated, three follow-up stashes recorded."
doc_type: memory
docline:
  date: 2026-07-26T00:00:00Z
  ms.topic: closure
schema_version: "1.0"
source: docs/memory/2026-07-26/106-S-post-merge-closure-memory.md
title: "106-S Post-Merge Closure — Ship Session Memory"
---

# 106-S Post-Merge Closure — Ship Session Memory

## Shipment Outcome

- **Shipment**: 106-S — "Provision markdownlint (P-008) tooling"
- **Status**: SHIPPED and ARCHIVED
- **Merge commit**: `59269785c502ec268a9eaaf11c6e70b76e3cee0a` on `origin/main`
  (merge-commit strategy, P-009)
- **Branch**: `chore/stage-106-S` — consolidated PR landing implementation AND
  revised planning/backlog artifacts together (operator-directed, not the usual
  Stage-plans-only split)
- **PR**: #300 — <https://github.com/softwaresalt/backlogit/pull/300>
- **Closure branch**: `chore/close-106-s` (this session)
- **Origin stash**: B3D30415 (bug, medium) — archived/harvested

## Items Done / Archived

| Item | Type | Terminal status | Location | Commit |
|---|---|---|---|---|
| 126-F | feature | done | `.backlogit/archive/` | `59269785` |
| 126.001-T | task (config) | done | `.backlogit/archive/` | `59269785` |
| 126.002-T | task (Makefile/scripts) | done | `.backlogit/archive/` | `59269785` |
| 126.003-T | task (guard tests) | done | `.backlogit/archive/` | `59269785` |
| 126.004-T | task (repo-wide CI) | done | `.backlogit/archive/` | `59269785` |
| 126.005-T | task (P-008 reconcile) | done | `.backlogit/archive/` | `59269785` |
| 106-S | shipment | archived | `.backlogit/archive/` | `59269785` |
| 054-DL | deliberation | archived (pre-existing) | `.backlogit/archive/` | — |

`done` tasks live in `.backlogit/archive/` without `archived_from` provenance —
this matches the repo norm (e.g. `001.001-T`…`001.005-T`). Only the shipment
(archived via the `archive` operation) and top-level features carry
`archived_from`.

## The `_title` Config Decision + Empirical Verification

The crux of the shipment. Backlog/docline artifacts carry frontmatter `title:`
**plus** a body `# H1`; markdownlint's default MD025 `front_matter_title` regex
counts `title:` as the H1, so the body `# H1` double-counts and MD025 fires (229
files). Fix — the exact `.markdownlint.json`:

```json
{
  "default": false,
  "MD001": true,
  "MD025": { "front_matter_title": "^\\s*_title\\s*[:=]" },
  "MD041": true
}
```

Retargeting **MD025** to a sentinel `_title` key (no artifact has it) stops the
double-count with **zero file edits**; **MD041 stays default** so `title:` still
credits it (retargeting MD041 instead fails ~1,262 files). Empirical progression
over 1839 tracked files:

- **250** (default rules: MD001=1, MD025=229, MD041=20)
- → **21** (`_title` config only, zero edits)
- → **0 / 1839** (config + 21 structural fixes: 20 `SKILL.md` leading-H1 for
  MD041, 1 H3→H4 fix for MD001)

The gate is repo-wide, SHA-pinned, Node 22, and hard-fails CI (promotion to a
branch-protection *required check* deferred to stash 918BCDAF).

## Closure Anomaly (tool limitation, handled)

`shipment ship 106-S --sha 59269785` set the shipment to `shipped` and recorded
commit traceability on the shipment, then **failed** at
`record commit traceability: persist item 054-DL commit … refusing to write
archived artifact 054-DL without provenance`. Root cause: feature 126-F carries
`source_deliberation_id: 054-DL`, so `ship` tries to stamp the merge commit onto
the **already-archived** deliberation, and the archived-artifact write guard
rejects the ship writer's payload (it does not carry `archived_from`). The
shipment's final relocation (queue→archive) never ran.

**Resolution**: completed the relocation with `backlogit archive 106-S`, which
moved the shipment to `.backlogit/archive/106-S.md` with `status: archived` +
`archived_from` + commit — matching the canonical shipped-shipment end-state
(e.g. `001-S`…`105-S`). Stamping the merge commit onto the archived deliberation
is non-essential (054-DL is properly archived). **Follow-up worth filing
upstream**: `shipment ship` should either skip already-archived linked
deliberations or route their traceability write through the archive operation.

## Review Summary (pre-merge, PR #300)

Non-converging Copilot reviewer (finding counts 4→2→3→4→5→5→0 across post-pivot
cycles). Applied the operator hard cap: fix genuine bugs / fail-open gate-guards;
accept pure doc-nits as backlog.

- **Fixed** (correctness / fail-open guards): MD025 `Len==1` key assertion;
  `actions/setup-node@` prefix + 40-hex SHA; exact `make md-lint` line match;
  `node-version == "22"` guard; and the final cycle's **`continue-on-error`
  fail-open guard** (`842617a3`) — the job/steps now must keep `continue-on-error`
  unset/false.
- **Accepted as backlog** (pure artifact-text accuracy): F2 task-trace gap
  (03EFBBAC) and the residual `blocking/required` + `gitignore`-corpus wording
  nits (C63AF32E).
- Terminal cycle-7 review landed **fresh at HEAD `842617a3` with 0 new threads**.
  §1.9 clean (0 pending request, latest review == HEAD, 0 unresolved, CI green,
  MERGEABLE/CLEAN). Operator approved and merged.

## Quality Gates (closure branch)

- markdownlint repo-wide (closure branch): **0 / 1841** — the 1,839 pre-closure
  corpus plus the two new closure docs (this memory file + the compound learning),
  both now covered by the gate
- `go test ./...` ✅ | `go vet ./...` ✅ | `golangci-lint run` ✅ | gofmt clean on
  touched files (pre-merge, `842617a3`)
- `backlogit doctor`: 1 **pre-existing, unrelated** issue only (`016.001-R`
  orphaned review) — not introduced by this closure

## Compound Learnings Produced

- NEW: `docs/compound/2026-07-26-markdownlint-frontmatter-title-double-count.md`
  — rule-scoped markdownlint config resolves the frontmatter-title/body-H1 double
  count: scope MD025's `front_matter_title` to a sentinel key, keep MD041 default;
  guard the gate against fail-open regression (continue-on-error, node version,
  substring look-alikes). Cites `.markdownlint.json` and the 250→21→0 progression.
- No existing compound entry covered markdownlint config — net-new, no
  consolidation/replacement needed (compound-refresh Phase 2: keep-all).

## Compact-Context Summary

- Ran compact-context assessment (mandatory step). `docs/memory/`: **28 files,
  117 KB** (29 after adding this memory) — under all thresholds (max_files=40,
  max_size=500 KB). Only dated dirs are 2026-07-22/23 (3–4 days old), far under
  the 14-day threshold. **Phase 2 candidates: 0.** No compaction performed this
  cycle; nothing archived.

## Next-Cycle Items (follow-up stashes recorded)

| Stash | Kind / Priority | Status | Notes |
|---|---|---|---|
| 918BCDAF | task / medium | queued | Promote the repo-wide `md-lint` job to a branch-protection **required check** (admin action) |
| 03EFBBAC | task / medium | queued | Add a dedicated 21-file remediation task (e.g. 126.006-T) + shipment membership; reconcile plan↔task unit numbering |
| C63AF32E | task / low | queued | Residual doc-wording accuracy: `blocking/required` vs `hard-fails CI`/`required-check` across deliberation/`126.004-T`/`126-F`, and `gitignore`-corpus phrasing in archived `054-DL` |
| 7F0A6E89 | upstream template | queued | Pre-existing — `SKILL.md.tmpl` template parity (external autoharness repo) |
| (tool) | upstream | note | `shipment ship` traceability write fails on already-archived linked deliberations — route via archive op or skip |
