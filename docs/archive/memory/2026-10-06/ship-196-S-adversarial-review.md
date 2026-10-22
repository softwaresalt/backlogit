# Ship 196-S: pre-PR adversarial review (cycle 1)

Reviewed HEAD `423ae694` against origin/main `c4c458b9`. Report-only reviewers ran on four model tiers:

| Reviewer | Model | Findings |
|---|---|---|
| Anchor (code-review) | gpt-6-sol, high | 2 x P1 |
| Correctness Reviewer | claude-opus-5 | P0/P1 0, P2 5, P3 6 |
| Go Reviewer | gpt-5.6-terra | none |
| Schema-CLI-Docs Coupling | claude-haiku-4.5 | 1 x P0, 2 x P1, 2 x P2, 1 x P3 (all triaged below) |

Final triage: P0 = 0, P1 = 0. Outcome: READY_WITH_FOLLOWUPS.

* Anchor P1, Safe-Close step 2 "Reject blocked or other nonterminal shipment state" rejects active:
  * The sentence is pre-existing and identical on origin/main. The diff did not introduce it.
  * It is plan-verbatim (A3 quote, plan line ~1773) and is not on Ship's live Step 6 path, which calls ShipShipment
    directly.
  * Reclassified P2 pre-existing and deferred. Reuse `6387A6A2` (item 1, positive match).
* Anchor P1, E1 approval is inferred:
  * This is a governance residual already disclosed in the E1 header.
  * The Orchestrator dispatch explicitly selected and confirmed `checkpoint-20261006-073956.json` under recorded
    operator authority.
  * The operator's later verbatim E1.4 instruction directly prescribes the E1 resume mechanics.
  * Ship cannot edit plans (P-010). Recorded as residual risk, not a code finding.
* Coupling reviewer, each refuted:
  * P0 safe-close atomicity: SKILL already mandates the under-lock re-check, and the skill is agent-executed, with no
    Go implementation.
  * P1 `pragma_database_list` syntax: a valid SQLite table-valued pragma that passes `db.ValidateQuery`.
    `TestUSR7_` executes it through MCP and is GREEN.
  * P1 `--cwd` placement: a cobra persistent flag, accepted after the subcommand (used all session).
  * P2 manifest-read wording: a frontmatter read is not the checkpoint state dump.
  * P3 checksum CI pin: autoharness drift tooling owns this.
* Correctness P2 gate-disabled U4 evidence: refuted.
  `validateMemberGateEvidence` (`internal/core/shipment_gate.go`) skips non-task/subtask members, so an active
  feature needs no gate evidence.
* Correctness P2 U6 checksum currency: verified.
  Recipe output equals the manifest for all three files at `423ae694`, and was recorded in the 196.006-T comment.
* P-021 deferrals, reused (positive match):
  * `7C9340AC`: Step 4.0 ordering, per-claim pragma, "immediately before".
  * `6387A6A2`: Safe-Close nonterminal; condition without `expected_status`.
  * `52D18E44`: Ship Step 6 never invokes safe-close.
* P-021 deferrals, new:
  * `5B2F5EC1`: attestation trust-claim wording / independent anchor.
  * `4E0F44CE`: pre-archived `archived_status`.
  * `5E3FBAC7`: non-shipment recovery disposition.
  * `CC86D2E1`: U4/U8 harness hardening.
* No code fix was required. Review-fix cycles used: 1 of 3.