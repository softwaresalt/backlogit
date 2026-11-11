---
title: "212-S M19 Wave 7 U4 acceptance"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave7-converged"
---

## Recovery and custody

The operator selected and confirmed `checkpoint-20261109-052455.json`.
Its conforming Ship-owned state was restored, summarized with Engram CLI,
and only that checkpoint was resolved. No cursor, gate, counter or unresolved
pointer was pruned. The protected older checkpoint remains untouched with
SHA-256 `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.

The live schema-validated Ship route matched the Orchestrator handoff.
MCP metadata and `pragma_database_list` independently attested the served
workspace and storage roots. The session's scoped P-012 declaration permits
read-only, no-follow, handle-attested reads of logs for tasks 004 through 007.
The existing U4 start count was exactly one; U5 through U7 each had zero.
One initial opened-path normalization check refused before reading any log;
correcting the Windows extended-path prefix handling allowed verification.

The handoff outcome is recorded in workspace-relative form:

* `handoff_status`: `PASS`
* `shipment_id`: `212-S`
* `checkpoint_filename`: `checkpoint-20261109-052455.json`
* `served_workspace_root_rel`: `.`
* `served_storage_root_rel`: `.backlogit`
* `manifest_path_rel`: `.backlogit/queue/212-S.md`
* `index_file_rel`: `.backlogit/backlogit.db`
* `timestamp_utc`: `2026-11-09T05:32:00Z`

Stage's four published file hashes matched. The amendment note's own
observed SHA-256 was
`37d859923dc8abceb55cd51f5bf7c2be3d27a8f9065ee06cd8973488177ed61c`;
it did not publish a self-hash. All five Stage paths were committed by
exact path in `50e845cf`. No planning content was authored by Ship.

## U4 implementation and review

The original started cursor and harness commit `d708efe4` were preserved.
The four amended contract refinements landed in the two authorized IPC
production files only. Non-procfs POSIX intentionally fails closed;
the existing process-identity producer was not changed.

Standard report-only review found one additional same-contract P1:
overlapped pipe buffers needed explicit liveness through completion/drain.
Supplemental three-reviewer adversarial review corroborated it by majority;
no additional high-confidence contract defect was established.
The in-scope correction adds `runtime.KeepAlive(data)` after `c.run`
in both Windows Read and Write.

Final standard report-only readiness is `READY_WITH_FOLLOWUPS`,
P0=0/P1=0/P2=0/P3=0. The required follow-up is native POSIX runtime
verification; Windows-host POSIX checks are source-only. The review
independently verified HEAD `50e845cfdf905b4bbe1384621c340cf609c4d669`
and these exact source blobs, now committed without changes in `cb41ca57`:

* Unix: `62a7ef53cc0de5edf2aec88d2159372285d3ece9`
* Windows: `ca90909005aaff05eb8f085270925b0a7749e7c4`
* Immutable harness: `129c4bd723c09c43f3764a59c9d26cee27dc1ca8`

U4 used five of five implementation observations cumulatively:
two preserved observations, a source-shape guard failure, a passing amended
implementation, and the passing buffer-liveness correction. No sixth build
attempt occurred. Subsequent quality/convergence confirmations were not
implementation repair attempts. All historical counters remain preserved.
This session used two review-fix dispatches and two standard review passes.

Commit `cb41ca57` is linked to `211.004-T`, which was moved to `done`.
Both its queue removal and archive addition must be committed together.
Telemetry was disabled for the preserved task context; no epoch was invented.

## Wave 7 convergence

After task completion, these gates passed:

* `go test -run=^$ -count=1 ./...`
* `go vet ./...`
* `bin/golangci-lint.exe run --timeout=10m`
* `gofmt -l cmd internal tests tools` with empty output
* `go test -timeout=30m -count=1 -race -v -run '^TestU4_' ./internal/cli`

All three new U4 top-level witnesses executed and passed. The existing
helper-only skip remains; there were no new skipped witnesses.
`^TestU4a_` remained green. Format evidence covers first-party source,
not the known ignored tool-cache traversal limitation of `gofmt -l .`.
No broad-cache format PASS is asserted.

`go test -count=1 -v -run '^TestU2i_' ./internal/cli` reconfirmed named
assertion RED in the pending U6 command and U7 documentation subtests.
The selector compiled and ran; it did not panic or time out.

**FULL_SUITE_DEFERRED: wave 7**

The sole open selector is `^TestU2i_`, owned by `211.015-T`.
Its green-makers 003 and 004 are done; 005, 006 and 007 remain scheduled
for waves 8, 9 and 10. Its declared closing wave is 10.
Compile, vet, lint, first-party format and the declared scoped command
passed, and the open selector was re-confirmed assertion RED.
No unexpected failure was accepted. An unfiltered run while that selector
remains open would classify rather than verify its expected failure.
The one authorized post-U7 full-suite run remains unconsumed.

## Next boundary

M19 remains immutable: 16 done, three active claim-assigned members,
zero queued, blocked, unsupported or active residuals.
Wave 8 admits only `211.005-T` after a fresh attested snapshot.
Do not reclaim U5, U6 or U7; append each task's first start only at its
claim-time gate. No U5 harness or production implementation is claimed here.

Residual `D2FCA0FB` remains required in eventual PR and closure records.
Native POSIX verification and the broad-format cache limitation remain
explicit. No push, PR, merge, closure or successor shipment mutation occurred.
