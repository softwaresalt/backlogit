---
chunk_strategy: h1-h2-h3
description: 'Pre-push multi-model adversarial review for shipment 086-S (unify malformed-JSONL-line handling across the two per-item event-log parsers) feature commit bd1e62d. Three independent reviewers (gpt-5.3-codex, gemini-3.1-pro-preview, claude-opus-4.7) reviewed the shared events.ParseEventLine convergence (internal/events/reader.go + internal/db/rehydration.go) plus its two new tests, with a data-integrity mandate on (a) parser convergence, (b) observable-skip / no silent data loss, and (c) no masking of transient/retryable failure as a permanent skip. Unanimous result: zero findings, all three data-integrity properties CONFIRMED by every reviewer, no HIGH-confidence P0/P1. GATE PASS.'
doc_type: closure
docline:
    ms.date: 2026-07-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-07-086-S-feature-pr-adversarial-review.md
title: 086-S Malformed-JSONL-Line Convergence — Feature PR Pre-Push Adversarial Review
---

# Adversarial Review — 086-S Feature PR (pre-push)

**Date:** 2026-07-07
**Shipment:** 086-S — Unify malformed-JSONL-line handling across event-log parsers
**Feature / Task:** 086-F / 086.001-T
**Branch:** `feat/086-malformed-jsonl-convergence`
**Commit reviewed:** `bd1e62d` — `fix(db): converge malformed-JSONL-line handling via shared ParseEventLine`
**Gate:** Pre-push multi-model adversarial review (operator-mandated for every PR)
**Scope reviewed:** `git show bd1e62d` — 4 files, +295 / −17
- `internal/events/reader.go` — new exported `ParseEventLine` helper + `ReadAllEvents` refactor
- `internal/db/rehydration.go` — `parseItemLogFile` refactor + removal of now-unused `encoding/json`
- `internal/db/rehydration_malformed_line_test.go` — new convergence/observability/regression test
- `internal/events/reader_test.go` — new `ParseEventLine` contract table test

## Reviewers

| Reviewer | Model | Result |
|---|---|---|
| A | gpt-5.3-codex (high) | PASS — no findings |
| B | gemini-3.1-pro-preview (high) | PASS — no findings |
| C | claude-opus-4.7 (high) | PASS — no findings |

Each reviewer ran independently against the committed diff and read the decision
(`docs/decisions/2026-07-07-...-deliberation.md`) and plan
(`docs/exec-plans/2026-07-07-...-convergence-plan.md`) so the **decision** was
reviewed alongside the code.

## Data-Integrity Mandate — Consensus Verdicts

### (a) Convergence — both parsers agree

**CONFIRMED (3/3, HIGH confidence).** Both `parseItemLogFile`
(`rehydration.go`) and `ReadAllEvents` (`reader.go`) now delegate the
blank / whitespace / malformed / valid decision, the `ItemID` backfill, and the
`strings.TrimSpace` CRLF (`\r`) trimming to the single shared
`events.ParseEventLine`. Line numbering is 1-based on both paths
(`parseItemLogFile` `i+1` over `strings.Split`; `ReadAllEvents` `lineNum++`
pre-parse); the only structural difference — `strings.Split` yields a phantom
trailing empty element after a final `\n` — is skipped as blank on both paths
and produces no event-count or warning divergence. Reviewers confirmed both
paths emit `line=2` for the same malformed fixture. Ordering preserved.

### (b) No silent data loss — every meaningful skip is observable

**CONFIRMED (3/3, HIGH confidence).** Every malformed-line skip now emits a
structured `slog.Warn("skipping malformed event log line", "item", …, "path",
…, "line", <1-based>, "error", …)` on **both** paths. This closes the
pre-existing **silent** `continue` in `ReadAllEvents` (the doctor fallback
previously dropped malformed lines with no log at all) and replaces
rehydration's prior `return nil, err` — which caused the caller to drop **every
event for the item** — with warn-and-skip that retains the surrounding valid
events. Net observability improvement; no meaningful event is dropped silently.

### (c) No masking of transient/retryable failure

**CONFIRMED (3/3, HIGH confidence).** Only deterministic, permanent
`json.Unmarshal` per-line failures are downgraded to warn+skip. Transient /
retryable failures still propagate as errors and are **not** swallowed:
- `os.ReadFile` error in `parseItemLogFile` → returned wrapped.
- `os.Open` error in `ReadAllEvents` → returned wrapped (`os.IsNotExist`
  legitimately mapped to empty, not an error).
- `scanner.Err()` still returned by `ReadAllEvents`, so mid-stream I/O read
  errors and `bufio.ErrTooLong` (over-long line, 1 MB cap) continue to surface
  rather than being masked as a skip.

## Test Rigor

- **Red gate genuine (3/3):** on pre-change code `parseItemLogFile` returns
  `nil, fmt.Errorf("parse item log line: %w", err)` on the malformed fixture and
  the `WalkDir` caller drops every event for the item; the convergence subtest
  therefore fails before the fix and passes after (both parsers return the 2
  valid events, nil error). Verified failing in the harness red run.
- **Test isolation (3/3):** the observability test mutates the global slog
  default via `slog.SetDefault`, restores the prior default with `t.Cleanup`
  (LIFO restore correct even with two stacked `captureSlog` calls per subtest),
  and deliberately omits `t.Parallel()`. Reviewer C noted the same pattern is
  already used by `066_rehydrate_dup_warning_test.go` and
  `070_rehydrate_logger_di_test.go` — established, non-flaky precedent.

## Scope Discipline

**CONFIRMED.** The change touches only the malformed-JSON per-line policy. The
adjacent oversized-line divergence (`bufio.Scanner` 1 MB cap vs unbounded
`os.ReadFile`) and the doctor-aggregate skip-count signal are explicitly
deferred follow-ups and were left untouched. The intentional unwrapped-error
return from `ParseEventLine` (consumed immediately by each caller's structured
`slog.Warn`) is a documented, reviewed Principle-I deviation — no reviewer
flagged a concrete correctness consequence.

## Findings

**None.** Zero findings at any severity from any of the three reviewers.

| Confidence tier | P0 | P1 | P2 | P3 |
|---|---|---|---|---|
| HIGH | 0 | 0 | 0 | 0 |
| MEDIUM | 0 | 0 | 0 | 0 |
| LOW | 0 | 0 | 0 | 0 |

## Gate Decision

**GATE PASS (unanimous 3/3).** No HIGH-confidence P0/P1 gate-blockers; no
findings of any severity. The skip-with-warning convergence (a) makes both
parsers agree, (b) does not silently drop meaningful events (every skip is
observable via `slog.Warn`), and (c) does not mask a transient/retryable failure
as a permanent skip. Cleared to push and open the feature PR.

## Local Quality Gates (pre-push)

- `go build ./...` — PASS (confirms `encoding/json` correctly removed).
- `go vet ./...` — PASS.
- `golangci-lint run ./internal/events/... ./internal/db/...` — PASS.
- `gofmt -l` (LF-normalized changed files) — clean.
- `go test ./...` — PASS (all packages).
