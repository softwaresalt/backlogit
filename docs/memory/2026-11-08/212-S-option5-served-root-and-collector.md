---
title: 212-S Option 5 recovery and collector gate evidence
shipment_id: 212-S
task_id: 211.016-T
checkpoint_filename: checkpoint-20261108-194645.json
---

## Authorization and recovery

The operator's explicit "Option 5" authorizes the bounded collector follow-up,
selected-checkpoint restoration, Cycle 4 review, and guarded continuation.
The selected Ship checkpoint was valid and conforming, restored with a bounded
Engram-assisted summary, and resolved through MCP at 2026-11-08T21:12:02Z.
The unrelated active checkpoint was not selected or mutated.

Protected checkpoint SHA-256:
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
The startup consumer enumeration returned 154 records with zero observed
anomalies; this is not a claim to have independently enumerated the operator's
215-record historical census. An absent optional ResumeHint was not an anomaly.

Engram daemon readiness was GREEN with the expected workspace binding.
Bounded retrieval returned three largely unrelated records; the selected
checkpoint and exact handoff remain authoritative for this cursor.
No durable history, checkpoint pointer, counter, or gate verdict was pruned.

## Served-root handoff outcome

```yaml
handoff_status: PASS
shipment_id: 212-S
checkpoint_filename: checkpoint-20261108-194645.json
served_workspace_root_rel: .
served_storage_root_rel: .backlogit
manifest_path_rel: .backlogit/queue/212-S.md
index_file_rel: .backlogit/backlogit.db
scoped_p012_raw_read_declaration: Orchestrator Served-Root Handoff Procedure step d static frontmatter verification of .backlogit/queue/212-S.md
timestamp_utc: 2026-11-08T20:49:15Z
```

Ship independently checked the MCP workspace catalog and the single main row
of `pragma_database_list`, including before the guarded item-log reads.
The main database is a direct child of the attested storage root.
The existing matching shipment branch was retained throughout.
The lifecycle topology gate passed with exactly one active shipment, 212-S,
and one implementation worktree.

## Collector verification

The collector replaces repeated PowerShell ancestor metadata cmdlets with
.NET metadata calls and fresh per-observation directory memos.
Policy fingerprints also use independent memos.
The native DIR_SUMMARY correction remains; ordinary FILE paths remain strict.

| Gate | Observed result |
|---|---|
| PowerShell AST parser | PASS |
| Static scheduler simulation | PASS, 257/257 assertions |
| Exact scoped race integration command | PASS, 59/59 named and 34/34 real-Git cases |
| Live Capture | PASS, 165.322 seconds within the 300-second bound |
| Repository compile-only suite | PASS, native 0 |
| Repository vet | PASS, native 0 |
| Pinned lint | Default timeout; unchanged retry with `--timeout 10m` PASS |
| Changed Go file gofmt | PASS, no findings |
| UTF-8 LF and final newline | PASS on all three modified files |
| Git diff whitespace | PASS |

The live capture is not a few-second result. Observation 1 hashing took
76.972 seconds and Observation 2 hashing took 7.494 seconds.
Retained phase timings and snapshot witnesses are in
`bin/_u2j_perf_20261108_211219/`.
The final receipt SHA-256 is
`7e2ef99a5272c22085fcaf5ca608a708615532f6e96102498dedbc30c462e013`.

## Cycle 4 local review

The standard report-only review ran first with Constitution, Go, Security,
Concurrency, Template Integrity, and coordinator-assisted learnings coverage.
It returned READY with P0/P1/P2/P3 all zero.
Indexed PowerShell symbols and some Go relationships were unavailable;
exact-source and diff checks supplied degraded structural coverage.

The three-model report-only adversarial pool was:

* Anchor: OpenAI GPT-6.1 Sol, high reasoning
* Reviewer A: Anthropic Claude Haiku 5.5, high reasoning
* Reviewer C: Anthropic Claude Sonnet 5.5, high reasoning

The pool returned no P0/P1 defects, one majority minor advisory, and five
unique minor advisories. These are retained as six P3 residual-risk notes,
not represented as fixed defects or omitted from the readiness record.
Combined readiness is READY_WITH_FOLLOWUPS for the reviewed collector diff.
No new implementation cycle was opened.

| Advisory | Handling |
|---|---|
| Non-directory ancestor refuses rather than recording absence | Accepted fail-closed behavior; no demonstrated acceptance-contract defect |
| Historical source hashes alongside final hashes | Historical sections are superseded observations, not current reviewed identities |
| Dense historical timing/process evidence | Retained for custody; no history deletion or compaction expansion in this work unit |
| Conservative tilde-number alias rejection | Expressly required 8.3/device-alias refusal; no weakening |
| Directory-summary handling in worktree enumeration | Unreproduced advisory; native policy-summary case passes; retain fail-closed behavior |
| Additional alias/memo branch coverage | Optional strengthening remains a residual risk, not a claim of exhaustive race coverage |

Review tools automatically spooled oversized output into ignored `bin/tmp`.
The adversarial coordinator therefore marked literal zero-artifact compliance
invalid. There were no intentional reviewer source, backlog, artifact, or
remediation writes. This infrastructure side effect is disclosed, not silently
represented as zero filesystem activity.

Reviewed diff base: `7f3fd6694e67e962bec21544d344f10f4949e6fc`.
Reviewed raw source identities:

| Path | SHA-256 |
|---|---|
| `scripts/wave-scheduler-sim.ps1` | `a03d88b1c90f1e50c075bd01599b7ad5af851379a531b3826d9078fac290659e` |
| `tests/integration/claim_start_wave_sim_contract_test.go` | `e296dd9bedd09d019f73df220c24606f6d415a2c35c922222d7d3e396aa2b086` |
| `docs/memory/2026-11-08/212-S-U2j-C1/manifest.md` | `1ee79c56eb439b0de8080537b3cf900e9b3969631976ca9adc1c1b52b87825ec` |

These identities do not establish current-HEAD PR readiness after later commits.
Runtime follow-up remains required: independent Task015 pre/post comparison,
assertion RED acceptance, and pending Wave 3/4 convergence.

## Preserved scheduler cursor

Frozen release membership remains S17/M16 with feature 211-F excluded.
Ten task members are done; Task015 is active and Tasks003 through007 are
claim-assigned and unstarted. Wave 3 and Wave 4 remain PENDING.
Historical failure counters and the single post-U7 full-suite authority remain
unchanged. No unfiltered full suite was executed during this collector work.

Scoped P-012 raw-log reads were declared for `.backlogit/logs/211.015-T.jsonl`
and `211.003-T.jsonl` through `211.007-T.jsonl`.
Safe Windows no-follow handle reads checked each ancestor and target,
rejected reparse attributes, matched canonical opened targets, and verified
both-root containment before strict JSONL parsing.
Task015 has exactly one current-epoch and all-log ship start record:
`WORK_STARTED: 212-S`, timestamp 2026-11-08T02:20:06.051045Z.
Tasks003 through007 each have zero starts and authoritative shipment-claim
events. No start was appended, deleted, or replayed.
Original red baseline remains
`30908ecb8438c0b3e7fe8bd514fa3680e35d90e5`.
Item logs are git-ignored; their absence from that historical Git tree is
not a requirement to synthesize or restore baseline log files.

No shipment disposition, successor claim, PR creation, merge, or post-merge
closure occurred in this recovery phase.
