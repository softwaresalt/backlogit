---
title: "210-S PR 638 review remediation handoff"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-10-31/ship-210-s-pr638-review-halt.md
description: "Scoped PR 638 remediation results and remaining verification limits."
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 210-S
  feature_id: 209-F
  status: blocked_full_test_gate
---

## Served-root handoff

* Shipment: `210-S`, already active; no claim was attempted
* Served workspace root: `.`
* Served storage root: `.backlogit`
* Served index: `.backlogit/backlogit.db`
* Manifest path: `.backlogit/queue/210-S.md`
* Handoff status: `active`, updated at `2026-10-31T06:59:28.7708084Z`
* Ordered items: `209-F`, `209.001-T`, `209.002-T`, `209.003-T`
* The supplied handoff records a scoped P-012 frontmatter read of the manifest
  using Windows no-follow and opened-path verification, with all path
  components checked for reparse points; its result matched the shipment tool
* The parent Ship session independently matched the supplied roots against the
  MCP metadata catalog and the main database pragma; the bounded implementation
  executor relied on that attestation

## Current state

* Branch: `feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`
* PR: [#638](https://github.com/softwaresalt/backlogit/pull/638), open
* HEAD: `3e0616364ecdaed15df226ae6a0299e6b924dedf`
* Index sync succeeded; the lifecycle topology gate passed
* Checkpoint enumeration returned 122 records, with no anomalies or active
  Ship checkpoint; concrete hook events through sequence 4380 were acknowledged
* Engram readiness timed out; direct-file fallback was used

## Resolved implementation findings

The three P-021 C1 findings have local implementations and passing targeted
regressions. GitHub threads remain parent-owned and are not claimed resolved.

* `AssociateCommit` acquires lifecycle-global before artifact and prior-link
  snapshots. A preheld-C shipment/member caller needs verified global ownership
  in the existing workspace-bound ordered context token; C-only participants
  fail closed before snapshots or persistence. Standalone nonmembers may reuse
  C without global. Both standalone and cascade archive-order regressions pass,
  as does pending-manifest recovery.
* Both stash acquisition paths retain OS-exclusive ownership. Fresh raw
  sidecars fail promptly; abandoned old sidecars recover; live aged handles
  cannot be stolen. Windows deletion is attached to the owned handle before
  close, rather than using unsafe close-then-unlink. Unix unlinks while locked
  and rejects orphaned or replaced inodes during acquisition.
* `OpenReadOnly` never SQLite-opens the source. It brackets database bytes and
  metadata with repeated WAL reads, validates WAL headers, salts and checksum
  chains, and materializes the committed prefix in a temporary copy. SQLite
  quick-check and checkpoint precede `VACUUM INTO` a unique shared-memory URI.
  A pinned query-only connection owns the returned snapshot; temporary files
  are removed before return, including failed attempts.

The original regressions were observed RED before production edits. Additional
RED coverage exposed non-retained stash ownership and SQLite silently ignoring
an invalid WAL despite successful quick-check of the stale database. Explicit
first-frame salt validation also received a separate RED-to-GREEN check.

## Targeted verification

Every Go verification invocation used these contained environment settings:

```powershell
$env:TEMP="$PWD\logs\go-cache-pr638-review"; $env:TMP=$env:TEMP; $env:GOCACHE=$env:TEMP
```

* `go test -run=^$ -count=1 ./...`: PASS, baseline compile only
* `go test ./internal/core -run '^TestReview' -count=1 -timeout=2m`:
  expected RED for all three supplied regressions before production changes
* `go test ./internal/db ./internal/core -run 'TestReview|TestArchiveItem_ArchiveEventCannot' -count=1 -timeout=3m`:
  PASS
* `go test ./internal/core ./internal/db -run 'TestReview|TestTaskLock|TestLockOrder_|Test.*Stash|TestArchiveItem_ArchiveEventCannot|Test.*(CrossProcess|ProcessLock)' -count=1 -timeout=5m`:
  PASS, core 49.864s and db 41.994s
* `go test ./internal/db -count=1 -timeout=5m`: PASS, 218.306s
* `go test ./internal/core -run 'TestReviewAssociate|TestArchiveItem_ArchiveEventCannot|TestReviewStash|TestReviewReconstitution' -count=5 -timeout=5m`:
  PASS, 58.074s
* `go vet ./internal/core ./internal/db`: PASS
* `go test ./internal/core ./internal/db -run 'TestReview|TestTaskLock|TestArchiveItem_ArchiveEventCannot' -count=1 -timeout=3m`:
  db PASS, 72.916s; core hit one Windows `Access is denied` during durable
  artifact replacement in the cascade archive case, not lifecycle acquisition
* `go test ./internal/core -run '^TestArchiveItem_ArchiveEventCannotBeSplitFromMutationByCThenBWriter$' -count=3 -timeout=3m`:
  PASS, both cases in all three retries, 14.249s
* `go test ./internal/core -run 'TestReview|TestTaskLock|TestArchiveItem_ArchiveEventCannot' -count=1 -timeout=3m`:
  final isolated core PASS, 12.729s
* Scoped `gofmt -l` and `git diff --check`: PASS
* `golangci-lint run ./internal/core/... ./internal/db/...`: BLOCKED, exit 3;
  the installed linter supports export-data version 2, but the installed Go
  toolchain produced version 4. `GOLANGCI_LINT_CACHE` also used the contained
  cache directory. No tool installation or toolchain replacement was attempted.

Coverage includes the cold preview's entire storage-tree bytes and names,
uncheckpointed committed WAL visibility, unchanged source DB/WAL/SHM bytes and
names, temporary cleanup before return, write rejection, and continuous upserts
with repeated checkpoints and snapshot reads.

## Narrower review remediation

The report-only review's two P1 findings and the P2 WAL tail finding received
RED-to-GREEN regressions in this bounded follow-up. The scope remained core/db
safety, tests, the existing lock-order design document, and this handoff.

* Stash acquisition rejects symlinks, Windows junction/reparse components,
  hardlinked sidecars, and nonregular targets before refresh. Unix walks
  directories with no-follow `openat` and opens the leaf relative to that pinned
  ancestry. Windows reuses the verified directory pin and NT-relative no-follow
  create helper, checks attributes/link count and the opened final path before
  truncation. Age-plus-kernel ownership and handle-owned deletion are retained.
  Tests also call the platform opener directly, bypassing the pathname precheck.
* C-held shipment/member regressions independently hold global, then call with
  C-only context and assert typed lock-order rejection with no global attempt,
  unchanged Markdown, and no commit-link insertion. Verified global-plus-C
  succeeds. A separate RED-to-GREEN persist-boundary regression closes the
  membership reclassification path: persist cannot request global under C.
* A full checksum-invalid uncommitted tail after a validated commit now retains
  that prefix. A checksum-invalid frame with a commit marker, or a later
  same-generation full frame carrying a commit marker, fails closed. Synthetic
  crash-tail and real SQLite spill/rollback/reuse tests compare with independent
  SQLite recovery. The reused same-salt WAL keeps leftover full frames; both
  recovery and the snapshot read the new committed value and zero rolled-back
  rows, without changing source WAL bytes.
* The `AssociateCommit` lock-order table now states global before snapshots/C,
  the ordered-token restriction, and standalone nonmember compatibility.

Follow-up verification used the contained cache/TEMP/TMP settings above:

* Compile-only baseline across `./...`: PASS
* Initial linked-sidecar, C-held shipment/member, and crash-tail tests: RED,
  each exposing its reported defect before production edits
* First focused GREEN: core 29.235s, db 1.784s
* SQLite rollback/reuse, later-commit corruption, verified global-plus-C, and
  standalone/cascade archive compatibility: PASS, core 11.878s, db 2.339s
* Final acquisition-boundary C-to-global regression: RED before its guard
* Broad targeted safety suite (`TestReview`, task locks, lock order, stash,
  archive ordering, cross-process/process locks): PASS, core 66.033s,
  db 38.682s
* Existing Go 1.24.0 `TestReview|TestArchiveItem_ArchiveEventCannot`: core PASS,
  133.686s, including Windows junction/direct-opener tests; db's continuous
  writer/checkpoint test exhausted the 30s snapshot bracket deadline while
  cross-platform compilation was also active
* Isolated Go 1.24.0 db `^TestReview` retry: PASS, 60.289s
* Go 1.24.0 `go vet ./internal/core ./internal/db`: PASS
* Linux/amd64 and Darwin/amd64 core test binaries: compile PASS; no Unix
  runtime execution was available
* Scoped docline lint for both modified documents: valid, zero findings
* Scoped `gofmt -l`, all delta Go files checked for LF-only bytes, and
  `git diff --check`: PASS
* Stash SHA-256 remained
  `368353F9825DD73D85236071BEBDA8FFF01A340C47965C448BD4D8A10B103A02`

One follow-up test edit initially nested named test functions, causing a setup
failure. The syntax was corrected in one adjustment; the subsequent targeted
run passed. No full suite or golangci-lint was launched by this follow-up executor.

## Final salt-tail review remediation

The final in-scope P2 received one RED-to-GREEN implementation attempt.
Both salt-mismatch and checksum-invalid tail paths now inspect all remaining
aligned full frames for current-generation commit evidence. A salt mismatch
cannot hide a later commit; old-generation commit markers remain acceptable.

Verification used existing Go 1.24.0 with `GOTOOLCHAIN=go1.24.0` and
`TEMP`, `TMP`, and `GOCACHE` all set to `logs/.go-cache-pr638-review`.

* `go test -run=^$ -count=1 ./internal/db`: PASS, 1.032s
* `go test ./internal/db -run '^TestReviewSnapshotWALSaltTailWithLaterCommit$' -count=1 -timeout=2m -v`:
  RED before production edits, exit 1, 3.043s. Both
  `salt-corrupted-then-current-commit` and
  `checksum-invalid-then-salt-corrupted-then-current-commit` returned no error
  instead of `errSnapshotChanged`. Both old-generation acceptance controls
  passed.
* `go test ./internal/db -run '^TestReviewSnapshotWAL' -count=1 -timeout=2m -v`:
  GREEN, exit 0, 3.712s. Both new regressions and acceptance controls passed,
  along with crash-tail, later-commit corruption, first-frame validation, and
  independent SQLite rollback/reuse coverage.
* `go test ./internal/db -count=1 -timeout=5m`: PASS, exit 0, 263.335s
* Go 1.24 `gofmt -l` on the two changed/new database files and
  `git diff --check`: PASS
* Stash SHA-256 before and after this fix:
  `403BB41CB68A7797153DDE49080D837020507BA7A356E520BC6AFF195B35A2B9`

No prior source/test/design edits or stash entries `17214531` and `C2CFE06E`
were changed. No repository-wide test or lint run was restarted. The parent's
unrelated CLI daemon-lock missing-path/EOF full-gate failure, captured in
`C2CFE06E`, remains a delivery blocker, not a successful delivery. The 30s
snapshot starvation/ABA limitations and absence of source locking remain.

## Remaining limits

The widened bracket detects observed WAL-generation changes and database
changes; explicit WAL validation avoids relying on quick-check alone. This is
tested consistency evidence for cooperating SQLite WAL writers, not an
unconditional mathematical proof against ABA file replacement, hostile external
mutation, or arbitrary filesystem behavior. Source SQLite locks are not taken.
Committed-prefix acceptance follows observable SQLite WAL generation and commit
markers; arbitrary corruption that erases those markers is not distinguishable
from an interrupted uncommitted tail by this lock-free reader.
The retry deadline is 30s; ordinary filesystem calls are synchronous and cannot
be forcibly interrupted by that context.
Continuous writers can exhaust the stable-bracket deadline under load; the
isolated Go 1.24 retry passed but does not remove that availability limitation.

Runtime verification was on Windows. Unix lock changes still require the
parent's platform gates. The full repository test suite was not run by this
executor. Full tests, current-toolchain lint, review, GitHub replies, thread
resolution, CI, P-018 and merge gates remain outstanding. Post-merge closure has
not begun. The single Windows replacement-access failure remains a recorded
intermittent limitation; its immediate isolated three-run retry passed.

At delegation, the parent reported its Go 1.27 full suite RUNNING and
`internal/cli` failures in `TestU175_044/_061_LintFix` from the linter export-data
mismatch. That full-gate status remains parent-owned; this executor did not
restart it or run concurrent full tests/lint.

HEAD remains `3e0616364ecdaed15df226ae6a0299e6b924dedf`. No commit, push,
branch change, review reply, thread resolution, merge, backlog mutation or
disposition operation was performed by this executor. The supplied regression
file and this memory document were preserved. A pre-existing modification to
`.backlogit/stash.jsonl` was observed and left untouched.
The operator's entry `17214531` and all prior working-tree changes were
preserved; no stash write was performed.

## Parent validation and delivery halt

The parent independently passed served-root attestation and the lifecycle
topology gate. The startup checkpoint scan had no active Ship cursor or
validation anomaly, and the concrete hook queue was empty.

The operator's new stash entry `17214531` was preserved in local commit
`555a83ea` (`chore(backlog): stash 17214531`). It was not edited or triaged.

* Repository lint: PASS using the existing Go 1.24 toolchain and
  `golangci-lint run --timeout=5m`, after moving the Unix-only unlink helper
  into Unix-only compilation
* Repository `go vet ./...`: PASS with hidden, workspace-contained scratch
* Full local `go build ./cmd/backlogit`: PASS
* All 12 changed/new Go files: canonical gofmt and LF-only bytes
* Both changed Markdown documents: authoring docline lint PASS
* Unfiltered `go test -timeout=30m ./...`: NOT PASS

The first full run used Go 1.27 and failed linter fixtures on incompatible
export data, timed out the core package at 30 minutes, and exhausted the
concurrent snapshot deadline. It was stopped after those failures.
An intermediate Go 1.24 attempt was stopped when visible scratch directories
caused wildcard package discovery to race fixture cleanup.

The final attempt used existing Go 1.24, `GOFLAGS=-p=1` for serialized package
execution, and hidden contained scratch under `logs/`. It failed existing
CLI daemon-lock tests, including `TestU3_StopMCPDaemon_NotRunning`
(`runtime/mcp-daemon.lock` missing-path inspection) and helper-process tests
with EOF. The parent stopped the already-failed run; it is not full-suite
success, and later packages were not all verified. No assertion was weakened.

Scratch and build caches were retained under ignored hidden `logs/`
directories, not deleted. The edits remain scoped to core/db safety,
regression tests, lock-order documentation, and this handoff.

Delivery is blocked by the full test gate. No fixing commit has been pushed,
no fixing-SHA review reply has been posted, and none of the three open
Copilot threads has been resolved by the parent. PR readiness still refers
to an older HEAD; required CI and P-018 have not been satisfied for the
remediation. No merge or post-merge closure was attempted.

The independent daemon-runtime failure was captured once as deferred entry
`C2CFE06E`, with task `209.003-T`, feature `209-F`, shipment `210-S`, and
PR `638` source references. This is a P-021 C2 capture, not permission to
repair that separate contract or waive the failed gate. The same ID is the
task, run, and closure-handoff residual-risk reference.

Final narrow local review found no P0/P1 in the corrected boundaries. Its
remaining salt-tail P2 received a further RED-to-GREEN correction and passing
database-package tests. Final lint, vet, build, and formatting passed again.
The source remediation may be preserved as a local conventional commit;
it remains unshipped until the full test and remote gates are satisfied.

## Absolute-path resumption probe

The operator explicitly selected `checkpoint-20261101-023919.json` for
resumption. Independent served-root attestation and lifecycle topology passed;
the checkpoint was conforming and Ship-owned, and engram was reachable.

The approved old session caches under `logs/` were removed without traversing
reparse points. A focused Go 1.24 probe used literal absolute, separate
workspace-contained temporary and build-cache directories. The command
`go test ./internal/cli -run 'TestU2_|TestU3_' -count=1 -v` exited 1.
`TestU3_StopMCPDaemon_NotRunning` still reported the missing-path inspection
error. The prior parent commands had also used absolute `Join-Path $PWD`
paths, so relative TEMP is not an established cause of these observed runs.

No default-OS-temp comparison or passing full-suite result was observed in
this resumption. The existing deferred entry `C2CFE06E` remains unchanged;
no second capture was created. The checkpoint remains active, and no push,
fixing-SHA reply, thread resolution, merge, or post-merge closure occurred.

## Go 1.26.5 resumption

The operator identified the Go 1.24.0 Windows `os.Root.Name` defect and the
Go 1.27 compiler export-data incompatibility with the installed linter.
Using the cached Go 1.26.5 toolchain independently confirmed that both
`TestU175_044_LintFix` and the focused `TestU2_` / `TestU3_` daemon tests pass.
The complete CLI package passed in 173.116s. Repository vet and full CLI
build also passed under Go 1.26.5. Repository lint passed with the CI five-minute
budget; all 13 remediation Go files remained gofmt-clean and LF-only, and
authoring docline lint passed for this handoff.

The unfiltered `go test -timeout=30m ./...` run with serialized packages
did not pass. Core failed archive/Git strategy and ancestry tests, two
Windows journal replacement checks with `Access is denied`, and a shipment
gate fixture. Core timed out after 30 minutes while running
`TestReconcileResumeConsumedNonceBinding`. The already-failed full run was
stopped; the remaining packages were not all verified.

The contained fixture roots discover this checkout's parent Git repository:
`TestHeadSHABounded` returned the actual branch HEAD for a no-repository
fixture. A runtime-only `GIT_CEILING_DIRECTORIES` isolation probe made the
shipment evidence fixture pass but did not make the archive strategy,
Git-worktree, or HEAD probes pass. No source or assertion was changed.

The current-HEAD reviewer found no new P0/P1 in the inspected remediation
boundaries and retained P2 residual `D3C8D13B`. Its missing Git-tool coverage
was checked by the parent: the complete committed delta contains 16 files
and the inspected on-disk source matches HEAD exactly. Delivery remains
blocked by the full test gate regardless of that source review.

The daemon-investigation capture `C2CFE06E` was not edited or removed; Stage
owns any disposition in light of the confirmed toolchain diagnosis.
`checkpoint-20261101-023919.json` remains active. No push, review reply,
thread resolution, remote readiness update, merge, or closure occurred.
The Go 1.26.5 session scratch directories were cleaned after all its processes
were stopped, without following reparse points.
