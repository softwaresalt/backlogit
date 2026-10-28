---
doc_type: memory
schema_version: "1.0"
title: "Ship session: 190-S generic-path queued-abandon guard"
session_date: 2026-10-19
agent: ship
---

# 190-S Ship session (dark mode, P-017)

* Shipment 190-S claimed on branch `feat/generic-path-queued-abandon-guard-and-governed-edge-b1` (served roots attested; pipeline-topology pre_claim and post_claim passed).
* Tasks 190.001-T to 190.005-T built in dependency order (B-U3 to B-U7), each with a WORK_STARTED record; all completed and archived.
* B-U4 audit: no production caller moves a queued shipment to abandoned; closed as a recorded no-op.
* Local review: multi-reviewer pass (bypass, correctness/tests); P3 findings fixed in b6325aca (trimmed provenance, extra edge tests).
* Deferred (P-021 C2, captured as stash at closure): disposition_* custom fields can still be set on a queued shipment by a generic UpdateArtifact (AC3 of 190.005-T is enforced for the status edge only).
* Environment: MCP times out under load; CLI used after Served-Root Attestation; transient lifecycle-gate contention retried.
