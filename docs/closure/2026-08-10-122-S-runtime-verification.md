---
chunk_strategy: h1-h2-h3
description: "Runtime verification for 122-S (checkpoint administrative disposition — abandon and quarantine), covering CLI surfaces with both absolute and relative --cwd invocation."
doc_type: closure
docline:
    date: 2026-08-10T00:00:00Z
    status: accepted
    tags:
        - runtime-verification
        - checkpoints
        - 122-S
schema_version: "1.0"
source: docs/closure/2026-08-10-122-S-runtime-verification.md
title: "122-S Checkpoint Administrative Disposition — Runtime Verification"
---

# 122-S Checkpoint Administrative Disposition — Runtime Verification

## Verdict: PASS (after one fix-forward cycle)

## Surface

CLI (`cmd/backlogit`), backed by `internal/core` (`AbandonCheckpoint`,
`QuarantineCheckpoint`, `ResolveDispositionTarget`) and `internal/events`
(checkpoint schema, disposition sidecar, read-only `ListCheckpoints`). MCP
tool surfaces (`backlogit_abandon_checkpoint`, `backlogit_quarantine_checkpoint`,
`backlogit_list_checkpoints`) share the same core entry points exercised
below, so this CLI-level verification covers their behavior by construction.

## Environment Precheck

* Built a fresh binary directly from the merged `origin/main` tip
  (commit `04d14fbe`, which includes both PR #342 — the feature — and
  PR #343 — the relative-`--cwd` fix described below) via
  `go build -o backlogit-verify.exe ./cmd/backlogit` — NOT the pre-existing
  `C:\Tools\backlogit.exe`, which predates this shipment's changes.
* Verification ran in an isolated, disposable scratch workspace under
  `docs/scratch/runtime-verify-122-S-final/` (deleted after use; no residue,
  confirmed via `git status --short`).
* Extensive automated coverage already exists (`go test ./...` full suite
  green, `go vet ./...` clean, `golangci-lint run ./...` clean) — this
  runtime pass targets the compiled-binary, real-filesystem, real-CLI-flag
  gap that unit and integration tests running via `go test` do not close.

## Fix-Forward Finding: Relative `--cwd` Regression

The **first** runtime verification pass (against merge commit `5b963353`,
before the fix below) found that every `checkpoint abandon` / `checkpoint
quarantine` invocation with a relative `--cwd` value failed with `target
escapes checkpoints directory`. Root cause: `ResolveDispositionTarget` built
a directory path via `filepath.Join` without normalizing to absolute, so
`confineToStorageRoot` double-joined the relative workspace root onto itself.
This was fixed and merged as PR #343 (commit `04d14fbe`) with a regression
test (`TestResolveDispositionTarget_WorksWithRelativeWorkspaceRoot`) confirmed
RED before the fix and GREEN after. The verification below is the **second**
pass, against the corrected binary.

## Verification Steps and Results

All commands below were run with a **relative** `--cwd` value
(`docs\scratch\runtime-verify-122-S-final`), the exact invocation pattern
that exposed the regression above.

1. **`checkpoint list` (read-only contract)** — seeded one valid active
   checkpoint, one malformed checkpoint, one valid checkpoint intended for a
   later "reject quarantine" case. `list` returned all three, correctly
   flagged the malformed one with `needs_quarantine: true` and a
   shell-safe, quoted `remediation_command`, and — critically — did **not**
   move the malformed file (list is read-only per 136-F/U9).

2. **`checkpoint abandon` on a valid, active checkpoint** — succeeded,
   returned `{"disposition":"abandoned", ...}`. Confirmed via the follow-up
   `list` call that `status` transitioned to `"abandoned"` in place (file not
   moved).

3. **`checkpoint abandon` on a malformed checkpoint** — refused with
   `checkpoint is malformed; use quarantine instead of abandon`, naming the
   correct verb. File remained in the checkpoints directory (not moved,
   not rewritten).

4. **`checkpoint quarantine` on that same malformed checkpoint** — succeeded,
   returned `{"disposition":"quarantined", ...}`. Confirmed the quarantined
   file exists at `archive/checkpoints/checkpoint-verify-quarantine.json`
   with a `.disposition.json` sidecar alongside it, and no longer exists at
   its original checkpoints-directory path.

5. **`checkpoint quarantine` on a valid, active checkpoint** — refused with
   `checkpoint is valid; use abandon instead of quarantine`, naming the
   correct verb. File remained untouched.

6. **Final `checkpoint list`** — confirmed exactly two checkpoints remain in
   the checkpoints directory (the abandoned one, in place with
   `status: abandoned`; the still-active reject-quarantine one, untouched),
   and the quarantined file is correctly absent from the list (it lives in
   the archive now). `needs_quarantine: 0` confirms no further malformed
   files remain.

## Protected Invariants Re-Confirmed at the Binary Level

* Abandon and quarantine are disjoint: each verb refused the other's target
  class with the exact naming-the-correct-verb error message.
* `checkpoint list` never mutated the filesystem (malformed file remained in
  place until the explicit `quarantine` call).
* Quarantine moved the file byte-for-byte into the archive with a sidecar
  disposition record; abandon rewrote status in place without moving the
  file.
* Disposition works correctly under a relative workspace root — the specific
  case broken pre-fix and the primary target of this second verification
  pass.

## Residual Exposure

This repository dogfoods a separately pinned `C:\Tools\backlogit.exe` for its
own harness operations. That binary predates 122-S and does not yet expose
the abandon/quarantine verbs; it is rebuilt and re-pinned independently of
this repository's release cadence (documented in
`docs/design-docs/checkpoint-administrative-disposition.md`).

## Backlog Follow-Ups (accepted, non-blocking)

Three hardening items were identified during PR review and accepted as
backlog work rather than blocking the merge (review-fix cycle limit reached;
none affect the protected invariants verified above for the common,
single-agent usage pattern):

* `136.014-T` — TOCTOU race between classification and move in
  `QuarantineCheckpoint`.
* `136.015-T` — `moveNoReplace` discards its unwind error on a rare
  double-failure branch.
* `136.016-T` — the quarantine move path does not yet respect the
  `durable_writes` opt-in config flag.
