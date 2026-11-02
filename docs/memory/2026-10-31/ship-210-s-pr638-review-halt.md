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
  status: blocked_circuit
---

## Resumed operator evidence handoff

The Orchestrator reports that the full Go suite in shell `468` exited 0 with
`GOTOOLCHAIN=go1.26.5` and `GOFLAGS=-count=1 -p=1`. Native WSL focused
shells `406` and `407` are reported PASS, and static gates are reported
passed, including shell `479`. This documentation update did not
independently observe those runs or establish current-HEAD review readiness.
The `BLOCKED` verdict below is historical and remains the recorded verdict;
the reports do not by themselves clear review or PR gates.

Stage P-021 C6 decisions `078-DL` and `080-DL` defer three crash-window
candidates to `7E14915F` and `D3C8D13B`. Decision `079-DL` defers the
format-cache remedy to `DF533572`. Other follow-ups are `C2CFE06E` and
`F2BC9C37`.

The Orchestrator supplied this workspace-relative handoff: workspace `.`,
storage `.backlogit`, index `.backlogit/backlogit.db`, and manifest
`.backlogit/queue/210-S.md`. Shipment `210-S` is reported `active`, with
`updated_at` `2026-10-31T06:59:28.7708084Z` and ordered items `209-F`,
`209.001-T`, `209.002-T`, `209.003-T`. Its scoped P-012 declaration reports
Windows `CreateFileW` with `OPEN_REPARSE_POINT` and
`GetFinalPathNameByHandleW` verification. This documentation update did not
independently repeat the handle verification or query shipment state.

## Current delivery evidence

The following section records the earlier halt, not a new verification run.

Shipment `210-S` remains active; delivery is `blocked_circuit`.
The bounded round-three leaf stopped at its three-failed-corrective-execution
limit. Local HEAD remains
`8ff9f2f3b1a38cb85a2709da2492fedf95908204`, remote `f43d967f`.
The working tree contains an unverified partial DB fix and failing core
remediation harnesses. It is not ready for commit, push, or merge.
Fresh full-suite, native Unix, hosted checks, and source review remain
parent-owned and pending. Historical results elsewhere do not cover this tree.

The three new threads received progress-only replies, not fixing claims:
`4434730479` -> `4434984360`, `4434730488` -> `4434984359`, and
`4434730495` -> `4434984358`. They remain unresolved. PR readiness was
updated to `BLOCKED` for exact pushed HEAD `f43d967f`; reply files were
removed after confirmed API success.

The bounded failed-corrective-execution circuit is open. No further test or
implementation attempt is authorized by this handoff. Resume requires
operator guidance on the remaining core fixture/locking work and the DB
draft, not reuse of the older successful test gate. The in-scope findings
remain open; no backlog task or deferred expansion was created for them.

The original comments `4433027865`, `4433027878`, and `4433027884` are
distinct from these new P-021 C1 findings:

* `4434730479`, thread `PRRT_kwDORzozKM6yYGvU`: unresolved. Core production
  capture/lock/recovery logic was not changed. RED assertions exposed the
  prose-only snapshot and omitted structured owner lock. The coordinated
  sizing fixture is not yet valid: its latest version selects a task as the
  new task's parent. No sizing serialization verdict is claimed.
* `4434730488`, thread `PRRT_kwDORzozKM6yYGvZ`: partially implemented,
  unverified. The DB reader pins its caller-bound directory, opens relative
  leaves without following redirects, checks regularity/link count and
  opened identity, and rejects unsafe leaves before allocating snapshot
  cache. Windows expands 8.3 spelling without `EvalSymlinks`; Unix uses
  no-follow directory-relative opens. Core uses `Lstat` for the missing-index
  check. Existing WAL checksum/materialization logic is retained.
* `4434730495`, thread `PRRT_kwDORzozKM6yYGvd`: unresolved. A valid forced
  plan with UNASSIGNED tasks reproduced successful concurrent
  `AppendComment(WORK_STARTED)` during the mutation hook. No production C
  locking or eligibility change was made.

All execution used `GOTOOLCHAIN=go1.26.5` and absolute, separate contained
directories below `logs/.pr638-review-round3`: `TEMP`/`TMP` in `tmp`,
`GOTMPDIR` in `gotmp`, and `GOCACHE` in `cache`.
No full repository tests were executed; the baseline was compile-only.

```text
go test -run=^$ -count=1 ./...
go test ./internal/db -run '^TestRound3SnapshotRejectsRedirectedSource$' -count=1 -timeout=2m
go test ./internal/core -run '^TestRound3(ReconstituteSnapshotOwners|ForceReconstituteHoldsUnassignedMemberLog)$' -count=1 -timeout=2m
```

* Baseline command: PASS, session `408`
* DB RED: all four cases failed the rejection assertion, session `409`;
  DB symlink, WAL symlink, dangling WAL symlink, and hardlinked DB
* DB attempted GREEN: source rejection assertions passed, but the test
  compared `DirEntry` access-time metadata rather than directory names and
  failed in the hardlink case, session `410`. Its new test-only assertion
  was corrected to compare names, not rerun. The same command's `gofmt`
  wildcard was not expanded on Windows and produced an error.
* Core initial execution: build failure for nonexistent `Artifact.Size`,
  session `411`; not accepted as RED. The harness now reads `custom_fields`.
* Core execution `412`: owner/prose assertions were RED, but the feature
  sizing fixture lacked a size field. The unassigned case did not reach its
  hook, so that apparent pass was rejected as vacuous.
* Core execution `413`: sizing fixture failed early on invalid task parent;
  the corrected unassigned plan passed preview, reached its mutation hook,
  and failed the concurrency assertion as expected. No core GREEN exists.
* Failed corrective executions: `410`, `411`, and `413`; bounded stop
* Vet, lint, broader regressions, replacement-seam/cache-leak tests, and
  final compile after all edits: pending; no passing verdict is claimed

The retained Unix parent wrapper, Windows `GetLongPathNameW` repair, and their
existing tests are unchanged. Stashes `F2BC9C37` and `17214531`, residual
`D3C8D13B`, all backlog/checkpoint state, and planning fields were not edited.
No subagents, commits, pushes, branches, or new planning/review artifacts
were created. Existing assertions outside the new harness were not changed.
No WSL execution occurred in this cycle. Earlier DrvFS rename-denial evidence
is not native Linux successful-rename proof. Noncooperating basename
check/unlink is not atomic CAS; root-alias expansion remains outside scope.

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

## Initial execution state

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

## Windows 8.3 short-path remediation

This later bounded delegation ran on HEAD `3cbe1e1a` for PR 638, shipment
`210-S`, feature `209-F`. It supersedes earlier execution-environment notes,
not the historical test evidence above. The parent attested the served
workspace `C:\Source\GitHub\backlogit`, storage `.backlogit`, MCP catalog and
database pragma agreement, index sync, and passing lifecycle topology.
Engram was reachable but its changed-code graph was stale/incomplete; current
file confirmation used the documented fallback.

The regression uses `GetShortPathNameW` on a real workspace-contained directory
with parent name `stash-shortpath-long-parent-suffix`. The host returned
`STASH-~1`, including short spellings of earlier ancestry components. Alias
support is a required assertion, not a skipped or simulated pass. All four
cases were observed RED before production edits: new and abandoned ownership
were rejected by the directory pin's short/long pathname comparison; fresh
ownership was correctly rejected but subsequent aged recovery failed; live-aged
ownership returned validation failure instead of `os.ErrExist`.

The requested seven-line `EvalSymlinks` insertion was not applied.
Check-before/resolve/check-after could accept a directory link installed after
the first check: resolution would bless its target as the destination for the
later pin. Instead, Windows `openStashLockHandle` expands only the parent
pathname's 8.3 spelling with `GetLongPathNameW` before using the unchanged
directory pin. This API preserves link pathname components rather than
canonicalizing their targets. The two-call sized buffer supports long paths
without a fixed `MAX_PATH` limit; empty or outgrown results fail closed.
The common acquisition precheck, pinned NT-relative no-follow creation,
reparse/link-count checks, final-handle pathname verification, retained
exclusive ownership, and handle-owned deletion remain unchanged.

A direct-opener regression bypasses the initial pathname precheck to model
a junction ancestor installed in that window. Its leaf directory is regular,
so rejection exercises the opened final-path comparison rather than only the
leaf reparse check. The target bytes, timestamp and directory entries remain
unchanged. This is boundary coverage, not an exhaustive concurrent-replacement
proof or a deterministic race scheduler.

Every command below used this absolute, separated workspace-only environment:

```powershell
$env:GOTOOLCHAIN='go1.26.5'
$env:TEMP='C:\Source\GitHub\backlogit\logs\.pr638-shortpath\tmp'
$env:TMP=$env:TEMP
$env:GOCACHE='C:\Source\GitHub\backlogit\logs\.pr638-shortpath\go-build'
```

Exact scoped verification commands and observed output excerpts:

```text
go version
go version go1.26.5 windows/amd64

go test -run=^$ -count=1 ./internal/core
ok github.com/softwaresalt/backlogit/internal/core 0.840s [no tests to run]

go test ./internal/core -run '^TestReviewStashWindowsShortPath$' -count=1 -timeout=2m -v
actual 8.3 parent alias: STASH-~1
--- FAIL: TestReviewStashWindowsShortPath (0.04s)
    --- FAIL: TestReviewStashWindowsShortPath/new (0.00s)
    --- FAIL: TestReviewStashWindowsShortPath/abandoned (0.00s)
    --- FAIL: TestReviewStashWindowsShortPath/fresh (0.02s)
    --- FAIL: TestReviewStashWindowsShortPath/live-aged (0.00s)
FAIL	github.com/softwaresalt/backlogit/internal/core	2.622s
exit 1

go test ./internal/core -run '^TestReviewStash' -count=1 -timeout=2m -v
actual 8.3 parent alias: STASH-~1
--- PASS: TestReviewStashWindowsShortPath (0.03s)
    --- PASS: TestReviewStashWindowsShortPath/new (0.01s)
    --- PASS: TestReviewStashWindowsShortPath/abandoned (0.00s)
    --- PASS: TestReviewStashWindowsShortPath/fresh (0.01s)
    --- PASS: TestReviewStashWindowsShortPath/live-aged (0.01s)
--- PASS: TestReviewStashRejectsJunctionAncestorAfterPrecheck (0.10s)
--- PASS: TestReviewStashRejectsJunctionComponent (0.10s)
--- PASS: TestReviewStashRejectsLinkedSidecars (0.05s)
--- PASS: TestReviewStashSidecarOwnership (0.04s)
--- PASS: TestReviewStashBlockingLockDoesNotStealLiveAgedHandle (0.01s)
--- PASS: TestReviewStashBlockingLockRecoversAbandonedSidecar (0.01s)
PASS
ok  	github.com/softwaresalt/backlogit/internal/core	2.939s
exit 0
```

The GREEN alias cases acquire/release new or abandoned locks, preserve fresh
and live-aged sidecars, recover them once eligible, exclude a long-name
contender while the alias owner is live, and leave no sidecars under either
spelling. One implementation attempt was needed.

Scoped quality checks also passed:

* `go vet ./internal/core`: exit 0, no diagnostics
* `golangci-lint run ./internal/core/...`: exit 0, no diagnostics;
  `GOLANGCI_LINT_CACHE` was
  `C:\Source\GitHub\backlogit\logs\.pr638-shortpath\golangci`
* `gofmt -l internal/core/task_lock_windows.go internal/core/stash_sidecar_review_windows_test.go`:
  no output; both changed Go files were checked LF-only
* `git diff --check`: no errors; Git warned that it would convert the memory
  document's LF to CRLF when next touched
* Scoped MCP docline authoring lint: `valid: true`, zero violations/findings

Changed paths are `internal/core/task_lock_windows.go`,
`internal/core/stash_sidecar_review_windows_test.go`, and this memory document.
No edits to `internal/core/stash_lock.go`, Unix production code, daemon/Git/
archive tests, backlog, stash, branches, commits, or pushes were made.
The pre-existing untracked checkpoint
`.backlogit/checkpoints/checkpoint-20261101-023919.json` was left untouched.
Test artifacts were created only under `logs/.pr638-shortpath`.

Status remains `blocked_full_test_gate`. No full suite or repository lint was
launched by this executor. The parent's contained-temp full gate did not pass:
temporary roots inherit the outer `.git`, `TestHeadSHABounded` returned actual
HEAD, core archive Git probes failed, journal replacement reported access
denied, and core exhausted the 30-minute timeout. Git-ceiling isolation did
not make all Git probes pass. No external OS-temp full-suite pass is claimed;
the earlier daemon diagnosis is obsolete after Go 1.26 diagnosis, and Stage
owns deferred stash disposition. Parent-owned full CLI regressions, gates,
review and shipment authority remain outstanding.

## Historical parent validation and delivery halt

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

## Latest Windows short-path test portability verification

This test-only follow-up on supplied HEAD `3cbe1e1a` supersedes earlier
short-path test prerequisite and verification notes, not their historical
RED-to-GREEN evidence. Only
`internal/core/stash_sidecar_review_windows_test.go` and this appended section
were edited.

The regression no longer derives the repository root with `filepath.Abs`,
compares `t.TempDir` using `filepath.Rel`, or requires `filepath.IsLocal`.
Repository-contained TEMP is an agent execution constraint, not stash behavior.
The created directory is expanded to its LONG spelling with the sized
`GetLongPathNameW` calls before `GetShortPathNameW`. The prerequisite still
requires a real case-insensitive pathname difference and now requires `~`
anywhere in the full short path. An existing user/runner ancestor alias can
therefore satisfy it even when new 8.3 name creation is disabled. Diagnostics
report alias presence without printing external paths. No skips, simulated
aliases, or weakened feature assertions were introduced.

All commands used these absolute, separated workspace-contained settings:

```powershell
$env:GOTOOLCHAIN='go1.26.5'
$env:TEMP='C:\Source\GitHub\backlogit\logs\.pr638-shortpath\tmp'
$env:TMP=$env:TEMP
$env:GOTMPDIR=$env:TEMP
$env:GOCACHE='C:\Source\GitHub\backlogit\logs\.pr638-shortpath\go-build'
$env:GOLANGCI_LINT_CACHE='C:\Source\GitHub\backlogit\logs\.pr638-shortpath\golangci'
```

Exact verification commands:

```text
go version
go test -run=^$ -count=1 ./...
go test ./internal/core -run '^TestReviewStash' -count=1 -timeout=2m -v
go vet ./internal/core
golangci-lint run ./internal/core/...
gofmt -l internal/core/task_lock_windows.go internal/core/stash_sidecar_review_windows_test.go
```

* `go version`: `go1.26.5 windows/amd64`
* Repository compile-only baseline: PASS, exit 0; no tests executed
* Focused stash regressions: PASS, exit 0, core 2.882s; all four
  new/abandoned/fresh/live-aged short-path cases and both junction tests passed
* Real-alias diagnostic:
  `actual 8.3 alias verified; created directory has alias: true`
* Core vet and lint: PASS, exit 0, no diagnostics
* Scoped gofmt: PASS, no output
* Python byte verification: PASS; both Go files are LF-only, containment
  assertions are absent, LONG expansion precedes SHORT lookup, and the
  full-path real-alias prerequisite is present

Before/after SHA-256 checks confirm the production fix is byte-identical:
`f4006d5cfb9e92f9c63674935aa474b7619a0239363caeba96f87b9c2b2f65b8`.
The test suffix beginning at the four-case loop, including the original
prevalidation-bypass junction-ancestor test, is also byte-identical:
`5f3fd922516a91843c5e5fb190c10e2d1dfb6df792fd063ed49f24132c8e9279`.
This preserves acquisition/recovery, kernel-busy ownership, unchanged contents,
long-name contention exclusion, and no-leak assertions.

Default hosted Windows OS TEMP validation was NOT executed: this agent cannot
write outside the workspace. This contained run exercised a newly created
directory alias, not an ancestor-only alias with new-name creation disabled.
The latter is supported by the verified prerequisite code but is not claimed
runtime-tested here.

Status remains `blocked_full_test_gate`. The inherited repository Git discovery,
Windows rename/access-denied failures, and 30-minute core timeout remain
NOT PASS. No full test gate was restarted or waived. Final repository and
hosted-platform gates remain parent-owned. No production, backlog, stash,
checkpoint, commit, push, branch, or other Git operation was performed.

## Parent short-path verification

The parent reviewed the complete short-path code and test delta against
`3cbe1e1a`. The Windows spelling expansion retains the existing pinned,
no-follow acquisition and handle-owned release. The portable regression
retains all four ownership cases and junction rejection. This narrow
report-only review found no new P0/P1/P2/P3 issue; existing follow-up
`D3C8D13B` remains unchanged.

Independent Go 1.26.5 verification:

* Requested CLI stash/deliberation/reconstitution selector: PASS
* Complete CLI package, `-count=1 -timeout=10m`: PASS, 214.338s
* Core `TestReview|TestStash`, `-count=1 -timeout=5m`: PASS, 10.233s
* Repository lint with five-minute budget, vet, and full CLI build: PASS
* Both changed Go files: canonical gofmt and LF-only bytes

These results do not replace the unfiltered full-suite gate. Its earlier
Git-fixture, replacement-access, and timeout failures remain unresolved in
this execution environment. Default-OS-temp validation was not run, and
`TestHeadSHABounded` was not represented as passing. The selected checkpoint
remains active. No push, review reply, thread resolution, merge, or closure
was performed.
The short-path execution scratch was cleaned without following reparse points.

## Hosted delivery progress

Validated source and completed handoff were pushed in `f43d967f`.
All three Copilot findings received substantive file-backed replies citing
`fcd5e4ef` and `f336bb21`, followed by confirmed GraphQL resolution:

* `4433027865`: reply `4434717133`, thread `PRRT_kwDORzozKM6yT9bm`
* `4433027878`: reply `4434717129`, thread `PRRT_kwDORzozKM6yT9bp`
* `4433027884`: reply `4434717131`, thread `PRRT_kwDORzozKM6yT9bt`

Each mutation returned `isResolved: true`. Temporary reply files were removed.
Pre-merge runtime and releasability reports are in `docs/closure/`.
They preserve the manual-adapter boundary and do not assert an upgraded
daemon, successful deployment, or completed post-merge closure.
Hosted checks and the current-HEAD Copilot request were still running at
capture. The final PR readiness block must use the eventual pushed metadata
HEAD; source remains identical to tested `f336bb21`.

## Supplemental P1 Unix sidecar release remediation

Status: `blocked_review_gate`; new validation pending. This bounded execution
started from exact local HEAD
`14faa2575ed48c1b247801c7892baff443a5787c`, with pushed parent `f43d967f`.
The worktree was initially clean. No commit, push, branch, merge, backlog,
stash, checkpoint, daemon, or index mutation was performed. No subagents were
used. No new plan, review artifact, or backlog item was created.

ProposedAction: remediate supplemental F2 within the authorized P-021 C1
stash-sidecar ownership contract. ActionRisk: high, explicitly approved by
the operator for this bounded local change. ActionResult: applied locally;
native Unix validation and fresh parent full/review gates remain pending.

### Implementation and scope

* Unix retains the final no-follow parent descriptor through release.
  `Fstat` checks the owned inode; no-follow `Fstatat` checks the captured
  basename's device, inode, regular type, and single-link count. Only matching
  ownership permits directory-relative `Unlinkat` while flock is held.
  Release never resolves an ancestor pathname.
* A private wrapper embeds `*os.File` and owns its Unix parent. `sync.Once`
  makes direct `Close` release both resources exactly once, even on a leaf
  close error. Acquisition failures and busy handles use the same cleanup.
  Direct close does not authorize sidecar removal.
* Observed replacement or identity drift fails closed and leaves the
  replacement untouched. Normal release still removes its sidecar; permanent
  lockfiles are not substituted for ownership-safe removal.
* Windows retains real short-path expansion, no-follow acquisition, and
  owned-handle deletion. Only the internal wrapper/signatures and close error
  context change. Unsupported platforms remain fail closed.
* All-component no-follow checks remain. F1 symlinked-root compatibility,
  including macOS `/tmp` aliases, is not normalized or trusted here.
  Existing accepted orphan-window residual `D3C8D13B` is not refactored.
  AssociateCommit, WAL helpers, source metadata, and unrelated APIs are untouched.

Changed paths:

```text
internal/core/stash_lock.go
internal/core/stash_lock_file.go
internal/core/task_lock_unix.go
internal/core/task_lock_windows.go
internal/core/task_lock_other.go
internal/core/stash_sidecar_release_shape_test.go
internal/core/stash_sidecar_release_unix_test.go
docs/memory/2026-10-31/ship-210-s-pr638-review-halt.md
```

### RED and focused verification

All execution scratch and caches are absolute, separated, and contained:

```powershell
$base='C:\Source\GitHub\backlogit\logs\.pr638-unix-release'
$env:TEMP="$base\tmp"
$env:TMP="$base\tmp"
$env:GOTMPDIR="$base\tmp"
$env:GOCACHE="$base\go-build"
$env:GOLANGCI_LINT_CACHE="$base\lint"
$env:GOTOOLCHAIN='go1.26.5'
$env:GOPROXY='off'
```

The first compile-only baseline used installed default Go 1.27.0 and passed.
Subsequent RED, focused GREEN, cross-compilation, and lint used cached
Go 1.26.5. The final Go 1.26.5 repository compile-only check also passed.
No unfiltered full suite was executed.

Exact Go commands, with `GOOS` unset for Windows execution:

```text
go test -run=^$ -count=1 ./...
go test ./internal/core -run '^TestReviewStash(ReleaseUnixSourceContract|DirectCloseIsDeterministic)$' -count=1 -timeout=2m -v
go test ./internal/core -run '^TestReviewStash' -count=1 -timeout=2m -v
go test ./internal/core -run '^TestReviewStash|^TestStash|^TestTaskLock' -count=1 -timeout=5m
go vet ./internal/core
golangci-lint run --timeout=5m ./internal/core/...
```

Before production edits, the Go/parser source-shape harness compiled and
failed assertions for early parent close and missing relative owned-inode
unlink. Direct repeated `Close` also failed as expected. Before implementation,
the Unix behavioral regressions compiled for Linux and Darwin without
referencing an undeclared implementation type.

The first GREEN attempt exposed a Windows test portability issue: `Stat`
after close returns an invalid-handle error, not `os.ErrClosed`. The test now
checks `WriteString` after close with `errors.Is` through `require.ErrorIs`;
the exact-once close assertion is unchanged. No production workaround was
added. The focused Windows selector then passed in 4.939s and, after adding
basename close-error context for platform lint, passed again in 2.097s.
Existing fresh, abandoned, live-aged, linked/junction, and real 8.3 short-path
regressions passed. These are focused results, not full-gate evidence.

Windows lint initially identified a Unix-only basename field as unused.
The basename now provides close-error context on both platforms. Scoped lint
then passed, exit 0, for `GOOS=windows`, `linux`, and `darwin`.
Windows and Darwin final-source core vet passed. Linux core vet passed before
the close-error-context-only adjustment.

Scoped `gofmt -l` returned no paths; all seven changed Go files were LF-only.
`git diff --check` passed. The existing handoff's authoring docline check
passed with zero findings, including the MDP008 namespace contract:

```text
go run ./cmd/backlogit docs lint --profile authoring --path docs/memory/2026-10-31/ship-210-s-pr638-review-halt.md --format json
```

Cross-compilation used `GOARCH=amd64`, separately setting `GOOS=linux` and
`GOOS=darwin`, with these output paths:

```text
go test -c -o C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp\core-linux-red.test ./internal/core
go test -c -o C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp\core-darwin-red.test ./internal/core
go test -c -o C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp\core-linux-green.test ./internal/core
go test -c -o C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp\core-darwin-green.test ./internal/core
```

All four cross-compilations passed. Darwin behavioral execution was not
available or claimed.

### Linux WSL execution boundary

Read-only probes found WSL Ubuntu and a functioning Linux runtime, but no
Unix Go or lint executable. The cross-compiled Go 1.26.5 Linux test binary was
therefore executed directly. Its working directory was
`/mnt/c/Source/GitHub/backlogit/internal/core`; `TMPDIR`, `TEMP`, `TMP`, and
`GOTMPDIR` were all set to the absolute workspace-contained
`/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp`.
The Go and lint cache paths used the corresponding contained `go-build` and
`lint` directories.

The first WSL invocation failed test-flag argument transport, exit 2, before
running tests. The second used `/bin/sh -c` to pass the following exact binary
selector, with the contained environment described above:

```text
/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp/core-linux-green.test -test.run=^TestReviewStash -test.count=1 -test.timeout=2m -test.v
```

The second invocation executed tests and exited 1. DrvFS blocked the
ancestor-rename prerequisite with permission denied while the parent pin was
held. The missing-basename fixture returned `ENOENT` from owned-handle
`Fstat`, unlike expected native Unix renamed-inode semantics. This is a concrete
contained-filesystem proof boundary, not a passing Linux selector.

The same execution passed replacement-sidecar successor, symlink, hardlink,
and directory cases; normal cooperating successor safety; direct close
parent-descriptor cleanup and leaf-close-error/no-double-close cases; the
source contract; and existing fresh/abandoned/live-aged and linked-sidecar
cases. No assertion was weakened or skipped, and no native Linux or Darwin
behavioral pass is claimed. Execution stopped at this boundary without a
third WSL attempt.

### Remaining review and validation risks

`Fstatat` followed by `Unlinkat` is not an atomic compare-and-delete.
Cooperating holders do not mutate a flock-owned inode: deletion completes
before a new path owner can return, and orphaned openers fail acquisition
identity checks. The regressions demonstrate an old owner cannot delete a
held replacement successor or a normal post-release successor.
Noncooperating writers with directory mutation authority can still exchange
the basename between the identity check and unlink. Observed drift fails
closed, and the retained parent prevents ancestor-symlink redirection, but
the advisory-lock contract does not eliminate that concurrent hostile-writer
window.

The production changes invalidate the former exact-source `f336bb21` full
pass and the immutable-source-head proof associated with `14faa257`.
Parent-owned follow-up must obtain fresh full gates, native Unix behavioral
evidence, source review, and current pushed-HEAD hosted CI/Copilot readiness.
Existing resolved remote threads are historical; no new reply or resolution,
Live MCP upgrade, merge, or closure was attempted.

## Latest authoritative Unix portability proof

Status: `blocked_review_gate`; fresh full-gate and review validation pending.
This section supersedes the earlier failed DrvFS selector verdict, not the
historical RED evidence or the outstanding native-platform proof requirement.
Local HEAD remains `14faa2575ed48c1b247801c7892baff443a5787c`.
One bounded corrective implementation attempt changed only
`internal/core/task_lock_unix.go`,
`internal/core/stash_sidecar_release_unix_test.go`, and this handoff.

The ancestor test accepts only `EACCES`, `EPERM`, or `EBUSY` when the retained
parent pin prevents rename. Before returning, that branch asserts successful
release, removal of the original sidecar, unchanged external victim bytes,
name and identity, no moved directory or parent symlink, closed leaf and
parent descriptors, and idempotent close without closing reused descriptors.
It is an assertion-bearing safety branch, not a skip or silent fallback.
When rename succeeds, the original rename-plus-external-symlink deletion
regression still executes unchanged.

Unknown owned-handle `Fstat` identity now refuses release with
`errors.Join(os.ErrPermission, err)`, preserving the underlying errno and
matching the existing `Fstatat` refusal category. The missing-basename test
retains its strong `ErrorIs(os.ErrPermission)` and unchanged victim assertions.
No replacement, cooperating-successor, or direct-close assertion was weakened.

### Exact commands and environment

Every Windows Go command used the existing cached Go 1.26.5 toolchain and
these absolute workspace-contained scratch/cache settings:

```powershell
$env:GOTOOLCHAIN='go1.26.5'
$env:TEMP='C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp'
$env:TMP=$env:TEMP
$env:GOTMPDIR=$env:TEMP
$env:GOCACHE='C:\Source\GitHub\backlogit\logs\.pr638-unix-release\go-build'
$env:GOLANGCI_LINT_CACHE='C:\Source\GitHub\backlogit\logs\.pr638-unix-release\lint'
$env:GOPROXY='off'
```

```text
go version
go test -run=^$ -count=1 ./...
go test ./internal/core -run '^TestReviewStash' -count=1 -timeout=2m -v
go vet ./internal/core
golangci-lint run ./internal/core/...
gofmt -l internal/core/task_lock_unix.go internal/core/stash_sidecar_release_unix_test.go
git --no-pager diff --check
```

* `go version`: `go1.26.5 windows/amd64`
* Before-edit repository compile-only baseline: PASS, exit 0, no tests run
* Corrected-source Windows selector: PASS, exit 0, core 1.667s; retained-parent
  and relative-unlink source contracts, deterministic close, real short-path
  new/abandoned/fresh/live-aged cases, both junction tests, linked-sidecar
  rejection, and existing ownership/recovery cases executed
* Windows core vet and focused core lint: PASS, exit 0, no diagnostics
* Scoped gofmt and diff whitespace checks: PASS, no findings; Git emitted
  existing LF-to-CRLF conversion warnings for stash and handoff
* Both edited Go files: LF-only bytes, exactly one final newline

An initial toolchain probe additionally set `GOSUMDB=off` and refused cached
toolchain verification before compilation. Removing that setting produced
the baseline PASS above; no Go/package installation was performed.

Linux cross-compilation and Unix-source vet used the same contained settings:

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
go test -c -o C:\Source\GitHub\backlogit\logs\.pr638-unix-release\tmp\core-linux-portability.test ./internal/core
go vet ./internal/core
```

Both commands passed, exit 0. The binary contains the corrected local source.
The existing selector was executed directly on the available WSL runtime:

```powershell
wsl.exe --list --quiet
wsl.exe -d Ubuntu-26.04 -- /bin/sh -c 'cd /mnt/c/Source/GitHub/backlogit/internal/core && export TMPDIR=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp TEMP=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp TMP=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp GOTMPDIR=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp GOCACHE=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/go-build GOLANGCI_LINT_CACHE=/mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/lint && /mnt/c/Source/GitHub/backlogit/logs/.pr638-unix-release/tmp/core-linux-portability.test -test.run=^TestReviewStash -test.count=1 -test.timeout=2m -test.v'
```

An initial invocation with `-d Ubuntu` returned
`WSL_E_DISTRO_NOT_FOUND` before tests. The read-only distribution listing
identified the registered name `Ubuntu-26.04`; only the invocation name was
corrected. No runtime installation or system configuration change occurred.

### Executed branches and preserved limits

The corrected WSL selector passed, exit 0, with no skips:

* Ancestor redirection: DrvFS retained-pin rename denial branch executed,
  logged `permission denied`, and passed every safety/resource assertion
* Native Unix successful-rename-plus-symlink branch: NOT executed; no native
  ext4 or Darwin behavior proof is claimed
* Replacement: successor, symlink, hardlink, directory, and missing basename
  all passed; missing basename now preserves permission-refusal semantics
* Normal cooperating successor and released-owner exclusion: passed
* Direct close: normal and forced leaf-close-error cases passed, including
  parent cleanup and no-double-close behavior
* Source contracts, deterministic close, fresh/abandoned/live-aged ownership,
  blocking recovery, and all linked-sidecar rejection variants: passed

The previous old-source source-shape RED remains the RED evidence. No new
behavior RED was manufactured where DrvFS prevents the attack itself.
The retained parent eliminates outside-ancestor redirection; basename
exchange between `Fstatat` and `Unlinkat` remains a documented
noncooperating-writer residual, not atomic compare-and-delete.
No F1 root-alias normalization was added. Future alias trust-anchor follow-up
`F2BC9C37` and accepted P2 residual `D3C8D13B` remain unchanged.

SHA-256 checks preserve all other pre-existing modified/untracked local
source files and `.backlogit/stash.jsonl`. No subagents, commits, pushes,
backlog, stash, checkpoint, branch, mount, or system-configuration operations
were performed. All test/build scratch stayed under `logs/.pr638-unix-release`.
Bounded proof is PASS; no fresh unfiltered full-suite or repository-wide gate
PASS is claimed. Fresh full gates, native successful-rename evidence, source
review, and current pushed-HEAD hosted readiness remain parent-owned.
