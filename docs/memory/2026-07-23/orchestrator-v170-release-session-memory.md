---
session: orchestrator-v170-release
date: 2026-07-23
agent: Orchestrator
phase: session-completion
release: v1.7.0
---

# Session Memory — Orchestrator v1.7.0 Release + 103-S Ship + BD8DBB85 Intake

## Session Outcome

Cut and published **v1.7.0** with one small MCP parity feature folded in, shipped
via the Stage → Ship pipeline, then captured a consumer-feedback bug into the
stash for a future Stage cycle.

## Completed Work Items

| ID | Title | Final Status |
|---|---|---|
| 120-F | Spike (pre-release scoping) | closed |
| 121-F | Spike (pre-release scoping) | closed |
| 053-DL | CLI/MCP `list_items` parity deliberation | archived |
| 122-F | MCP `list_items` priority/owner filter parity | archived |
| 122.001-T | Add priority/owner params to `backlogit_list_items` | archived |
| 103-S | Shipment (122-F) | archived |
| BD8DBB85 | Stash: `blocked→queued` state-machine gap + doctor doc contradiction | active (awaiting Stage triage) |

## Pull Requests (all merged, merge commits — P-009)

| PR | Purpose | Merge Commit |
|---|---|---|
| #289 | Staging 103-S (053-DL deliberation → 122-F harvest) | `8b4e924c` |
| #290 | Feature 122-F implementation | `311b3840` |
| #291 | Post-merge closure (archive 103-S/122-F/122.001-T/053-DL) | `7daf8c30` |
| #292 | Stash intake BD8DBB85 + Copilot-review correction | `09708da8` |

## Release

- **v1.7.0** annotated tag at `7daf8c30`, pushed → `release.yml` run `30050181986` succeeded.
- Artifacts: 6 platform binaries (linux/darwin/windows × amd64/arm64) + `SHA256SUMS`.
- Version wired via ldflags `-X .../internal/version.Version=v1.7.0`.
- `v1.6.0..v1.7.0`: 304 commits (27 feat, 78 fix), no breaking changes.

## Files Modified (feature + closure)

| File | Change |
|---|---|
| `internal/mcp/tools.go` | +8 — added `priority`/`owner` params to `backlogit_list_items` |
| `internal/mcp/list_items_filter_parity_test.go` | NEW |
| `internal/cli/list_filter_parity_test.go` | NEW |
| `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` | NEW |
| `.backlogit/stash.jsonl` | +1 — stash entry BD8DBB85 (corrected text) |

## Key Decisions

- **053-DL run as full deliberation** (doc-only output) → decided option A (fold MCP
  `list_items` parity into v1.7.0), planned, harvested to 122-F, shipped clean.
- **123-F fsync durability deferred** to a later standalone release (larger release
  unit; scoped to the pre-existing Windows atomicfile remove-before-rename data-loss
  window). Raised low→medium.
- **v1.7.0 cut** rather than waiting for 123-F, keeping the release small and clean.

## BD8DBB85 — Verified Bug (for Stage)

Consumer feedback confirmed accurate against source on 2026-07-23:

1. **State machine has no path back to `queued`.** Default transitions
   (`internal/config/defaults.go`): `queued:{active,blocked}`,
   `active:{done,blocked,review,shipped,abandoned}`, `blocked:{active}`,
   `review:{done,accepted,rejected}`, `done:{archived}`. A blocked item can only
   resume as `active`; `active` also lacks `→queued`.
2. **Doc contradiction.** `docs/cli-reference/backlogit_doctor.md` long-help says
   `--status blocked (and --status queued to resume)` — documents a `blocked→queued`
   transition the runtime rejects.
3. **Runtime enforcer** is the pre-hook `hooks.ValidateStatusTransition`
   (`internal/hooks/builtin_pre.go:36`), wired in `internal/core/workspace.go:118`,
   gated by `Lifecycle.ValidateTransition`. The standalone `core.ValidateTransition`
   helper (`internal/core/harness_status.go:28`) is NOT the production path.

**Decision needed (Stage):** either (a) add a return-to-ready transition
(`blocked→queued` and/or `active→queued`) to the transition map, OR (b) correct the
`backlogit_doctor` doc to `--status active`. Cross-ref prior deliberation
`docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`.

## Deferred / Blocked (future sessions)

- **BD8DBB85** — next Stage triage (this session's follow-on).
- **123-F** — fsync durability implementation (standalone release).
- **6FA0829B / 7F0A6E89** — blocked external stashes (Principle IV, upstream `*.tmpl`).

## Next Steps

1. Stage BD8DBB85 → deliberation/plan → decision (a) or (b) → harvest → shipment.
2. Later: Ship the resulting shipment; then 123-F as its own release unit.
