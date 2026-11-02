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
verification_verdict: PASS_WITH_FOLLOW_UP
---

## Resumed evidence handoff

The Orchestrator reports that the full Go suite completed in shell `468` with
exit 0, `GOTOOLCHAIN=go1.26.5`, and `GOFLAGS=-count=1 -p=1`. The
Orchestrator also reports native WSL focused runs in shells `406` and `407`
as PASS, and static gates as passed, including shell `479`. These are supplied
reports, not observations independently repeated for this documentation update.
They do not establish independent current-HEAD review readiness.

The prior `BLOCKED` verdict is superseded by `PASS_WITH_FOLLOW_UP` after the
round-3 source fixes were applied: `schedulerBaselineClaimKey` in the
mutable-field set, and `reconstituteDirectOwners` in the captured event IDs.
Stage P-021 C6 decisions `078-DL` and `080-DL` defer three crash-window
candidates to `7E14915F` and `D3C8D13B`; `079-DL` defers the format-cache
remedy to `DF533572`. Other follow-ups are `C2CFE06E` and `F2BC9C37`.
These decisions do not themselves establish that current-HEAD review or PR
gates have passed.

## Registry finding disposition

The prior review flagged the `reconstitute_shipment` CLI fallback in
`.autoharness/backlog-registry.yaml` as omitting `--dry-run`. The finding is
dismissed as a false positive. The operation maps `dry_run: "dry_run"` in its
`params` block, the same pattern `reorder_shipment` uses. The test
`TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario` in
`internal/cli/shipment_reconstitute_test.go` pins both the CLI command and that
param mapping. How the external consumer applies `params` at fallback time is
outside this repository. The dismissal relies on the pinned mapping and is
recorded as a residual in the review record.

## Contract and environment

Supplemental review found a Unix release-path safety gap after this evidence
was captured. A local retained-directory repair now changes source beyond
the validated artifact below. At the previous halt, fresh full validation and
native Unix successful-rename evidence were pending; the newer reported runs
above have not been independently observed here. Earlier probes are historical.

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
  verdict: PASS_WITH_FOLLOW_UP
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
    - independent assessment of the reported fresh full-gate evidence
    - independent assessment of the reported native Unix focused runs
    - current-HEAD review and applicable PR gates
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
Action result: earlier local verification and newer reported runs are recorded
above; actual merge remains subject to independent current-HEAD CI, review,
and operator-authorization gates.

## Releasability handoff

The merge-only workflow does not upgrade a running daemon. The closure record
must describe monitoring and rollback before an operator later adopts the
runtime. No new follow-up entry is needed: the existing residual retains its
stable stash identity, and optional unperformed automation is not represented
as successful execution.
