---
chunk_strategy: h1-h2-h3
description: 'Post-merge runtime verification for shipment 075-S — surface the covering feature (id + title) in CLI and MCP shipment views (feat/075-covering-feature-display, PR #164, merge 842e888). PASS: a source-built backlogit binary confirms the read-only render-time projection end-to-end. Scenario A (covering feature present): shipment get emits a top-level covering_feature {id, title} sibling and the shipment list table renders a COVERING FEATURE column (001-F — Auth hardening); the object never appears inside custom_fields. Scenario B (zero-feature shipment): a task-only manifest omits covering_feature entirely on get and the table cell is blank. Scenario C (read-only invariant): three get + three list calls leave the manifest file bytes and the SQLite index byte-identical, and covering_feature is never persisted to the manifest. Backed by the shipped unit suites (internal/core, internal/cli, internal/mcp) that assert the same projection, omit, no-leak, and no-mutation invariants.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-075-S-covering-feature-display-runtime-verification.md
title: 075-S covering feature display — Post-Merge Runtime Verification
---

# Runtime Verification — 075-S surface covering feature in shipment views

- **Date**: 2026-07-02
- **Shipment**: `075-S` · Feature `075-F` · Tasks `075.001-T`, `075.002-T`, `075.003-T`
- **PR / merge**: #164 · merge commit `842e8883899ba25ce9c31840c89806ed2e032549`
- **Branch**: `feat/075-covering-feature-display` (verified from `post-merge/075-covering-feature-display`)
- **Surface**: `cli` + `mcp` · **Mode**: manual (source build) + unit
- **Verdict**: **PASS**

## Affected runtime surfaces

The change adds a read-only, render-time projection of a shipment's covering feature to
the shipment read paths shared by both surfaces:

- CLI `backlogit shipment get` (JSON) — top-level `covering_feature` object
- CLI `backlogit shipment list` (table) — a `COVERING FEATURE` column
- MCP `backlogit_get_shipment` / `backlogit_list_shipments` — the same top-level projection

Both surfaces marshal the single `core.ShipmentView` type
(`internal/core/shipment_covering.go`), so field name and omit semantics have one source
of truth. `DeriveCoveringFeature` resolves the root covering feature from the shipment
manifest (`custom_fields.items`) via a pure `bldb.GetItem` read — no `loadArtifact`, no
write path.

## Environment prechecks

- Global `C:\Tools\backlogit.exe` (v1.3.0, build date 2026-07-01) **predates** this change
  and cannot exercise the new field. Built the binary from source on the closure branch
  (`go build -o bin/backlogit-075.exe ./cmd/backlogit`, exit 0) and ran the source binary
  for all CLI scenarios below. Binary is `bin/`-gitignored and was removed after
  verification (not committed).
- Verification workspace: a throwaway `backlogit init` workspace under `%TEMP%` (removed
  after the run). Index synced (`backlogit sync`) after each mutation before reads, because
  create emits `index may be stale after mutation`.

## Scenarios executed (source build)

| # | Scenario | Command | Expected | Observed | Result |
|---|---|---|---|---|---|
| A | Covering feature present (JSON) | `shipment get 001-S` (manifest `[001-F]`) | top-level `covering_feature: {id:"001-F", title:"Auth hardening"}`; NOT in `custom_fields` | exactly that; `custom_fields` held only `items` | ✅ |
| A | Covering feature present (table) | `shipment list --format table` | `COVERING FEATURE` column shows `001-F — Auth hardening` | column rendered `001-F — Auth hardening` | ✅ |
| B | Zero-feature shipment (JSON) | `shipment get 002-S` (manifest `[001.001-T]`, a task) | `covering_feature` omitted entirely | field absent; only `custom_fields.items` present | ✅ |
| B | Zero-feature shipment (table) | `shipment list --format table` | `COVERING FEATURE` header present, cell blank | header present, cell blank for `002-S` | ✅ |
| C | Read-only invariant | 3× `shipment get 001-S` + 3× `shipment list` | manifest file + index byte-identical; no persisted `covering_feature` | manifest SHA `2FCE08DF…` and DB SHA `ABF04750…` unchanged; `covering_feature` absent from manifest file | ✅ |

## Load-bearing invariants confirmed at runtime

- **Read-only**: `covering_feature` is a top-level sibling with `omitempty`; it is never
  written into `custom_fields` and never persisted — the retroactive manifest mutation the
  B8FF7590 determination forbids does not occur. Manifest + index hashes are stable across
  repeated reads.
- **Zero-feature ⟹ omitted**: a manifest with no dotless root feature yields a nil pointer,
  which `omitempty` drops on every surface; the table renders an empty cell under a present
  header.
- **CLI == MCP parity**: both surfaces go through the shared `core.ShipmentView` shaper, so
  the field name (`covering_feature`) and omit behavior are identical (also guarded by the
  shipped `internal/mcp` parity tests).

## Evidence

- Whole-suite gates from the build session (memory checkpoint
  `docs/memory/2026-07-02-075-S-task3-mcp-checkpoint.md`): `go test ./...` PASS,
  `go vet ./...` PASS, `golangci-lint run` PASS (exit 0), changed files gofmt-clean.
- Shipped unit tests assert the same invariants exercised here:
  `internal/cli` (`COVERING FEATURE` column; top-level `covering_feature`; zero-feature
  omit; no-persist), `internal/core` (`DeriveCoveringFeature`, `NewShipmentView`,
  `NormalizeShipmentItems`), `internal/mcp` (`shipment_covering_test.go`: projection on
  get + list, items-never-null, no `custom_fields` leak, read-only regression, CLI==MCP
  parity).
- Report-only code review during build: 0 P0/P1/P2/P3.
- CI on PR #164 at HEAD `e94ca3e`: **4/4 green** (`test (1.23)`, `test (1.24)` required,
  `CLI Reference Drift`, `Docline frontmatter gate`).

## Copilot review note

The single Copilot thread on PR #164 — `DeriveCoveringFeature` guards `ws==nil` but not
`ws.DB==nil`, so a `Workspace{}` without a DB could panic in `bldb.GetItem` — was resolved
before merge (thread `isResolved: true`). The fix aligns with the existing fail-safe-on-nil
compound learning
(`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`): guard the
nil precondition and fail closed (`ok=false`) rather than dereference.

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: CLI `shipment get` (JSON top-level field) + `shipment list` (table
  column) via source build; MCP inherits structurally through the shared `core.ShipmentView`
  shaper and is covered by `internal/mcp` unit tests.
- BLOCKED prerequisites: none
- Risky action state: none — pure read-only projection; no persistence, no destructive path
- Follow-up recommendations: none new. The one tech-debt item surfaced during plan-review
  (consolidate the duplicate MCP `normalizeShipmentItems` into a single exported
  `core.NormalizeShipmentItems`) is already stashed as `17D29DDC` for Stage.
