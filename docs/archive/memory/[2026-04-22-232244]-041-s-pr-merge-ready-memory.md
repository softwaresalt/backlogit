# Ship Session Memory — 041-S PR Merge Ready

**File**: `docs/memory/[2026-04-22-232244]-041-s-pr-merge-ready-memory.md`
**Session Phase**: Copilot comment remediation complete; awaiting user merge approval
**Shipment**: 041-S — Write Durability and Hook Reliability

---

## Shipment State

| Field | Value |
|---|---|
| Shipment ID | 041-S |
| Status | `active` (PR open, awaiting merge) |
| Branch | `ship/041-s-write-durability-hook-reliability` |
| PR | [#58](https://github.com/softwaresalt/backlogit/pull/58) |
| CI | ✅ All 3 checks passing (test 1.23, test 1.24, CLI Reference Drift) |
| Copilot comments | ✅ All 7 replied and resolved |
| Last commit | `ba5edcb` — Copilot review fixes |

---

## Items Completed

| ID | Title | Status | Commit |
|---|---|---|---|
| 040.001-T | Add fsync to hook event queue writes | done | 7b2897c |
| 040.002-T | Wire syncAppendLine into AppendHookEvent | done | 7b2897c |
| 040.003-T | Add fsync to telemetry JSONL harvest writes | done | 7b2897c |
| 040.004-T | Fix hook queue stale-lock TOCTOU race | done | 7b2897c |
| 040-F | Write Durability and Hook Reliability | done | 7b2897c |

---

## Key Decisions and Rationale

1. **TOCTOU fix via rename-based lock claim** (`hook_events.go`): Replaced
   `Remove+retry` TOCTOU window with atomic `Rename(.lock → .recovering)`.
   `os.Rename` is atomic on POSIX. The old `.recovering` file is removed before
   the rename attempt so stale recovery files don't block future sessions.

2. **`runtime.GOOS == "windows"` gate on pre-remove** (`fsutil.go`,
   `checkpoint.go`, `harvest.go`): POSIX `os.Rename` atomically overwrites the
   destination — no pre-remove needed. Windows `os.Rename` fails with
   `ERROR_ALREADY_EXISTS` — pre-remove required. Gating on `runtime.GOOS`
   preserves POSIX atomicity while keeping Windows compatibility.

3. **`syncAppendLine` / `syncWriteFileAtomic` helpers** (`fsutil.go`): Extracted
   shared fsync-before-close + atomic rename logic into unexported helpers to
   avoid duplication across `hook_events.go`, `hook_checkpoint.go`,
   `telemetry/checkpoint.go`, and `telemetry/harvest.go`.

---

## Copilot Review Fixes (commit `ba5edcb`)

| Comment ID | File | Fix |
|---|---|---|
| 3125679353 | `telemetry/shipment_041_harness_test.go:27` | `filepath.Join` instead of `dir + "/"` |
| 3125679429 | `events/shipment_041_harness_test.go:15` | Updated RED comment → passes post-fix |
| 3125679475 | `telemetry/checkpoint.go:100` | `os.Remove` gated on `runtime.GOOS == "windows"` |
| 3125679501 | `events/fsutil.go:61` | Same gate + updated docstring |
| 3125679530 | `events/fsutil_test.go:59` | `filepath.Join(t.TempDir(), "no_such_dir", …)` |
| 3125679552 | `events/fsutil_test.go:104` | Same for second Unix path |
| 3125679566 | `events/fsutil_test.go:7` | Removed stale "not implemented" header |

---

## Commits on Branch

| SHA | Message |
|---|---|
| `7b2897c` | feat(events): add write durability and TOCTOU-safe lock recovery |
| `d9033bb` | fix(events): pre-remove dest on Windows before os.Rename in harvest.go |
| `ba5edcb` | fix(events): address Copilot review comments on PR #58 |

---

## Next Steps

1. **User merge approval** — PR #58 is ready. Approve merge on GitHub.
2. **Post-merge closure** — After merge:
   - Invoke `operational-closure` skill
   - Check `040-F` for `source_stash_id` / `source_deliberation_id` in
     `custom_fields` and archive/remove as applicable
   - Evaluate `docs/ARCHITECTURE.md` and `README.md` for updates
   - Invoke `compound` skill for Windows GOOS-gated rename pattern learning
   - Broadcast `[SHIP] Shipment session complete: 041-S shipped`
3. **Update shipment**: `backlogit shipment ship 041-S --sha <merge-sha>`
