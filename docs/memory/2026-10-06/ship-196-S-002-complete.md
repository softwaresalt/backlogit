# Ship 196-S: 196.002-T (U2) complete

* Shipment: 196-S (dark mode, P-017; scope [196-S] only). Wave 3, E1.4 order.
* Deliverable: `.github/agents/_orchestrator.agent.md`, commits `cdda97f9` (U2 Changes 1–4)
  and `231d008d` (Step 4.4 P3 wording fix).
* `exempt_baseline_sha`: `72a074f1` (clean tree). The must-fail probe exited 1 with
  "USR1 not green" and no marker.
* Step 4.3:
  * The exempt command printed `EXEMPT_VERIFY_OK:196.002-T`. Owner `^TestUSR1_` is GREEN
    (3/3 subtests).
  * USR7, claim-start, and 155 regressions are GREEN.
  * Path pass: the orchestrator file only. Content pass: docs only.
  * Vet and lint are clean. Markdownlint reports 0 issues.
  * Each of the six uniqueness anchors appears exactly once.
* Step 4.4: READY_WITH_FOLLOWUPS. P0=0, P1=0, P2=1, P3=6.
  * The P2 is deferred as `CB0F604E` (DISCOVERY-STATUS: AMBIGUOUS `2B8B3E84`). The R8
    `docs/memory/` record conflicts with Ship's P-011 clean-tree gate. This fails closed.
* E1.2 residual: A3.3 item 2 evidence for 196.001-T and 196.003-T was captured late and
  closed by deterministic recomputation against pinned SHAs.
* Stash usage (E1.4 rule 5): none.

## AC4 dry-run (read-only, main checkout, workspace-relative)

| Step | 196-S (queued/active path) | 195-S (archived) |
|---|---|---|
| a | PASS: main worktree, git-common-dir equals git-dir equals `<ws>/.git` | PASS (same) |
| b | PASS: exactly one storage root, `.backlogit` | PASS (same) |
| c | PASS: no reparse points; storage root is a direct child | PASS (same) |
| d attest | PASS: catalog `root_path` equals `<ws>`, `storage_root` equals `<ws>/.backlogit`; pragma returns 1 main row whose file is `<ws>/.backlogit/backlogit.db` | PASS (same) |
| d bind | PASS: `queue/196-S.md` only, status `active`; id, `updated_at`, and the ordered 10 items equal MCP | PASS: `archive/195-S.md` only, status `archived`; id, `updated_at`, and the ordered 8 items equal MCP |

No queued shipment exists, so 196-S, which is active, stands in for the queue path.

Negative case: the candidate root is the parent of `<ws>`. Step b FAILs because there is no
`.backlog` or `.backlogit` directory. The attestation also FAILs, because `root_path` does not
equal the candidate. The expected halt is `SERVED_ROOTS_UNRESOLVED`.

Next: 196.005-T (U5), then the wave-3 Step 4.6 gate.
