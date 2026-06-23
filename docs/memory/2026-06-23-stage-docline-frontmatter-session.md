---
title: "Stage session — docline frontmatter standardization (065-F)"
description: "Session-end checkpoint: stash 29A71E9C harvested into feature 065-F + 11 tasks + shipment 065-S."
date: 2026-06-23
doc_type: memory
status: complete
stash_ids: [29A71E9C]
feature_id: 065-F
shipment_id: 065-S
---

## Outcome
Stash entry `29A71E9C` (high, feature) was taken through the full Stage pipeline and
harvested into a QUEUED backlog feature with a decomposed, dependency-wired task graph
and an assembled shipment ready for Ship.

## Artifacts produced
- Deliberation: `docs/decisions/2026-06-22-docline-frontmatter-standardization-deliberation.md`
  (depth=deep, decided, recommends Option B; resolves all 6 design questions)
- Plan: `docs/exec-plans/2026-06-22-docline-frontmatter-standardization-plan.md`
  (11 units T1–T11; Constitution Check; Plan Hardening (required=yes); Plan Review attempt 1 FAIL → attempt 2 PASS)

## Backlog created
- Feature: `065-F` "Standardize documentation frontmatter on docline base schema" (status: queued)
- Tasks (parent 065-F):
  - `065.001-T` T1 — taxonomy/field-mapping/profile-split decision doc (docs)
  - `065.002-T` T2 — operator policy sign-off gate (decision; blocks T9)
  - `065.003-T` T3 — neutral body-preserving frontmatter codec (Go, TDD)
  - `065.004-T` T4 — policy-as-code BaseFrontmatter model + validator (Go, TDD)
  - `065.005-T` T5 — classifier + idempotent normalizer (Go, TDD)
  - `065.006-T` T6 — docline application service lint/plan/apply + SafeResolve (Go, TDD)
  - `065.007-T` T7 — backlogit docs CLI adapter (Go, TDD)
  - `065.008-T` T8 — MCP parity tools (Go/MCP, TDD)
  - `065.009-T` T9 — bulk migrate in-scope docs in <=25-file batches (content)
  - `065.010-T` T10 — CI gate + Makefile + negative smoke (config/CI)
  - `065.011-T` T11 — authoring guide + ARCHITECTURE update (docs)
- Shipment: `065-S` (status: queued) — 12 items, feature-first topological order

## Dependency edges (blocks; durable in file frontmatter)
T2←T1; T4←T1,T2,T3; T5←T3,T4; T6←T4,T5; T7←T6; T8←T6,T7; T9←T2,T7; T10←T7,T9; T11←T1,T7.
Critical path: T1→T2→T4→T5→T6→T7→T9→T10. No cycles.

## Key mechanics learned (durable)
- `backlogit dep add` (via `core.AddDependency`, `internal/core/dependencies.go`) persists each
  edge to BOTH the index `item_deps` table AND the source task file's `dependencies:` YAML
  frontmatter list, so the edge survives `backlogit sync` / rehydration. No manual frontmatter
  edit is required.
- `backlogit add --section name=value` only honors `description`; acceptance criteria must be
  added via `backlogit update <id> --section "Acceptance Criteria=..."` (managed section markers).
- `backlogit shipment create --items a,b,c` accepts the full membership at create time
  (parent-first); there is no `add_to_shipment` CLI in this environment.
- `backlogit stash archive <id>` archives a consumed entry; no promotion-target flag, so the
  forward reference was appended to the entry text via `stash edit` before archiving.

## Design decisions (proposed defaults; Q1 `ingested_at` / Q2 `source` ownership pending T2 operator sign-off)
1. Scope = `docs/**` minus `docs/memory/**` & `docs/archive/**`, plus README/AGENTS/ARCHITECTURE;
   exclude `.github/**` (autoharness-generated, conflicting prompt-artifact schema) and `prompt.md`.
2. Authoring/ingestion profile split: repo authors {title, doc_type, source, description}; pipeline
   owns {content_sha256, source_path, authoritative ingested_at}. `source` = repo-relative POSIX path;
   `ingested_at` seeded once at migration.
3. doc_type vocabulary closed: reference, decision, spike, plan, closure, research, review, learning,
   spec, design, guide. Legacy category fields fold under the `docline` namespace.
4. Durable native `backlogit docs lint`/`migrate` (CLI+MCP), no throwaway script.
5. Go-native CI gate via `backlogit docs lint`.
6. Idempotent, batched per-subdir, git-reversible migration.

## Open operator questions (do NOT block harvest; gate T9 via T2)
- Q1: `ingested_at` seed-once at migration vs pipeline-owned authoritative value.
- Q2: `source` repo-relative POSIX path convention acceptable to graphtor-docs ingestion.

## Deferred stash (NOT consumed)
- `21E17BFC` (low, feature) singleton-MCP
- `71A2CB10` (low, task) compact-memory
- `0F65FBC9` (high, bug) ID-conflict-bug

## Status
Pipeline complete through Step 6. Feature `065-F` is QUEUED; shipment `065-S` is the handoff
token to Ship. Stage owns nothing further here.
