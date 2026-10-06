# Ship 196-S Erratum E1 Resume Record

## Session state

- Shipment `196-S`, P-017 dark mode, scope `[196-S]` only. `merge_approval_pre_authorized`
  is true and `admin_fallback_pre_authorized` is false.
- Branch
  `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`,
  HEAD `9e023324` at resume. `origin/main` is `c4c458b9`.
- Routing: the resolved route is gpt-6-luna / openai / xhigh. This runtime cannot honor it,
  so `ROUTING_DEGRADED` is declared.
- Plan: `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, Amendment 3,
  Erratum E1 (E1.1–E1.4).

## Resume (E1.3)

- Checkpoint `checkpoint-20261006-073956.json` (owner `ship`) was selected and confirmed by
  the Orchestrator under the operator approval recorded in the E1 header (`d9e00a6d`).
- Anomaly-first enumeration: 81 Ship checkpoints, 0 needing quarantine, 0 quarantined. The
  selected checkpoint is the only active `ship` candidate.
- E1.3 preconditions all passed:
  - E1 is effective.
  - Scope is `[196-S]`.
  - The tree is clean.
  - `git merge-base --is-ancestor d9e00a6d HEAD` exited 0.
- Phase 0′ guards held:
  - `.backlogit/hooks.yaml` is absent.
  - The local exclude still covers `checkpoint-20261005-052350.json`.
- R14 Served-Root Attestation passed:
  - Catalog `workspace.root_path` and `workspace.storage_root` equal the served workspace
    root and the served storage root `.backlogit`.
  - `pragma_database_list` returned exactly one `main` row, `.backlogit/backlogit.db`.
- Live state matches the checkpoint:
  - 196.001-T and 196.003-T are `active`, each with exactly one valid `WORK_STARTED: 196-S`
    in its current epoch.
  - 196.004-T, 196.007-T, and 196.008-T are `done` (archived).
  - 196.002-T, 196.005-T, 196.006-T, and 196.009-T are `active` claim-assigned, with no
    start record.
- After the confirmed resume, the checkpoint was resolved. This commit carries the
  resolution as the E1.1 resume step 1 traceability commit.

## Next step

Apply the E1.4 rule 1 order to 196.001-T, then 196.003-T. Then run the Phase 2 item 21
wave-2 Step 4.6 gate, then Phase 3.
