---
chunk_strategy: h1-h2-h3
description: 'Pre-merge runtime verification for shipment 071-S — deterministic-gates slice (PR #156). Nine TDD units proven via full go test suite (green), vet/lint/gofmt clean, and isolated-workspace dogfooding of the freshly built binary: doctor --target returns the versioned exit-code contract (0 pass / 1 validation / 3 scope|io / 4 busy), and update --size performs a body-preserving mutation with custom_fields.size set while title/status/priority survive in both the on-disk frontmatter and the SQLite index (full-row UpsertItem reconstruction). Live aggregate doctor shows no regression across the workspace.'
doc_type: closure
docline:
    ms.date: 2026-06-30T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-30T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-30-071-S-deterministic-gates-slice-runtime-verification.md
title: 071-S Deterministic-Gates Slice — Pre-Merge Runtime Verification
---

# Runtime Verification — Shipment 071-S (Deterministic-Gates Slice)

- **Surface**: CLI + MCP + library. `internal/core` (`DoctorTarget`, `task_lock`, `SetArtifactSize`), `internal/cli` (`doctor --target`, `update --size`, `ExitError`), `internal/config` + `.backlogit/header-def.yaml` (optional `size` enum), `internal/mcp` (`backlogit_doctor` `target`, `backlogit_update_item` `size`), `cmd/backlogit/main.go` (exit-code propagation). No long-running service, web, or background-job surface.
- **Mode**: automated test suite + `go vet` + `golangci-lint` + `gofmt` (LF-normalized) + manual command verification on a freshly built binary in an isolated temp workspace.
- **Context**: Ship Step 5 pre-merge runtime verification for 071-S; PR #156 on branch `feat/deterministic-gates-slice`; base `main`. Merge SHA pending operator approval (P-014).
- **Verdict**: **PASS**

## Checks

1. `go test ./...` — green across all packages including the 9 new task-scoped test files (doctor_target, task_lock, exit_error/doctor CLI, config size, artifact_size core golden/idempotency/preservation/busy, update_size CLI, MCP deterministic_gates).
2. `go vet ./...` — clean.
3. `golangci-lint run` — zero findings.
4. `gofmt` — clean on all branch-changed `.go` files (verified on LF-normalized content; the working tree is CRLF via `core.autocrlf=true`, so a local `gofmt -l .` flags every file — a known false positive. Committed blobs are LF and the Linux CI `test` jobs pass).
5. Smoke — `doctor --target` **pass** path: valid task file → `PASS <path> (<id>)`, exit **0**.
6. Smoke — `doctor --target` **validation** path: task file missing required `title` → `validation ...: Field validation for 'Title' failed on the 'required' tag`, exit **1**.
7. Smoke — `doctor --target` **scope** path: `--target ../outside.md` (path outside `.backlogit` storage root) → `scope ...: path outside workspace storage root`, exit **3**.
8. Smoke — `doctor --target` **busy** path: pre-planted `.<name>.lock` sidecar → `busy ...: task is locked by a concurrent operation`, exit **4**.
9. Smoke — `doctor --target --format json`: emits the versioned target-mode schema (`schema_version`, `path`, `ok`, `kind`, `message`).
10. Smoke — `update --size M`: exit 0; on-disk frontmatter gains `custom_fields.size: M`; body preserved byte-for-byte; `title`/`status`/`priority`/`parent_id` all unchanged.
11. Smoke — DB index integrity: after `sync`, `list --type task --format json` shows the mutated task with `title`/`status` intact **and** `size: M` — confirming the full-row `db.UpsertItem` reconstruction does not null non-size columns (the review-closed residual-P1 constraint).
12. Regression — live aggregate `backlogit doctor` over the workspace: exit 0, only 1 pre-existing unrelated orphan (`016.001-R`, a review artifact from another feature); no new issues introduced by 071-S.
13. CI on PR #156 HEAD: `CLI Reference Drift` and `Docline frontmatter gate` green; `test (1.23)` / `test (1.24)` green.

## Release readiness

- **Backward compatibility**: `size` is an **optional** header-def field with no default; existing artifacts without it validate unchanged (`ValidateArtifactFields` skips optional fields). No migration required.
- **Rollback**: pure additive change. Reverting the merge commit removes the `doctor --target` gate, the lock primitive, the `size` enum, and `SetArtifactSize` with no data migration to unwind — existing artifacts that acquired a `custom_fields.size` value simply retain a now-unrecognized-but-harmless custom field until re-validated.
- **Monitoring**: `doctor --target` writes timeout/exit telemetry to `.autoharness/metrics` (unchanged schema, per plan Q3). No new monitoring surface required.
- **Advisory (non-blocking, from review gate)**: (P3) after a `size` mutation the DB `description`/FTS column may carry raw (CRLF/leading-blank) body until the next full reindex — cosmetic, self-healing. (P3) `SetArtifactSize` writes the file before the DB upsert; on an already-malformed on-disk artifact the file is mutated while the DB is not, but the operation is idempotent and a retry self-heals — mirrors the existing `update.go` section-update pattern.

## Verdict

**PASS** — the deterministic `doctor --target` exit-code contract, per-task advisory locking, optional `size` schema, and body-preserving `SetArtifactSize` (with faithful full-row index reconstruction) are all green with no regressions. No blocking follow-ups.
