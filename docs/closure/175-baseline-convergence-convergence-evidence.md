---
title: "175-F baseline convergence terminal evidence"
source: docs/closure/175-baseline-convergence-convergence-evidence.md
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

Terminal convergence evidence for feature 175-F, task 175.012-T (shipment 169-S).
The gate results below were observed by running the task's
`exempt_verification_command` against the tree at the time of recording.

## Gate results

go test ./...: GREEN

go vet ./...: GREEN

gofmt -l .: GREEN

verify-terminal-lint.ps1 windows: GREEN

verify-terminal-lint.ps1 linux: GREEN

TERMINAL-LINT-OK:175-F

## Tracked nolint inventory

Every tracked `//nolint` directive, as `path:line | linter-set | justification`.

# BEGIN NOLINT-INVENTORY
internal/cli/commit_association_parity_test.go:154 | staticcheck | intentional: characterizes deprecated behavior
internal/cli/commit_association_parity_test.go:248 | staticcheck | intentional: characterizes deprecated behavior
internal/cli/update.go:300 | errcheck | best-effort cleanup; error intentionally ignored
internal/cli/update.go:304 | errcheck | best-effort cleanup; error intentionally ignored
internal/cli/update_utc_test.go:35 | gosec | re-exec of the test binary is the standard hermetic-subprocess pattern
internal/core/025_doctor_harness_test.go:328 | unused | used in pending TestDoctor tests
internal/core/artifact_references.go:246 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/artifact_references_indeterminate_test.go:52 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/artifact_references_test.go:253 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/artifact_references_test.go:304 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/artifact_references_test.go:370 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/gate_evidence_test.go:115 | gosec | test-controlled path
internal/core/migrate_queue.go:81 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/shipment_lifecycle.go:1316 | errcheck | best-effort cleanup; error intentionally ignored
internal/core/templates/service.go:134 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/dependencies.go:128 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/merge_sync.go:150 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/queries.go:393 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/rehydration.go:162 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/rehydration.go:199 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/rehydration.go:425 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/rehydration.go:480 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/rehydration.go:722 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/schema.go:53 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/schema.go:166 | gosec | tableName validated by columnNameRe above
internal/db/schema.go:198 | gosec | tableName validated by columnNameRe above
internal/db/schema.go:246 | gosec | indexName validated above
internal/db/schema.go:276 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/schema_gen.go:81 | errcheck | best-effort cleanup; error intentionally ignored
internal/db/stash.go:140 | errcheck | best-effort cleanup; error intentionally ignored
internal/docline/154_003_apply_guard_test.go:43 | gosec | known test-file path
internal/docline/schema_contract_test.go:33 | gosec | repo-relative test fixture path
internal/faultline/evidence_conformance_test.go:84 | gosec | golden fixture
internal/faultline/parity/driver.go:379 | gosec | src is a test-owned seed fixture path
internal/faultline/parity/driver.go:383 | gosec | fixture clone, not sensitive
internal/faultline/parity/scenarios.go:132 | gosec | path is a workspace-owned artifact under backlogitDir
internal/mcp/tools.go:1479 | errcheck | best-effort cleanup; error intentionally ignored
internal/mcp/tools.go:1483 | errcheck | best-effort cleanup; error intentionally ignored
tests/contract/checkpoint_create_opt_in_test.go:120 | errcheck | best-effort cleanup; error intentionally ignored
tests/contract/merge_sync_contract_test.go:38 | errcheck | best-effort cleanup; error intentionally ignored
tests/contract/telemetry_tool_test.go:58 | errcheck | best-effort cleanup; error intentionally ignored
tests/contract/tools_real_test.go:51 | errcheck | best-effort cleanup; error intentionally ignored
tests/integration/webhook_test.go:99 | errcheck | best-effort cleanup; error intentionally ignored
# END NOLINT-INVENTORY
