---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'PASS-only planning governance implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Test-first plan for exact-byte formal PASS evidence, contained paths, and one stateless governed mutation per CLI process.'
docline:
    date: 2026-07-14T18:35:00Z
    origin: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
        - DB1F9026
        - 062A67C0
    review_state: blocked
---

# PASS-only Planning Governance Implementation Plan

## Problem Frame

The repository needs a small fail-closed gate that Stage and direct harvest can invoke with the existing one-command Cobra model. V1 admits only complete attributed formal `PASS` evidence bound to the exact plan bytes. It deliberately excludes machine-managed waivers, ADVISORY confirmation, tokens, sessions, and lifecycle state.

**Origin:** `docs/decisions/2026-07-14-plan-review-governance-deliberation.md`; backlog deliberation `052-DL`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| P1 | Bind formal review to exact bytes | `105.006-T` parses the unique final record and canonical digest. |
| P2 | Require complete attributed evidence | `105.002-T`, `105.006-T`, and `105.020-T` require every configured persona result. |
| P3 | Admit PASS only | Every formal validator, Stage copy, and harvest copy rejects FAIL and ADVISORY. |
| P4 | Contain plan access | `105.013-T` rejects lexical and real-path escapes before reads or leases. |
| P5 | Close validation/mutation race | `105.014-T` and `105.015-T` hold one cooperative lease through one core operation. |
| P6 | Fit one-command Cobra | `105.016-T` exposes one stateless process with one strict typed request. |
| P7 | Register and discover the command | `105.017-T` and `105.018-T` cover root and CLI-only parity. |
| P8 | Keep generated docs current | `105.019-T` owns the two generated pages. |
| P9 | Close Stage and harvest bypasses | `105.003-T`, `105.005-T`, and `105.022-T` reject direct and legacy paths. |
| P10 | Preserve Constitution Check | `105.001-T`, `105.004-T` guard required impl-plan output. |
| P11 | Keep tests bounded | Evidence, runtime, and control-plane integration contracts are separate one-file tasks. |
| P12 | Preserve external parity | Stash `823BADF4` covers the four generated upstream templates. |
| P13 | Defer machine waivers | Stash `062A67C0` records authenticated waiver work outside `094-S`. |

## Scope Boundaries

### In Scope

- Formal record parser, strict schema, canonical digest, and attributed PASS validator.
- Workspace-relative lexical and symlink/junction/reparse containment.
- Cooperative plan lease for one operation.
- Governed broker for four existing core mutations.
- Stateless `plan-apply` CLI, root registration, registry parity, and generated docs.
- Mirrored plan-review, Stage, harvest, and impl-plan contract updates.
- Four bounded integration contract files.

### Out of Scope

- Waiver records or verdicts.
- Authorized, reserved, consumed, expiry, authorizer, reference, scope, or disposition fields.
- Authorization payload hashes, tokens, fingerprints, owner sessions, handles, or long-lived transport.
- Reserve, validate, consume, confirmation, or plan-record-writing commands.
- ADVISORY admission in Stage or direct harvest.
- New reviewer dispatch APIs.
- External autoharness template writes.
- Shipment requeue implementation from stash `DB1F9026`.
- Size-estimation intake `D7B1B33D`.

## Exact Formal Record Contract

The parser recognizes H2 headings only outside fenced code. Gate mode requires exactly one final section named `## Formal Plan Review Record`. It contains exactly one fenced YAML mapping and ends immediately after the closing fence plus one terminal LF. Duplicate headings or keys, a non-final record, malformed YAML, unknown fields, prose or whitespace after the record, or fenced-example confusion fails closed.

Canonicalization:

1. require valid UTF-8 without BOM;
2. normalize CRLF to LF and reject bare CR;
3. require exactly one terminal LF;
4. remove only the validated final formal-record block and its single separator LF;
5. compute lowercase SHA-256 over every remaining byte.

Any edit to plan or review content changes the digest. Any bytes appended after the final formal record are rejected before hashing.

The strict record is:

~~~markdown
## Formal Plan Review Record

```yaml
record_type: formal_review
schema_version: "1.0"
state: final
review_id: <unique-id>
plan_path: <workspace-relative-docs/exec-plans-path>
plan_digest_sha256: <lowercase-64-hex>
reviewed_at: <UTC>
required_personas: [<persona-names>]
persona_results:
    - persona: <name>
      definition_path: <workspace-relative-path>
      dispatch_id: <runtime-id>
      dispatch_status: <returned|failed>
      model_provider: <value-or-unknown>
      finding_count: <non-negative-integer>
      disposition: <PASS|ADVISORY|FAIL>
verdict: <PASS|ADVISORY|FAIL>
```
~~~

Every required persona appears exactly once with attributed dispatch evidence. Admission requires every dispatch status and disposition, plus the final verdict, to be PASS. A represented failed dispatch or FAIL/ADVISORY disposition remains valid audit evidence but is non-admitting. Missing/unattributed results, stale digest, wrong path, or malformed fields are also non-admitting. There is no waiver schema and no alternate admission record.

## Stateless Command and Dependency Contract

`backlogit plan-apply --plan <workspace-relative-plan>` accepts exactly one strict typed mutation request from stdin or explicit arguments. One process:

1. resolves and contains the plan path;
2. acquires its cooperative lease;
3. re-reads the plan and validates unique final exact formal PASS;
4. dispatches exactly one allowlisted existing core mutation;
5. releases the lease only after core returns.

Initial operation tags and payloads are strictly typed:

- `create_item` — one artifact type, parent when required, title, status, and supported fields;
- `add_dependency` — source ID, target ID, and supported dependency type;
- `create_shipment` — title and initial exact item IDs;
- `add_to_shipment` — shipment ID and item ID.

Unknown fields, unknown operations, multiple requests, or direct command chaining fail before mutation. There are no stateful gate subcommands.

Dependency direction is:

`internal/cli -> internal/governed -> {internal/plangate, internal/core}`

`internal/plangate` imports no core or CLI. Core imports no CLI or governed package.

## Task Map

Every task is below two hours, references at most two implementation/test files, adds fewer than five production/test functions, and has fewer than four scenario groups.

| Task | Concern | Files | Depends on |
|---|---|---|---|
| `105.001-T` | Constitution Check contract RED/GREEN | `tests/integration/planning_governance_contract_test.go` | none |
| `105.002-T` | plan-review copies, evidence production only | `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md` | `105.006-T`, `105.020-T` |
| `105.003-T` | Stage PASS-only routing | `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md` | `105.016-T`, `105.022-T` |
| `105.004-T` | impl-plan Constitution output | `.github/skills/impl-plan/SKILL.md`, `plugin/skills/impl-plan/SKILL.md` | `105.001-T` |
| `105.005-T` | harvest PASS-only routing | `.github/skills/harvest/SKILL.md`, `plugin/skills/harvest/SKILL.md` | `105.016-T`, `105.022-T` |
| `105.006-T` | formal parser/digest/validator | `internal/plangate/canonical.go`, `internal/plangate/canonical_test.go` | `105.020-T` |
| `105.013-T` | plan path containment | `internal/plangate/path.go`, `internal/plangate/path_test.go` | `105.021-T` |
| `105.014-T` | cooperative per-operation lease | `internal/governed/lease.go`, `internal/governed/lease_test.go` | `105.013-T`, `105.021-T` |
| `105.015-T` | one-operation broker | `internal/governed/apply.go`, `internal/governed/apply_test.go` | `105.006-T`, `105.013-T`, `105.014-T`, `105.021-T` |
| `105.016-T` | stateless plan-apply leaf command | `internal/cli/plan_apply.go`, `internal/cli/plan_apply_test.go` | `105.015-T`, `105.021-T` |
| `105.017-T` | root registration/invocation | `internal/cli/root.go`, `internal/cli/root_expansion_test.go` | `105.016-T` |
| `105.018-T` | intentional CLI-only registry parity | `internal/cli/registry_parity_test.go` | `105.017-T` |
| `105.019-T` | generated root and command docs | `docs/cli-reference/backlogit.md`, `docs/cli-reference/backlogit_plan-apply.md` | `105.017-T` |
| `105.020-T` | evidence integration contract | `tests/integration/planning_gate_evidence_test.go` | none |
| `105.021-T` | runtime integration contract | `tests/integration/planning_gate_runtime_test.go` | none |
| `105.022-T` | mirrored control-plane contract | `tests/integration/planning_gate_control_plane_test.go` | none |

Obsolete tasks `105.007-T` through `105.012-T` were removed from the shipment and workspace before this map was assembled. Their waiver/session/core-broker design is not retained.

## TDD and Quality-gate Sequence

1. Add `105.001-T`, `105.020-T`, `105.021-T`, and `105.022-T`; record bounded RED failures.
2. Implement formal parser/digest and containment test-first.
3. Implement cooperative lease and one-operation broker; run race/failure tests.
4. Implement the leaf command, then register it through the real root.
5. Update intentional CLI-only parity and regenerate only the two affected reference pages.
6. Update plan-review, Stage, harvest, and impl-plan mirrored copies.
7. Require evidence, runtime, control-plane, and Constitution contracts GREEN.
8. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and require `gofmt -l .` to emit no output.
9. Run `go run ./cmd/backlogit docs lint` and `go run ./cmd/gen-docs docs/cli-reference`; require zero drift.
10. Verify all referenced files and external handoff paths.

## Runtime Negative Matrix

- Missing formal record: zero mutation.
- FAIL or ADVISORY: zero mutation.
- Incomplete, duplicate, unattributed, malformed, non-final, or stale evidence: zero mutation.
- Plan edit before or while waiting for lease: re-read fails or uses the newly validated exact state; no stale admission.
- Absolute, traversal, symlink, junction, reparse, or outside-root path: zero mutation.
- Unknown operation or extra payload field: zero mutation.
- Direct Stage/harvest command, prose readiness, `skip_review`, or `force_harvest_no_gates`: zero mutation.

## Deferred Waiver Boundary

Stash `062A67C0` is the sole future machine-waiver intake and is not in `094-S`. A future shipment may begin only with authenticated GitHub authority: a live non-dismissed review by a maintainer with verified repository permission, bound to repository, exact path/digest, scope, intended disposition, and expiry. Prefer one process owning any complete authorization manifest lifecycle. V1 implements none of this.

## Provenance and External Boundaries

Repaired promotion links remain `8CD8F46A -> 105-F`, `CA877CD1 -> 105.004-T`, and `A4BE2FAD -> 106-F`. Active stash `823BADF4` covers PASS-only parity for the external Stage, plan-review, harvest, and impl-plan templates. `D7B1B33D` remains active only in PR #239; PR #240 is closed unmerged.

## Risks and Rollback

- Formal dispatch may remain unavailable; preflight blocks honestly.
- Cooperative lock crash recovery must not allow overlapping operations; focused lease tests cover ownership and stale recovery.
- A root or registry omission can make the command unreachable; dedicated tasks cover each surface.
- Generated template drift remains until `823BADF4` closes.

Rollback is a normal commit revert. No data migration or destructive rollout is planned.

## Constitution Check

- **I:** dependency direction and typed errors preserve idiomatic Go boundaries.
- **II (NON-NEGOTIABLE):** four bounded contract files establish RED before implementation and GREEN afterward.
- **III/IV (NON-NEGOTIABLE):** path containment precedes reads and lease derivation; no out-of-tree write is planned.
- **V:** review identity, persona attribution, digest, operation, and failure are inspectable.
- **VI:** each task is one concern, below three files/five functions/four scenario groups.
- **VII (NON-NEGOTIABLE):** protected scratch and unrelated state are not modified.
- **VIII:** high-risk governance receives exact negative and race tests, while speculative waivers are deferred.
- **IX:** evidence remains Git-backed and human-readable.
- **X:** one stateless operation minimizes hidden context.
- **XI:** downstream merge and shipping remain outside Stage.

No constitutional violation, exception, or current waiver exists.

## Plan Review

### Gate Decision: BLOCKED

**Formal plan-review provenance:** NOT RUN. This invocation exposes no semantic reviewer-subagent dispatch tool and has no attributed persona outputs.

**Waiver authorization:** NONE. Machine-managed waivers are outside v1. This simplification request is not a bootstrap approval.

**Current disposition:** shipment `094-S`, feature `105-F`, and all sixteen tasks are blocked. Copilot PR review is supplemental architecture feedback, not formal persona evidence.

**Required unblock:** successful formal multi-persona evidence for these exact final plan bytes, or a new explicit durable operator bootstrap approval scoped only to installing PASS-only governance and expressly acknowledging that it is not formal PASS. Shipment intake separately requires supported requeue or an explicitly approved artifact-preserving procedure.
