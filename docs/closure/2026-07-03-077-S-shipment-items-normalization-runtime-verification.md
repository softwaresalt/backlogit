---
chunk_strategy: h1-h2-h3
description: 'Post-merge runtime verification for shipment 077-S — consolidate the duplicated shipment-items read-edge normalization into a single exported core.NormalizeShipmentItems, delete the internal/mcp normalizeShipmentItems copy, and relocate the never-null JSON wire-shape invariant into the core function return contract (feat/077-shipment-items-normalization, PR #168, merge c848740). PASS: a behavior-preserving internal Go refactor validated by the whole-suite gates on the closure branch (go test ./... PASS, go vet ./... PASS, golangci-lint run exit 0) plus the shipped unit/integration suites. The moved core suite (internal/core/shipment_normalize_test.go) asserts the mapping across nil-map, missing-key, nil-raw, []string, []any, and the empty-[]string edge, and that the function NEVER returns nil; the retained MCP end-to-end guard (internal/mcp/shipment_response_test.go TestListShipments_EmptyItems_NeverNull) asserts backlogit_list_shipments still emits custom_fields.items as a JSON array (never null) for an empty shipment through the thin handleListShipments adapter that now delegates to core.NormalizeShipmentItems. No new runtime surface, telemetry, external contract, or persistence; the one nil-vs-empty divergence between the former pure reader and the former MCP mutator was reconciled to the never-null superset.'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-077-S-shipment-items-normalization-runtime-verification.md
title: 077-S shipment-items normalization — Post-Merge Runtime Verification
---

# Runtime Verification — 077-S consolidate shipment-items normalization

- **Date**: 2026-07-03
- **Shipment**: `077-S` · Feature `077-F` · Task `077.001-T`
- **PR / merge**: #168 · merge commit `c8487407d5ddb19d26c754ce82606df929e35f46`
- **Branch**: `feat/077-shipment-items-normalization` (verified from `post-merge/077-shipment-items-normalization`)
- **Surface**: `mcp` (and shared `core` read edge) · **Mode**: unit/integration + whole-suite gates
- **Verdict**: **PASS**

## Affected runtime surfaces

The change is a behavior-preserving internal Go refactor with no new external contract:

- `internal/core/shipment.go` — the pure `[]any`/`[]string` → `[]string` mapper was renamed
  `shipmentItems` → exported `NormalizeShipmentItems` and hardened so it **never returns nil**
  (nil artifact, nil `CustomFields`, missing `items` key, or nil raw all yield `[]string{}`;
  `[]string` is copied; `[]any` is filtered to strings). It is now the single source of truth.
- `internal/mcp/tools.go` — the duplicated inline `normalizeShipmentItems` mutator was **deleted**;
  `handleListShipments` is now a thin adapter that inits a nil map only as an assignment target and
  sets `custom_fields.items = core.NormalizeShipmentItems(shipment)`.
- Shared shipment read paths (`GetShipment` via `normalizeShipmentArtifact`,
  `shipment_covering.go`, `shipment_lifecycle.go`) inherit the single mapper by call-site rename.

MCP `backlogit_list_shipments` and CLI/MCP `shipment get` marshal the same normalized value; the
never-null wire-shape guarantee now lives in the core function's return contract rather than being
re-implemented at the MCP boundary.

## Verification approach

This is an internal, behavior-preserving refactor (no new user-facing surface, no persistence
change), so verification is by the shipped test suites plus the whole-suite quality gates rather
than a scripted CLI walkthrough.

### Whole-suite gates (closure branch `post-merge/077-shipment-items-normalization`)

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./cmd/backlogit` | exit 0 |
| Tests | `go test ./...` | **PASS** (all packages ok/cached) |
| Vet | `go vet ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Format | `gofmt -l internal cmd tests scripts` | see note |

**gofmt note**: the local Windows working tree is checked out with CRLF line endings, so `gofmt -l`
flags every `.go` file in the repo (300+), including files untouched by this closure. This is a
line-ending artifact of the local checkout, not a content issue — the closure branch changes **no**
Go source. The authoritative `gofmt`/format gate runs on LF in CI (the `Docline frontmatter gate`
and `test` jobs) and passed at merge; `go vet` and `golangci-lint` (which include format-sensitive
analyzers) both return exit 0 locally.

### Shipped test evidence

- `internal/core/shipment_normalize_test.go` (**moved** from `internal/mcp`, with an added
  empty-`[]string` case) — asserts `NormalizeShipmentItems` across nil-map, missing-key, nil-raw,
  `[]string` (copied, non-aliased), `[]any` (string-filtered), and empty-`[]string` inputs, and
  that the return is **never nil** (empty input → non-nil `[]string{}`).
- `internal/mcp/shipment_response_test.go` `TestListShipments_EmptyItems_NeverNull` (**retained**)
  — end-to-end guard that `backlogit_list_shipments` emits `custom_fields.items` as a JSON array
  (never `null`) for an empty shipment, exercising the thin `handleListShipments` adapter.
- Existing `internal/core` shipment suites (atomic, covering, lifecycle, state-integrity) continue
  to pass against the renamed call sites, confirming behavior preservation.

## Load-bearing invariants confirmed

- **Single source of truth**: one exported `core.NormalizeShipmentItems`; the MCP duplicate is
  gone (`grep func normalizeShipmentItems internal/mcp/tools.go` → no match).
- **Never-null wire shape**: the invariant is enforced by the core return contract and exercised
  on both the core unit path and the MCP end-to-end path. The former divergence (empty `[]string{}`
  → the old pure reader returned nil while the old MCP mutator returned non-nil) is reconciled to
  the non-nil superset.
- **Read-only**: `NormalizeShipmentItems` maps into a fresh slice (`make`+`copy`/`append`); it does
  not mutate its input's slice; the MCP adapter's only write is the map assignment it already owned.

## Evidence

- PR #168 CI at merge HEAD: **4/4 green** — `test (1.23)`, `test (1.24)`, `CLI Reference Drift`,
  `Docline frontmatter gate`.
- Merge SHA `c8487407…` verified present on local `main` (HEAD) and as the recorded shipment commit.
- Whole-suite gates re-run on the closure branch (above).

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: MCP `backlogit_list_shipments` never-null shape (end-to-end) + shared core
  normalization edge (unit); no new surface introduced.
- BLOCKED prerequisites: none
- Risky action state: none — behavior-preserving internal refactor; no persistence or destructive path
- Follow-up recommendations: none new. The CLI `shipment list` null-vs-`[]` parity gap and the
  broader CLI/MCP command-parity audit are already tracked as stash `7ECBAC7E` and `E16F4664`.
