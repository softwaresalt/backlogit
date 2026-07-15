---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Planning governance gates implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Implementation plan for exact-byte formal evidence, immutable waiver lifecycle, lock-held governed mutations, contained plan paths, and explicit Constitution Checks.'
docline:
    date: 2026-07-14T18:35:00Z
    origin: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
        - DB1F9026
        - 3E12DC97
    review_state: blocked
---

# Planning Governance Gates Implementation Plan

## Problem Frame

The repository needs one fail-closed gate for Stage-managed and direct harvest. Formal persona evidence is not currently bound to the exact reviewed bytes; waiver lifecycle fields can be raced or altered; separate validation and mutation creates a TOCTOU window; raw reservation tokens are readable; caller-supplied plan paths lack real-path containment; and direct harvest can ambiguously accept ADVISORY. Both harvest copies and legacy Stage bypass flags must use the same governed operation. Impl-plan must continue to emit an explicit Constitution Check.

**Origin:** `docs/decisions/2026-07-14-plan-review-governance-deliberation.md`; backlog deliberation `052-DL`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| G1 | Attributed independent review | G2 produces complete persona dispatch/results. |
| G2 | Bind formal verdict to reviewed bytes | G0A/G0B require one final formal record and exact canonical digest. |
| G3 | Keep formal and waiver modes distinct | G0B and G0C use separate strict schemas over one terminal-record parser. |
| G4 | Immutable auditable waiver | G0C hashes every authorization field, including `intended_disposition`. |
| G5 | Single private owner | G0D stores only a token hash; one long-lived owner session receives the raw token in-process once and never exposes it. |
| G6 | No validate-then-mutate race | G0E/G0F hold the gate lock/lease through typed mutation commit. |
| G7 | Contain every plan path | G0G rejects lexical and real-path escapes before file access. |
| G8 | Correct lifecycle ownership | G2 is evidence-only; G3 owns Stage mode; G5 owns direct mode. |
| G9 | Fail closed on ADVISORY | Stage requires durable confirmation; direct harvest accepts PASS/waiver only. |
| G10 | Close direct and legacy bypasses | G3/G5 route every mutation and both legacy flags through the broker. |
| G11 | Preserve canonical file writes | G0D reuses `internal/atomicfile.WriteFileAtomic`; no duplicate writer. |
| G12 | Emit Constitution Check | G4 requires the exact heading and principle/deviation mapping. |
| G13 | Guard mirrored surfaces | G1 verifies both plan-review, Stage, harvest, and impl-plan copies. |
| G14 | Preserve generated-source parity | Stash `823BADF4` covers all four external templates. |
| G15 | Preserve source promotion links | Canonical source-stash metadata and archived IDs survive repeated sync. |
| G16 | Keep blocked intake isolated | Shipments remain blocked; `D7B1B33D` stays active and outside both shipments. |

## Scope Boundaries

### In Scope

- Shared terminal-record parser/digest, formal schema, waiver schema, reservation lifecycle, gate lease, path containment, and governed mutation broker.
- Thin repository-native gate CLI.
- Mirrored plan-review, Stage, harvest, and impl-plan contracts.
- Integration contract test with fewer than five functions.
- Durable provenance repair for the three harvested source entries.

### Out of Scope

- Reviewer persona implementation or runtime dispatch APIs.
- External autoharness template writes from this workspace.
- Shipment requeue implementation from stash `DB1F9026`.
- Size-estimation stash `D7B1B33D` or standalone PR #240.
- Treating inline or hosted review as formal evidence.

## Canonical Terminal-record Contract

The Markdown-aware parser recognizes H2 headings only outside fenced code. Gate mode permits exactly one final terminal record, using exactly one of these headings:

- `## Formal Plan Review Record`
- `## Operator Waiver Ledger`

The selected heading is the final H2, contains exactly one fenced YAML mapping, and ends immediately after the closing fence plus one terminal line ending. The other heading must be absent. Duplicate headings or keys, both record types, malformed YAML, unknown fields, another heading, prose, whitespace, or bytes after the allowed terminal line ending fail closed. Fenced examples do not count.

Canonical digest decodes valid UTF-8 without BOM, rejects invalid UTF-8 or bare CR, normalizes CRLF to LF, and requires exactly one terminal LF. The parser removes only the validated terminal block and its one separator LF, then computes lowercase SHA-256 over every remaining canonical byte. Any edit to plan or review content changes the digest; appended EOF content is rejected. Formal and waiver schemas share this parser/digest but never share verdict semantics.

## Final Formal Review Schema

~~~markdown
## Formal Plan Review Record

```yaml
record_type: formal_review
schema_version: "1.0"
state: final
review_id: <unique-id>
plan_path: <workspace-relative-plan-path>
plan_digest_sha256: <lowercase-64-hex>
reviewed_at: <UTC>
required_personas: [<persona-names>]
persona_results:
    - persona: <name>
      definition_path: <path>
      dispatch_id: <runtime-id>
      dispatch_status: returned
      model_provider: <value-or-unknown>
      finding_count: <integer>
      disposition: <PASS|ADVISORY|FAIL>
verdict: <PASS|ADVISORY|FAIL>
advisory_confirmation: <absent unless ADVISORY stage-managed proceed>
```
~~~

Every required persona has exactly one successful attributed result. Missing/failed/unattributed persona output makes the record invalid and the verdict FAIL. Before Stage or direct harvest, recompute the current canonical digest and require exact match. Duplicate/non-final/malformed records, stale digest, edited plan bytes, and trailing content block before mutation.

For `ADVISORY`, only Stage-managed flow may proceed, and only when `advisory_confirmation` is a mapping containing operator, durable authorization reference, confirmed UTC time, the same plan path/digest, and `scope: stage_managed`. Direct harvest accepts formal `PASS` or valid waiver only; it always rejects ADVISORY.

## Final Waiver Schema and Immutability

~~~markdown
## Operator Waiver Ledger

```yaml
record_type: operator_waiver
verdict: WAIVED
schema_version: "1.0"
waiver_id: <unique-id>
state: <reserved|consumed>
plan_path: <workspace-relative-plan-path>
plan_digest_sha256: <lowercase-64-hex>
authorizer: <operator>
authorization_reference: <durable-reference>
missing_capability: <value>
reason: <value>
issued_at: <UTC>
expires_at: <UTC>
residual_risk: <value>
intended_disposition: <allow_stage_managed_harvest|allow_direct_harvest>
completion_mode: <stage_managed|direct_harvest>
authorization_scope: exact_plan
authorized_phases: <mode-valid-list>
authorization_payload_sha256: <lowercase-64-hex>
reserved_at: <UTC>
reservation_owner: <run-id>
reservation_token_sha256: <lowercase-64-hex>
consumed_at: <UTC, consumed only>
consumed_by_harvest_ids: [<ids, consumed only>]
shipment_id: <stage_managed consumed only>
```
~~~

The immutable authorization payload is the typed known-key mapping of record/verdict/schema identity, waiver ID, plan path/digest, authorizer/reference, missing capability, reason, issued/expiry, residual risk, intended disposition, completion mode, `authorization_scope`, and phases. Serialize it as deterministic UTF-8 JSON with lexicographically sorted keys, preserved array order, no insignificant whitespace, no trailing newline, and fixed escaping; hash with lowercase SHA-256. Reservation records that hash, and every governed operation compares it with the owner-held expected hash. Changing any immutable field, even with otherwise valid YAML, fails.

Only lifecycle fields may transition: absent to reserved fields, then `state`, `consumed_at`, exact harvested IDs, and mode-valid shipment ID. Reserved records forbid consumed fields. `stage_managed` requires phases `[harvest, shipment_assembly]`, matching intended disposition, and shipment ID when consumed. `direct_harvest` requires `[harvest]`, matching disposition, forbids shipment ID, and cannot authorize later shipment work. Unknown modes, phases, fields, or combinations fail.

Reservation generates a cryptographically random raw token and returns it once to the winning long-lived in-process `GateSession`. The process retains it in memory across governed mutations; Stage or direct harvest owns and drives that session through a non-secret handle, but the raw token never crosses stdout/stderr, RPC/tool results, transcript, plan, checkpoint, log, telemetry, or error text. Only SHA-256/fingerprint is persisted; validation hashes the session-held token and compares under lock with `crypto/subtle.ConstantTimeCompare`. A losing caller cannot copy ledger values or the public session handle to assume ownership.

## Governed-operation and Containment Contract

Every reserve, governed mutation, and consume starts with a workspace-relative plan path under `docs/exec-plans/`. Reject absolute, UNC/volume, empty, dot-dot, non-Markdown, missing, or non-regular paths. Resolve from the configured workspace root, inspect every component, reject symlink/junction/reparse components, resolve the real target, and require it remains within the root before any read, lock derivation, or write.

A mutation is one typed governed operation, never `validate` followed by a separate backlog command:

1. Resolve the contained plan path and acquire its cross-process gate lock.
2. Re-read and validate terminal record, current digest, formal evidence or waiver payload, owner/token hash, mode, phase, state, and expiry under lock.
3. Stage one allowlisted Markdown-first create, dependency, link, adoption, or shipment-membership mutation through existing core services.
4. At the publication linearization point, while still locked, re-read the record, recompute digest/payload, recheck expiry and ownership, then atomically publish the one canonical Markdown source mutation.
5. Drift, expiry, or conflict before publication aborts staged work with zero durable mutation. SQLite is derived: refresh/retry it under the lock; if refresh still fails after source publication, return an explicit committed-but-index-stale result and never replay the canonical mutation.

All compliant plan-record writers acquire the same lock. Race tests pause between initial validation and commit, then attempt plan edits, expiry, reserve/consume, and payload changes; each must block or cause zero mutation. Skills call only the governed command.

Repository durability is Git-backed. Governance needs atomic visibility, not power-loss fsync. Reuse `internal/atomicfile.WriteFileAtomic`, whose same-directory temp/rename contract already provides complete-file visibility; do not re-inline the primitive or expand it with fsync.

## Implementation Units

### G1 — Planning-governance integration contract

**Task:** `105.001-T`
**Files:** `tests/integration/planning_governance_contract_test.go`
**Effort:** S; one file; fewer than five functions.
**Dependencies:** none.

Add RED/GREEN path-specific checks for both terminal schemas, persona completeness, immutable payload, token secrecy, governed-operation races, containment, lifecycle ownership, direct ADVISORY rejection, legacy bypasses, mirrored files, and Constitution Check.

### G0A — Shared terminal parser and digest

**Task:** `105.006-T`
**Files:** `internal/plangate/canonical.go`, `internal/plangate/canonical_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G1 RED.

### G0B — Formal record validator

**Task:** `105.009-T`
**Files:** `internal/plangate/formal.go`, `internal/plangate/formal_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0A.

### G0C — Immutable waiver schema

**Task:** `105.010-T`
**Files:** `internal/plangate/waiver.go`, `internal/plangate/waiver_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0A.

### G0D — Atomic reservation lifecycle

**Task:** `105.007-T`
**Files:** `internal/plangate/reservation.go`, `internal/plangate/reservation_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0C and G0G.

Use shared atomicfile, one-winner CAS, a long-lived owner session with a process-private raw token, constant-time hash comparison, and mode-valid consume transitions.

### G0E — Lock-held gate lease

**Task:** `105.011-T`
**Files:** `internal/plangate/governed.go`, `internal/plangate/governed_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0B and G0D.

### G0F — Governed core mutation broker

**Task:** `105.012-T`
**Files:** `internal/core/governed_mutation.go`, `internal/core/governed_mutation_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0E.

### G0G — Workspace plan-path containment

**Task:** `105.013-T`
**Files:** `internal/plangate/path.go`, `internal/plangate/path_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G1 RED.

### G0H — Thin gate CLI

**Task:** `105.008-T`
**Files:** `internal/cli/gate.go`, `internal/cli/gate_test.go`
**Effort:** S; two files; no more than four functions.
**Dependencies:** G0F.

### G2 — Formal dispatch/evidence only

**Task:** `105.002-T`
**Files:** `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md`
**Dependencies:** G0B.
**Boundary:** plan-review creates evidence; it never reserves, mutates, validates lifecycle ownership, or consumes.

### G3 — Stage-managed ownership

**Task:** `105.003-T`
**Files:** `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md`
**Dependencies:** G2 and G0H.

### G4 — Constitution Check output

**Task:** `105.004-T`
**Files:** `.github/skills/impl-plan/SKILL.md`, `plugin/skills/impl-plan/SKILL.md`
**Dependencies:** G1 RED.

### G5 — Direct-harvest ownership

**Task:** `105.005-T`
**Files:** `.github/skills/harvest/SKILL.md`, `plugin/skills/harvest/SKILL.md`
**Dependencies:** G2 and G0H.

## Dependency Graph

`G1 RED -> {G0A, G0G, G4}`; `G0A -> {G0B, G0C}`; `{G0C, G0G} -> G0D`; `{G0B, G0D} -> G0E -> G0F -> G0H`; `G0B -> G2`; `{G2, G0H} -> {G3, G5}`; then G1 GREEN.

Every task is one or two files, fewer than five production/test functions, and targets under two hours.

## TDD and Quality Gates

1. Record G1 RED for formal byte binding, waiver immutability, token secrecy, races, containment, ADVISORY, bypasses, and mirrors.
2. Implement G0A/G0G test-first, then G0B/G0C, G0D, G0E, G0F, and G0H.
3. Prove duplicate/stale formal record and every immutable-field tamper fails.
4. Race plan edits/expiry/consume against governed mutation and require zero mutation.
5. Prove raw token never leaves the owner-session process or appears in output, transcript, persisted plan, logs, or errors, and loser/session-handle adoption fails.
6. Run supported-platform lexical, symlink, junction/reparse containment tests.
7. Apply G2/G3/G5 and G4; require mirrored-contract GREEN.
8. Run positive formal PASS, Stage ADVISORY-confirmed, stage waiver, and direct waiver smokes.
9. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and `gofmt -l .` with no output.
10. Run `go run ./cmd/backlogit docs lint` and cross-reference checks.

## Decisions and Rationale

- **One parser, distinct schemas:** exact-byte canonicalization is shared; formal PASS and WAIVED remain semantically separate.
- **Plan-review is evidence-only:** the run that mutates owns lifecycle credentials.
- **Governed operation, not precheck:** validation and commit share one lock/lease boundary.
- **Private token:** only a one-way fingerprint persists.
- **Immutable authorization:** only enumerated transition fields can change.
- **Direct ADVISORY blocks:** avoids a second confirmation protocol in direct mode.
- **Reuse atomicfile without fsync:** repository records rely on Git durability and need atomic visibility.
- **Contain before access:** lexical and real-path checks precede reads, locks, and writes.

## Provenance Repair and Deferred Boundaries

Canonical metadata now maps `8CD8F46A -> 105-F`, `CA877CD1 -> 105.004-T`, and `A4BE2FAD -> 106-F`. Target artifacts carry `source_stash_id`, kind, priority, path, text, and applicable deliberation; archived records carry `harvested_artifact_id` and harvested reason. Two repository-native syncs rebuilt all three as `state: harvested` with exact `stash_links`. Repair stash `3E12DC97` was then retired with traceability.

Stash `823BADF4` remains the external Principle IV handoff for Stage, plan-review, impl-plan, and harvest templates. Closure requires regeneration parity for terminal formal/waiver records, lifecycle ownership, governed mutation, ADVISORY, bypass, and Constitution Check behavior.

Stash `DB1F9026` still tracks supported shipment requeue. Generic `blocked -> active` remains forbidden. `D7B1B33D` remains active intake and is independently isolated in PR #240; it is not in this plan or either shipment.

## Risks and Hardening

- Runtime dispatch may be unavailable; preflight and halt.
- Cross-platform reparse semantics require Windows and Unix fixtures.
- A crash strands the owner-session process token and reserved waiver; never recover or log it automatically.
- A malformed broker could block Stage or permit bypass; integration and race tests are mandatory.
- External generated-source drift remains until `823BADF4` closes.

**Hardening signals:** public workflow/CLI contract, governance authority, concurrency, filesystem containment, and operator checkpoints are present. Careful, investigate-first hardening is required.

**Rollback:** revert implementation commit. Git preserves records; no destructive migration is planned. Rollback owner is Ship, and validation window is the first supported Stage run.

## Constitution Check

- **I:** Go work is isolated into leaf parser/schema/lease/path units, a core broker, focused tests, and thin CLI.
- **II (NON-NEGOTIABLE):** integration RED precedes implementation; each unit is test-first; all suites end GREEN.
- **III/IV (NON-NEGOTIABLE):** workspace-relative real-path containment precedes all gate file operations; external templates remain stash-only.
- **V:** exact digest, attributed persona output, immutable payload hash, owner/token fingerprint, and transitions are observable.
- **VI:** each task is one concern, at most two files and fewer than five functions; no new dependency is required.
- **VII (NON-NEGOTIABLE):** no destructive action or scratch-file mutation is authorized.
- **VIII:** elevated governance/concurrency risk receives hardening and race tests.
- **IX:** records remain human-readable, Git-backed Markdown/YAML.
- **X:** compact canonical records avoid transcript dependence.
- **XI:** downstream delivery remains merge-commit-only; Stage does not merge.

No constitutional violation or current waiver exists.

## Plan Review

### Gate Decision: BLOCKED

**Formal plan-review provenance:** NOT RUN. This invocation has no semantic reviewer-subagent dispatch tool, so no independent persona outputs exist.

**Waiver authorization:** NONE. This bounded refinement direction is not a plan-review waiver.

**Current disposition:** shipment `094-S`, feature `105-F`, and all thirteen tasks `105.001-T` through `105.013-T` are blocked. No harvest or Ship readiness may be inferred.

**Required unblock:** successful formal multi-persona evidence for this exact final plan, or a new explicit exact-plan operator waiver. Ship intake separately requires supported requeue or an explicit artifact-preserving replacement procedure.

### Informal Assessment

Copilot findings informed this refinement but do not constitute formal multi-persona gate evidence or a PASS verdict.
