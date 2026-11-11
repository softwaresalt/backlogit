---
title: "212-S U1f producer follow-up RED witness"
doc_type: memory
schema_version: "1.0"
producer: "go-engineer leaf"
recorded_at_utc: "2026-11-07T04:11:37Z"
encoding: "UTF-8"
baseline_head: "779f1200"
approval_scope: "Same-review F1/F6/F7 harness remediation only"
supplied_assertion_baseline_sha256: "9d771c0d21acf6e679a8b8f572ea6196848418280a2dbc520c59e296d92f240c"
assertion_preservation:
  original_nodes: 45
  original_testify_calls: 40
  original_native_assertion_calls: 5
  preserved: true
  independent_token_serialization_sha256: "b8d7e0a165686f329c2c3c6bfe6c127406242e1d07556b52b790594de29f6050"
native_result:
  exit: 1
  run_events: 4
  skips: 0
  race_reports: 0
  panics: 0
  build_failures: 0
  test_timeouts: 0
unexpected_errors:
  - "Read probe requested lines 125+ of a 115-line file; subsequent full read succeeded"
  - "Literal search used nonexistent shipment_lifecycle_journal.go; shipment_ops.go located instead"
  - "Initial preservation checker counted only testify calls but expected 45; corrected checker includes five native assertions"
  - "Overbroad literal baseline-hash search in bin/tmp timed out; no retry or source change"
  - "Lock script refuses nonexistent new proof path; initial creation followed by acquisition for verification"
---

## Native witness

Every Go invocation sourced `bin/tmp/212-go-env.ps1`: local cached Go 1.24.0,
readonly offline modules and workspace caches. Follow-up commands used
TEMP/TMP `bin/_h8a2f10c4/t`; the initial baseline compile used the environment
script's workspace TEMP.

```powershell
. ./bin/tmp/212-go-env.ps1
$env:TEMP = Join-Path $PWD 'bin/_h8a2f10c4/t'
$env:TMP = $env:TEMP
go test -run=^$ -count=1 ./internal/core
go test -count=1 -race -timeout=30m -json -run '^TestU1f_' ./internal/core
go vet ./internal/core
& ./bin/golangci-lint.exe run ./internal/core --timeout=5m --out-format=json
gofmt -l internal/core/shipment_reconstitute_hardening_test.go
```

Native JSON ran from `2026-11-07T04:09:41Z`; both F1 cases emitted
`U1f F1 return-before-stash-release assertion` and remained blocked while
the active stash holder was live. Holders were released before joins.
F1 and existing subtest cleanup markers completed. F6 cancellation at the
global attempt/acquired barriers returned validation rather than cancellation;
the canceled global attempt also acquired global. Subtests 1 and 3 failed;
subtest 2 passed. Compile-only, scoped vet, pinned core lint (`Issues: []`),
formatting and scoped diff whitespace checks passed.

## Preservation and coverage bounds

All 45 original assertion call nodes survive with multiplicity. The supplied
baseline hash above identifies the caller's baseline; the independent lexical
token serialization uses a different encoding and is not claimed to reproduce
that hash. All other functions and source bytes outside the existing U1f
function are unchanged after newline normalization. No imports, top-level
functions, subtest names, guard or production changes were made.

F1 observes cancellation return before holder release, with bounded selects,
no sleeps/stress and no observation hooks. Launch acknowledgement does NOT
prove exact entry into `stashMu.Lock`, or precise failure-time entry.

F6 uses existing global barriers and a missing-artifact discriminator for
cancellation before preliminary validation; it does not observe each scan.
Final Markdown/JSONL snapshots and empty pending journals do not prove absence
of transient post-cancel writes. F7 changes file-backed YAML bytes under the
existing global hook, verifies different digests and rejects without changed
artifact/event snapshots or pending journals. Inline unsorted two-source input
holds the lower membership and probes the higher through the existing lock:
the higher remains free at lower-ID contention. Membership cancellation
returns before holder release and cleanup completes.

## Boundary

Commit scope is only the permitted test delta and this producer's RED proof.
No full-suite/broad core test, source fix, planning/status/claim, restore,
backlog/checkpoint, push or PR operation was performed. Historical convergence
failures and the immutable `210317` checkpoint remain untouched.
