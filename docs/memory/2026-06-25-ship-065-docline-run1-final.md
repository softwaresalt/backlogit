---
title: "Ship 065-S docline frontmatter — run 1 final session memory"
doc_type: learning
source: docs/memory/2026-06-25-ship-065-docline-run1-final.md
description: "Ship run-1 closure state for shipment 065-S: tooling stack complete, gated tasks deferred, PR awaiting operator sign-off + merge."
---

## Ship 065-S — Run 1 Final Session Memory

**Date:** 2026-06-25
**Agent:** Ship
**Shipment:** `065-S` — Standardize documentation frontmatter on docline base schema
**Feature:** `065-F`
**Branch:** `feat/065-docline-frontmatter` (base `5b34ed1d` off `main`)
**Mode:** RUN 1 of 2 (tooling stack only; bulk migration + CI gate deferred to RUN 2 pending operator sign-off)

## Outcome

Run-1 scope (the 8 non-gated tooling tasks) is **complete, green, reviewed, and committed**.
PR is being taken to merge-ready. **HALT for operator** on (a) the 065.002-T Q1/Q2 policy
sign-off and (b) merge approval. Do NOT merge; do NOT run T9/T10.

## Per-task status

| Task | Title | Status | Commit |
|---|---|---|---|
| 065.001-T | Taxonomy/field-mapping decision doc | ✅ done | `32a03057` |
| 065.002-T | **Operator policy sign-off gate** | ⛔ active (HUMAN GATE — deferred to run 2) | — |
| 065.003-T | Body-preserving codec | ✅ done | `023d1bce` |
| 065.004-T | BaseFrontmatter model + validator | ✅ done | `df3087a5` |
| 065.005-T | Classifier + idempotent normalizer | ✅ done | `07bfc6b4` |
| 065.006-T | Application service (lint/plan/apply) | ✅ done | `c0dac99e` |
| 065.007-T | `backlogit docs` CLI adapter | ✅ done | `b6ead62c` |
| 065.008-T | MCP parity tools | ✅ done | `790909d3` |
| 065.009-T | **Bulk migrate** | ⛔ active (depends on 002 — deferred to run 2) | — |
| 065.010-T | **CI enforcement gate** | ⛔ active (depends on 009 — deferred to run 2) | — |
| 065.011-T | Authoring guide + ARCHITECTURE/AGENTS | ✅ done | `e93055e8` |

8/11 done. 3 gated tasks remain `active` under the still-active shipment 065-S (run 2
will build them on this same branch). They were verified NOT done.

## Review remediation (this session)

Adversarial review (3 reviewers, report-only) found **no HIGH-confidence consensus
P0/P1 blockers**. Body-preservation and idempotency invariants confirmed sound.
MEDIUM findings were fixed on-branch in commit `68640b7f`:

- **C1** — `atomicWrite` now preserves the original file mode (0644 default for inserts).
- **M1/M2** — whole-tree apply bypass closed via shared `docline.ValidateApplyPath`
  guard (`ErrWholeTreeApply`) wired into both CLI and MCP apply boundaries; rejects
  empty + root-equivalent scopes (`.`, `docs/..`).
- **M3/M4** — move-never-drop hardened: scalar `docline:` value preserved; colliding
  fold keys preserved under `<k>_topN` instead of overwriting. Both idempotent.

LOW findings **L1–L4 deferred** to follow-up stashes for Stage:
`0615F487` (L1 zero-write preflight), `B349CBED` (L2 full-schema validation),
`A2436E1E` (L3 dry-run flag), `AE53BC5C` (L4 TOCTOU re-read).

Closure report + §7 remediation table: `docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md` (committed `28b5afd8`).

## Quality gates (all GREEN)

- `go test ./...` ✅ (all packages incl. contract + integration)
- `go vet ./...` ✅
- `golangci-lint run ./...` ✅ (0 issues)
- gofmt LF-blob check on all changed `.go` files ✅ (CLEAN — authoritative check, CRLF noise ignored)

## ⛔ OPERATOR GATE — 065.002-T (surface at HALT)

Two open policy questions must be answered by the operator before RUN 2 bulk migration:

- **Q1 — `ingested_at` ownership.** Recommended: **seed-once at migration** (normalizer
  preserves any existing non-empty value; idempotent). Alternative: fully pipeline-owned.
- **Q2 — `source` convention.** Recommended: **repo-relative POSIX path** (e.g.
  `docs/decisions/x.md`). Question for operator: is that acceptable to graphtor-docs, or is
  a full origin URI required?

Recommendations are from the 065.001-T decision doc
(`docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md`), which reserves a
"Policy Decisions (operator-confirmed)" section for the run-2 sign-off.

## Next steps (RUN 2, same branch, after sign-off)

1. Record operator Q1/Q2 answers in the 065.001-T decision doc.
2. Build 065.009-T (bulk migrate, ≤25-file batches, reviewable) → done.
3. Build 065.010-T (CI gate + Makefile target) → done.
4. Move 065.002-T → done (gate satisfied). Close/ship shipment 065-S with merge SHA.
5. Post-merge closure protocol (archive, knowledge graduation, compound-refresh, compact-context).

## Operational notes

- agent-intercom DEGRADED — `[SHIP]` broadcasts logged to session output only.
- backlogit MCP not in function set → CLI mode via `.\backlogit.exe`. Mutations STRICTLY
  sequential (SQLite lock deadlock risk); ONE command per terminal call.
- gofmt gate: use LF-blob check (`git show ":<file>" | gofmt -d`), not raw `gofmt -l .`
  (repo has `core.autocrlf=true`, no `.gitattributes` → CRLF noise).
- Stay on `feat/065-docline-frontmatter` through merge approval — never checkout `main`.
- Merge strategy MUST be merge commit (P-009); operator performs the admin merge.

## Run-1 review remediation (Copilot cycles 2–7, all resolved)

After PR #136 was opened, Copilot review ran across successive HEADs. Every CODE
finding was fixed via TDD; the gated cli-reference policy items were deferred. Final
HEAD `2d3c3c90` has a fresh Copilot review covering it with **0 unresolved threads**;
CI green (test 1.23, test 1.24, CLI Reference Drift).

- `68640b7f` — adversarial-review MEDIUM fixes: apply gate (`ErrWholeTreeApply`), fold
  move-never-drop, atomic write mode preservation.
- `ac1083c5` — cross-platform POSIX path normalization (`toPOSIX`); fixed a Linux-only
  CI failure (`filepath.ToSlash` does not convert `\` on Linux).
- `887522ad` — Windows rename retry, profile validation (`ParseProfile`), removed the
  decorative `--dry-run` flag from `docs migrate` (addresses stash `A2436E1E`).
- `18bbe121` — `collectInScopeDocs` skips excluded subtrees via `filepath.SkipDir`;
  markdown H1→`##` (MD025) on three docs.
- `a366bd3d` — `ApplyMigration` preflight validates all changes before any write
  (all-or-nothing); dropped a dead `errors.Is` test guard.
- `f82d9f57` — walk skips all non-`docs/` top-level dirs; removed the unused `profile`
  arg from the `backlogit_docs_migrate` MCP tool (CLI/MCP parity).
- `727edeb5` — `Normalize` fails fast (`ErrMissingSeedTime`) when `ingested_at` needs
  seeding but `Now` is zero (defensive; Q1 ownership remains gated).
- `582332b0` — `resolveFormat` uses the command writer for TTY detection + validates
  `--format` (fail fast on typos).
- `2d3c3c90` — removed redundant `RegisterTools()` double-registration in an MCP test.

### Gated cli-reference policy item (run 2)

Copilot repeatedly flagged that generated `docs/cli-reference/**` (from `cmd/gen-docs`)
carries only `title`/`description` and would fail `backlogit docs lint`. This cannot be
resolved in run 1 without pre-empting the gated Q2 `source`-format decision or making a
Stage-level scope-exclusion call. Resolved-with-justification on the PR and tracked in
stash `98C4F063` (related `E4B7767C`). It will re-surface on future reviews until run-2
migration or a scope decision addresses it.

### Follow-up stashes open for Stage triage

`0615F487` (L1 zero-write preflight — partially addressed by the `a366bd3d` preflight),
`B349CBED` (L2 full JSON-schema validation), `A2436E1E` (L3 `--dry-run` — DONE this run,
flag removed), `AE53BC5C` (L4 apply-time TOCTOU re-read), `E4B7767C` (run-2 gen-docs vs
docline scope conflict), `98C4F063` (cli-reference docline policy).
