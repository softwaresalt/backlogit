# 205-S Ship Session Checkpoint

Status: HALTED at Wave 5 (`204.005-T`) with `EXEMPT_COMMAND_MISMATCH` (contract defect, requires Stage amendment).

## Completed

* Branch `feat/205-s-enforce-single-backlogit-mcp-daemon-per-workspace`; shipment `205-S` active (claim returned an MCP timeout but the post-claim topology gate and re-read confirmed `active`).
* Wave 1 `204.001-T`: RED `bb253f97`, GREEN `603f642b`.
* Wave 2 `204.002-T`: RED `a97f4986`, GREEN `9de33940`.
* Wave 3 `204.003-T`: RED `f3951b9a`, GREEN `5e9b5363`.
* Wave 4 `204.004-T`: `e70e8fcc`, `6646e304`. No RED phase was observable: the U2/U3 implementation already satisfied the subprocess test and the parity allowlist is not enforced as a requirement by the existing parity test.
* Tasks 204.001-T..204.004-T are `done` (auto-archived by the governed move).

## Wave 5 halt

* `exempt_verification_command` probe failed before the deliverable (expected). Baseline `6646e304`.
* `go run ./cmd/gen-docs docs/cli-reference` generated `backlogit_mcp_status.md`, `backlogit_mcp_stop.md` and updated `backlogit_mcp.md`; `docs lint` reports 0 violations.
* The post-deliverable gate fails: it requires a `### Synopsis` heading followed by the synopsis text, but the mcp status and stop commands have no `Long` help text, so gen-docs emits a fenced usage block with no `### Synopsis` heading.
* Fix options (Stage decision): (a) amend the exempt command regexes to match the generated format (preferred, stays docs-only); or (b) reclassify the task to allow adding `Long` to the two cobra commands (a `.go` change, not docs-only).

## Deviations to record

* The `204.004-T` item log carries a duplicate `WORK_STARTED: 205-S` (an MCP append timed out, was retried without a re-read first). Log lines were not deleted; operator decision pending.
* golangci-lint cannot run in this environment (export data version mismatch); go vet, gofmt and tests were used.
* ENGRAM_DEGRADED.

## Next steps

Stage amends `204.005-T` contract, then Ship resumes at Wave 5, runs review, PR, Copilot gate, merge, closure.

## Resume (Wave 5, 2026-10-29 evening)

* Served-root attestation passed (workspace root and storage root match, index file is backlogit.db under storage root).
* WORK_STARTED: 205-S already present once on 204.005-T; not re-appended.
* Red probe failed as required (missing generated docs); gen-docs run; EXEMPT_VERIFY_OK:204.005-T; commit e8f72a2c tracked; 204.005-T done.
* Gates: build, vet, gofmt clean; cli, errors, tests/... pass serially. golangci-lint unusable here (export data mismatch), so TestU175_*_LintFix fail when run in parallel or at all (environmental). internal/core timed out at 30m on TestMigrateCustomFieldLinks_* (core untouched by this branch).
* Local review at e8f72a2c: READY_WITH_FOLLOWUPS (P0 0, P1 0, P2 0, P3 1, stashed).
