---
title: "212-S M19 U3 amended-scope resumption"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave6-converged"
status: "complete"
---

## Custody and recovery

Operator Option 5 selected and confirmed `checkpoint-20261109-040035.json`.
Its conforming Ship state was restored, summarized after Engram CLI memory
retrieval, and only that checkpoint was resolved. The full enumeration had
161 summaries, no validation anomalies, and two active Ship checkpoints.
The older protected checkpoint remains untouched with SHA-256
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.

The fresh routing resolver validated config and returned Ship
`gpt-6.1-sol/openai/medium/long_context`. Backlog MCP and Engram CLI responded;
index sync indexed 2261 artifacts. No initial intake reconciliation ran.

Served-root handoff outcome: PASS; shipment `212-S`; selected checkpoint
`checkpoint-20261109-040035.json`; workspace `.`, storage `.backlogit`,
manifest `.backlogit/queue/212-S.md`, index `.backlogit/backlogit.db`;
handoff timestamp `2026-11-09T04:08:00Z`.
Independent MCP workspace metadata and main-database checks matched.

## Amendment and scheduler

Stage's four published file digests matched. The amendment note's own
observed SHA-256 was
`bc5b3567409c70b531d7b6f8a4d6396e4f3129c92354b9a1e7ff493cea9a86db`;
no expected self-digest was published. Commit `f19ab9b5` contains the five
changed amendment files. The requested prior halt record was already
tracked and unchanged.

Frozen S20/M19, all empty green-regression arrays, original counters and
the single unconsumed post-U7 full-suite authority were restored unchanged.
Live exact-ID status/dependency snapshot: 14 done, five active, zero queued,
blocked or unsupported. U3 alone is the authorized started cursor;
004-007 are claim-assigned with zero starts, waiting on dependencies.
The graph is acyclic. Remaining waves are 6=003 through 10=007.
Live status catalog and registry mapping were read; scheduler replay
passed 279/279 assertions across 38 scenarios. Compile-only and topology
passed.

Scoped P-012 declaration permits only read-only raw cursor log reads
`.backlogit/logs/211.003-T.jsonl` through `211.007-T.jsonl`. Windows
no-follow opens rejected reparse components and verified final opened
paths. All five logs have one shipment claim; U3 has exactly one start
and 004-007 have none. U3 was not reclaimed or restarted.

## Build and review

Freeze-scope mode, moderate risk, Option 5 authorized: production changes
only `internal/mcp/server.go` and `internal/mcp/session_mux.go`.
Committed behavioral harness `3377abd4` stays unchanged.

Build consumed two of five implementation attempts: one compile correction
and a junction-containment correction. Parent independently observed
20 top-level U3 tests plus 14 subtests passing with race enabled.
U3a, compile, vet, pinned lint and changed-source format passed.
Broad `gofmt -l .` exits 2 on ignored `bin/` content; no broad format
PASS is claimed and no cache files were modified.

Standard report-only review against HEAD `f19ab9b5` returned BLOCKED:
P0=0, P1=1, P2=4, P3=0. In-scope completions are exact-key argument
containment validation, nonblocking error dispatch for cancellation intake,
SDK protocol-envelope validation ordering, and numeric-ID preservation.
Unbounded pending workers under output backpressure require separate
admission-policy design and are out of scope under P-021 C1.

Required adversarial escalation ran report-only with three reviewers.
No unanimous findings; a majority P2 identified endpoint ownership on
pre-registration rejection, an in-scope completion. Unique observations
concerned parser differentials, clean-EOF semantics, numeric exponent
conversion cost and cancellation responses. Baseline identity/runtime
coverage was incomplete there; parent must bind the returned observations
to the unchanged reviewed diff and obtain focused verification, not treat
that report as approval.

Reviewed production working blob identities:
`server.go=ac42827ecfd7d17e54d6cb4780331e8f865c57c9`,
`session_mux.go=cb5c2c78b176b531fe20946ae7d9304b27881353`.
No source commit or task completion has occurred yet.

## Completion and convergence

The same-contract remediation completed on implementation attempt five.
Attempt three rechecked the baseline, attempt four exposed cancellation
timing, and attempt five corrected it. No sixth attempt is authorized.
The final report-only remediation review returned READY_WITH_FOLLOWUPS:
P0=0, P1=0, P2=1, P3=0. All five in-scope review findings were resolved.
The remaining P2 is captured as `D2FCA0FB`, with
`DISCOVERY-STATUS: LOOKUP-UNAVAILABLE`; active stash had no match, while
authoritative archived-stash and residual-risk discovery was unavailable.
The entry is capture-only and must be cited in eventual PR/closure records.

Focused ignored runtime probes passed six tests plus nine subtests:
real docs-handler junction shadowing, parser ambiguity, invalid protocol,
extreme numeric IDs/cancellation, blocked-output cancellation and rejected
endpoint cleanup. EOF remains disconnect/cancel/join, not a new half-close
contract; cancellation responses are allowed. Symbolic numeric exponent
conversion was not benchmarked and is not claimed to be performance-proven.

Source commit `bafdf3836b846bc1dce533175126b40348a63f52` contains exactly
the reviewed production blobs: server
`ac42827ecfd7d17e54d6cb4780331e8f865c57c9`, mux
`9da18b522e436ad854c01953085671343cf533d0`.
Commit association persisted and the governed done gate passed. The task
relocated from queue to archive with status done; both sides are staged
together for the lifecycle commit.

Wave 6 convergence passed repository compile, vet, pinned lint,
first-party source format and race-enabled U3. Broad cache traversal
format remains a separate known limitation, not a broad format PASS.
U2i was re-run and remained assertion RED only for unfinished CLI
daemon/prewarm wiring and generated references. No timeout or package
build failure was treated as expected RED.

`FULL_SUITE_DEFERRED: wave 6`

Open selector `^TestU2i_`, owner `211.015-T`, has remaining green-makers
004 (wave 7), 005 (wave 8), 006 (wave 9), 007 (wave 10). Task 003 is done.
Compile, vet, lint, source format and the declared scoped command passed;
the still-open selector was re-confirmed assertion RED. The full suite is
deferred rather than classified around its designed failure. The single
post-U7 unfiltered full-suite authority remains unconsumed.

## Remaining obligations

Runtime follow-up includes junction argument shadowing, cancellation
under blocked output, rejection cleanup and peer survival.

U2i remains accepted open RED, selector
`go test -count=1 -run '^TestU2i_' ./internal/cli`, green-makers 003-007,
closing wave 10. Original baseline
`30908ecb8438c0b3e7fe8bd514fa3680e35d90e5` is unchanged.
Waves 7-10 are not yet executed. No full suite, push, PR, merge, closure
or successor mutation occurred. All prior counters remain; current session
adds one completed task, five U3 build-loop observations and one review-fix
cycle, with zero Wave 6 convergence failures.

Unrelated hook, memories, historical checkpoints and prior final-handoff
changes remain outside exact-path staging.
