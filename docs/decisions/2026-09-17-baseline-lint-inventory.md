---
chunk_strategy: h1-h2-h3
description: Complete unbounded golangci-lint v2.13.2 supported-platform baseline inventory (497 unique findings across the Windows-primary and Linux supported surfaces) and one-owner-per-finding mapping for the 175-F baseline-convergence DAG
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-17-baseline-lint-inventory.md
title: "Baseline Lint Inventory: 175-F Baseline-Convergence DAG"
docline:
    feature_id: 175-F
    linter: golangci-lint
    linter_version: 2.13.2
---

# Baseline Lint Inventory: 175-F Baseline-Convergence DAG
## Provenance
The authoritative baseline is the complete unbounded golangci-lint v2.13.2 structured run captured with the same invocation semantics as `scripts/verify-baseline-lint.ps1` (uncapped: `--max-issues-per-linter 0 --max-same-issues 0 --uniq-by-line=false`, JSON to stdout). The Windows-primary surface yields exactly **489 findings across 160 files** (459 errcheck + 30 staticcheck). The Linux surface adds **8 Linux-only errcheck findings** on `_unix.go` sources that do not compile under Windows, and excludes **4 Windows-only findings** on `_windows.go` sources. The unique supported-platform union across `windows` and `linux` is therefore **497 findings**; the Windows-Linux intersection is 485. This complete union is authoritative; the 489 Windows count is not all-platform, and the default display cap truncates the first page and is never used to scope the shipment.
The committed machine-readable inventory lives at `docs/decisions/baseline-lint-inventory.json` (SHA-256 of the canonical checked-in LF inventory bytes — equivalently the post-U1 LF working-tree bytes, not the host CRLF checkout — `f20b3c9c116f1ba6a2a25b33fc51f6b7772fc56581a0f7793b387bf9c837c1d1`). It carries `primary_goos: windows`, `supported_goos: [windows, linux]`, the 489-row Windows `findings` array, the 8-row Linux-only `additional_findings` array (each tagged `goos: linux`), and `surface_exclusions` (`windows`: 0, `linux`: the 4 Windows-only keys). Each row carries the exact identity fields `path`, `line`, `column`, `linter`, `message`, `owner_task_id`. Identity tuples are unique across the union; every finding has exactly one remediation owner.
## Reconciliation
By linter (union of 497):
- `errcheck`: 467
- `staticcheck`: 30

By package (dir): 15 packages carry findings.
- `internal/cli`: 234
- `internal/core`: 81
- `internal/telemetry`: 69
- `internal/db`: 59
- `tests/integration`: 18
- `internal/mcp`: 11
- `tests/contract`: 7
- `internal/events`: 5
- `internal/stash`: 5
- `internal/core/gate`: 2
- `internal/core/templates`: 2
- `internal/atomicfile`: 1
- `internal/canonical`: 1
- `internal/gateevidence`: 1
- `internal/hooks`: 1

Total files with findings across the supported union: 166 (160 Windows-primary + 6 Linux-only).
The 4 Windows-only exclusions (present on Windows, absent on Linux) are recorded in `surface_exclusions.linux` as `path|line|column|linter|message` keys and are already owned within the Windows `findings` array.
## Ownership mapping (one owner per finding)
Exactly 94 finding-remediation tasks own the 497 findings. Each remediation task owns 1-16 exact findings across at most 2 files/packages. Baseline-control (U1 `175.001-T`), support (U40 `175.040-T`, U14 `175.014-T`), and terminal-convergence (U12 `175.012-T`) own zero findings. Tasks `175.095-T` through `175.098-T` own the 8 Linux-only findings.

| owner_task_id | findings | files |
|---|---|---|
| `175.002-T` | 1 | 1 |
| `175.003-T` | 1 | 1 |
| `175.004-T` | 11 | 2 |
| `175.005-T` | 8 | 2 |
| `175.006-T` | 13 | 2 |
| `175.007-T` | 16 | 2 |
| `175.008-T` | 7 | 2 |
| `175.009-T` | 6 | 2 |
| `175.010-T` | 11 | 2 |
| `175.011-T` | 10 | 1 |
| `175.013-T` | 12 | 2 |
| `175.015-T` | 10 | 2 |
| `175.016-T` | 9 | 2 |
| `175.017-T` | 5 | 2 |
| `175.018-T` | 6 | 2 |
| `175.019-T` | 4 | 2 |
| `175.020-T` | 4 | 2 |
| `175.021-T` | 3 | 2 |
| `175.022-T` | 3 | 2 |
| `175.023-T` | 8 | 2 |
| `175.024-T` | 15 | 2 |
| `175.025-T` | 6 | 2 |
| `175.026-T` | 13 | 2 |
| `175.027-T` | 5 | 2 |
| `175.028-T` | 2 | 2 |
| `175.029-T` | 1 | 1 |
| `175.030-T` | 1 | 1 |
| `175.031-T` | 2 | 2 |
| `175.032-T` | 2 | 2 |
| `175.033-T` | 5 | 2 |
| `175.034-T` | 4 | 1 |
| `175.035-T` | 1 | 1 |
| `175.036-T` | 8 | 2 |
| `175.037-T` | 7 | 2 |
| `175.038-T` | 14 | 1 |
| `175.039-T` | 6 | 2 |
| `175.041-T` | 9 | 1 |
| `175.042-T` | 13 | 1 |
| `175.043-T` | 12 | 1 |
| `175.044-T` | 12 | 1 |
| `175.045-T` | 14 | 1 |
| `175.046-T` | 16 | 1 |
| `175.047-T` | 8 | 2 |
| `175.048-T` | 6 | 2 |
| `175.049-T` | 6 | 2 |
| `175.050-T` | 6 | 2 |
| `175.051-T` | 6 | 2 |
| `175.052-T` | 5 | 2 |
| `175.053-T` | 4 | 2 |
| `175.054-T` | 4 | 2 |
| `175.055-T` | 2 | 2 |
| `175.056-T` | 2 | 2 |
| `175.057-T` | 2 | 2 |
| `175.058-T` | 2 | 2 |
| `175.059-T` | 2 | 2 |
| `175.060-T` | 2 | 2 |
| `175.061-T` | 2 | 2 |
| `175.062-T` | 4 | 2 |
| `175.063-T` | 4 | 2 |
| `175.064-T` | 2 | 2 |
| `175.065-T` | 2 | 2 |
| `175.066-T` | 2 | 2 |
| `175.067-T` | 2 | 2 |
| `175.068-T` | 2 | 2 |
| `175.069-T` | 2 | 2 |
| `175.070-T` | 2 | 2 |
| `175.071-T` | 2 | 2 |
| `175.072-T` | 2 | 2 |
| `175.073-T` | 2 | 1 |
| `175.074-T` | 2 | 2 |
| `175.075-T` | 8 | 2 |
| `175.076-T` | 7 | 2 |
| `175.077-T` | 4 | 2 |
| `175.078-T` | 4 | 2 |
| `175.079-T` | 2 | 2 |
| `175.080-T` | 2 | 2 |
| `175.081-T` | 2 | 2 |
| `175.082-T` | 2 | 2 |
| `175.083-T` | 2 | 2 |
| `175.084-T` | 2 | 2 |
| `175.085-T` | 2 | 2 |
| `175.086-T` | 2 | 2 |
| `175.087-T` | 16 | 2 |
| `175.088-T` | 2 | 2 |
| `175.089-T` | 2 | 2 |
| `175.090-T` | 3 | 2 |
| `175.091-T` | 2 | 2 |
| `175.092-T` | 2 | 2 |
| `175.093-T` | 10 | 1 |
| `175.094-T` | 8 | 2 |
| `175.095-T` | 3 | 2 |
| `175.096-T` | 2 | 2 |
| `175.097-T` | 2 | 1 |
| `175.098-T` | 1 | 1 |
## Complete finding table
### `175.002-T` (1 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/atomicfile/atomicfile_bench_test.go` | 48 | 15 | errcheck | Error return value of `f.Close` is not checked |
### `175.003-T` (1 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/canonical/canonical.go` | 228 | 5 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
### `175.004-T` (11 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/add.go` | 68 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/add.go` | 120 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/shipment.go` | 73 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 108 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 156 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 194 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 275 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 309 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 349 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/shipment.go` | 376 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment.go` | 442 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.005-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/adopt.go` | 31 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/queue_cmd.go` | 65 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/queue_cmd.go` | 139 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/queue_cmd.go` | 148 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/queue_cmd.go` | 181 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/queue_cmd.go` | 189 | 18 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/queue_cmd.go` | 194 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/queue_cmd.go` | 196 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.006-T` (13 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/archive.go` | 30 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/archive.go` | 41 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/archive.go` | 53 | 18 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/archive.go` | 58 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 107 | 19 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/doctor.go` | 124 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/doctor.go` | 152 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/doctor.go` | 156 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 159 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 161 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 315 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 318 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/doctor.go` | 320 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.007-T` (16 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/checkpoint.go` | 344 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/checkpoint.go` | 345 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/checkpoint.go` | 346 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/checkpoint.go` | 347 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/checkpoint.go` | 348 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/checkpoint.go` | 349 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/checkpoint.go` | 353 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/checkpoint.go` | 357 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/checkpoint.go` | 358 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/checkpoint.go` | 516 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/checkpoint.go` | 585 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/metadata.go` | 49 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/metadata.go` | 85 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/metadata.go` | 115 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/metadata.go` | 209 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/metadata.go` | 222 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.008-T` (7 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/checkpoint_remediation_test.go` | 168 | 65 | staticcheck | ST1008: error should be returned as the last argument |
| `internal/cli/stash.go` | 48 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash.go` | 79 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash.go` | 144 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash.go` | 174 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash.go` | 208 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash.go` | 238 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.009-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/jsonrpc_test.go` | 126 | 9 | staticcheck | QF1011: could omit type func() error from declaration; it will be inferred from the right-hand side |
| `internal/cli/status_cmd.go` | 30 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/status_cmd.go` | 38 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/status_cmd.go` | 40 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/status_cmd.go` | 45 | 16 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/status_cmd.go` | 47 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.010-T` (11 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/list.go` | 206 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/list.go` | 261 | 15 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/update_test.go` | 32 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 60 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 88 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 130 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 153 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 176 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 203 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 229 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_test.go` | 262 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.011-T` (10 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/migrate.go` | 105 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 110 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 138 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/migrate.go` | 146 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/migrate.go` | 171 | 13 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/migrate.go` | 185 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 194 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 197 | 12 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/migrate.go` | 198 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 200 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.013-T` (12 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/get.go` | 44 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/get.go` | 70 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/get.go` | 142 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/get.go` | 153 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/get.go` | 162 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/get.go` | 168 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/get.go` | 170 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/get.go` | 177 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/get.go` | 179 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/registry_parity_test.go` | 489 | 21 | errcheck | Error return value of `freshWS.Close` is not checked |
| `internal/cli/registry_parity_test.go` | 538 | 21 | errcheck | Error return value of `freshWS.Close` is not checked |
| `internal/cli/registry_parity_test.go` | 579 | 21 | errcheck | Error return value of `freshWS.Close` is not checked |
### `175.015-T` (10 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/move.go` | 60 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/move.go` | 87 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/move.go` | 90 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/move.go` | 111 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/move.go` | 125 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/move.go` | 129 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/move.go` | 135 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/move.go` | 137 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/tty_test.go` | 19 | 15 | errcheck | Error return value of `r.Close` is not checked |
| `internal/cli/tty_test.go` | 20 | 15 | errcheck | Error return value of `w.Close` is not checked |
### `175.016-T` (9 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/dep.go` | 50 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/dep.go` | 75 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/dep.go` | 83 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/dep.go` | 107 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/dep.go` | 112 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/dep.go` | 136 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/dep.go` | 148 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/update_sync_test.go` | 30 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_sync_test.go` | 65 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
### `175.017-T` (5 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/archive_durable_write_test.go` | 77 | 36 | errcheck | Error return value of `workspace.Close` is not checked |
| `internal/core/size_compositions_batch_test.go` | 37 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/size_compositions_batch_test.go` | 86 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/size_compositions_batch_test.go` | 109 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/size_compositions_batch_test.go` | 131 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.018-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/archive_test.go` | 36 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/archive_test.go` | 383 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/archive_test.go` | 593 | 13 | errcheck | Error return value of `ws.DB.Close` is not checked |
| `internal/core/archive_test.go` | 597 | 32 | errcheck | Error return value of `newDB.Close` is not checked |
| `internal/core/doctor_test.go` | 189 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/doctor_test.go` | 244 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.019-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/doctor.go` | 908 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/doctor.go` | 1464 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/core/doctor.go` | 1537 | 17 | errcheck | Error return value of `f.Close` is not checked |
| `internal/core/shipment_reconcile_append_windows.go` | 90 | 18 | errcheck | Error return value of `file.Close` is not checked |
### `175.020-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/shipment_reconcile_evidence_windows.go` | 56 | 18 | errcheck | Error return value of `file.Close` is not checked |
| `internal/core/stash_harvest_atomicity_harness_042_test.go` | 44 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/stash_harvest_atomicity_harness_042_test.go` | 87 | 13 | errcheck | Error return value of `ws.DB.Close` is not checked |
| `internal/core/stash_harvest_atomicity_harness_042_test.go` | 99 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
### `175.021-T` (3 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/060_archive_rollback_integrity_harness_test.go` | 55 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/060_archive_rollback_integrity_harness_test.go` | 161 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/shipment_reconcile_fs_windows.go` | 154 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
### `175.022-T` (3 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/artifacts_expansion_test.go` | 34 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/artifacts_expansion_test.go` | 55 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/shipment_reconcile_snapshot_windows.go` | 66 | 18 | errcheck | Error return value of `file.Close` is not checked |
### `175.023-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/archive_consistency_harness_042_test.go` | 42 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/archive_consistency_harness_042_test.go` | 86 | 13 | errcheck | Error return value of `ws.DB.Close` is not checked |
| `internal/core/archive_consistency_harness_042_test.go` | 134 | 16 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/archive_consistency_harness_042_test.go` | 142 | 23 | errcheck | Error return value of `database2.Close` is not checked |
| `internal/core/shipment_test.go` | 36 | 36 | errcheck | Error return value of `workspace.Close` is not checked |
| `internal/core/shipment_test.go` | 1454 | 22 | errcheck | Error return value of `reopened.Close` is not checked |
| `internal/core/shipment_test.go` | 1480 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/shipment_test.go` | 1483 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
### `175.024-T` (15 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/migrate_links_canonical_test.go` | 28 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 58 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 88 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 118 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 157 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 213 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 237 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 269 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_links_canonical_test.go` | 295 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/workspace.go` | 151 | 18 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/workspace.go` | 160 | 17 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/workspace.go` | 167 | 17 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/workspace.go` | 226 | 18 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/workspace.go` | 231 | 18 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/workspace.go` | 272 | 17 | errcheck | Error return value of `database.Close` is not checked |
### `175.025-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/connection.go` | 88 | 11 | errcheck | Error return value of `db.Close` is not checked |
| `internal/db/connection_spike_test.go` | 86 | 17 | errcheck | Error return value of `db.Close` is not checked |
| `internal/db/connection_spike_test.go` | 112 | 17 | errcheck | Error return value of `db.Close` is not checked |
| `internal/db/connection_spike_test.go` | 143 | 17 | errcheck | Error return value of `db.Close` is not checked |
| `internal/db/connection_spike_test.go` | 168 | 17 | errcheck | Error return value of `db.Close` is not checked |
| `internal/db/connection_spike_test.go` | 184 | 17 | errcheck | Error return value of `db.Close` is not checked |
### `175.026-T` (13 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/connection_test.go` | 39 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/connection_test.go` | 47 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/connection_test.go` | 67 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/connection_test.go` | 77 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/connection_test.go` | 93 | 32 | errcheck | Error return value of `first.Close` is not checked |
| `internal/db/connection_test.go` | 97 | 33 | errcheck | Error return value of `second.Close` is not checked |
| `internal/db/connection_test.go` | 112 | 32 | errcheck | Error return value of `first.Close` is not checked |
| `internal/db/connection_test.go` | 116 | 33 | errcheck | Error return value of `second.Close` is not checked |
| `internal/db/connection_test.go` | 150 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/dependencies.go` | 58 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/dependencies.go` | 69 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/dependencies.go` | 196 | 15 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/dependencies.go` | 201 | 13 | errcheck | Error return value of `rows.Close` is not checked |
### `175.027-T` (5 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/queries.go` | 310 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/queries.go` | 332 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/queries.go` | 474 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/queries.go` | 495 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/telemetry_schema.go` | 129 | 15 | errcheck | Error return value of `f.Close` is not checked |
### `175.028-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/events/hook_events.go` | 126 | 16 | errcheck | Error return value of `qf.Close` is not checked |
| `internal/events/reader.go` | 35 | 15 | errcheck | Error return value of `f.Close` is not checked |
### `175.029-T` (1 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/gateevidence/formal.go` | 223 | 2 | staticcheck | QF1003: could use tagged switch on schema |
### `175.030-T` (1 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/hooks/webhook.go` | 172 | 24 | errcheck | Error return value of `resp.Body.Close` is not checked |
### `175.031-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/mcp/contract_consistency_test.go` | 196 | 38 | errcheck | Error return value of `(*github.com/softwaresalt/backlogit/internal/core.Workspace).Close` is not checked |
| `internal/mcp/export_command_map_test.go` | 38 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.032-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/mcp/merge_sync_harness_test.go` | 98 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/mcp/reliability_test.go` | 42 | 21 | errcheck | Error return value of `s.Workspace.Close` is not checked |
### `175.033-T` (5 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/mcp/dynamic_test.go` | 31 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/mcp/dynamic_test.go` | 85 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/mcp/server_init_test.go` | 49 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/mcp/server_init_test.go` | 67 | 30 | errcheck | Error return value of `ws1.Close` is not checked |
| `internal/mcp/server_init_test.go` | 106 | 38 | errcheck | Error return value of `(*github.com/softwaresalt/backlogit/internal/core.Workspace).Close` is not checked |
### `175.034-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/stash/jsonl.go` | 100 | 12 | errcheck | Error return value of `tmp.Close` is not checked |
| `internal/stash/jsonl.go` | 101 | 12 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/stash/jsonl.go` | 105 | 12 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/stash/jsonl.go` | 109 | 12 | errcheck | Error return value of `os.Remove` is not checked |
### `175.035-T` (1 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/stash/jsonl_test.go` | 124 | 15 | errcheck | Error return value of `f.Close` is not checked |
### `175.036-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/checkpoint_test.go` | 112 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/checkpoint_test.go` | 141 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/checkpoint_test.go` | 172 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_test.go` | 46 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_test.go` | 64 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_test.go` | 82 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_test.go` | 121 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_test.go` | 136 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
### `175.037-T` (7 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/events_harvest.go` | 304 | 20 | errcheck | Error return value of `tcFile.Close` is not checked |
| `internal/telemetry/events_harvest.go` | 310 | 20 | errcheck | Error return value of `sfFile.Close` is not checked |
| `internal/telemetry/events_harvest.go` | 339 | 10 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest_since_test.go` | 80 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_since_test.go` | 98 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_since_test.go` | 116 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest_since_test.go` | 141 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
### `175.038-T` (14 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/reporter.go` | 666 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 679 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 680 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 683 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 739 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/reporter.go` | 764 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/reporter.go` | 882 | 4 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 896 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 899 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 925 | 4 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 1001 | 4 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 1014 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 1017 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 1033 | 4 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
### `175.039-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/session_store.go` | 28 | 16 | errcheck | Error return value of `db.Close` is not checked |
| `internal/telemetry/session_store.go` | 34 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/telemetry/shipment_040_report_harness_test.go` | 62 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/shipment_040_report_harness_test.go` | 73 | 21 | errcheck | Error return value of `storeDB.Close` is not checked |
| `internal/telemetry/shipment_040_report_harness_test.go` | 126 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/shipment_040_report_harness_test.go` | 159 | 35 | errcheck | Error return value of `sqliteDB.Close` is not checked |
### `175.041-T` (9 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/migrate.go` | 54 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 56 | 18 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/migrate.go` | 58 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 60 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 77 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/migrate.go` | 83 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/migrate.go` | 93 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 96 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/migrate.go` | 103 | 17 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.042-T` (13 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/telemetry.go` | 438 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 459 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 461 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 462 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 463 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 464 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 470 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 473 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 475 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 476 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 477 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 478 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 480 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.043-T` (12 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/telemetry.go` | 345 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 346 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 361 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 419 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 420 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 422 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 423 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 429 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 432 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 433 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 435 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 436 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.044-T` (12 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/telemetry.go` | 93 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/telemetry.go` | 111 | 14 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 132 | 14 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 152 | 14 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 179 | 14 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 219 | 14 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 261 | 15 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/cli/telemetry.go` | 306 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 308 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 329 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/telemetry.go` | 343 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/telemetry.go` | 344 | 14 | errcheck | Error return value of `fmt.Fprintln` is not checked |
### `175.045-T` (14 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/reporter.go` | 196 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 198 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 214 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 274 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 275 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 277 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 318 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 357 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 555 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 557 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 570 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 591 | 3 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 657 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
| `internal/telemetry/reporter.go` | 658 | 2 | staticcheck | QF1012: Use fmt.Fprintf(...) instead of WriteString(fmt.Sprintf(...)) |
### `175.046-T` (16 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/docs.go` | 190 | 16 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/docs.go` | 191 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 192 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 193 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 194 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 195 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 196 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 248 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/docs.go` | 250 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/docs.go` | 253 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 256 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 258 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 262 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 271 | 15 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/docs.go` | 274 | 13 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/docs.go` | 276 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.047-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/shipment_reconcile_shipped.go` | 87 | 18 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/shipment_reconcile_shipped.go` | 104 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment_reconcile_shipped.go` | 121 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/shipment_reconcile_shipped.go` | 218 | 14 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/size_composition_parity_test.go` | 23 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/size_composition_parity_test.go` | 78 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/size_composition_parity_test.go` | 106 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/size_composition_parity_test.go` | 157 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.048-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/delete.go` | 31 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/delete.go` | 34 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/delete.go` | 42 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/delete_search_test.go` | 33 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/delete_search_test.go` | 77 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/delete_search_test.go` | 125 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.049-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/get_test.go` | 33 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/get_test.go` | 62 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/get_test.go` | 94 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/link.go` | 45 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/link.go` | 74 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/link.go` | 106 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.050-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/list_test.go` | 26 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/list_test.go` | 57 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/list_test.go` | 86 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/move_test.go` | 34 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/move_test.go` | 60 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/move_test.go` | 105 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.051-T` (6 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/root.go` | 343 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/root.go` | 366 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/root.go` | 381 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/shipment_reconcile_shipped_test.go` | 51 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment_reconcile_shipped_test.go` | 80 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment_reconcile_shipped_test.go` | 106 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.052-T` (5 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/dep_test.go` | 78 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/dep_test.go` | 114 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update.go` | 97 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update.go` | 334 | 17 | errcheck | Error return value of `fmt.Fprintln` is not checked |
| `internal/cli/update.go` | 338 | 15 | errcheck | Error return value of `fmt.Fprintf` is not checked |
### `175.053-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/hooks.go` | 48 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/hooks.go` | 100 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/query_status_test.go` | 32 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/query_status_test.go` | 81 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.054-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/search.go` | 34 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/search.go` | 43 | 16 | errcheck | Error return value of `fmt.Fprintf` is not checked |
| `internal/cli/update_utc_test.go` | 83 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/update_utc_test.go` | 96 | 11 | errcheck | Error return value of `ws2.Close` is not checked |
### `175.055-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/add_test.go` | 207 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/comment.go` | 50 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.056-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/commit_association_parity_test.go` | 77 | 21 | errcheck | Error return value of `freshWS.Close` is not checked |
| `internal/cli/deliberate.go` | 39 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.057-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/memory.go` | 48 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/metadata_parity_test.go` | 40 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.058-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/move_relocate_test.go` | 29 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/query.go` | 33 | 18 | errcheck | Error return value of `ws.Close` is not checked |
### `175.059-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/reconcile.go` | 43 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/shipment_covering_test.go` | 60 | 16 | errcheck | Error return value of `ws.Close` is not checked |
### `175.060-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/shipment_list_items_test.go` | 128 | 16 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/size_composition_columns_test.go` | 110 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.061-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/cli/stash_correct.go` | 37 | 18 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/cli/stash_correct_test.go` | 45 | 16 | errcheck | Error return value of `database.Close` is not checked |
### `175.062-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/canonical_cache_test.go` | 34 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/canonical_cache_test.go` | 70 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/delete_crashsafe_harness_042_test.go` | 39 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/delete_crashsafe_harness_042_test.go` | 75 | 13 | errcheck | Error return value of `ws.DB.Close` is not checked |
### `175.063-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/hierarchy.go` | 46 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/hierarchy.go` | 129 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/stash.go` | 64 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/core/stash.go` | 624 | 18 | errcheck | Error return value of `file.Close` is not checked |
### `175.064-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/060_stash_harvest_rollback_harness_test.go` | 41 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/archive_reconcile_test.go` | 32 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.065-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/blocking_cascade.go` | 69 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/blocking_cascade_test.go` | 37 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.066-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/commits.go` | 315 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/commits_test.go` | 31 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.067-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/gate_transition_test.go` | 55 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/harness_status.go` | 54 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.068-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/harness_status_test.go` | 70 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/migrate_links.go` | 229 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.069-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/migrate_links_test.go` | 37 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/migrate_queue_test.go` | 50 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.070-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/naming.go` | 60 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/queue.go` | 106 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.071-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/queue_test.go` | 28 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/core/shipment_reconcile_snapshot.go` | 152 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.072-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/size_composition.go` | 341 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/core/stash_provenance_test.go` | 35 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.073-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/gate/runner_test.go` | 143 | 13 | errcheck | Error return value of `fmt.Fprint` is not checked |
| `internal/core/gate/runner_test.go` | 146 | 13 | errcheck | Error return value of `fmt.Fprint` is not checked |
### `175.074-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/templates/service_sync_test.go` | 31 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/core/templates/service_test.go` | 30 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.075-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/rehydration_link_validation_harness_042_test.go` | 61 | 22 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/rehydration_link_validation_harness_042_test.go` | 90 | 22 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/rehydration_link_validation_harness_042_test.go` | 127 | 22 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/rehydration_link_validation_harness_042_test.go` | 159 | 22 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/schema.go` | 89 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/schema.go` | 166 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/schema.go` | 194 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/schema.go` | 238 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.076-T` (7 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/schema_gen_test.go` | 151 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/schema_gen_test.go` | 191 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/schema_gen_test.go` | 208 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/telemetry_schema_test.go` | 23 | 35 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/db/telemetry_schema_test.go` | 39 | 13 | errcheck | Error return value of `rows1.Close` is not checked |
| `internal/db/telemetry_schema_test.go` | 43 | 13 | errcheck | Error return value of `rows2.Close` is not checked |
| `internal/db/telemetry_schema_test.go` | 95 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.077-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/gate_evidence_schema_test.go` | 38 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/gate_evidence_schema_test.go` | 63 | 12 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/links.go` | 78 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/links.go` | 91 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.078-T` (4 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/logs.go` | 215 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/logs.go` | 235 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/stash.go` | 98 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/stash.go` | 223 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.079-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/cascade_delete_test.go` | 32 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/dependencies_test.go` | 22 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.080-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/duplicates.go` | 34 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/gate.go` | 125 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.081-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/gate_evidence.go` | 34 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/gate_evidence_rehydrate_test.go` | 56 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.082-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/links_test.go` | 27 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/queries_expansion_test.go` | 26 | 35 | errcheck | Error return value of `database.Close` is not checked |
### `175.083-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/schema_gen.go` | 102 | 18 | errcheck | Error return value of `rows.Close` is not checked |
| `internal/db/task_children_index_test.go` | 23 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.084-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/db/upsert_custom_fields_test.go` | 122 | 35 | errcheck | Error return value of `database.Close` is not checked |
| `internal/db/upsert_projection.go` | 153 | 18 | errcheck | Error return value of `rows.Close` is not checked |
### `175.085-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/events/stream.go` | 319 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/events/telemetry.go` | 45 | 15 | errcheck | Error return value of `f.Close` is not checked |
### `175.086-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/mcp/metadata_parity_test.go` | 36 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `internal/mcp/section_bugs_test.go` | 37 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.087-T` (16 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/context_window_test.go` | 139 | 22 | errcheck | Error return value of `sqliteDB.Close` is not checked |
| `internal/telemetry/harvest.go` | 244 | 12 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 263 | 10 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 289 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 401 | 11 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 402 | 13 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 440 | 11 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 441 | 13 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 449 | 11 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 450 | 13 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 467 | 11 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 468 | 13 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 474 | 10 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/harvest.go` | 475 | 12 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 479 | 12 | errcheck | Error return value of `os.Remove` is not checked |
| `internal/telemetry/harvest.go` | 489 | 12 | errcheck | Error return value of `os.Remove` is not checked |
### `175.088-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/correlator.go` | 212 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/helpers_test.go` | 17 | 29 | errcheck | Error return value of `db.Close` is not checked |
### `175.089-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/telemetry/session_events.go` | 95 | 10 | errcheck | Error return value of `f.Close` is not checked |
| `internal/telemetry/session_store_test.go` | 42 | 16 | errcheck | Error return value of `db.Close` is not checked |
### `175.090-T` (3 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `tests/contract/dep_type_parity_test.go` | 45 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/contract/telemetry_tool_test.go` | 44 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/contract/telemetry_tool_test.go` | 119 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.091-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `tests/contract/hook_events_test.go` | 33 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/contract/merge_sync_contract_test.go` | 129 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.092-T` (2 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `tests/contract/tools_expansion_test.go` | 32 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/contract/tools_real_test.go` | 36 | 29 | errcheck | Error return value of `ws.Close` is not checked |
### `175.093-T` (10 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `tests/integration/workflow_test.go` | 90 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 111 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 134 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 154 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 175 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 186 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
| `tests/integration/workflow_test.go` | 202 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 227 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 276 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/workflow_test.go` | 303 | 10 | errcheck | Error return value of `ws.Close` is not checked |
### `175.094-T` (8 findings)
| path | line | col | linter | message |
|---|---|---|---|---|
| `tests/integration/merge_sync_integration_test.go` | 62 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 34 | 29 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 57 | 16 | errcheck | Error return value of `f2.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 128 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 134 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 244 | 15 | errcheck | Error return value of `f.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 266 | 10 | errcheck | Error return value of `ws.Close` is not checked |
| `tests/integration/shipment_workflow_test.go` | 272 | 17 | errcheck | Error return value of `ws2.Close` is not checked |
### `175.095-T` (3 findings, linux-only)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/shipment_reconcile_append_unix.go` | 27 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
| `internal/core/shipment_reconcile_append_unix.go` | 40 | 18 | errcheck | Error return value of `file.Close` is not checked |
| `internal/core/shipment_reconcile_evidence_unix.go` | 99 | 18 | errcheck | Error return value of `file.Close` is not checked |
### `175.096-T` (2 findings, linux-only)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/shipment_reconcile_fs_unix.go` | 30 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
| `internal/core/shipment_reconcile_lock_unix.go` | 30 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
### `175.097-T` (2 findings, linux-only)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/core/shipment_reconcile_snapshot_unix.go` | 32 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
| `internal/core/shipment_reconcile_snapshot_unix.go` | 39 | 18 | errcheck | Error return value of `file.Close` is not checked |
### `175.098-T` (1 findings, linux-only)
| path | line | col | linter | message |
|---|---|---|---|---|
| `internal/events/item_log_lock_unix.go` | 39 | 21 | errcheck | Error return value of `dirFile.Close` is not checked |
