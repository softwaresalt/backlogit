---
session: shipment-104s-closure
date: 2026-07-23
agent: Orchestrator
phase: post-merge-closure
shipment: 104-S
---

# Session Memory — Shipment 104-S Return-to-Queued Fix (BD8DBB85)

## Summary

Staged, shipped, and closed the BD8DBB85 stash: backlogit's state machine only
allowed `blocked → active` with no path to `queued`, contradicting its own
doctor doc. Option A added `blocked → queued` AND `active → queued` to the
validated status-transition map, plus a load-time upgrade path for persisted
configs.

## Completed Work Items

| ID | Title | Final Status |
|---|---|---|
| 104-S | Return-to-queued transition (BD8DBB85) | shipped/archived |
| 124-F | Return-to-queued status transitions | done/archived |
| 124.002-T | Add queued to active/blocked in both default transition maps | done/archived |
| 124.003-T | Update tests + stale comments for new transitions | done/archived |
| 124.004-T | Load-time normalization/upgrade for persisted transitions map | done/archived |

## PRs

| PR | Purpose | Merge Commit | Parents |
|---|---|---|---|
| #293 | Staging artifacts (shipment/feature/tasks/decision/plan/memory) | `70712c2e` | `09708da8` + `334d2b9` |
| #294 | Code fix (8 .go files) | `96664088` | `70712c2e` + `be4c5267` |
| #295 | Post-merge closure (backlog reconciliation + closure doc) | `369e862a` | `96664088` + `adfb5bc9` |

All merges used merge commits (P-009 verified: 2 parents each). All merges had
explicit operator approval (P-014). §1.9 Copilot gate passed clean on each PR's
final HEAD.

## Files Modified — Code Fix (PR #294)

| File | Change |
|---|---|
| `internal/config/defaults.go` | `DefaultHooksConfig().Lifecycle.Transitions`: added `queued` to `active` + `blocked` |
| `internal/hooks/builtin_pre.go` | `DefaultTransitions()`: identical edit (empty-map fallback) |
| `internal/hooks/builtin_pre_test.go` | 2 new rows in `AllDefaultTransitions` |
| `internal/config/transitions_sync_test.go` | NEW — `reflect.DeepEqual` sync-guard between the two default sites |
| `internal/config/loader.go` | `priorGeneratedDefaultTransitions` frozen package-level `var` + `upgradeLegacyTransitions` in `LoadHooks` |
| `internal/config/hooks_normalize_test.go` | NEW — legacy→upgraded, custom→preserved, absent→fallback |
| `internal/core/shipment_state_integrity_test.go` | Stale comment updated |
| `internal/core/gate_transition.go` | `redirectGate` bypass rationale reworded |

## Files Modified — Closure (PR #295)

| File | Change |
|---|---|
| `.backlogit/queue/{104-S,124-F,124.002-T,124.003-T,124.004-T}.md` | Moved → `.backlogit/archive/` |
| `.backlogit/hooks_queue.jsonl` | Updated by shipment lifecycle ops |
| `docs/closure/2026-07-23-104-S-return-to-queued-transition-closure.md` | NEW closure record |

## Files Added — Session Completion (PR #296)

These docs were created after PR #295 merged and shipped via a separate
session-completion PR (they are NOT in merge commit `369e862a`).

| File | Change |
|---|---|
| `docs/compound/2026-07-23-persisted-config-load-time-default-map-upgrade.md` | NEW compound learning |
| `docs/memory/2026-07-23/shipment-104s-closure-memory.md` | NEW — this file |

## Key Decisions & Rationale

- **Option A (widen the validated transition map)** chosen over Option B (a
  docs-only correction that would have changed the doctor doc to prescribe the
  supported `--status` resume path instead of changing code). Option A itself
  widens `hooks.ValidateStatusTransition`'s validated map to add the return-to-
  `queued` edges, resolving the code↔doc contradiction at the enforcement layer.
- **Persisted-config gap (124.004-T):** absent transition maps inherit the new
  default via the empty-map fallback; only PERSISTED explicit maps need the
  load-time `upgradeLegacyTransitions` normalizer. Deep-equal against a frozen
  prior-default snapshot (a package-level `var`, since Go maps cannot be `const`);
  upgrade only on exact match, preserve customized maps.
  Mirrors `PreTaskCompletionGate.Normalize()` (082-F).

## Gotchas Encountered

- **Ship subagent shares the workspace.** The `ship-104-s` subagent ran
  `backlogit move` commands that archived the done tasks in the real working
  tree but did NOT commit them to the feature branch — PR #294 contained only
  the 8 `.go` files. Recovered by committing the reconciliation post-merge in
  PR #295. Convention: item done/archival committed on feature branch; shipment
  queue→archive deferred to post-merge `ship_shipment`.
- **`ship_shipment` requires `shipment claim` first** (queued→active→shipped);
  shipping a `queued` shipment errors "shipment status conflict".

## Session State (SQL deferred_stash)

| id | status |
|---|---|
| blocked-transition-doc-mismatch | closed:104-S:PR294:96664088:PR295:369e862a |
| orphan-016-001-R | pending_triage |

## Next Steps / Deferred Work

- Triage `016.001-R` orphan (deferred per operator: "Triage 016.001-R separately later").
- `123-F` fsync durability — larger standalone future release (operator wants it shipped by itself).
- Blocked external `*.tmpl` stashes (6FA0829B / 7F0A6E89) — Principle IV containment (outside cwd).
