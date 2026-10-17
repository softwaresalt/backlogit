---
chunk_strategy: h1-h2-h3
description: 'Design note for the S11 deterministic workflow-policy enforcement engine (145-S / 163-F): the policy prerequisite graph, commit-SHA-bound evidence, fail-closed evaluator, authenticated audited overrides, surface parity and shipment roll-up'
doc_type: design
title: "S11 Workflow-Policy Enforcement Engine"
source: docs/design-docs/s11-workflow-policy-enforcement-engine.md
schema_version: "1.0"
docline:
    date: 2026-10-17T00:00:00Z
    status: accepted
    tags:
        - workflow-policy
        - enforcement
        - fail-closed
        - overrides
---

# S11 Workflow-Policy Enforcement Engine

`internal/policyengine` turns prose workflow gates (P-002/P-004 test-first, review,
CI, merge readiness) into a machine-enforced prerequisite graph with commit-SHA-bound
evidence. It consumes the S10 core (`internal/evidencedag`) as a library: the registry
is decoded with `evidencedag.DecodeStrict`, validated as an `evidencedag.Declaration`,
evaluated by `evidencedag.Gate` and rolled up with `evidencedag.Report.Rollup`. The
engine adds only the policy node types, their evidence rules, the override policy and
the shipment roll-up policy. A guard test fails if the package grows its own traversal,
cycle, reachability or roll-up functions.

The shipped registry is `.github/policies/workflow-policy-registry.yaml`. It rolls out
**report-only** first.

## Core extensions (S10 package)

Three additive changes to `internal/evidencedag`, each tested there:

* `DecodeStrict(data, v)` exposes the strict JSON/YAML decoder `Load` uses (bounds,
  fold-aware duplicate keys, no anchors/aliases/merge keys, no trailing data).
* `FamilyWorkflowPolicy` joins the closed family set. Its nodes have no S4 artifact.
* `Input.Verdicts` / `GateInput.Verdicts` let a caller supply the own-evidence outcome
  (`verified`, `unmet`, `error`) of a `workflow-policy` node. Verdicts for any other
  family, for undeclared nodes, with an unsupported status, or alongside evidence or an
  executor failure are `ErrInvalidInput`. A node with no verdict has no S4 evidence
  family and evaluates as `error`. Applicability, waivers and upstream propagation still
  apply, and the verdict is part of the cache key.

## Graph and registry (U1, 163.001-T)

Six node types in canonical order: `harness-scaffold` -> `red-phase-verify` ->
`green-phase-verify` -> `review-complete` -> `ci-pass` -> `merge-ready` (the only
terminal). A registry (JSON or YAML, `schema_version: 1`) declares an explicit `mode`
(`report-only` or `enforce`), each node once with explicit `enabled` and `overridable`
flags, and the ordering edges. `LoadRegistry` rejects: empty or nil graphs, duplicate or
unknown node types, missing nodes, an omitted `enabled`/`overridable`, a disabled
`merge-ready`, a missing canonical edge, and (through the core) dangling, self,
duplicate and cyclic edges. Input is capped (256 KiB, 64 nodes, 32 edges) and error
text is bounded and sanitized.

A disabled gate becomes `not-applicable` through the core applicability predicate
(`gate.<id>` shipment attribute). Because the core rolls up terminal nodes only and a
not-applicable node does not bridge propagation, the engine additionally requires every
enabled gate to be satisfied: a failure upstream of a disabled gate still blocks.

## Evidence and evaluation (U2, 163.002-T)

A `Record` binds `task_id`, `node`, `commit_sha`, `recorded_at` and `outcome` with a
SHA-256 digest over every field (the artifact is digested in compact form). Bundles are
JSON only and strictly decoded; they cannot carry an operator, interactivity or
override field.

`Engine.Check` verifies, per enabled gate: exactly one record; shape, task/node binding
and digest; the commit is an ancestor of the head commit; `recorded_at` is not before
the commit time and not in the future (5 minute skew); and, for each direct predecessor
with verified evidence, that the successor commit descends from the predecessor commit
and was recorded no earlier. Failures carry a structured `Violation{code, node, reason}`;
propagated failures are `upstream_unmet` / `upstream_unverifiable`.

* **Protected refs.** The head commit must be reachable from a configured
  `refs/remotes/...` ref. Local branches are never accepted as protected refs, and when
  none resolves the decision is unverifiable. A local-only head is `head_unpublished`.
* **red-phase-verify** consumes the S9 `red_green` artifact: it must validate through
  `faultline.DecodeAndValidate`, be bound to the task, the record commit and the payload
  head SHA, and its status decides the outcome.
* **review-complete / ci-pass** come only from the injected `Derivation` interface,
  implemented by `internal/evidence.GitHub` (the S7 single derivation source). Self-
  asserted records for them are refused, a degraded source blocks, and the derived head
  SHA and PR number must match the request.
* **Rollout.** `enabled` is per gate. `report-only` keeps the truthful verdict
  (`review_ready`) and the full error list and only relaxes `allowed`. A malformed
  request is an error and a blocked decision in every mode. `Options.Mode` may tighten
  but never relax the registry mode.

## Overrides (U3, 163.003-T)

An override waives exactly one `overridable` gate (P-002.1 harness-exempt applies to
`harness-scaffold` and `red-phase-verify`). Authority is a signed, short-lived, out-of-
band operator token verified by the S10 `WaiverService`: an ed25519 key held by a human
principal in a trust file outside the workspace, bound to the task, gate and head commit,
valid for at most 24 hours, with a single-use nonce. Automation keys are denied.

* The request has no operator-identity field; the principal is read from the verified
  key. A self-asserted identity anywhere (bundle, token body) is ignored or rejected.
* Tokens are honoured only from an interactive CLI (stdin and stdout are terminals) or
  when `AllowNonInteractiveOverrides` is explicitly set. MCP and agent surfaces deny
  them by default.
* Tokens for non-overridable gates are denied before verification, so the nonce is not
  consumed. A waiver masks only its own gate: the engine still blocks on any other
  failing gate.
* Every accepted override is appended to the S10 hash-chained ledger, anchored in an
  external anchor store, recording principal, justification and gate.
  `VerifyOverrideLog` (CLI `policy verify-overrides`) detects edited, reordered,
  extended, truncated, emptied or deleted logs (`evidencedag.ErrLedgerTampered`).
  Re-presenting the identical token is an idempotent retry bound to the same task, gate
  and head; a different token reusing a consumed nonce is denied.

## Surface parity (U3)

`Engine.Check` is the single evaluation entry point. `Engine.CheckBundle` is the shared
thin adapter for JSON-carrying MCP and agent callers. The CLI is `backlogit policy
check` (and `policy verify-overrides`). The verdict never depends on the surface;
`policyengine` tests run every scenario on every surface and compare the decisions, and
a CLI test compares the CLI output with the shared engine. No MCP tool is registered:
the S7-S10 packages have none, and an MCP tool would be a thin `CheckBundle` call.

## Shipment roll-up (U4, 163.004-T)

`Engine.CheckShipment` evaluates every manifest task with `Check` and rolls them up with
the core `Report.Rollup`: each task is a terminal node. The shipment is review-ready only
when every task is ready. An empty manifest, a missing task (`task_missing`), an extra
task, duplicate IDs and any malformed task block. `ShipmentDecision.Verdict` and
`Contribution` plug the roll-up into the S10 DAG as the terminal node `workflow-policy`
through `evidencedag.Compose`, so the one authoritative review-ready gate sees it.

Incident replays are fixtures in `internal/policyengine/testdata/incidents`:
`INC-P002-131S-148F` (implementation without harness or red evidence) and
`INC-P002-152F-134S` (evidence recorded out of order) must block, and a control
scenario must pass.

## Known limitations

* **Library wiring.** The roll-up is not yet called from `backlogit shipment ship` or
  the move-to-review transition, and no MCP tool or agent dispatch calls `CheckBundle`
  yet; `policy check` is the only wired surface. Wiring each transition is a follow-up.
* **Evidence authenticity.** `harness-scaffold`, `green-phase-verify` and `merge-ready`
  records are integrity-checked and tied to git history, not signed. An actor who can
  write the evidence file can attest to a real, correctly ordered commit. Only
  `red-phase-verify` (S9 artifact) and the derived gates carry independent proof.
* **Remote refs are local mirrors.** Protected refs are `refs/remotes/...`, which a
  local user can move with `git update-ref`; run `git fetch` first and treat the check as
  a CI-side gate. A server-side check against the hosting API is the stronger variant.
* **Disabled gates.** A disabled gate removes its own evidence and the ordering checks
  across it. Predecessor ordering is checked between adjacent enabled gates only.
* **red-phase-verify** binds the S9 artifact's head commit (where the tests pass), so
  its commit is not the harness-only commit.
* **Override interactivity** is a TTY heuristic (stdin and stdout) plus the token; it is
  not an identity proof. The signed token is the authority.
* **Remote derivation** requires the `gh` CLI and `--repo`; without it the derived gates
  block.
* **Local git trust.** `ExecGit` strips ambient `GIT_*` variables but trusts the working
  repository's object database.
