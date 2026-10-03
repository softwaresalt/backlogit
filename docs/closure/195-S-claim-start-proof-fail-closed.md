---
title: "195-S claim-start fail-closed proof"
description: "Provenance-checked fail-closed claim-start evidence; fixture-only inputs are quoted."
chunk_strategy: h1-h2-h3
doc_type: closure
schema_version: "1.0"
source: "docs/closure/195-S-claim-start-proof-fail-closed.md"
---

# 195-S claim-start fail-closed proof

Fixture-only note: all task and shipment mutations occurred in isolated ignored fixture workspaces; no live backlog mutation occurred during the proof window.

PROOF_SCOPE: 2A355F83
PROOF_BINARY_COMMIT: 2ba005fc4ad03f8be5765435b0ecbe49b412168f
PROOF_BINARY_DIRTY: false
PROOF_WORKSPACE_OVERRIDE: UNSET
PROOF_LIVE_STATE: UNCHANGED
PROOF_LIVE_MANIFEST_COUNT: 4074

ROW started-member-is-residual: PASS
Commands: fixture init and setup; comment add T1 WORK_STARTED: 001-S; get both members and read both logs; classify wave admission.
Output excerpt: 001.001-T is active residual because the current epoch has one WORK_STARTED record; expected halt is WAVE_NO_PROGRESS (active residual).

ROW marker-mismatch-is-residual: PASS
Commands: fixture init and setup; guarded single-line fixture edit; sync; get both members and read logs; classify wave admission.
Output excerpt: backup=C:\Source\GitHub\backlogit\logs\2a355f83-proof\195.007-T-20261003T012355Z\ws-marker-mismatch-is-residual\.backlog\queue\001.001-T.md.proof-backup; backup SHA-256=72A676A6E471C85782D605329D4D3AF74F0FCD5BE206EBB39161FB25852542D5; original SHA-256=72A676A6E471C85782D605329D4D3AF74F0FCD5BE206EBB39161FB25852542D5; edited SHA-256=B95D62897D71086E2817035E238977C4835DC7EA382D557C087FD5B59A4825E2; edited file was UTF-8 without BOM. 001.001-T reports marker OTHER-S and is an active residual.

ROW missing-log-is-indeterminate: PASS
Commands: fixture init and setup; rename T1 JSONL to T1.jsonl.withheld (never delete); get member state and classify wave admission.
Output excerpt: 001.001-T log is missing and retained at C:\Source\GitHub\backlogit\logs\2a355f83-proof\195.007-T-20261003T012355Z\ws-missing-log-is-indeterminate\.backlog\logs\001.001-T.jsonl.withheld; expected halt is WAVE_CLAIM_STATE_INDETERMINATE (log missing).

PROOF_RESULT: PASS