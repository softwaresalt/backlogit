# Stage session — F3844849 malformed-JSONL-line handling convergence

- **Date:** 2026-07-07
- **Agent:** Stage (planning-only, P-010 boundary respected — no source code written)
- **Mode:** Operator AFK, full downstream autonomy
- **Stash processed:** `F3844849` (low / task) — "Unify malformed-JSONL-line handling"

## Outcome (handoff token)

- **Shipment (queued):** `086-S` → hand off to Ship
- **Feature:** `086-F` — "Unify malformed-JSONL-line handling across event-log parsers"
- **Task:** `086.001-T` — "Converge malformed-JSONL-line handling via shared ParseEventLine helper" (parent `086-F`, test-first, size S, ~2h)
- Shipment manifest (parent-first): `[086-F, 086.001-T]`

## Artifacts

- Decision: `docs/decisions/2026-07-07-malformed-jsonl-line-handling-convergence-deliberation.md` (docline 0 violations)
- Plan (review gate PASS): `docs/exec-plans/2026-07-07-malformed-jsonl-line-handling-convergence-plan.md` (docline 0 violations)

## Key decisions

- **Convergence policy: A — skip-with-warning** (skip-and-continue). Rationale: a
  malformed line is a *permanent, non-retryable* parse failure; erroring (Policy B)
  bricks an item's rehydration with no recovery path. Distinct from the
  batch-failure anti-pattern (transient/retryable → propagate error). Source doc
  083-S finding #8 explicitly recommends converging on the lenient doctor behavior.
- **Convergence approach: 2 — shared `events.ParseEventLine` helper** used by both
  `parseItemLogFile` (internal/db/rehydration.go:495) and `ReadAllEvents`
  (internal/events/reader.go:24). Prior art `bufio-scanner-incomplete-fix-missed-db-package`
  documents this exact re-divergence class → centralize the parse/skip decision.
- **Observability:** structured `slog.Warn` (item + path + 1-based line + reason) on
  every skip in BOTH paths — closes the pre-existing silent-skip gap in `ReadAllEvents`.

## Divergence confirmed in code (grounded, not guessed)

- `internal/events/reader.go:44-46` — `json.Unmarshal` err → `continue` (silent skip, bufio.Scanner).
- `internal/db/rehydration.go:508-510` — `json.Unmarshal` err → `return nil, err`; caller
  rehydration.go:394-398 logs warn + drops ALL of that item's events. `encoding/json` sole
  use is line 508 (deterministically unused after refactor).

## Plan-review gate

- Multi-persona (Go PASS, Scope PASS, Constitution ADVISORY). No P0/P1 at any point.
- Initial merged = ADVISORY (2×P2). Revised plan to absorb all P2s + high-value P3s →
  final **PASS** (`<!-- plan-review-attempt: 1 -->`). Data-integrity lens: no P0/P1;
  skip is observable, batch-failure vs permanent-corruption tension reconciled.

## Deferred / out of scope (candidate future stash)

- Adjacent oversized-line divergence (bufio.Scanner 1MB cap vs unbounded ReadFile+Split).
- Doctor-aggregate skipped-line signal (stronger-than-warn observability).
- Optional: thread injected `cfg.logger` through `rehydrateItemLogs → parseItemLogFile`.

## Commit hygiene

- Path-scoped commit: 086-F/086-S/086.001-T queue files, stash.jsonl + archive/stash.jsonl
  (F3844849 archival), decision, plan, this memory.
- Explicitly NOT staged: `.backlogit/hooks_queue.jsonl`, `.backlogit/memories.json`,
  `.backlogit/telemetry-sessions.jsonl`, `docs/cli-reference/*`, `internal/**` CRLF noise,
  `.github/agents/*`, `.gitignore`, `start.ps1`, `diff.txt`. No `git add -A`.

## Next step (Ship)

Claim shipment `086-S`, implement `086.001-T` test-first per the plan, then build/CI/PR.
