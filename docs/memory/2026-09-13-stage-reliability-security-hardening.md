# Stage Session Memory — 2026-09-13 reliability + PR#425 security hardening

**Mode**: P-017 dark-factory, DARK_MODE_ACTIVE. Stage-only (no source/build/branch/PR).
**Commit**: 88c54c7a (harvest) then review-fix cycle 1 on branch `chore/stage-154-s-reliability` (NOT pushed — integration reserved for Ship / operator).

## Dark-mode activation scope (P-017)
- **Activation scope**: full staging pipeline for the 2026-09-13 reliability + PR#425 security-hardening backlog on branch `chore/stage-154-s-reliability`; planning artifacts only (exec-plans, deliberations, `.backlogit/queue` tasks, shipment manifests, memory). NO production source, build, test execution, push, or PR create/merge.
- **merge_preapproval**: true (Ship may merge the resulting shipments without a further Stage gate once its own gates pass).
- **admin_fallback**: false (no admin-branch fallback authorized; work stays on the feature/chore branch).
- **preclaim_force**: false (`--force` pre-claim NOT authorized; 152-S implicit numeric predecessor caveat stands as a Ship-time claim decision).
- **Stop conditions**: STOP if any P0/P1 review finding remains after remediation; STOP if a fix would require touching production source / build / push / PR or otherwise exceed authorized planning scope; STOP on role-boundary (P-010) or gate-integrity (P-005/P-006) violation.
- **Preservation invariants**: P-009 (no self-authorized promotion beyond planning) and P-016 (single-active cursor; no parallel resume / new worktree) preserved throughout.
- **Visibility mode**: local-only (agent-intercom pack not active this session); milestones recorded to this memory record instead of broadcast.
- **Final reviewed HEAD semantics**: the review gate re-runs against the working-tree state that becomes the review-fix-cycle-1 commit HEAD on `chore/stage-154-s-reliability`; that commit is the authoritative reviewed HEAD handed to Ship. Prior harvest HEAD 88c54c7a is superseded for review purposes.

## Stash triage (8 scoped IDs, all archived `state: removed`)

| Stash | Kind | Route | Disposition |
|---|---|---|---|
| 6D53A33F | bug | deliberate | Cross-workspace (autoharness gate presentation). No in-repo item. Non-blocking. Decision doc recorded. |
| CC0EBB59 | bug | deliberate→plan→harvest | 173-F / 154-S (enabling precondition) |
| 1E0C2251 | bug | deliberate→plan→harvest | 172-F / 153-S (with FE440C62) |
| FE440C62 | bug | deliberate→plan→harvest | 172-F / 153-S (with 1E0C2251) |
| 3B661FCE | feature | deliberate→plan→harvest | 168.007-T/168.008-T → 149-S |
| BFF76433 | feature | deliberate→plan→harvest | 169.007-T/169.008-T → 150-S |
| BF18DA1D | feature | deliberate→plan→harvest | (merged w/ BFF76433) 169.007-T/169.008-T → 150-S |
| 4E210DB4 | feature | deliberate→plan→harvest | 170.008-T → 151-S |

## Grouping (Step 1.5)
- Group A (concurrency, same code surface): FE440C62 + 1E0C2251 → 172-F / 153-S. Internal edge FE→CAS.
- Group B (shipment lifecycle surface): CC0EBB59 → 173-F / 154-S.
- Group C (PR#425 workspace-writer threat model): 3B661FCE / BFF76433+BF18DA1D / 4E210DB4 → hardening tasks into existing 168/169/170 + 149/150/151-S.

## Artifacts created
- Decisions: 6d53a33f-disposition, concurrency-writer-hardening-deliberation, shipment-claim-scheduler-reconciliation-deliberation, pr425-workspace-writer-hardening-deliberation.
- Plans: concurrency-writer-hardening-plan, shipment-claim-scheduler-reconciliation-plan, pr425-workspace-writer-hardening-plan.

## Gates
- Plan hardening (P-006): all three plans declared + carry `## Plan Hardening`. Satisfied.
- Plan review (multi-agent-dispatch): attempt 1 FAIL (P1 each), remediated, attempt 2 PASS all three. Records appended to each plan.

## Shipments
- 153-S (queued, root): 172-F + 172.001–007-T.
- 154-S (queued, root): 173-F + 173.001–005-T.
- 149-S (queued): +168.007-T, +168.008-T.
- 150-S (queued): +169.007-T, +169.008-T.
- 151-S (queued): +170.008-T.

## Dependency edges (real contracts; NO numeric adjacency added)
- 172: 002←001; 003←002; 004←002,003; 005←002; 006←002; 007←005,006.
- 173: 002←001; 003←001; 004←001,002,003.
- 168.007←168.001,168.003; 168.008←168.007; 169.007←169.002,169.003; 169.008←169.007; 170.008←170.003.
- Existing DAG (141-S chain; 149/150/151 edges) unchanged. Reliability shipments are independent roots; reliability-first execution order is a scheduling preference only.

## Blockers / caveats for the dark run
- 152-S claimability: autoharness pre_claim implicit numeric predecessor still active (6D53A33F remedy is out-of-workspace; `--force` NOT authorized). Ship-time claim caveat, not a Stage blocker.
- 170.008-T: Option A required before Ship claims 151-S, else block 151-S (no document-only closure).

## Next steps (Ship, not Stage)
- Claim shipments in reliability-weighted order; confirm hardening-task sequencing before claiming 149/150/151-S.

## Review-fix cycle 1 (DARK_MODE_ACTIVE) — same-contract-surface completions

**Trigger**: local review gate findings on the 88c54c7a harvest (7 P1 blockers + P2 integrity items).

### P1 resolutions (final reviewed contracts)
1. **Claim-scheduler (173-F)**: universal claim-activation marker is the final contract; record-only claim mode DROPPED everywhere (plan U1–U5, deliberation, tasks 173.001/002). Rollback (U3/173.003) clears the marker key and restores an emptied `custom_fields` map to nil.
2. **PR#425 replay (170.008-T)**: documentation-only closure FORBIDDEN. Requires externally-protected ATOMIC compare-and-consume nonce state; any fold proof must rest ENTIRELY on independently-protected state. Otherwise 151-S blocked.
3. **168 trust anchors**: 168.009-T (new) makes revoke authoritatively externally revoke/tombstone — advisory-only status flip returns non-success/fails closed. 168.008-T no longer permits a documentation-only role/validity residual (fail-closed enforcement required). 170.009-T (new) makes required-auth enablement externally authoritative so removing/malforming workspace auth policy cannot downgrade 170-F enforcement.
4. **169 attestation**: 169.007-T persists durable independently-verifiable signed material (envelope or digest-pinned protected reference), SUPERSEDING the presence-based forgeable 169.003-T branch; 169-F NOTE + 169.003-T updated to remove the stale metadata-presence-only acceptance model.
5. **172 concurrency**: reviewer disagreement resolved as INDEPENDENT — Unit 2 (archived_status CAS) no longer depends on Unit 1 (lock order); artificial 172.005←172.002 / 172.006←172.002 edges REMOVED, rewired to the Unit-2 RED harness. Stale-write scope explicitly narrowed to `archived_status` everywhere (general whole-artifact CAS/reload-merge is out of scope, stated in-task).
6. **Test-first ordering restored (NON-NEGOTIABLE)**: added RED predecessor harness tasks + edges so tests are written and observed failing before implementation — 172.008-T (lock-order barrier), 172.009-T (stale-write), 173.006-T (claim lifecycle), 173.007-T (read surface), 168.010-T (revoke/role/validity), 169.008-T (repurposed to RED attestation), 170.010-T (replay/auth-downgrade). GREEN work split out: 172.010-T (BulkUpdate GREEN), 169.009-T (attestation GREEN). Each ≤4 scenarios / ≈2h.
7. **Security-safe rollback**: plan hardening rollback guidance rewritten — rollback disables the affected security feature or rolls back the whole shipment while PRESERVING external revocation + nonce-consumption state; never restores forgeable/replayable behavior.

### P2 integrity fixes
- Causal event ordering preserved on lock moves; deterministic lock-barrier/instrumentation verification (NOT `go test -race` as proof of no ABBA inversion) — 172.001/172.002/172.008.
- Typed per-item conflict for BulkUpdateStatus (id + status transition), not bare `[]string` — 172.006/172.010.
- Explicit `custom_fields` marker deletion + nil-map restoration; `omitempty` applies to the MAP field, not individual keys — 173.001/173.003.
- Internal/core-only accessor replaced by a SUPPORTED CLI/MCP/data-transport read surface — 173.002/173.005.
- doc_type frontmatter `learning`→`decision` on all four new decision files.
- Split oversized test tasks: 169.008-T (→RED 169.008 + GREEN 169.009), 172.007-T (→172.007a RemoveArtifactLink GREEN + 172.010 BulkUpdate GREEN), 173.004-T (narrowed to ≤3 concurrency/integration scenarios; lifecycle scenarios moved to RED 173.006/173.007).
- Dark-mode memory record updated (this section + activation-scope block).
- **Provenance (LIMITATION)**: governed canonical-delivery correction is UNSUPPORTED for the 7 generically-archived entries — `stash correct` requires a prior `harvested_artifact_id` (absent), and `stash harvest` would mint duplicate features (scope violation). Hand-editing `archive/stash.jsonl` is prohibited. Compensating machine-readable provenance is preserved via embedded `[STASH-ID]` tags in every task/plan/deliberation body + plan `references:`. 6D53A33F remains generic by design. Recorded as residual P2.

### New task DAG (cycle 1)
- **172** (Unit 1 lock-order): 172.008(RED,←172.001); 172.002←172.001,172.008; 172.004←172.002,172.003,172.008. (Unit 2 archived_status, INDEPENDENT): 172.009(RED,no dep); 172.005←172.009; 172.006←172.009; 172.007(RemoveArtifactLink GREEN)←172.005,172.009; 172.010(BulkUpdate GREEN)←172.006,172.009.
- **173**: 173.006(RED),173.007(RED) no dep; 173.001←173.006; 173.002←173.007; 173.003←173.006; 173.004←173.001,173.002,173.003; 173.005←173.002.
- **168**: 168.010(RED)←168.001,168.003; 168.007←168.010; 168.008←168.007,168.010; 168.009←168.005,168.007,168.010.
- **169**: 169.008(RED)←169.003; 169.007←169.002,169.003,169.008; 169.009(GREEN)←169.007.
- **170**: 170.010(RED)←170.003; 170.008←170.003,170.010; 170.009←170.002,168.007,170.010.

### Shipment membership (cycle 1 additions)
- 149-S: +168.009-T, +168.010-T. 150-S: +169.009-T. 151-S: +170.009-T, +170.010-T.
- 153-S: +172.008-T, +172.009-T, +172.010-T. 154-S: +173.006-T, +173.007-T.
