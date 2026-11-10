---
title: "212-S PR 645 CI cycle 1 portable MCP fixtures"
doc_type: reference
source: "docs/memory/2026-11-10/212-S-pr645-ci-cycle1-portable-fixtures.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: "212-S"
feature_id: "211-F"
phase: "step5-ci-remediation-cycle1"
---

## Hosted failure and authorization

The operator's 2026-11-10 Step 5 continuation directive authorizes
diagnosis, P-021 C1 classification, minimal in-scope CI remediation,
verification and current-HEAD review/push. No merge is approved.
This is fix-CI cycle 1 of 5; no shipment/task lifecycle replay is involved.

At HEAD `f6c3b36a552870219bd9574804b5a290f6113477`, seven hosted checks
passed and the Linux `test` check failed after 13m4s:
run `42885271269`, job `130072075054`.
Only `internal/cli` failed. The complete top-level failure inventory is:

* `TestU5_VerifyMCPDaemonAttachmentRefusesWorkspaceAndVersionMismatch`
* `TestU5_NonExclusiveStatusProbeAndIdleAutoShutdown`
* `TestU4_DeriveMCPDaemonEndpointDeterministicAndWorkspaceIsolated`
* `TestU4_ListenAndDialMCPDaemonIPCMultiClient`
* `TestU6_PrewarmAndLazyAutoStartColdRaceConvergence`
* `TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival`

Copilot review `5848079050` completed on that HEAD with no open issues,
no review threads and no threadless findings. Its Linux validation
overview is a risk note, not a finding. P-018 returned SATISFIED, exit 0.
Any remediation commit invalidates that review freshness.

## C1 classification and delegated repair

The installed fix-ci skill identified test-fixture defects within the
existing U4/U5/U6 acceptance contracts:

* U4: Linux preserves an absolute parent path in nested `os.Root.Name()`.
  The fixtures must open relative parent roots to exercise the declared
  nested-root case and boundary assertions.
* U5/U6: long `t.TempDir()` names exceeded the 100-byte Unix socket limit.
  POSIX daemon fixtures must use the existing short `u4TempDir` helper.

Both corrections complete the exact authorized test contracts under
P-021 C1. Neither expands the runtime contract nor changes production
endpoint limits. The delegated delta is limited to:

* `internal/cli/mcp_ipc_test.go`
* `internal/cli/mcp_daemon_multisession_test.go`

All existing assertions, guard counts, skips and timeouts are preserved.
Windows retains its original daemon-fixture cleanup behavior. No
production Go file, plan, deliberation, backlog or checkpoint was edited.
The existing Windows private-wrapper P2 remains open; its helper was
not part of this Linux fixture repair.

## Verification and limitations

The delegate reported:

* All six supplied failing families on pinned Windows Go, `-count=1`:
  exit 0, 49.901 seconds
* The same families, `-count=2`: exit 0, 52.201 seconds
* CLI vet/lint, modified-file formatting and diff checks: exit 0
* Actual Linux U4 probe on the WSL repository mount: exit 1 because that
  filesystem rejects Unix socket binding with `operation not supported`
* Restricted Linux endpoint derivation, relative-root/refusal and source
  guards: exit 0

The WSL mount limitation is not a Linux runtime PASS. Hosted Linux must
validate socket binding, multi-client behavior and daemon readiness.

The probe's three owned Linux tooling directories initially appeared
under ordinary `bin/tmp/ci645-*` names and contaminated recursive Go
package discovery. They were renamed to underscore-prefixed names
without deleting or overwriting contents. The 46-package first-party
inventory was restored; repo-wide vet and lint subsequently passed.
The initial loader panic/lint error is retained as a tooling diagnostic,
not attributed to the test-fixture patch or counted as a suite execution.

## Next gates and preserved state

Commit this bounded repair, require a clean worktree, then execute fresh
full local CI validation under the operator's current fix-ci directive.
Preserve Ordinal 18's historical PASS and failed-count 14 without reset;
the new validation must have its own HEAD-bound receipt, never reuse the
Ordinal 18 token or claim its old suite verified changed tests.

Before pushing, require full build and updated current-HEAD local
readiness. After pushing, require fresh hosted CI, Copilot completion,
thread/body disposition and independent section 1.9 verification.
P-014 approval remains absent; do not merge.

All 22 task members remain complete, open RED is empty, the same feature
branch/worktree is retained, and historical checkpoints remain untouched.
