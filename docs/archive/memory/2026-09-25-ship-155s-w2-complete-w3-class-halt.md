---
title: Ship 155-S W2 Complete; W3 Halted by P-002.4 YAML Surface
date: 2026-09-25
status: blocked
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 1a0641757b66094f5ba4d531d0d4bdeb53674f76
---

# Ship 155-S W2/W3 Continuation

## W2 completion

Resumed from the operator-selected Ship checkpoint
`checkpoint-20260925-054533.json`; backlogit `get_checkpoint` returned
`valid: true`. The operator resolved the prior doc-lint and hook-stream halts.
The Orchestrator added `doc_type: guide` and `source` to the scratch pickup
document. Its PRE_EXISTING_3 hash was re-baselined from
`3405c0888ef658d77816856b23703787a1a684dbc15edc5ea20dfc11874629ae` to
`03d8db6da70a8d93b78caff5e1a73c188cb37c3c0f33f0d1d36784266320b79d`; Ship
did not edit that document. Deferred entry `2F7FCA8B` remains unchanged for
Stage, and the existing tools-line entry `4A0B7BCF` remains Stage's.

174.075-T passed under the approved index-blob swap:

* Exact exempt command: exit 0, `EXEMPT_VERIFY_OK:174.075-T`; doc lint returned
  `valid: true`, zero findings.
* Required regression selector: exit 0, exactly two top-level PASS lines, no
  FAIL or SKIP.
* The `_ship.agent.md` backup at
  `logs/diagnostics/174075-operator-ship-agent-backup.md` was restored
  byte-exactly; backup and restored SHA-256:
  `a627c45a0fbd2386e6e5b1316961f0f99626de9933532b4f6d30a3c52f39aedb`.
  After W2 commit, its only diff from HEAD is the operator-owned tools line;
  the index is clean.

Commits:

* `6f6c34e134de3f643b0fd6cfd1a67ab0e83906b4` — `chore(core): archive completed task 174.074-T`
* `e422d319` — `chore(core): record archive commit for 174.074-T`
* `3e0fcb8cacb11c0b929adfad187551de20f15602` — `docs(core): pin 30m full-suite budget`
* `1a0641757b66094f5ba4d531d0d4bdeb53674f76` — `chore(core): archive completed task 174.075-T`

174.075-T is done and archived. No full-suite command was run.

## W3 174.076-T halt

The exact W3 exempt pre-work command exited 1 without a marker at both Ship
Step 4.1a and build-feature Step 0. After the manifest edit, the exact command
passed with `EXEMPT_VERIFY_OK:174.076-T`; the required Go 1.24.0 regression
selector passed with exactly one top-level PASS and no FAIL or SKIP.

The W3 deliverable changes only `.autoharness/harness-manifest.yaml`, updating
the two LF-normalized checksums and appending the `073-DL rev22` sentence to
each existing drift reason. The fail-closed P-002.4 path gate then rejected
the YAML manifest: `docs-only` permits Markdown, instruction, prompt, and
agent artifacts, not this YAML metadata file. No classification was relaxed,
and no W3 commit or push was made. The manifest change remains uncommitted;
174.076-T remains active for Stage to amend the plan/classification.

The W3 operator-file backup at
`logs/diagnostics/174076-operator-ship-agent-backup.md` was restored
byte-exactly, SHA-256
`a627c45a0fbd2386e6e5b1316961f0f99626de9933532b4f6d30a3c52f39aedb`.
`git diff HEAD -- .github/agents/_ship.agent.md` contains only the operator
tools-line change; `git diff --cached` is empty.

## W-174074-ext and stop state

The delta report is `logs/diagnostics/155s-w2w3-delta-report.json`.
`.backlogit/hooks_queue.jsonl` was admitted to W-174074-ext exclusion class
(c) only after verifying the original 910,051 snapshot bytes are an exact
prefix of the current file and all appended lines are well-formed hook events.
At this checkpoint, suffix sequence numbers 3199-3201 validate; no malformed
suffix lines were found.

The branch is at `1a0641757b66094f5ba4d531d0d4bdeb53674f76`, eight commits ahead
and zero behind. No push occurred because W3 did not pass its P-002.4 path
gate. Do not run the governed full suite; the task contract requires Stage to
resolve the YAML classification first.

## Checkpoint and final preservation verification

Created `checkpoint-20260925-202625.json` in blocked-W3 phase after strict
JSON pre-validation (payload SHA-256
`1629fcf8fdfa2dc9ba64d9c92337415555f5c1d3a63179cddcdf979f825fc81f`).
`backlogit checkpoint get` returned `valid: true`. Only after that successful
resume checkpoint was persisted, the selected
`checkpoint-20260925-054533.json` was resolved successfully. No other
checkpoint was resolved.

Rechecked `.backlogit/hooks_queue.jsonl` after checkpoint operations: its
first 910,051 bytes still hash to the PRE_EXISTING_3 snapshot value
`0d1b8e7e1a0a22345264801a1200d1f3a124724005fceff02bab842805f880b9`; the
771-byte suffix is the same three valid events (seq 3199-3201), with no
malformed lines. The W-174074-ext append-only condition remains satisfied.
Updated reports are
`logs/diagnostics/155s-w2w3-delta-report.json` and
`logs/diagnostics/174076-p0024-delta-report.json`.

The independent W3 swap evidence is recorded in
`logs/diagnostics/174076-index-swap-evidence.json`: the backup and restored
working `_ship.agent.md` SHA-256 both equal
`a627c45a0fbd2386e6e5b1316961f0f99626de9933532b4f6d30a3c52f39aedb`;
the only working-tree diff from HEAD is the operator-owned `tools:` line,
and the index is empty. The PRE_EXISTING_3 whole-file hash for that path was
`84ae9310879a4593534e9633ba25664fb228cfad6afd1cbb5c0d8caf58c0ca88`; it
predates the committed W2 changes, so the preserved operator edit is proven
by the exact swap-backup hash and the tools-line-only diff rather than by
whole-file hash equality to that earlier snapshot.

W3 remains uncommitted and active for Stage. No push or repository-wide suite
was run; stop here pending Stage's resolution of the docs-only/YAML surface
conflict and separate full-suite authorization.
