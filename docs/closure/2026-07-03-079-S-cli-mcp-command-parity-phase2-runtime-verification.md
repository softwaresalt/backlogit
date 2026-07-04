---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 079-S — CLI/MCP command parity phase-2. Verifies five new CLI command families against a scratch workspace built from HEAD 6257fab: (U1) backlogit link add/list/remove mirrors add_link/remove_link/get_links with directed-link JSON and link-type validation; (U2) backlogit hooks poll/ack mirrors poll_hook_events/ack_hook_events returning {derived_signals, events} and {acked_seq}; (U3) backlogit memory save mirrors save_memory returning {ok:true}; (U4) backlogit comment add mirrors append_comment returning {ok:true} over the shared core.AppendComment path; (U5) backlogit metadata types/wit/templates mirrors list_types/get_wit_metadata/list_templates. Registry op-map flip (U6), discoverability docs (U7), and regenerated cli-reference (U8) are validated by the shipped drift gate and whole-suite gates. Verdict PASS.'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-runtime-verification.md
title: 079-S CLI/MCP command parity phase-2 — Pre-Merge Runtime Verification
---

# Runtime Verification — 079-S CLI/MCP command parity phase-2

- **Date**: 2026-07-03
- **Shipment**: `079-S` · Feature `079-F` · Tasks `079.001-T`..`079.008-T` (+ subtasks)
- **PR**: #172 · **not merged** (halted at merge-ready per P-014 / Constitution Principle VII)
- **Branch**: `feat/079-cli-mcp-command-parity-phase2` · HEAD `6257fab`
- **Surface**: `cli` (five new command families) · **Mode**: scripted scratch-workspace walkthrough + whole-suite gates
- **Verdict**: **PASS**

## Affected runtime surfaces

Five new CLI command families are added; each is the CLI fallback for an MCP-only tool and
reuses the same shared `core`/`events` path so persisted/indexed state and success-JSON shape
are identical across surfaces:

- **U1 `backlogit link add|remove|list`** (`internal/cli/link.go`) — fallback for
  `add_link` / `remove_link` / `get_links`. List reuses the extracted `core.GetLinks`
  (nil→`[]`); `handleGetLinks` delegates to the same function.
- **U2 `backlogit hooks poll|ack`** (`internal/cli/hooks.go`) — fallback for
  `poll_hook_events` / `ack_hook_events` over `events.PollHookEvents`/`AckHookEvents`.
- **U3 `backlogit memory save`** (`internal/cli/memory.go`) — fallback for `save_memory`
  over `events.SaveMemory` (resolved workspace root).
- **U4 `backlogit comment add`** (`internal/cli/comment.go`) — fallback for `append_comment`
  over the extracted `core.AppendComment`; `handleAppendComment` delegates to the same
  function, threading the server's shared `*events.EventWriter` to preserve in-process
  append serialization (Copilot review fix, HEAD `6257fab`).
- **U5 `backlogit metadata types|wit|templates`** (`internal/cli/metadata.go`) — fallback
  for `list_types` / `get_wit_metadata` / `list_templates`.

The remaining units are non-runtime deliverables: U6 (registry op-map flip of 10 rows +
the load-bearing `TestRegistryParity_FlagAndPositionalParity` drift/flag-parity gate,
`.autoharness/backlog-registry.yaml`), U7 (MCP→CLI discoverability docs, `docs/reviews/` +
`docs/design-docs/`), U8 (regenerated `docs/cli-reference/`). These are validated by the
shipped tests, the `CLI Reference Drift` CI gate, and whole-suite gates — not by a runtime
walkthrough.

## Verification approach

A fresh binary was built from branch HEAD (`go build -o backlogit.exe ./cmd/backlogit`),
throwaway workspaces were initialized under `%TEMP%` (`backlogit init`), and each surface was
exercised end-to-end against real workspace state. No production/`.backlogit` state was mutated.

### U1 — `link add | list | remove`

| Scenario | Command | Observed |
|---|---|---|
| Happy path add | `link add 001.001-T 001.002-T related_to` | exit 0, JSON `{"link_type":"related_to","source_id":"001.001-T","target_id":"001.002-T"}` |
| Reflected in list | `link list 001.001-T` | one edge with `created_at`; `links` is a JSON array |
| Remove | `link remove 001.001-T 001.002-T related_to` | exit 0, JSON `{…,"status":"removed"}` |
| List after remove | `link list 001.001-T` | `"links": []` (array, never null) |
| Invalid link type | `link add 001.001-T 001.002-T relates-to` | exit 1, `Error: add link: backlogit: invalid link type: "relates-to"` |

`core.GetLinks` normalizes nil→`[]`, matching the `get_links` MCP handler shape.

### U2 — `hooks poll | ack`

| Scenario | Command | Observed |
|---|---|---|
| Poll | `hooks poll --consumer-id rtv` | exit 0, JSON `{"derived_signals":[], "events":[…]}`; concrete `create_artifact` events with `seq`, `payload` |
| Ack | `hooks ack --consumer-id rtv --seq 1` | exit 0, JSON `{"acked_seq":1}` |

Envelope shape matches `poll_hook_events` (`{derived_signals, events}`) and `ack_hook_events`
(`{acked_seq}`).

### U3 — `memory save`

| Scenario | Command | Observed |
|---|---|---|
| Happy path | `memory save --key rtv-key --summary "runtime memory"` | exit 0, JSON `{"ok":true}` |

Matches the `save_memory` MCP success envelope.

### U4 — `comment add`

| Scenario | Command | Observed |
|---|---|---|
| Happy path | `comment add 001.001-T --actor rtv --comment "runtime check"` | exit 0, JSON `{"ok":true}` |

Matches the `append_comment` MCP success envelope. The MCP handler passes the shared
`s.Events` writer so concurrent tool calls serialize on the per-item JSONL append exactly as
before the U4 extraction; the one-shot CLI passes `nil` for a fresh writer.

### U5 — `metadata types | wit | templates`

| Scenario | Command | Observed |
|---|---|---|
| List types | `metadata types` | exit 0, JSON array of configured types (`feature`, `task`, …) |
| Describe type | `metadata wit feature` | exit 0, `{"type":"feature","description":…}` |
| List templates | `metadata templates` | exit 0, JSON array of template definitions |
| Unknown type | `metadata wit not_a_type` | exit 1, `Error: describe type: unknown artifact type "not_a_type"` |

Mirrors `list_types` / `get_wit_metadata` / `list_templates`.

## Whole-suite gates (branch HEAD `6257fab`)

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./...` | exit 0 |
| Tests | `go test ./...` | **PASS** (all packages ok, incl. contract + integration) |
| Vet | `go vet ./...` | exit 0 |
| Lint | `golangci-lint run` | exit 0 (0 findings) |
| Format | `gofmt -l .` | see note |

**gofmt note**: the local Windows working tree is checked out with CRLF line endings (no
`.gitattributes`, `core.autocrlf` on, blobs stored LF), so `gofmt -l` flags touched `.go`
files as a line-ending artifact, not a content issue. The authoritative format/vet gates run
on LF in CI and are green.

## CI evidence (PR #172)

- CI at HEAD `6257fab`: **4/4 green** — `test (1.23)`, `test (1.24)`, `CLI Reference Drift`,
  `Docline frontmatter gate`.
- CI at the prior HEAD `9895ced` was also 4/4 green; the `CLI Reference Drift` gate passed on
  first run because `docs/cli-reference/` was regenerated idempotently (U8) before push — no
  fix-ci cycle needed for drift this shipment.
- One Copilot review thread (concurrency of `core.AppendComment`'s EventWriter) was raised on
  `9895ced`, fixed in `6257fab`, replied to, and resolved. The fresh Copilot re-review on
  `6257fab` raised no new line-level threads.

## Load-bearing invariants confirmed

- **CLI↔MCP shape parity**: `comment add` and `memory save` emit `{"ok":true}`; `link`/`hooks`/
  `metadata` emit the same envelopes/arrays as their MCP counterparts. Each CLI command routes
  through the identical shared `core`/`events` function used by the MCP handler.
- **Never-null links**: `link list` never emits `null`/absent `links` — `core.GetLinks`
  normalizes to `[]`.
- **Append serialization preserved**: MCP `append_comment` continues to serialize per-item
  JSONL appends through the shared `s.Events` writer after the U4 extraction.
- **Input validation**: `link add` rejects invalid link types and `metadata wit` rejects
  unknown artifact types with non-zero exit + clear sentinel errors.
- **Registry honesty (U6)**: `TestRegistryParity_FlagAndPositionalParity` fails CI on any
  future MCP↔CLI op-map drift — unresolvable `cli_command` path, flag not exposed by the target
  command, positional-arity mismatch, or a required flag omitted from the template.

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: CLI `link add|remove|list`, `hooks poll|ack`, `memory save`,
  `comment add`, `metadata types|wit|templates` — all against scratch workspaces built from HEAD.
- BLOCKED prerequisites: none
- Risky action state: none — additive CLI commands + two behavior-preserving core extractions
  (`GetLinks`, `AppendComment`); no destructive or persistence-schema change.
- Follow-up recommendations:
  - `merge_sync` CLI fallback deferred to phase-3 (write-by-default; needs guardrails) — kept
    `mcp_only` with rationale in the registry.
  - `log_telemetry` intentional-permanent `mcp_only` (telemetry is read/report-only on the CLI
    surface) — documented rationale retained.
  - External autoharness `.tmpl` parity (stash `EED25928`) remains out of scope for this
    shipment (out-of-tree; Principle IV).
