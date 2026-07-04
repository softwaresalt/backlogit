---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 079-S — CLI/MCP command parity phase-2. Consolidates CI status (4/4 green at HEAD 6257fab), Copilot review readiness (§1.9 PASS: fresh review covers HEAD, zero unresolved Copilot threads; one concurrency thread fixed and resolved), runtime verification (PASS across five new CLI families), invariants to preserve (CLI↔MCP shape parity, never-null links, MCP append serialization, registry drift honesty), and the rollback path (git revert of additive commits). Deployment path is merge-only (library + CLI binary; no service/migration). Readiness: READY WITH CONDITIONS — operator merge approval + admin bypass of the PR-Review ruleset (author-identity cannot self-approve), merge-commit strategy only (P-009). Not merged this run (P-014 / Principle VII).'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-closure.md
title: 079-S CLI/MCP command parity phase-2 — Pre-Merge Operational Closure
---

# Operational Closure — 079-S CLI/MCP command parity phase-2

- **Date**: 2026-07-03
- **Mode**: `pre-merge`
- **Shipment**: `079-S` · Feature `079-F` · Tasks `079.001-T`..`079.008-T` (+ subtasks)
- **PR**: #172 — https://github.com/softwaresalt/backlogit/pull/172
- **Branch**: `feat/079-cli-mcp-command-parity-phase2` · HEAD `6257fab`
- **Verification report**: `docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-runtime-verification.md` (verdict **PASS**)
- **Readiness**: **READY WITH CONDITIONS** (operator merge approval + admin bypass; merge-commit strategy only)

## Change summary

Fills the remaining CLI-fallback gaps for MCP-only tools so every automatable operation has a
resolvable `cli_command`, guarded by a load-bearing registry drift gate. Five new CLI command
families (`link`, `hooks`, `memory`, `comment`, `metadata` discovery) each reuse the shared
`core`/`events` path used by the corresponding MCP handler (no logic duplication, test-first).
U6 flipped 10 registry rows from `mcp_only:true` to resolvable `cli_command`, added a
flag/positional/required-flag parity assertion, and fixed a real pre-existing `stash add`
drift. U7 updated discoverability docs; U8 regenerated `docs/cli-reference/`.

## CI status

| Check | Conclusion (HEAD `6257fab`) |
|---|---|
| `test (1.23)` | success |
| `test (1.24)` | success |
| `CLI Reference Drift` | success |
| `Docline frontmatter gate` | success |

Whole-suite local gates: `go build ./...` ✅ · `go vet ./...` ✅ · `go test ./...` ✅ ·
`golangci-lint run` ✅ (0 findings) · `gofmt -l` = CRLF false-positives only (CI-LF authoritative).

## Review status (§1.9 pre-merge readiness gate)

- **Check 1 — completion**: no pending Copilot review request. ✅
- **Check 2 — freshness**: latest Copilot review (`2026-07-04T05:59:06Z`) `commit.oid == 6257fab == headRefOid`. ✅
- **Check 3 — threads**: zero unresolved Copilot threads. One thread (concurrency of
  `core.AppendComment`'s per-call `EventWriter`) was raised on `9895ced`, fixed in `6257fab`
  (thread the shared `s.Events` writer through `AppendComment`), replied to, and resolved. ✅
- **Gate: PASS.**
- `reviewDecision`: `REVIEW_REQUIRED` — reflects the branch-protection **PR-Review ruleset**
  requiring an approving review that the sole author-identity cannot self-supply. This is the
  expected operator admin-bypass situation (same as 078-S), not a Copilot-gate failure.

## Invariants to preserve

- **CLI↔MCP shape parity**: each new CLI command emits the same success envelope as its MCP
  counterpart (`{"ok":true}` for comment/memory; structured JSON for link/hooks/metadata).
- **Single source of truth**: CLI and MCP route through the identical shared `core`/`events`
  function — `core.GetLinks`, `events.PollHookEvents`/`AckHookEvents`, `events.SaveMemory`,
  `core.AppendComment`, `core.ListTypes`/`DescribeType`.
- **Never-null links**: `link list` / `get_links` normalize nil→`[]`.
- **MCP append serialization**: `append_comment` serializes per-item JSONL appends through the
  server's shared `s.Events` writer (preserved after the U4 extraction).
- **Zero-Timestamp behavior**: `core.AppendComment` keeps the pre-extraction zero-Timestamp
  indexing behavior (deliberately not mirroring `LinkCommit`'s timestamp).
- **Registry honesty**: `TestRegistryParity_FlagAndPositionalParity` fails CI on any MCP↔CLI
  op-map drift (unresolvable path, unexposed flag, positional-arity mismatch, or a required
  flag omitted from the template).

## Pre-deploy audits

- No database migration, no persistence-schema change, no config/flag rollout. Additive CLI
  commands + two behavior-preserving core extractions.
- `.autoharness/backlog-registry.yaml`: 10 rows flipped to `cli_command`; `log_telemetry` and
  `merge_sync` intentionally retained as `mcp_only` with rationale.
- `docs/cli-reference/` regenerated idempotently — `CLI Reference Drift` CI gate green.

## Deployment / rollout path

- **Merge-only.** This ships a Go library + CLI binary; there is no deployed service, canary,
  or maintenance window. Consumers pick up the new commands on the next `go build` / binary
  refresh.
- **Merge strategy: merge commit only (P-009).** Squash/rebase are disallowed for this repo.

## Post-merge checks

1. Rebuild `backlogit.exe` from merged `main` and confirm the five new command groups appear in
   `backlogit --help` and `backlogit metadata --help`.
2. Run the shipped drift gate on `main`: `go test ./internal/cli/ -run TestRegistryParity`.
3. Confirm `docs/cli-reference/` on `main` re-generates with no diff (`go run ./cmd/gen-docs`).

## Healthy signals

- New CLI commands return exit 0 with MCP-isomorphic JSON on happy paths.
- Registry drift gate green on `main`.
- No increase in per-item JSONL corruption or interleaving under concurrent MCP `append_comment`.

## Failure signals

- Drift gate fails on `main` (registry/command divergence).
- `append_comment` interleaving or malformed JSONL lines under concurrency (would indicate the
  shared-writer fix regressed).
- `link list` emitting `null` links (normalization regressed).

## Risky action record

None. All changes are additive CLI commands plus two behavior-preserving core extractions. No
destructive, migration, or rollout-sensitive action was taken. `merge_sync` (write-by-default)
was deliberately deferred rather than forced into a CLI fallback.

## Monitoring plan

- **Owner**: Ship operator (Derek Williams).
- **Signals**: CI on `main` post-merge; the `TestRegistryParity_*` and `CLI Reference Drift`
  gates are the durable guards. No runtime dashboards apply (CLI library).

## Rollback trigger

- Post-merge CI on `main` fails on the drift gate or test matrix, or a defect is found in any
  new CLI command family.

## Rollback procedure

- `git revert` the feature commits on a fresh branch and open a revert PR. Because the change is
  additive (new commands + registry rows + docs), revert is low-risk: removing the CLI commands
  restores the prior `mcp_only` reachability. No data migration to unwind.

## Validation window

- Through the next Ship session / next shipment intake (the drift gate provides continuous
  regression protection thereafter).

## Readiness recommendation

**READY WITH CONDITIONS.** All quality gates, runtime verification, and the §1.9 Copilot
readiness gate pass at HEAD `6257fab`. Merge is gated on:

1. **Explicit operator merge approval** (P-014 / Constitution Principle VII — not merged this run).
2. **Admin bypass of the PR-Review ruleset** — the author-identity cannot self-supply the
   required approving review (same as 078-S).
3. **Merge-commit strategy** (P-009) — squash/rebase disallowed.

## Follow-up

- `merge_sync` CLI fallback → phase-3 (write-by-default; needs guardrails). Retained `mcp_only`.
- `log_telemetry` → intentional-permanent `mcp_only` (read/report-only telemetry).
- External autoharness `.tmpl` parity (stash `EED25928`) → out of scope (Principle IV; out-of-tree).
- Post-merge closure (Step 6) — shipment archival + knowledge graduation — runs on a dedicated
  `post-merge/079-S` branch only AFTER operator-approved merge.
