---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
conditions:
  - id: live-ship-session-halt-rows
    description: "Live Ship sessions (_Ship custom agent, one per row, told 'ship 990-S; STOP before any claim — do not claim, do not mutate') halt CONDITION_B_UNSATISFIED with the row-specific reason for rows queued, abandoned and shipped-unattested; the shipped-attested proceed row clears condition (b) and stops before claim."
    satisfied: true
    evidence: "Runs 2026-10-20T19:19:13-07:00 to 19:19:16-07:00, CLI-only with --cwd on each row root under logs/af1e5075-fixture/ (storage_root asserted as <row root>\\.backlogit via metadata catalog; 176-S not found). queued: 154-S status queued, halt CONDITION_B_UNSATISFIED. abandoned: .backlogit/archive/154-S.md archived_status abandoned, halt CONDITION_B_UNSATISFIED. shipped-unattested: 154-S shipped, .backlogit/logs/154-S.jsonl has no CONDITION_B_ATTESTED comment, halt CONDITION_B_UNSATISFIED. shipped-attested (proceed): operator attestation 2026-10-06T23:31:36.0953115-07:00 after shipped event 2026-09-30T08:44:25.1831744-07:00, proceed, stopped before branch gate; rerun 2026-10-20T19:28:10-07:00 read both evidence files only through logs/safe-read.ps1 (no-follow open, opened path verified: SAFE_READ_OK for fixture queue\\154-S.md and logs\\154-S.jsonl) and reached the same proceed. The three halt rows fail closed on (P) and used plain Markdown reads; the helper is exercised on the proceed row where a safe read decides the outcome. No session claimed, created a branch, ran a gate or edited a file; fixture queue and archive file timestamps unchanged and live git status clean after the runs. See Live evidence (186.007-T)."
  - id: orchestrator-dry-routing-pass
    description: "Orchestrator dry routing pass for 193-S executes Step 2 item 1 (hold-label check and condition (b) check) and stops before TOPOLOGY_GATE: pre_claim."
    satisfied: true
    evidence: "_Orchestrator rerun 2026-10-20T19:28:12-07:00 on the live workspace (the 19:19:52 first run is superseded: its Get-Content reads were not a no-follow open, per PR #558 Copilot review). Every evidence read went through logs/safe-read.ps1 (FILE_FLAG_OPEN_REPARSE_POINT, reparse-component rejection, opened final path equals canonical target): SAFE_READ_OK opened=C:\\Source\\GitHub\\backlogit\\.backlogit\\archive\\154-S.md and SAFE_READ_OK opened=C:\\Source\\GitHub\\backlogit\\.backlogit\\logs\\154-S.jsonl. Root binding: metadata catalog storage_root C:\\Source\\GitHub\\backlogit\\.backlogit and a single pragma_database_list row for the storage-root backlogit.db. 193-S queued with no labels (no refusal); blocks-edge closure includes 186-S and 154-S; (P) archived_status shipped; (C) operator attestation 2026-10-06T23:31:36.0953115-07:00 after shipped event 2026-09-30T08:44:25.1831744-07:00, no revocation. Result proceed. No gate run, claim, routing to Ship or mutation."
  - id: fixture-directory-removal-disposition
    description: "Disposition, not removal: removal of the logs/af1e5075-fixture/ row directories is deferred to the operator, who has not approved deletion. The retained fixtures are git-ignored and harmless, and no fixture process is running."
    satisfied: true
    evidence: "Fixtures retained and git-ignored (git check-ignore logs/af1e5075-fixture matches); nothing deleted. Removal remains an operator action tracked by stash CBB7997C (created 2026-10-21T02:20:30Z)."
description: "Post-merge closure and compound refresh for 186-S (condition (b) pre-claim hold-label and attestation harness contract)."
doc_type: closure
docline:
  date: 2026-10-18T19:00:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 186-S
schema_version: "1.0"
shipment_id: 186-S
source: docs/closure/186-S-186-F-post-merge-closure.md
title: "186-S closure and compound refresh"
---

# 186-S closure and compound refresh

Context: feature 186-F (stash AF1E5075, 074-DL L1), PR #533, merge `95b2b91240004a8e0e8698ea4a1ff6dc7de5f521`. Contract-only change: three source-shape tests, Orchestrator Step 2 and Ship Step 0.5 text, and a manifest drift record. No runtime surface, so runtime-verification is not applicable.

## Releasability

`closure_status: READY_WITH_CONDITIONS`. Monitoring: none (agent-contract text). Rollback: revert PR #533. Owner: operator. Validation window: next routed shipment claim.

Conditions (recorded in the frontmatter `conditions:` list; evidence in the Live evidence section below):

1. Live fixture-bound Ship sessions per halt row: run and passed on 2026-10-20 (earlier hand simulations are superseded).
2. Post-merge Orchestrator dry routing pass for `193-S` (stops before `TOPOLOGY_GATE: pre_claim`): run and passed on 2026-10-20 (rerun with safe reads).
3. Fixture row directories under `logs/af1e5075-fixture/`: the operator has not approved deletion, so removal is deferred. The fixtures are retained, git-ignored and harmless, and removal stays an operator action tracked by stash `CBB7997C`. This condition is a disposition, not a removal.

## Live evidence (186.007-T)

Ship sessions used the installed `_Ship` agent, CLI-only (`backlogit.exe --cwd <row root>`, no backlogit MCP tool, no live-workspace access). Each session asserted `storage_root=<row root>\.backlogit` and that `176-S` returned not found, then ran Step 0.5 item 3.1 for `990-S`.

| Row | Fixture state of `154-S` | Observed | Reason |
|---|---|---|---|
| `queued` | `status: queued` | `CONDITION_B_UNSATISFIED` | not shipped |
| `abandoned` | `archived`, `archived_status: abandoned` | `CONDITION_B_UNSATISFIED` | archived without `archived_status: shipped` |
| `shipped-unattested` | `shipped`, log has no attestation | `CONDITION_B_UNSATISFIED` | shipped without a valid attestation |
| `shipped-attested` (proceed) | `shipped`, operator attestation after the shipped event | condition (b) satisfied, stopped before the branch gate | valid attestation |

All four sessions ran 2026-10-20T19:19:13-07:00 to 19:19:16-07:00. None claimed, created a branch, ran a gate or edited a file. The `shipped-attested` row is a copy of `shipped-unattested` plus one fixture attestation comment (timestamp `2026-10-06T23:31:36.0953115-07:00`, actor `operator`, leading line `CONDITION_B_ATTESTED: 154-S shipped 2026-09-30T15:44:25Z; fixture attestation for AF1E5075 proceed-row verification`).

The Orchestrator dry routing pass was rerun at 2026-10-20T19:28:12-07:00 against the live workspace for `193-S` and returned proceed: no labels, blocks-edge closure includes `186-S` and `154-S`, (P) archived with `archived_status: shipped`, (C) the operator attestation of `2026-10-06T23:31:36.0953115-07:00`. It stopped before `TOPOLOGY_GATE: pre_claim`. The first run (19:19:52) read the evidence with `Get-Content`, which is not a no-follow open with opened-path verification, so it was superseded: the rerun read both files only through `logs/safe-read.ps1` (no-follow open, reparse-component rejection, opened path equals canonical target) and recorded `SAFE_READ_OK` for `.backlogit/archive/154-S.md` and `.backlogit/logs/154-S.jsonl`. The proceed fixture row was rerun the same way at 19:28:10-07:00.

Observation: the agents noted that the CLI prints `workspace initialized` on every call and they did not diff the fixture index. Fixture Markdown files kept their earlier modification times and the live working tree stayed clean. The three halt rows halted on (P) (fail closed) and used plain Markdown reads; the safe-read helper was used on the proceed row and the Orchestrator pass, where the read decides the outcome.

## Compaction status (P-020)

`compaction_status: degraded`. compact-context ran with `target: all`, but its proposed archival of 30 memory files was reverted before this PR merged: live consumers (`.backlogit/queue/072-DL.md`, resolved checkpoints, `docs/memory/2026-10-04/...`, `docs/archive/plans/...`) still reference the original paths. `docs/memory/` is unchanged by this closure. The compaction defect is captured as a follow-up stash.

## Halt-row and proceed-row evidence (186.007-T)

* Halt rows `queued`, `abandoned`, `shipped-unattested` ran in isolated CLI-only fixtures under `logs/af1e5075-fixture/<row>/` (`--cwd <row root>`, storage root asserted as `<row root>\.backlogit`, `154-S` and `176-S` not found before seeding). Each halted `CONDITION_B_UNSATISFIED` with its row-specific reason (not shipped; archived without `archived_status: shipped`; shipped without attestation).
* Existing attestation on `154-S` (read-only check): timestamp `2026-10-06T23:31:36.0953115-07:00`, actor `operator`, leading line `CONDITION_B_ATTESTED: 154-S shipped 2026-09-30T15:44:25Z; ...`. It is after the `154-S` `shipment_status_changed` event to `shipped` (2026-09-30T08:44:25-07:00 equals 15:44:25Z). The 186-S claim was authorized on this basis.
* L1 covers agent-mediated claims only. Direct claims depend on L2 for (P). (C) stays an operator attestation until a recorded decision says otherwise.

## Source artifact provenance (untouched)

`custom_fields.source_stash_id` AF1E5075 is provenance only and was not mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept: the 186-S claim, five task moves and the ship all timed out client-side and converged server-side, as documented. Observation only: backlogit MCP calls queue behind a busy server (a concurrent `go test` made every call exceed 60 seconds), so poll state rather than re-issuing.

## Follow-ups

* Stash `E62ED538` (DEFERRED SCOPE EXPANSION): step 4a retry and resume-intake scope of item 3.1.
* Stash `CBB7997C`: operator-approved removal of the retained fixture directories.
* Residual P3 items listed in PR #533.
