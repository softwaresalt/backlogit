---
chunk_strategy: h1-h2-h3
description: "Execution plan for S1: checkpoint disposition security hardening, evidence-integrity, and schema hygiene"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s1-checkpoint-disposition-hardening-plan.md
title: "S1 Execution Plan — Checkpoint Disposition Hardening & Hygiene"
---

# S1 Execution Plan — Checkpoint Disposition Hardening & Hygiene

**Covering feature**: Checkpoint disposition security, evidence-integrity, and schema hygiene
**Deliberation**: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md (6CE00B88 decision)
**Stash members**: 302EFF07, A12BBAFA, F350503F, 6FA45E69, DBBA62AA, 6CE00B88
**Tier**: reliability/security (shipment sequence S1, first eligible)

## Problem Frame

The checkpoint disposition subsystem (internal/core, internal/events) carries a
symlink-traversal read gap, a sidecar write that lacks its own no-clobber
guarantee, a deprecated shell-unsafe schema field, thin test coverage on two
state-conflict classes, and an open redaction posture for git-tracked checkpoint
context. These are reliability/security defects and are staged first.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; errors wrapped with %w |
| II. Test-First (P-002) | Each unit: declaration -> RED harness -> GREEN impl (separate commits) |
| III. Workspace Isolation | All checkpoint paths resolve within workspace root; Lstat-gated |
| IV. CLI Containment | No writes outside cwd tree |
| V. Observability | Structured disposition evidence preserved |
| VI. Single Responsibility | Reuses existing seams (ensurePathContained, moveNoReplace) |
| VII. Destructive Approval | No destructive ops; removals gated by compat proof |
| VIII. Safety Modes | Fail-closed: symlink/oversized/collision rejected, not repaired |
| IX. Git-Friendly | Markdown/YAML artifacts |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 enforced by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Reject symlinked checkpoint targets in read/mutate-lite verbs (302EFF07, bug)
* Scope: add symlink rejection to the shared seam (RewriteCheckpointFile / ensurePathContained) so GetCheckpoint, GetCheckpointResult, ResolveCheckpoint inherit it uniformly. Resolve the FULL path (filepath.EvalSymlinks, or O_NOFOLLOW / Lstat each component) and reassert workspace-root containment against the RESOLVED path — not only the leaf filename — so a symlinked intermediate directory cannot escape. Prefer O_NOFOLLOW on the actual open to close the Lstat->open TOCTOU window.
* Files: internal/events checkpoint read path (~ensurePathContained), tests.
* Acceptance: a symlinked checkpoint filename AND a symlinked intermediate directory are both rejected by GetCheckpoint, GetCheckpointResult, and ResolveCheckpoint with ErrCheckpointTargetUnsafe; RED reproducers cover both the leaf-symlink and intermediate-dir-symlink cases against the pre-fix read path.
* Runtime surface: CLI/MCP checkpoint read verbs. Verification: RED tests read through symlink pre-fix, GREEN rejects.

### U2 — Sidecar create-only (no-replace) write (A12BBAFA)
* Scope: give the `<filename>.disposition.json` sidecar its own O_EXCL-equivalent create-only write path in internal/core/checkpoint_disposition.go so it cannot silently overwrite prior quarantine evidence independent of the payload move.
* Acceptance: (1) sidecar-only collision (fresh payload dest, occupied sidecar) refuses without clobber; (2) concurrent same-filename quarantine race refuses the second writer's sidecar without clobbering the first.
* Runtime surface: quarantine disposition. Verification: two RED tests as above, GREEN refuses.

### U3 — Checkpoint-context write-boundary guard + fail-closed secret scan (6CE00B88 decision)
* Scope: at the checkpoint-context write boundary add (a) a bounded, fail-closed size guard and (b) a fail-closed heuristic secret-pattern scan (reject on high-entropy tokens / known key prefixes) — because `.backlogit/checkpoints/` stays git-tracked and Principle XI forbids history rewrite, a prose caveat alone is not a control for an irreversible exposure. Git-tracking posture UNCHANGED. Full key-allowlist DEFERRED (open follow-up) but the heuristic scan is the enforceable floor.
* Acceptance: an over-limit context write is rejected fail-closed; a context write containing a high-entropy token / known secret prefix is rejected fail-closed with a clear error; caveat documented at the write path; no change to `.backlogit/checkpoints/` tracking.
* Runtime surface: create_checkpoint context write. Verification: RED oversized-context test AND RED secret-pattern test both rejected.

### U4 — Remove deprecated CheckpointSummary.RemediationCommand (F350503F)
* Scope: after confirming every consumer reads structured RemediationIntent, remove the field, its JSON tag, remediationQuarantineCommand, and asserting tests (checkpoint_corpus_test.go, checkpoint_lifecycle_test.go incl. TestListCheckpoints_RemediationCommandIsShellSafe).
* Acceptance: compatibility test proves no remaining caller reads RemediationCommand; removal test proves field + helper are gone; CLI/MCP checkpoint list/get unaffected.
* Runtime surface: checkpoint list/get output. Verification: compat scan + removal test.

### U5 — Pin conforming+resolved double-refusal invariant (6FA45E69, test-only)
* Scope: pin the conforming + status:resolved checkpoint double-refusal state-conflict class (I3 row 3) as a tested invariant in a unit that owns it.
* Acceptance: one focused unit asserts the double-refusal on a conforming+resolved document; no production delta.

### U6 — CLI coverage: resolve on already-abandoned checkpoint (DBBA62AA, test-only)
* Scope: add CLI coverage for `backlogit checkpoint resolve` on an already-abandoned checkpoint document in its own unit.
* Acceptance: assertion covers the already-abandoned resolve path; kept in its own unit (four-scenario limit respected).

## Dependency Graph

U4 (removal) after U1/U2/U3 land (shared checkpoint files, reduce churn). U5, U6
are test-only and independent. Suggested order: U1, U2, U3, U4, U5, U6.

## Runtime Verification and Closure

Units U1-U4 change checkpoint runtime surfaces; each requires a genuine RED
reproducer against the pre-fix path and a GREEN pass. Operational closure: no new
monitoring; regression tests are the durable closure artifact.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| U1 Lstat symlink rejection at shared read seam | High — traversal reads an out-of-tree file | Reject at the shared seam (ensurePathContained/RewriteCheckpointFile) so all read verbs inherit uniformly; RED reproducer reads through symlink pre-fix |
| U2 sidecar create-only write | Medium — silent overwrite of prior quarantine evidence | O_EXCL-equivalent create-only path; two RED tests (sidecar-only collision, concurrent race) |
| U3 checkpoint-context size guard | Medium — unbounded/unredacted durable state committed to git | Fail-closed bounded size guard + secrets caveat; tracking posture unchanged (Principle XI); key-allowlist deferred as open follow-up |
| U4 remove RemediationCommand field | Medium — breaking a public summary field | Compatibility scan proves no remaining reader before removal; removal + compat tests |

Rollback trigger: all units are additive guards or a compat-gated removal; a
regression is caught by the RED/compat tests before merge. Ownership: checkpoint
subsystem maintainers. Open follow-up: checkpoint-context key-allowlist (deferred,
YAGNI) recorded for a future decision.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — U4 removes a public summary field (compat-gated by U4 acceptance).
* security/auth/permission/compliance-sensitive: PRESENT — U1 symlink traversal, U3 unredacted durable state.
* migration/backfill/destructive/irreversible: PRESENT — U4 field removal (compat-proven first).
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent — changes are additive guards + a compat-gated removal.

Requires plan hardening: yes

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on), Security Reviewer (security-touching trigger: path traversal + unredacted durable state). Plan hardening was REQUIRED and is SATISFIED (## Plan Hardening present with ProposedAction/ActionRisk rows).

Findings and remediation:
- Security P2 (U1 symlink rejection covered only the leaf filename; parent-dir traversal + Lstat->open TOCTOU): REMEDIATED — U1 now resolves the full path (EvalSymlinks / O_NOFOLLOW per component), reasserts workspace-root containment against the resolved path, and adds an intermediate-dir-symlink RED reproducer.
- Security P2 (U3 secret protection was advisory-only against an irreversible git-tracked exposure): REMEDIATED — U3 now adds a fail-closed heuristic secret-pattern scan at the write boundary; full key-allowlist remains a recorded deferred follow-up.
- Architecture P3 (U3 bundles size guard + secrets caveat): acknowledged; both are the checkpoint-context write-boundary concern and remain one unit.
- Correctness: clean.

Residual advisory: checkpoint-context key-allowlist deferred (YAGNI) — recorded open follow-up, not blocking.
