# F013 Harness Manifest

**Feature**: F013 — Release Pipeline Fix
**Generated**: 2026-04-04
**Branch**: 013-release-pipeline-fix
**Import Check**: PASS
**Red Phase**: CONFIRMED (characterization-first — assertion failures, not panics)
**Execution Posture**: characterization-first (YAML infrastructure, no Go stubs)

## Test Files

| Tier        | Path                                          | Test Count |
|-------------|-----------------------------------------------|------------|
| integration | tests/integration/ci_compliance_test.go       | 4          |

## Stub Files

No Go stub files generated. F013 is YAML workflow infrastructure; the "implementation"
is editing `.github/workflows/ci.yml` and `.github/workflows/release.yml`. The tests
validate the YAML files directly and fail against the current broken state.

## Work Item Mapping

| Item ID          | Title                                    | Test Function                              | Harness Command                                                                                              | Status  |
|------------------|------------------------------------------|--------------------------------------------|--------------------------------------------------------------------------------------------------------------|---------|
| F013.T001.ST001  | Replace regex tag filter with glob       | TestReleaseTagTriggerUsesGlob              | `go test ./tests/integration/... -run TestReleaseTagTriggerUsesGlob -v`                                      | RED     |
| F013.T002.ST001  | Update Go version matrix                 | TestWorkflowGoVersionMatchesMod            | `go test ./tests/integration/... -run TestWorkflowGoVersionMatchesMod -v`                                    | RED     |
| F013.T003.ST001  | Resolve current SHAs                     | N/A (research task)                        | N/A                                                                                                          | PENDING |
| F013.T003.ST002  | Apply SHA pins to ci.yml                 | TestAllActionsUseSHAPins/ci.yml            | `go test ./tests/integration/... -run 'TestAllActionsUseSHAPins/ci.yml\|TestCheckoutStepsNoPersistCredentials/ci.yml' -v`       | RED     |
| F013.T003.ST003  | Apply SHA pins to release.yml            | TestAllActionsUseSHAPins/release.yml       | `go test ./tests/integration/... -run 'TestAllActionsUseSHAPins/release.yml\|TestCheckoutStepsNoPersistCredentials/release.yml' -v` | RED     |

## Package Structure

No new packages created. Tests are in the existing `tests/integration/` package.

* `tests/integration/ci_compliance_test.go` (new, colocated with existing integration tests)

## Test Helpers

```go
// findRepoRoot walks up from the test working directory to locate the
// repository root by finding go.mod.
func findRepoRoot(t *testing.T) string

// readCIWorkflow parses a GitHub Actions workflow YAML file into a ciWorkflow.
func readCIWorkflow(t *testing.T, path string) ciWorkflow

// workflowPaths returns the absolute paths to the CI and release workflow files.
func workflowPaths(t *testing.T) (ciPath, releasePath string)
```

## Failure Summary (Red Phase)

| Test | Failure Count | Root Cause |
|------|---------------|------------|
| TestReleaseTagTriggerUsesGlob | 2 | `[0-9]` regex found; `v*.*.*` glob missing |
| TestWorkflowGoVersionMatchesMod | 3 | Go 1.24 absent from ci.yml, release.yml matrices; build job uses 1.22 |
| TestAllActionsUseSHAPins | 13 | 3 in ci.yml + 10 in release.yml use version tags |
| TestCheckoutStepsNoPersistCredentials | 4 | 1 in ci.yml + 3 in release.yml missing persist-credentials |

## Notes

F013.T003.ST001 (Resolve current SHAs) is a research subtask with no automated test.
The worker must look up SHAs before ST002 and ST003 can proceed (dependency wired in backlogit).

F013.T003.ST002 and ST003 are blocked by ST001 in the backlogit dependency graph.
The build-orchestrator should execute ST001 first, then unblock and run ST002/ST003.
