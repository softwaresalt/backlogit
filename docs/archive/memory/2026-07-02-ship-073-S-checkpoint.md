# Ship checkpoint — 073-S create/update write-path nil-HeaderDef hardening

- **Date**: 2026-07-02
- **Agent**: Ship
- **Shipment**: `073-S` — "create/update write paths: fail closed on nil header-def"
- **Feature**: `073-F` · **Task**: `073.001-T` (single)
- **Branch**: `feat/073-artifacts-write-nil-headerdef` (off local `main` @ `1853043` Stage harvest commit)
- **Plan**: `docs/exec-plans/2026-07-02-artifacts-write-nil-headerdef-hardening-plan.md`

## Items completed

- `073.001-T` — **done**, feature commit `fbfe738`.
  - `internal/core/artifacts.go`: added shared helper `requireHeaderDef(ws *Workspace) error`
    (returns `fmt.Errorf("header definition not loaded; cannot validate artifact fields: %w",
    blerrors.ErrConfig)` when `ws.HeaderDef == nil`). Rewired `CreateArtifact` (~line 224) and
    `UpdateArtifact` (~line 514): replaced the fail-open `if ws.HeaderDef != nil` guards with a
    fail-closed `requireHeaderDef` call placed **before** `ApplyFieldDefaults`/`ValidateArtifactFields`
    (load-bearing ordering — `headerDef.ResolveFieldSchema` nil-derefs its receiver).
  - `internal/core/artifacts_headerdef_test.go` (new, `package core_test`): TDD red→green.
    `TestCreateArtifact_NilHeaderDefFailsClosed` (ErrConfig not ErrValidation, nil artifact, no file
    persisted, + loaded regression) and `TestUpdateArtifact_NilHeaderDefFailsClosed` (same
    fail-closed shape + on-disk bytes unchanged + loaded-update regression). Covers AC1–AC5.
- Shipment `073-S`: claimed → **active**; feature `073-F` → **active** (cascade); task queue file
  relocated to `.backlogit/archive/073.001-T.md` on `done` per registry routing.

## Decisions (with rationale)

- **`blerrors.ErrConfig`-wrap (→ MCP `internal`/500, non-zero CLI exit), NOT `ErrValidation`**: a
  nil workspace `HeaderDef` is a system/config precondition fault, not a user-correctable field
  error. Matches the plan Decision (Option B), the local `validateSizeValue`/`NewMetadataCatalog`
  convention, and the 072-S doctor sibling.
- **One shared helper, one task**: both call sites are the same fail-open shape; CLI + MCP inherit
  the fix structurally through the shared core functions.
- **Fail closed before `ApplyFieldDefaults` on create**: load-bearing to prevent a nil-panic in
  `ResolveFieldSchema` and to refuse the write before any field mutation/persist.

## Quality gates (full mandated set, in order)

- `go test ./...` → **PASS** (all packages incl. cli/mcp/contract/integration)
- `go vet ./...` → **PASS** (exit 0)
- `golangci-lint run` → **PASS** (exit 0)
- `gofmt -l .` → **PASS for change** (my 2 files LF-normalized clean, verified exit 0; whole-tree
  CRLF flags are a pre-existing Windows working-tree artifact affecting all tracked `.go` files;
  git stores LF via `autocrlf=true`, so CI gofmt on LF is clean)

## Review gate

- code-review (report-only): **APPROVE — no P0/P1**. Confirmed the ordering is correct at both
  sites, no remaining nil path reaches `ResolveFieldSchema`, error maps to `internal` not
  `validation_failed`, and the tests are genuine red-before/green-after regression guards.

## Runtime verification

- **PASS** — source build (`go build ./cmd/backlogit`, exit 0): loaded-path `add feature` (exit 0)
  and `update --title` (exit 0) non-regressed; nil branch covered by the two new unit tests. Temp
  binary removed (not committed). Report:
  `docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-runtime-verification.md`.

## Operational closure

- **READY** — merge-only, zero-blast-radius; no monitoring/rollback. No new follow-up stash
  (artifacts.go was the third/last recurrence site). Deferred advisories recorded, not stashed.
  Report: `docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-closure.md`.

## Branch / PR state

- **Feature commit** `fbfe738` (code + 073 backlog state). **Docs commit** to follow (this
  checkpoint + closure/runtime-verification), forming the reviewable HEAD.
- Operator's unrelated in-flux files (`.github/agents/auto-*.agent.md`, `.cursor/`,
  `.github/copilot/`, `.gitignore`, `.github/agents/.*.agent.md`) were **preserved untouched and
  excluded** from all commits. `.backlogit/hooks_queue.jsonl` is gitignored (not committed).
- **P-014**: HALT at operator merge-approval gate — no self-merge.
- **P-009**: to verify at merge-readiness (`allow_merge_commit=true`, squash/rebase=false).

## Circuit-breaker counters

- build-feature attempts: 1/5 · review-fix cycles: 0/3 (so far) · fix-ci cycles: 0/5 · consecutive
  failures: 0/3

## Next steps

- Push branch → create PR → request Copilot review → poll CI + review → resolve any Copilot threads
  (cycle limit 3) → run §1.9 pre-merge Copilot readiness gate (fail-closed) → present PR as
  merge-ready and **HALT at P-014**.
- Post-merge (later Orchestrator-routed session, after operator approval): merge-confirm gate →
  `post-merge/073-artifacts-write-nil-headerdef` branch → `shipment ship 073-S --sha <merge>` →
  compound-refresh (third-instance reinforcement) → compact-context → closure PR.
