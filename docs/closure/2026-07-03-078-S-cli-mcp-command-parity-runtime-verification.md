---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 078-S — CLI/MCP command parity: honest fallback map + highest-value gap fills. Verifies the three new/changed runtime CLI surfaces against a scratch workspace built from HEAD: (U3) backlogit shipment add mirrors the add_to_shipment MCP tool with a JSON {item_id, shipment_id, status:added} envelope, sentinel not-found error on a missing item, and cobra arg-count validation; (U4) backlogit shipment list normalizes custom_fields.items to a JSON array on every read edge — [] for an empty shipment and [<item>] after an add, never null; (U6) backlogit checkpoint create validates the --state-dump payload as a CheckpointV1, auto-populates created_at/status, round-trips through checkpoint list, and rejects an out-of-enum agent via the oneof validator. Non-runtime deliverables (U1 audit matrix, U2 registry op-map + drift test, U5 fallback guide) are validated by the shipped test suites and whole-suite gates. Verdict PASS.'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-078-S-cli-mcp-command-parity-runtime-verification.md
title: 078-S CLI/MCP command parity — Pre-Merge Runtime Verification
---

# Runtime Verification — 078-S CLI/MCP command parity

- **Date**: 2026-07-03
- **Shipment**: `078-S` · Feature `078-F` · Tasks `078.001-T`..`078.006-T` (+ subtasks)
- **PR**: #170 · **not merged** (halted at merge-ready per P-014 / Constitution Principle VII)
- **Branch**: `feat/078-cli-mcp-command-parity` · HEAD `0ea9ea2`
- **Surface**: `cli` (three commands) · **Mode**: scripted scratch-workspace walkthrough + whole-suite gates
- **Verdict**: **PASS**

## Affected runtime surfaces

Three CLI surfaces are new or behavior-changed; each mirrors an existing MCP tool contract:

- **U3 `backlogit shipment add <shipment-id> <item-id>`** (`internal/cli/shipment.go`,
  `newShipmentAddCmd`) — new command mirroring the `add_to_shipment` MCP tool.
- **U4 `backlogit shipment list`** (`internal/cli/shipment.go`, `newShipmentListCmd`) —
  now normalizes `custom_fields.items` via `core.NormalizeShipmentItems` on the read edge
  before rendering, mirroring the MCP list handler (folds `7ECBAC7E`).
- **U6 `backlogit checkpoint create --state-dump <json>`** (`internal/cli/checkpoint.go`,
  `newCheckpointCreateCmd`) — new command completing the CLI checkpoint lifecycle
  (create → list → get) for MCP-outage session continuity.

The remaining units are non-runtime deliverables: U1 (parity audit matrix, `docs/reviews/`),
U2 (registry op-map correction + `registry_parity_test.go` drift guard), U5 (MCP→CLI fallback
guide, `docs/design-docs/`). These are validated by the shipped tests and whole-suite gates,
not by a runtime walkthrough.

## Verification approach

A fresh binary was built from the branch HEAD (`go build ./cmd/backlogit`) into a temp
directory, a throwaway workspace was initialized (`backlogit init`), and each surface was
exercised end-to-end against real workspace state. No production/`.backlogit` state was mutated.

### U3 — `shipment add`

| Scenario | Command | Observed |
|---|---|---|
| Happy path | `shipment add 001-S 003.001-T` | exit 0, JSON `{"item_id":"003.001-T","shipment_id":"001-S","status":"added"}` |
| Reflected in list | `shipment list` | shipment `001-S` `custom_fields.items` = `["003.001-T"]` |
| Missing item | `shipment add 001-S 999-T` | exit 1, `Error: … load artifact 999-T: backlogit: not found` (sentinel) |
| Arg validation | `shipment add 001-S` | exit 1, `Error: accepts 2 arg(s), received 1` |

The JSON envelope and sentinel-error surface match the `add_to_shipment` MCP tool.

### U4 — `shipment list` items normalization

| Scenario | Observed `custom_fields.items` |
|---|---|
| Empty shipment (freshly created) | `[]` (array, **not** `null`, **not** absent) |
| After `shipment add` | `["003.001-T"]` |

The read edge always marshals a JSON array, matching `backlogit_list_shipments`.

### U6 — `checkpoint create`

| Scenario | Command | Observed |
|---|---|---|
| Happy path | `checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"rtv-s1","phase":"build","status":"active","resume_hint":"rtv"}'` | exit 0, JSON `{"path":".backlogit\\checkpoints\\checkpoint-…json"}` |
| Round-trip | `checkpoint list` | 1 checkpoint, `created_at` auto-populated, `status:active` |
| Invalid schema | `--state-dump '{…,"agent":"bogus",…}'` | exit 1, `checkpoint validation failed: Key: 'CheckpointV1.Agent' … 'oneof' tag` |

Create → list round-trips and schema validation (agent enum) behave as the checkpoint
lifecycle requires.

## Whole-suite gates (branch HEAD `0ea9ea2`)

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./cmd/backlogit` | exit 0 |
| Tests | `go test ./...` | **PASS** (all packages ok) |
| Vet | `go vet ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 |
| Format | `gofmt -l .` | see note |

**gofmt note**: the local Windows working tree is checked out with CRLF line endings, so
`gofmt -l` flags every `.go` file (a line-ending artifact, not a content issue). Verified real
formatting is clean by re-running `gofmt -l` on an LF-normalized copy of the changed files
(empty output). The authoritative format/vet gates run on LF in CI and are green.

## CI evidence (PR #170)

- CI at HEAD `0ea9ea2`: **4/4 green** — `test (1.23)`, `test (1.24)`, `CLI Reference Drift`,
  `Docline frontmatter gate`; `statusCheckRollup = SUCCESS`.
- The `CLI Reference Drift` gate initially failed (new commands lacked generated reference
  pages); resolved by regenerating `docs/cli-reference/` (`go run ./cmd/gen-docs`) for the two
  new commands — one fix-ci cycle.

## Load-bearing invariants confirmed

- **CLI↔MCP shape parity**: `shipment add` and `shipment list` marshal the same envelope/array
  shape as their MCP counterparts; `core.NormalizeShipmentItems` is the single normalization
  source of truth reused by the CLI list edge.
- **Never-null items**: `shipment list` never emits `null`/absent `custom_fields.items`.
- **Checkpoint schema enforcement**: `checkpoint create` rejects out-of-contract payloads
  (agent enum) and auto-populates lifecycle fields for `schema_version=1`.
- **Registry honesty (U2)**: `registry_parity_test.go` fails CI on any future MCP↔CLI op-map
  drift (unmapped tool, over-claimed cli_command, orphan mcp_tool, discovery-surface divergence).

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: CLI `shipment add`, CLI `shipment list` (items normalization), CLI
  `checkpoint create` — all against a scratch workspace built from HEAD.
- BLOCKED prerequisites: none
- Risky action state: none — additive CLI commands + a read-edge normalization; no destructive
  or persistence-schema change. `docs_migrate` fallback template was kept plan-only (apply is a
  gated escalation) to preserve low blast radius.
- Follow-up recommendations:
  - Stash `2827CB5F` (deliberation-record reconciliation: 078-F deliberation states the parity
    matrix location as `docs/cli-reference/`; it shipped under `docs/reviews/`). Routed to Stage —
    editing deliberation artifacts is outside Ship's role (P-010).
  - External autoharness `.tmpl` parity (stashes `EED25928` / `B55985DD`) remain out of scope
    for this shipment (out-of-tree; Principle IV).
