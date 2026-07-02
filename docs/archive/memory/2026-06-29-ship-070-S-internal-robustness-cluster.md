---
chunk_strategy: h1-h2-h3
description: Ship session memory for shipment 070-S (internal robustness cluster) — three TDD tasks complete, branch pushed, PR pending, awaiting merge approval
doc_type: memory
ingested_at: "2026-06-29T20:20:00Z"
schema_version: "1.0"
source: docs/memory/2026-06-29-ship-070-S-internal-robustness-cluster.md
title: Ship 070-S — internal robustness cluster
---

## Shipment

070-S "Internal robustness cluster: create-scan batching, db-log DI,
ValidateFields empty-vs-absent" — items 070-F (feature), 070.001-T, 070.002-T,
070.003-T. Branch: `feat/070-internal-robustness-cluster`.

## Items completed (TDD, all done)

- **070.001-T** Batch canonical-uniqueness scan for bulk CreateArtifact callers —
  commit `e4fefd3`. `scanCanonicalArtifacts` walked+parsed every queue/archive `.md`
  on each `CreateArtifact` call, so bulk callers (migrate external-import loop, stash
  harvest) were O(files) per create → O(N²). New `CanonicalCache` (refs map) scans
  once per batch and records freshly-created IDs so within-batch collisions are still
  caught. `WithCanonicalCache` create option; `scanCanonicalArtifactsFn` package var
  is the test seam (counts total scans). Wired both bulk callers
  (`importMigrationItems`, `HarvestStashByPriority` → `harvestStashEntryLocked`);
  single interactive creates unchanged (nil cache → scan per call). Cache scoped to
  one sequential batch, NOT concurrency-safe (documented); harvest holds the stash
  lock, migrate import loop is sequential. Per-call destination-path stat guard in
  CreateArtifact retained as defense-in-depth. YAGNI honored — no caching
  infrastructure beyond batch scope. Stayed within ~2h; no split needed.

- **070.002-T** Dependency-inject `*slog.Logger` into `internal/db` Rehydrate /
  warnOnDuplicateSourceIDs — commit `60753ae`. Variadic `RehydrateOption` +
  `WithLogger` + `newRehydrateConfig` (defaults to `slog.Default()`), preserving all
  ~70 existing 3-arg callers unchanged. Logger threaded through the Rehydrate body
  slog calls and `warnOnDuplicateSourceIDs` (now takes a logger param). `root.go`
  sync command passes `db.WithLogger(slog.Default())`. Tests capture log output via
  the injected logger only — no `slog.SetDefault` mutation (acceptance criterion).
  `applyLogLevel`'s `slog.SetDefault` wiring (root.go) intentionally KEPT (global CLI
  log-level config, out of scope).

- **070.003-T** Explicit empty-string-vs-absent-key distinction in ValidateFields
  minLength parity — commit `1ab8f0d`. `BaseFrontmatter` gains unexported
  `present map[string]bool`, populated by `FromMap` from source frontmatter keys
  (captured BEFORE defaults are applied, so a defaulted chunk_strategy/schema_version
  is not mistaken for present). ValidateFields minLength loop: present+blank →
  min_length violation; absent optional key → no violation; presence unknown (struct
  literal, not via FromMap) → preserves historical whitespace-only-only behavior, so
  direct callers are unaffected. Source:
  docs/closure/2026-06-28-069-S-docline-doctor-hardening-closure.md. Compound:
  empty-string-vs-sentinel-in-classification-2026-05-09. Note: task named
  `internal/core/fields.go` as a surface, but its `ValidateFields` is a different
  (enum/int custom-field) validator with no minLength — assessed NO-CHANGE; the real
  fix is entirely in `internal/docline` (frontmatter.go + validate.go).

## Quality gates (all GREEN, repo-wide)

- `go test ./...` — pass (incl. contract + integration suites).
- `go vet ./...` — clean.
- `golangci-lint run --timeout=5m ./...` (v1.64.8, matches CI) — zero findings.
- gofmt — clean on all 11 branch-changed `.go` files (verified on LF-normalized
  content; working tree is CRLF via core.autocrlf=true, so `gofmt -l .` flags every
  file — a false positive; committed blobs are LF and CI/Linux passes).

## Runtime verification (built binary, smoke-tested in isolated temp workspace)

- Task 1: bulk `add --type feature` ×3 → unique canonical IDs 001-F/002-F/003-F;
  `sync` clean (no spurious duplicate warnings).
- Task 2: `sync` rehydrate path logs through the wired logger (INFO output observed).
- Task 3: `docs lint --profile authoring` flags a present-but-empty `ingested_at: ""`
  with a `min_length` violation, and does NOT flag a doc that omits `ingested_at` —
  end-to-end confirmation of the empty-vs-absent parity.

## Decisions

- Used backlogit CLI throughout (MCP tools not exposed as function calls this
  session) — TOOL_DEGRADED with documented CLI fallbacks. INDEX_SYNC_OK.
- Moving items to `done` auto-archived them (queue → `.backlogit/archive/`); shipment
  070-S remains `active` (claimed) pre-merge. Shipment `ship` (manifest archival +
  merge SHA) deferred to post-merge closure (Step 6), per pipeline.

## Working-tree hygiene (NON-NEGOTIABLE — honored)

Left untouched/uncommitted throughout: modified `.github/agents/auto-mergeinstall.agent.md`,
`.github/agents/auto-tune.agent.md`, `.gitignore`; untracked `.cursor/`,
`.github/agents/.ship.agent.md`, `.github/agents/.stage.agent.md`,
`.github/agents/_orchestrator.agent.md`, `.github/copilot/`. Every commit used
explicit `git add <path>` — never `git add -A`/`.`.

## Commit SHAs (tracked via backlogit)

- 070.001-T → `e4fefd3b769113f687ce153fd8d9ef34df3a2a93`
- 070.002-T → `60753ae51c9c469ac5b76927fd08dd27ff2e6794`
- 070.003-T → `1ab8f0de6a29b0b2c5ee91e55a78501aedfe84ea`

## Next steps

Push `feat/070-internal-robustness-cluster`; open PR (MERGE COMMIT strategy only,
P-009); request Copilot review; resolve bot threads; run §1.9 pre-merge readiness
GraphQL gate; monitor CI green. **HALT before merge** — present PR merge-ready,
awaiting explicit operator approval (P-014). Do NOT merge.
