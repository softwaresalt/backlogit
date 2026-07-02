---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 073-S — create/update write-path nil-HeaderDef hardening (feat/073-artifacts-write-nil-headerdef). PASS: a source-built backlogit binary confirms the loaded (schema-present) create and update paths are non-regressed (backlogit add feature and backlogit update title both exit 0); the new fail-closed branch (nil ws.HeaderDef in CreateArtifact/UpdateArtifact returning a blerrors.ErrConfig-wrapped error → MCP internal / non-zero CLI exit) is unreachable via the normal CLI by design because config.WriteDefaults repopulates ws.HeaderDef at init, and is verified directly by TestCreateArtifact_NilHeaderDefFailsClosed and TestUpdateArtifact_NilHeaderDefFailsClosed (red before impl, green after) with loaded-HeaderDef regression guards and no-persisted-mutation assertions; full go test/vet/golangci-lint suite and changed-file gofmt clean.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-runtime-verification.md
title: 073-S create/update nil-HeaderDef — Pre-Merge Runtime Verification
---

# Runtime Verification — 073-S create/update write-path nil-HeaderDef hardening

- **Date**: 2026-07-02
- **Shipment**: `073-S` · Feature `073-F` · Task `073.001-T`
- **Branch**: `feat/073-artifacts-write-nil-headerdef`
- **Surface**: `cli` (also structurally `mcp`) · **Mode**: manual (source build) + unit
- **Verdict**: **PASS**

## Affected runtime surfaces

The change modifies the shared `core.CreateArtifact` and `core.UpdateArtifact`, which both surfaces
route through:

- CLI `backlogit add` / `update` / `move` — process exit code
- MCP `backlogit_create_item` / `backlogit_update_item` — error payload `type`

Only the (normally unreachable) nil-`HeaderDef` case changes: it moves from "succeed silently
unvalidated" to "fail closed" (an error wrapping `blerrors.ErrConfig` → MCP `internal` / non-zero
CLI exit). The schema-present create/update paths are unchanged.

## Environment prechecks

- Global `C:\Tools\backlogit.exe` predates this change — cannot exercise the new behavior. Built the
  binary from source (`go build -o bin/backlogit-073.exe ./cmd/backlogit`, exit 0) and ran the
  source binary for the CLI non-regression checks below. Temp binary removed after verification (not
  committed).

## Scenarios executed (source build, loaded workspace)

| Scenario | Command | Expected | Observed | Result |
|---|---|---|---|---|
| Init (schema present) | `backlogit init` | workspace initialized, `header-def.yaml` written | initialized | ✅ |
| Create (non-regression) | `backlogit add --type feature --title "Smoke feature"` | success, exit 0 | `Created feature: 001-F`, exit 0 | ✅ |
| Update (non-regression) | `backlogit update 001-F --title "Renamed ok"` | success, exit 0 | `Updated 001-F`, exit 0 | ✅ |

The loaded (schema-present) create/update paths are confirmed non-regressed at runtime.

## New fail-closed branch (nil HeaderDef → ErrConfig / non-zero exit)

- **Unreachable via normal CLI by design**: `config.WriteDefaults` (invoked at CLI init) always
  writes `header-def.yaml`, so `config.LoadHeaderDef` repopulates `ws.HeaderDef`; a real workspace
  never reaches the nil branch. This matches the plan's Reachability analysis (identical profile to
  072-S).
- **Verified directly by unit tests** (force `ws.HeaderDef = nil` after a loaded create):
  - `TestCreateArtifact_NilHeaderDefFailsClosed` — asserts `errors.Is(err, blerrors.ErrConfig)` is
    true, `errors.Is(err, blerrors.ErrValidation)` is false, the returned artifact is nil, and no
    artifact file is persisted (fail closed before persist).
  - `TestUpdateArtifact_NilHeaderDefFailsClosed` — same fail-closed error shape, plus the on-disk
    artifact bytes are unchanged (mutation refused before persist). Includes a loaded-HeaderDef
    update regression that still succeeds and persists.
  - Both are red before the impl (old `if ws.HeaderDef != nil` guard makes the write succeed) and
    green after.

## Load-bearing ordering confirmed

`requireHeaderDef(ws)` runs **before** `ApplyFieldDefaults`/`ValidateArtifactFields` at both call
sites, so a nil `HeaderDef` never reaches `headerDef.ResolveFieldSchema` (which dereferences its
receiver with no nil-guard). No nil-pointer panic path exists; verified by report-only code review.

## Evidence

- `go test ./...` PASS (all packages incl. cli/mcp/contract/integration); `go vet ./...` PASS;
  `golangci-lint run` PASS (exit 0); changed files gofmt-clean (LF-normalized).
- Report-only code review: **APPROVE — no P0/P1**.
- CI on the PR: pending (to be confirmed green on this HEAD before merge readiness is asserted).

## Follow-up recommendations

- No monitoring or rollback trigger required — zero-blast-radius defensive edge fix; the unit tests
  are the durable regression guard.
- No new out-of-scope follow-up: `internal/core/artifacts.go` was the third and last recurrence
  site named in the compound learning
  (`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`). The other
  nil-HeaderDef seams (`validateSizeValue`, `doctor_target.go`, `metadata_catalog.go`) already fail
  closed. The plan's deferred advisories (distinct agent-facing MCP error type; MCP tool-description
  enrichment) are recorded in the plan Decisions/Risks as YAGNI, not stashed.

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: CLI create/update exit 0 on the loaded path (source build); the nil branch via
  unit tests; MCP inherits structurally via the shared functions.
- BLOCKED prerequisites: none
- Risky action state: none (fail-closed, non-destructive — refuses a write; reversible via revert)
- Follow-up: none new
