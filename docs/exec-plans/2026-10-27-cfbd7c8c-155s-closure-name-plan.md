---
chunk_strategy: h1-h2-h3
description: 'Plan to conform the 155-S closure record name to docs/closure/155-S-174-F-post-merge-closure.md so the 177-S, 178-S, and 179-S pre_claim pipeline-topology gate recognizes predecessor 155-S (stash CFBD7C8C).'
doc_type: plan
docline:
  feature_id: 203-F
  shipment_id: 203-S
  source_decision: docs/decisions/2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md
  stash_id: CFBD7C8C
schema_version: "1.0"
source: docs/exec-plans/2026-10-27-cfbd7c8c-155s-closure-name-plan.md
title: 'Implementation Plan: Conform 155-S closure record name for 177-S, 178-S, and 179-S pre_claim gate'
---

# Implementation Plan: Conform 155-S closure record name for 177-S, 178-S, and 179-S pre_claim gate

## Objective

Make the `155-S` predecessor closure record recognizable to
`autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json`
(and sibling successors `178-S` and `179-S`) by conforming its filename to
`docs/closure/155-S-174-F-post-merge-closure.md`, and confirm the gate result.
Source decision:
`docs/decisions/2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md` (Option A,
stash `CFBD7C8C`).

## Problem Frame

Queued shipments `177-S`, `178-S`, and `179-S` depend on shipped predecessors
`155-S` (covering feature `174-F`) and `154-S` (covering feature `173-F`). While
`154-S` already conforms (`docs/closure/154-S-173-F-post-merge-closure.md`,
shipped in `202-S`), `155-S`'s post-merge operational closure record is named
`docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`.
The `pipeline-topology` `pre_claim` gate expects
`docs/closure/{shipment_id}-{feature_id}-post-merge-closure.md`, so it halts with
`PREDECESSOR_CLOSURE_UNRECOGNIZED` on `155-S`.

## Requirements Trace

| Requirement | Source | Implementation Unit |
|---|---|---|
| Rename `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md` to `docs/closure/155-S-174-F-post-merge-closure.md` via `git mv`, update `source:` frontmatter, and append `## Rename note` without altering gate keys | `2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md` (Option A) | U1 (`203.001-T`) |
| Repoint active doc references (`docs/exec-plans/2026-09-26-resumable-shipment-blocked-lifecycle-decided-plan.md`), verify zero external frontmatter references to the old path, and verify docline + markdownlint + `pre_claim` topology gate behavior | `2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md` (Option A) | U2 (`203.002-T`) |
| Re-probe `177-S` (and `178-S`, `179-S`) `pre_claim` gate on clean merged `main` during post-merge closure without claiming `177-S` | `2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md` (Option A) | U3 (Ship post-merge closure obligation) |

## Constraints

* **Sequencing**: `203-S` is declared a DAG root (`labels: [dag-root]`,
  `queue_position: 50`) with a non-gating `related_to` semantic link to `155-S`
  (provenance only). Do not add a `blocks` dependency on `155-S` (which would
  circularly fail `PREDECESSOR_CLOSURE_UNRECOGNIZED`), and do not modify
  `177-S`, `178-S`, or `179-S`.
* **Docs-only**: No Go source, test, or configuration file changes.
* **Gate-key preservation**: Top-level `closure_status`, `compaction_status`,
  and `conditions` in the renamed closure record must remain byte-identical;
  the only frontmatter change in U1 is updating `source:` to the new
  repo-relative path `docs/closure/155-S-174-F-post-merge-closure.md`.
* **Archive immutability (P-007)**: Never edit `.backlogit/archive/**` records.

## Implementation Units

### U1: Rename the 155-S closure record and update its source frontmatter (docs) — `203.001-T`

* **Changes**:
  * Run `git mv docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md docs/closure/155-S-174-F-post-merge-closure.md`.
  * Update the `source:` frontmatter line in `docs/closure/155-S-174-F-post-merge-closure.md` from `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md` to `docs/closure/155-S-174-F-post-merge-closure.md`.
  * Append a short `## Rename note` section at the end of `docs/closure/155-S-174-F-post-merge-closure.md` recording the old path (`docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`), stash `CFBD7C8C`, and shipment `203-S`.
  * Leave `closure_status`, `compaction_status`, `conditions`, and all existing body sections byte-identical.
* **Affected files**:
  * `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md` -> `docs/closure/155-S-174-F-post-merge-closure.md` (1 file rename).
* **Acceptance criteria**:
  * `docs/closure/155-S-174-F-post-merge-closure.md` exists, the old path does not exist, and `git diff -M --stat` records a rename.
  * In the frontmatter block, only the `source:` line changes (to `docs/closure/155-S-174-F-post-merge-closure.md`); `closure_status`, `compaction_status`, and `conditions` are untouched.
  * The only body change is the appended `## Rename note` section citing `CFBD7C8C` and `203-S`.
  * `go run ./cmd/backlogit docs lint --path docs/closure/155-S-174-F-post-merge-closure.md` reports `valid: true` with `0 violations`.
* **Verification**:
  * `git diff -M` against base confirms rename + single `source:` line update + appended `## Rename note`.
  * `go run ./cmd/backlogit docs lint --path docs/closure/155-S-174-F-post-merge-closure.md` exits `0`.
* **Execution posture**: characterization-first (docs-only).

### U2: Repoint decided-plan closure references and verify lint and topology gates (docs) — `203.002-T`

* **Changes**:
  * Update the two closure-record path references in `docs/exec-plans/2026-09-26-resumable-shipment-blocked-lifecycle-decided-plan.md` (`## Decision` and `## Traceability`) from `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md` to `docs/closure/155-S-174-F-post-merge-closure.md`, and repoint any machine-read/frontmatter references in `docs/**` or `.backlogit/queue/**` if any exist.
  * Leave immutable `.backlogit/archive/**` records untouched per P-007.
  * Run `go run ./cmd/backlogit docs lint` and `scripts/md-lint.ps1` to confirm 0 docline and P-008 markdownlint violations.
  * Run `autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json` (note: if uncommitted worktree changes cause `WORKTREE_DIRTY` before the predecessor check, verify after committing on the feature branch or record that `155-S` recognition is confirmed once the tree is clean, with authoritative clean-`main` confirmation in U3).
* **Affected files**:
  * `docs/exec-plans/2026-09-26-resumable-shipment-blocked-lifecycle-decided-plan.md` (1 file).
* **Acceptance criteria**:
  * No `source:` or `linked_artifacts:` frontmatter in `docs/**` or `.backlogit/queue/**` points to `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`.
  * `rg -uu -F 155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure` returns only the `## Rename note` in `docs/closure/155-S-174-F-post-merge-closure.md`, narrative provenance mentions in the `203-S` deliberation/plan/backlog/stash records, and immutable `.backlogit/archive/**` entries.
  * `go run ./cmd/backlogit docs lint` and `scripts/md-lint.ps1` pass with 0 findings.
  * `autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json` on a clean committed tree no longer reports `PREDECESSOR_CLOSURE_UNRECOGNIZED` on `155-S`.
* **Depends on**: U1 (`203.001-T`).
* **Execution posture**: characterization-first (docs-only).

### U3: Post-merge closure obligation — re-probe the successor pre_claim gate on clean main (no backlog task)

* **Action**:
  * On the merged clean `main` tree after `203-S`'s implementation PR merges, run `autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json` (and optionally `178-S`, `179-S`) read-only.
  * Record the exit code, token, and blocking predecessor (if any) in `docs/closure/203-S-203-F-post-merge-closure.md`. Do not claim `177-S`, `178-S`, or `179-S`.
* **Acceptance criteria**:
  * The gate result for `177-S` (and predecessor `155-S`) on merged `main` is recorded verbatim in the `203-S` closure record.
  * Any newly surfaced blocker is captured as a stash entry and not fixed inside `203-S`.
* **Why not a manifest task**: Ship executes all manifest tasks before the PR merges to `main`, whereas the authoritative `pre_claim` gate check requires the rename to be merged on clean `main`. Including U3 as a manifest task would create a circular self-merge dependency (exact lesson from `202-S`).

## Dependency Graph

`U1 (203.001-T) -> U2 (203.002-T)`, followed by post-merge closure obligation `U3`.

## Decisions and Rationale

* **Single-predecessor scope (`155-S` only)**: Matches the `202-S` precedent (`154-S` only), keeps the diff to two markdown files, and unblocks `177-S`, `178-S`, and `179-S` immediately while preserving `CFBD7C8C` for `153-S`, `195-S`, `196-S`, and `E2B351F3`.
* **`dag-root` label + `related_to` link + `queue_position: 50`**: Prevents `UNSEQUENCED_SHIPMENT` on `203-S` without adding a circular `blocks` dependency on `155-S`, and places `203-S` at the head of the queue ahead of `177-S..179-S` without mutating their manifests.
* **Update `source:` in U1**: Unlike `154-S` (whose `source:` pointed to its runtime-verification doc), `155-S`'s closure record sets `source:` to its own path (`docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`), so U1 must update `source:` to `docs/closure/155-S-174-F-post-merge-closure.md` to satisfy the docline contract.

## Risks and Caveats

* **Dirty worktree during Ship branch execution**: Running `autoharness gate pipeline-topology --phase pre_claim` on a dirty worktree returns `WORKTREE_DIRTY` before reaching the predecessor check. Ship should run the U2 check after committing U1/U2 on the feature branch (or defer the clean-`main` assertion to U3 after merge).
* **Branch naming for `203-S` claim**: As observed in `202-S` (`ship-202-s-session-memory.md`), Ship must use the topology-gate-compliant branch name derived from `203-S`'s title when claiming `203-S`.

## Constitution Check

* **I. Safety-First Go / Language**: `N/A` — pure documentation and backlog metadata chore; no Go code is added or modified.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — docs-only characterization-first units verified deterministically by `git diff -M`, `go run ./cmd/backlogit docs lint`, `scripts/md-lint.ps1`, and `autoharness gate pipeline-topology` (P-002.1 docs-only verification path).
* **III. Workspace Isolation and Security Boundaries**: `pass` — all file operations resolve inside `docs/` and `.backlogit/` within the repository root; no secrets or credentials are touched.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — all reads and writes remain strictly inside `C:\Source\GitHub\backlogit`.
* **V. Structured Observability**: `pass` — gate outcomes, diff checks, and lint results are recorded in task/closure artifacts.
* **VI. Single Responsibility**: `pass` — no new dependencies or packages introduced.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — `git mv` is a tracked, reversible git rename; no destructive commands are used.
* **VIII. Explicit Safety Modes for Elevated Risk**: `pass` — low-blast-radius docs rename; freeze-scope boundaries explicitly exclude `153-S`, `195-S`, `196-S`, and `.backlogit/archive/**`.
* **IX. Git-Friendly Persistence**: `pass` — uses `git mv` to preserve rename history (`git log --follow`) and atomic YAML/Markdown edits.
* **X. Agent Context Efficiency**: `pass` — uses targeted `rg`, `backlogit`, and `engram` CLI lookups.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — `203-S` will merge via a merge commit (`--merge`), never squash or rebase (P-009).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `absent` — no Go API, CLI flag, MCP tool, or schema changes; conforms an existing closure filename to the existing gate contract.
* **security, auth, permission, or compliance-sensitive behavior**: `absent` — touches only Markdown documentation under `docs/closure/` and `docs/exec-plans/`.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — single tracked `git mv` of one closure file; trivially reversible via `git revert`.
* **external integration, operator checkpoint, or external dependency**: `absent` — no external service or new dependency involved; dark-mode P-017 pre-authorization is active.
* **high runtime, rollout, or rollback risk**: `absent` — zero runtime code changes; rollback is a single `git revert` commit.

Requires plan hardening: no

## Runtime Verification and Closure

* **Changed runtime surfaces**: None (documentation-only chore). `runtime-verification` is non-applicable (`Full local build: not applicable — docs-only closure filename conformance`).
* **Operational closure expectations**:
  * Produce `docs/closure/203-S-203-F-post-merge-closure.md` with top-level `closure_status`, `compaction_status`, and `conditions` (if `READY_WITH_CONDITIONS`), plus `shipment_id: 203-S` and `feature_id: 203-F` matching the `{shipment_id}-{feature_id}-post-merge-closure.md` pattern.
  * Execute mandatory post-merge obligation **U3** on synchronized clean `main`: run `autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json` read-only and record the verdict in the `203-S` closure record.
  * Invoke `compact-context` (`target: all`) per P-020 before finalizing `compaction_status`.
  * Rollback trigger and procedure: if the rename breaks any docline or topology check, `git revert` the merge commit via a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist (4 of 5 dispatched subagents returned findings; Constitution Reviewer returned empty response mid-gate, so a complete sequential rubric pass was executed across all 5 selected personas per the plan-review terminal-states rule).
* plan_hardening: `Requires plan hardening: no` confirmed; no hardening signals present.
* constitution_check: `Constitution Check: pass` confirmed; all principles mapped with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 1 (applied).
  * P3-1 (`Architecture Strategist`, `docs/exec-plans/2026-10-27-cfbd7c8c-155s-closure-name-plan.md` / `docs/decisions/2026-10-27-cfbd7c8c-155s-closure-name-deliberation.md`): `docline.linked_artifacts` in the deliberation referenced the pre-rename closure path, and U2's acceptance criteria carried a parenthetical frontmatter exemption. Applied in place: `docline.linked_artifacts` updated to `docs/closure/155-S-174-F-post-merge-closure.md` and U2's acceptance criteria tightened so zero `source:` or `linked_artifacts:` frontmatter entries in `docs/**` or `.backlogit/queue/**` point to the old filename after U1/U2.
* runtime_verification_and_closure: Docs-only chore (`runtime-verification` non-applicable); post-merge obligation U3 (`autoharness gate pipeline-topology --mode agent --shipment 177-S --phase pre_claim --json` on synchronized clean `main`) and P-020 `compact-context` invocation are explicitly specified.

