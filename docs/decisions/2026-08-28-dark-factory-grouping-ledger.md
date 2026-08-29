---
chunk_strategy: h1-h2-h3
description: "Dark-factory stash grouping ledger for bounded activation scope"
doc_type: decision
schema_version: "1.0"
source: stage-dark-batch1
title: "Dark-Factory Bounded Stash Grouping Ledger"
---

# Dark-Factory Bounded Stash Grouping Ledger

**Dark-mode scope**: 28 original bounded stash IDs
**Already consumed by 130-S**: 5A4DBE3C, C1808666, B212512E, E053034D, 4863B04B, 48F28B8D, C0A382C7, B7CE5FF9 (8 IDs)
**Candidate 40A985BB**: Not found in stash — already consumed by 130-S closure. Decision: EXCLUDED (no entry to process).
**Active out-of-scope**: 302EFF07 (active but not in bounded set — excluded per dark-factory boundary)
**Active bounded entries**: 20 IDs remaining for planning

## Group 1 — Checkpoint Create/Write Path Security Hardening (SHIPPED — 130-S/131-S)

**Priority**: FIRST — highest priority, security-sensitive, active bugs
**Risk**: HIGH — data-integrity bugs + attack surface
**Status**: SHIPPED/ARCHIVED (130-S/131-S)
**Deliberation**: 060-DL
**Plan**: docs/exec-plans/2026-08-28-checkpoint-write-security-hardening-plan.md

| Stash ID | Priority | Kind | Summary |
|---|---|---|---|
| 3A33E404 | high | bug | CreateCheckpoint accepts malformed JSON, writes corrupt file |
| E429A031 | medium | task | Checkpoint create-boundary context duplicate detection |
| EA1F5912 | medium | task | Classify syncWriteFileAtomic outcomes on checkpoint creates |
| 35A27CD0 | medium | task | Checkpoint filesystem containment hardening (symlink/TOCTOU) |
| F89CADB7 | low | bug | CheckpointContext.Extra validation in emit() |


## Group 2 — syncWriteFileAtomic Windows Pre-Remove Fix (STAGED — 132-S)

**Priority**: SECOND — P1 data-loss bug, FC-4 mandate, operator directive
**Risk**: MODERATE — single-function bug fix with AST-verified regression coverage
**Status**: Staged as shipment 132-S
**Provenance**: Post-activation causal item from 131-S P-002 audit (CB71B412)
**Plan**: docs/exec-plans/2026-08-29-remove-syncwritefileatomic-preremove-plan.md

| Stash ID | Priority | Kind | Summary |
|---|---|---|---|
| CB71B412 | high | bug | syncWriteFileAtomic Windows pre-Remove data-loss risk |

**Grouping rationale**: Isolated — no code dependency on any other stash item.
CB71B412 touches only `internal/events/fsutil.go`. P1 severity and FC-4 mandate
require it to ship before lower-priority convergence/refactor groups.

## Group 3 — Docline Decode Convergence + Contract Cleanup (DEFERRED)

**Priority**: THIRD — high-priority convergence + upstream template
**Risk**: LOW-MEDIUM — refactor + doc update
**Status**: Active stash, awaiting Group 2 shipment completion

| Stash ID | Priority | Kind | Summary |
|---|---|---|---|
| 1787FD85 | high | task | Converge LintTree/PlanMigration on classifyDecodeFailure |
| 360A183F | high | task | Upstream checkpoint context wording to template |
| EC987334 | medium | task | Drop omitempty from MigrateReport collection fields |

## Group 4 — Checkpoint Governance & Disposition Hardening (DEFERRED)

**Priority**: FOURTH — deliberation-gated + state machine hardening
**Risk**: MEDIUM — deliberation-gated, deprecated field removal
**Status**: Active stash, requires deliberation resolution first

| Stash ID | Priority | Kind | Summary |
|---|---|---|---|
| 6CE00B88 | medium | unknown | Decide gitignore/redaction posture for checkpoint context |
| 5F4E0FC3 | medium | unknown | Decide whether create_checkpoint becomes governed |
| A12BBAFA | medium | task | Quarantine sidecar no-clobber write hardening |
| F350503F | medium | task | Remove deprecated CheckpointSummary.RemediationCommand |
| 6FA45E69 | low | task | Pin conforming+resolved double-refusal state-conflict test |
| DBBA62AA | low | task | CLI coverage for checkpoint resolve on abandoned doc |

## Group 5 — CLI/MCP Parity + Harness Hygiene (DEFERRED)

**Priority**: FIFTH — parity, hygiene, low blast radius
**Risk**: LOW — no security surface
**Status**: Active stash, some entries depend on Group 3 deliberations

| Stash ID | Priority | Kind | Summary |
|---|---|---|---|
| EB93E236 | low | task | Factor transport-agnostic response builder |
| 63E810D9 | medium | task | Structured JSON error envelope for CLI validation |
| 5672D73E | low | task | Add backlogit_docs_classify MCP tool |
| 66834D9E | low | task | Update commit-message.instructions.md scope guidance |
| BE32CAE2 | low | task | Forward repair for under-advanced shipment records |
| 633818E1 | low | task | Plugin bundle P-002 parity scope boundary recording |

## Summary

| Group | Entry Count | Priority Order | Status |
|---|---|---|---|
| Group 1 (130-S/131-S) | 5 | FIRST | SHIPPED/ARCHIVED |
| Group 2 (132-S) | 1 | SECOND | STAGED (CB71B412) |
| Group 3 | 3 | THIRD | DEFERRED |
| Group 4 | 6 | FOURTH | DEFERRED |
| Group 5 | 6 | FIFTH | DEFERRED |
| **Total** | **21** | | |

**Post-activation causal item**: CB71B412 (1 entry) added to scope per
P-002 incident decision INC-P002-131S-148F, FC-4 sequencing constraint.
