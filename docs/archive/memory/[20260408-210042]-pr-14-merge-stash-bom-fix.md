---
title: PR #14 Merge and Stash BOM Fix
description: Session memory for PR #14 closure and post-merge stash.jsonl BOM hardening
ms.date: 2026-04-08
---

## Outcome

PR #14 (`chore/ship-004-s-post-merge`) merged to main at SHA `61fcf854`.
Post-merge `backlogit sync` failed due to a UTF-8 BOM in `.backlogit/stash.jsonl`.
The BOM was stripped and `ReadJSONL` hardened to strip it defensively on future reads.
Final commit on main: `d1e769d`.

## What was done

1. Recovered context from prior checkpoint after network disconnect.
2. Confirmed CI green on `a35db77` (Go 1.23 + 1.24).
3. Verified all 4 unresolved Copilot review threads were stale (code was already correct).
4. Merged PR #14 → main at `61fcf854`.
5. Pulled main locally (fast-forward).
6. Fixed UTF-8 BOM in `.backlogit/stash.jsonl` (3-byte EF BB BF prefix).
7. Hardened `internal/stash/jsonl.go:ReadJSONL` to strip BOM from first line.
8. All tests pass (`go test ./...` green).
9. Committed at `d1e769d`.

## Key decisions

- BOM strip is done only on the first line (the only place a BOM can appear in JSONL).
- The sentinel `var utf8BOM = "\xef\xbb\xbf"` is package-level for clarity.
- Stale Copilot threads (`Ensured` vs `Migrated`, `os.Remove` warn, etc.) do not block merge when `mergeable_state` is `"clean"`.

## Root cause of BOM

Windows PowerShell `Set-Content` and some merge tools write UTF-8-BOM by default.
The same root cause affected `memories.json` (fixed in PR #14's `f2e3afa` commit).

## Shipment status

Shipment 004-S (F019 Data Quality & Tool Efficiency + F018 semantic links) is fully shipped and closed.

## Files changed this session

- `.backlogit/stash.jsonl` — BOM stripped
- `internal/stash/jsonl.go` — `ReadJSONL` hardened
