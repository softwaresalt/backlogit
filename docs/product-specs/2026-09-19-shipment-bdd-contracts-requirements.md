---
chunk_strategy: h1-h2-h3
description: "Deferred product/requirements spec proposing one YAML BDD acceptance contract per FUTURE shipment: Markdown/backlog stays the lifecycle source-of-truth while a hashed, referenced YAML file carries deterministic machine-readable acceptance (typed given/when/then, allowlisted runner IDs, stable scenario IDs, evidence binding). Explicitly does NOT retrofit 156-S or 149-S. handoff_status: deferred; no backlog entries or plans created."
doc_type: spec
schema_version: "1.0"
source: docs/product-specs/2026-09-19-shipment-bdd-contracts-requirements.md
title: "Product Spec (deferred) — Per-shipment YAML BDD acceptance contracts"
docline:
  author: Stage
  date: 2026-09-19
  status: proposed
  handoff_status: deferred
  dark_factory_ready: false
  scope: standard
---

**Owner:** Stage (planning/decomposition only — no application code, no backlog
entries, no plan handoff created by this document).
**Handoff:** `none` / `deferred`. There is intentionally **no**
`BRAINSTORM_HANDOFF_READY` marker because no plan handoff was requested.
**Adoption:** optional, future, pilot-first. **Do not retrofit** current
shipments `156-S` or `149-S`.

---

## 1. Product frame

Prose plans and prose shipment definitions accumulate corrections and semantic
contradictions over time. As a plan is amended, "current" intent drifts from
earlier sections, acceptance becomes ambiguous, and reviewers must re-read long
narrative artifacts to reconstruct what "done" means. This is expensive
(repeated full-review loops) and error-prone (silent contradictions).

The goal: make **shipment acceptance deterministic and compact** — a single,
machine-readable contract that states exactly what must be true for a shipment to
be accepted, expressed once, validated automatically, and rendered into
human-readable views instead of hand-maintained duplication.

## 2. Key architecture recommendation

- **Markdown/backlog remains the lifecycle source-of-truth.** Statuses,
  hierarchy, dependencies, and narrative context stay in the backlog artifacts.
- **YAML is the machine-readable acceptance contract only** — the compact,
  typed statement of acceptance scenarios. It is **referenced and hashed** by the
  shipment, not a competing lifecycle store.
- **Avoid two competing state stores.** The YAML contract MUST NOT duplicate or
  fork lifecycle state. It describes acceptance; it does not own status.

### 2.1 Provisional future path

- Recommended provisional location: `.backlogit/contracts/<shipment-id>.bdd.yaml`
  (for example `.backlogit/contracts/160-S.bdd.yaml`).
- This path is **provisional** and subject to a future schema/registry
  implementation decision. It is not a committed convention yet.

### 2.2 Lifecycle freeze semantics

- YAML contracts are **rewritten in place** before a shipment is claimed; Git
  history carries revisions (no appended correction logs inside the file).
- **After claim, the contract hash is frozen.** Any mutation of a claimed
  contract invalidates readiness and requires governed re-approval before the
  shipment can proceed.

## 3. Contract shape (typed, not free-form shell)

- **Stable scenario IDs** that never renumber across revisions.
- **Typed GIVEN/WHEN/THEN clauses** — no arbitrary shell strings.
- Actions and assertions reference **allowlisted runner IDs** plus structured
  arguments (never inline command strings).
- Each scenario defines: setup, action, expected state/evidence, applicable
  lifecycle phase, task mappings, and failure codes.
- **Traceability chain:** requirement -> scenario IDs -> TDD test IDs -> evidence
  IDs. Evidence MUST be non-vacuous and deterministic (machine-checkable), never
  a prose assertion of success.

## 4. Illustrative YAML (NOT an active contract)

The block below is **illustrative only** to convey shape. It is not a live
contract, not validated, and not bound to any shipment.

```yaml
# ILLUSTRATIVE ONLY — not an active contract, not validated, not bound.
schema_version: "1.0"
shipment_id: "PILOT-S"          # a NEW small pilot shipment, never 156-S/149-S
revision: 1
contract_sha256: "<computed-at-freeze>"   # placeholder; frozen at claim
scenarios:
  - id: PS-1
    requirement: R1
    phase: pre_commit
    task_mappings: ["PILOT.001-T"]
    given:
      type: repo_state
      args: { tree: clean, head: current }
    when:
      type: run_runner
      runner_id: task-lint
      args: { task_id: "PILOT.001-T", feature_id: "PILOT-F" }
    then:
      type: expect_exit
      args: { exit_code: 0 }
    evidence:
      id: EV-PS-1
      type: structured_json
      binds: task-lint-json
    failure_codes: ["LINT_NONZERO", "SCHEMA_INVALID", "TARGET_VACUOUS"]
```

## 5. Requirements (stable IDs)

- **R1 — Schema validation.** Every contract MUST validate against a published
  schema (types, required fields, enum ranges) before it is accepted; invalid
  contracts fail closed.
- **R2 — In-place correction.** Contracts are rewritten in place before claim; no
  appended correction/amendment history inside the file. Git carries revisions.
- **R3 — Stable IDs.** Scenario and evidence IDs are stable across revisions and
  never silently renumber.
- **R4 — Deterministic ordering.** Scenario evaluation order is deterministic and
  reproducible across runs and hosts.
- **R5 — Typed operations.** GIVEN/WHEN/THEN clauses are typed; actions and
  assertions reference allowlisted runner IDs plus structured args only.
- **R6 — Security / no command injection.** No arbitrary shell strings; runner
  IDs are allowlisted; args are structured and validated; untrusted input cannot
  reach a shell.
- **R7 — Lifecycle freeze.** On claim, the contract hash freezes; mutation
  invalidates readiness and requires governed re-approval.
- **R8 — Task coverage.** Every executable task in the shipment maps to at least
  one scenario; coverage is provable and reported.
- **R9 — DAG / invariant scenarios.** Contracts can express DAG and invariant
  acceptance (source/sink, reachability, acyclicity, wave bounds) as typed
  scenarios, not prose.
- **R10 — TDD generation & traceability.** Contracts drive TDD test generation;
  requirement -> scenario -> test -> evidence is traceable end to end.
- **R11 — Evidence binding.** Each scenario binds to non-vacuous, deterministic
  machine evidence (structured output, not prose claims).
- **R12 — Human-readable reporting.** A generated view renders the contract and
  its pass/fail state for humans; humans never hand-maintain a parallel copy.
- **R13 — Backward compatibility / optional adoption.** Adoption is optional;
  shipments without a contract continue to work under existing acceptance.
- **R14 — Failure behavior.** Validation, runner, and evidence failures produce
  explicit, typed failure codes and fail closed; no silent false-green.

## 6. Traceability model

`requirement (R#) -> scenario IDs (PS-#) -> TDD test IDs -> evidence IDs (EV-#)`.
Every requirement resolves to one or more scenarios; every scenario resolves to
one or more tests and exactly the evidence that proves it. Coverage gaps are a
validation failure, not a warning.

## 7. Success criteria

- A pilot shipment's acceptance is fully expressed by one validated YAML contract.
- Reviewers determine "done" from the contract + generated report, without
  reading a long prose plan.
- No contradiction is possible between "current" acceptance and an older section,
  because there is a single typed source and no appended history.
- Contract validation, coverage, and evidence checks run deterministically in CI.

## 8. In scope

- Contract schema, typed clause vocabulary, allowlisted runner-ID model, stable
  IDs, hash/freeze lifecycle, coverage + traceability + evidence binding,
  generated human-readable reporting, optional/backward-compatible adoption.

## 9. Out of scope

- Retrofitting `156-S` or `149-S` (explicitly excluded).
- Replacing backlog/Markdown lifecycle state with YAML.
- Any new backlog entries, plans, or implementation from this document.
- Registry/schema implementation details (deferred to a future decision).

## 10. Decisions

- Markdown/backlog stays lifecycle source-of-truth; YAML is acceptance-only.
- One contract per shipment, referenced and hashed by the shipment.
- Typed clauses + allowlisted runner IDs; no inline shell.
- Freeze-on-claim with governed re-approval on mutation.

## 11. Assumptions

- The harness can expose stable, allowlisted runner IDs for actions/assertions.
- Deterministic machine evidence is available for the scenarios that matter.
- Git history is an acceptable revision record for pre-claim edits.

## 12. Questions to resolve before planning

- Q1: What is the canonical schema and version-negotiation strategy?
- Q2: What is the exact allowlisted runner-ID registry and its arg schemas?
- Q3: Where does the contract live long-term (confirm `.backlogit/contracts/`)?
- Q4: How is the frozen hash bound into the shipment record and re-verified?
- Q5: What is the governed re-approval flow for a post-claim mutation?

## 13. Deferred questions

- DQ1: Should contracts support cross-shipment/global invariant scenarios?
- DQ2: Should generated reports be published as CI artifacts or PR comments?
- DQ3: Contract composition/inheritance for families of similar shipments.

## 14. Risks and tradeoffs

- **Primary risk: YAML becomes another drift source** (a second store that
  disagrees with the backlog). **Mitigation:** single ownership (acceptance
  only), schema validation, freeze hash, an automated validator, and
  **generated** human views — never hand-maintained duplication.
- Over-typing can make simple shipments verbose. Mitigation: minimal required
  fields; optional adoption; small-shipment pilot first.
- Runner-ID allowlist must stay in sync with the harness. Mitigation: validation
  fails closed on unknown runner IDs.

## 15. Recommended phased pilot

Pilot on a **new, small shipment (<= 5 tasks)** — not `156-S` or `149-S`. Phase 1:
schema + validator + one hand-authored contract + generated report. Phase 2:
TDD-test generation and evidence binding. Phase 3: freeze-on-claim + governed
re-approval. Evaluate cost/benefit before broad adoption.

## 16. Handoff status

`handoff_status: deferred`. No plan, no backlog entries, no
`BRAINSTORM_HANDOFF_READY`. This spec is a durable idea record only; a future
Stage session may intake it when prioritized.
