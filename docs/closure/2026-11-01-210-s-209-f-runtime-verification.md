---
title: "210-S reconstitution runtime verification"
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-11-01-210-s-209-f-runtime-verification.md
description: "Source-bound CLI probes and manual validation evidence for PR 638."
chunk_strategy: h1-h2-h3
shipment_id: 210-S
feature_id: 209-F
mode: pre-merge
verification_verdict: BLOCKED
---

## Contract and environment

Supplemental review found a Unix release-path safety gap after this evidence
was captured. A local retained-directory repair now changes source beyond
the validated artifact below. Fresh full validation and native Unix
successful-rename evidence are pending; earlier probes are historical.

The workspace validator manifest declares no surfaces and marks runtime
validation optional. The inferred CLI surface therefore uses the manual
adapter, with command observations and an operator-relayed test checkpoint.
No browser, deployment, or live upgraded MCP-daemon automation is claimed.

The fresh CLI artifact was built from
`f43d967f7affffde05daf533fd6e590f56041793`, with Go 1.26.5 and
`vcs.modified=false`. That commit has identical source and dependencies to
tested commit `f336bb21`. Build and runtime scratch were workspace-contained.

## Structured validator evidence

```yaml
validator_evidence:
  verdict: BLOCKED
  surfaces:
    - surface: cli
      adapter: manual
      manifest_entry: absent
      required: false
  probes:
    - command: backlogit shipment reconstitute --help
      expected: command, dry-run, force, and reason flags are registered
      observed: expected help and safety flags; exit 0
    - command: backlogit shipment reconstitute --plan not-created.yaml --dry-run
      expected: reject the absent input instead of applying a shipment change
      observed: contextual plan-path validation error; exit 1
  manual_checkpoints:
    - id: source-bound-full-suite
      actor: Orchestrator, relayed by operator
      source_commit: f336bb21
      command: go test -timeout=30m ./...
      toolchain: go1.26.5
      observed: all reported packages PASS; session 305 completed with exit 0
    - id: targeted-handler-and-storage-contracts
      observed: CLI parity and core review/stash regressions passed
      scope: recorded test evidence, not a new live MCP probe
  blocked_prerequisites:
    - fresh full-gate evidence for the retained-directory repair
    - native Unix successful-rename release validation
  unperformed_automation:
    - live reconstitution against an upgraded MCP daemon
    - deployment or executable installation
  follow_up_refs:
    - D3C8D13B
    - F2BC9C37
```

## Invariants and risk

Preserve recovery-before-snapshot ordering, ordered lock ownership, refusal
to steal fresh/live stash locks, short-path compatibility without following
reparse targets, committed WAL visibility, and zero-write preview behavior.
Storage and recovery invariants were exercised by the passing regression
suite, not inferred from the help probe.

Snapshot acquisition is bounded and may refuse a continuously changing
source. The equality bracket is not an unconditional ABA proof. Existing
created-artifact journal-window risk remains tracked in `D3C8D13B`.

Proposed action: validate and merge the reviewed runtime source.
Action risk: high because manifest recovery and locks protect persisted data.
Action result: locally verified; actual merge remains subject to independent
current-HEAD CI, review, and operator-authorization gates.

## Releasability handoff

The merge-only workflow does not upgrade a running daemon. The closure record
must describe monitoring and rollback before an operator later adopts the
runtime. No new follow-up entry is needed: the existing residual retains its
stable stash identity, and optional unperformed automation is not represented
as successful execution.
