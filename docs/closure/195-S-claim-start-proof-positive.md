---
title: "195-S claim-start positive proof"
description: "Provenance-checked positive claim-start evidence; fixture-only note #1 is quoted."
chunk_strategy: h1-h2-h3
doc_type: closure
schema_version: "1.0"
source: "docs/closure/195-S-claim-start-proof-positive.md"
---

# 195-S claim-start positive proof

Fixture-only note: all task and shipment mutations occurred in isolated ignored fixture workspaces; no live MCP operation occurred during proof steps 2-8; live backlog state remained unchanged.

PROOF_SCOPE: 2A355F83
PROOF_BINARY_COMMIT: 9f8d77569f254a0aae7b012e8a858a4f7cb36f68
PROOF_BINARY_DIRTY: false
PROOF_WORKSPACE_OVERRIDE: UNSET
PROOF_LIVE_STATE: UNCHANGED
PROOF_LIVE_MANIFEST_COUNT: 4072

ROW claim-marks-members: PASS
Commands: fixture init; add feature and two tasks; dep add T2 T1; shipment create and claim; shipment list --status active --format json; get shipment and both tasks; read both item JSONL logs.
Output excerpt: both tasks are active, manifested in order, marked 001-S, and have one claim event each with zero start records.

ROW wave1-admits-claim-assigned: PASS
Commands: fixture init and setup; read shipment and member frontmatter; read both JSONL logs; classify wave 1.
Output excerpt: ready_k={001.001-T}; claim-assigned waiting={001.002-T} because its dependency is nonterminal; active residuals=0; no WAVE_NO_PROGRESS or indeterminate result.

ROW start-recorded-once: PASS
Commands: fixture init and setup; apply Step 4.1b twice with comment add WORK_STARTED; get T1 and read its JSONL log.
Output excerpt: exactly one valid WORK_STARTED: 001-S record in the current epoch; status remains active; status_changed count remains 1.

PROOF_RESULT: PASS
