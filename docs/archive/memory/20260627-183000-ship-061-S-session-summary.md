---
title: "Ship session summary — 061-S Metadata and Section Sync Integrity"
date: 2026-06-27
agent: ship
shipment: 061-S
feature: 062-F
branch: feat/062-metadata-section-sync-integrity
status: pr-ready (awaiting operator merge)
---

# Ship session summary: 061-S (feature 062-F)

## Scope

Shipment `061-S` "Metadata and Section Sync Integrity" carrying feature `062-F`
(same title) plus 5 independent bugfix tasks. The `061-S -> 062-F` offset is a
known-benign ID-numbering artifact (precedent `057-S->058-F`, `060-S->061-F`);
no manifest re-alignment performed.

## Items completed

All 5 tasks implemented via TDD (red harness verified failing, then green) and
moved to `done`:

| Task | Title | Commit | 2026-05-22 bug still reproduced on current main? |
|------|-------|--------|--------------------------------------------------|
| 062.001-T | Restore CLI metadata parity in MCP | `2e5c74a6` | YES — `mcp/metadata.go` passed `nil` cliRoot so MCP catalog omitted CLI command data |
| 062.002-T | Align export-command-map workspace root | `b89d2c14` | YES — used `s.backlogitDir()` instead of workspace root for path + containment |
| 062.003-T | Re-upsert rewritten section bodies | `70731377` | YES (MCP path) — `mcp/tools.go writeSectionsToFile` never re-upserted DB/FTS; CLI path already did |
| 062.004-T | Stop CLI section corruption fallback | `eb1c9189` | YES (CLI path) — `cli/update.go` blanket-appended on ANY WriteSections error, masking malformed markers + duplicating sections |
| 062.005-T | Enforce MergeSync dry-run purity | `b4c14dca` | YES — `db/merge_sync.go` fallback rehydrate ran BEFORE the dryRun guard |

Review-hardening commit `2f245840` (fix(cli): validate section names and unify
CLI/MCP section-write guards) added on top, addressing a P2 review finding:
consolidated section-name validation into `parser.ValidateSectionName` (single
source of truth) used by BOTH cli and mcp write paths; hardened the CLI append
to fire only on `parser.ErrSectionNotFound`; symmetric `.tmp` cleanup on
WriteFile-error branches.

## Divergences from the stale 2026-05-22 plan

- 062.003: the defect was on the **MCP** write path (`writeSectionsToFile`), not
  the CLI path the plan implied — the CLI path already re-upserted. Fix targets
  the MCP path and a regression test locks it.
- 062.004: the corruption was specifically the **CLI** `update.go` blanket-append
  on any error. Fix introduces parser sentinels (`ErrSectionNotFound` /
  `ErrSectionMalformed`) and per-section classification.
- 062.001: solved the mcp->cli import cycle via dependency injection
  (`Server.CLICommandProvider func() []core.CommandInfo`, wired by cli's
  `wireMCPMetadataProvider`); mcp stays cobra-free.

## Quality gates (all green)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `golangci-lint run ./...` — exit 0 (v1.64.8)
- `go test ./...` — all packages ok
- `gofmt` — clean on all 14 changed `.go` files (LF-normalized; local Windows
  CRLF noise filtered, committed blobs are LF)

## Decisions / rationale

- gofmt/CRLF: no `.gitattributes` for `.go`; autocrlf stores LF blobs. Validated
  real gofmt cleanliness by LF-normalizing each file to a temp copy before
  `gofmt -l`. CI (Linux/LF) only fails on genuine format issues.
- Folded the late gofmt struct-alignment fix into 062.001 via non-interactive
  autosquash (`GIT_SEQUENCE_EDITOR=true`) for clean history.
- Section validation belongs in `parser` (co-located with the BEGIN/END grammar
  it enforces), not duplicated per call site.

## Branch / backlog state

- Branch `feat/062-metadata-section-sync-integrity` off `main` @ `02f5603b`.
- Tasks `062.001-T..062.005-T` -> `done` (routed to `.backlogit/archive/`).
- Shipment `061-S` -> `active` (claimed); feature `062-F` -> `active`.
- 061-S / 062-F intentionally LEFT OPEN for post-merge closure.

## Next steps

1. Push branch, open PR (merge-commit strategy per P-009).
2. Request Copilot review; run docline gate if docs touched; drive CI green.
3. Resolve Copilot threads (reply + resolve via gh api graphql), max 3 cycles.
4. HALT at merge gate — operator does the admin merge.
5. Post-merge (SEPARATE run): ship `061-S`, archive `062-F`, knowledge graduation.

## Follow-up stashes

None identified during this run.
