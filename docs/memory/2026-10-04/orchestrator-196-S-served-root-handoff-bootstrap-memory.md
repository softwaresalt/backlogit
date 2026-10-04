# Orchestrator Memory: 196-S Served-Root Handoff Bootstrap Record

This is the R8 outcome record and the bootstrap authorization record. They are required by
`docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, in the section
"Exception and Dispatch Preconditions (bootstrap)". All paths are workspace-relative.

## Authorization

* **Source:** P-017 dark-mode bounded scope set by the operator on 2026-10-04: "Run the
  pipeline in dark factory mode strictly scoped to the next shipments (196-S)". Per the
  plan, a dark-mode scope that names the shipment counts as explicit operator authorization.
* **Plan:** `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`
* **Amendment 1 carrier commit:** `0806278b` (a later commit of the plan, `a3110460`, is
  also an ancestor of `main` at `2741626a`)
* **Requirements applied:** R1 to R8 from the plan, plus R14 for Ship, until U9 is merged
  and loaded.

## Procedure Results (shipment `196-S`)

| Step | Result | Evidence (workspace-relative) |
|---|---|---|
| a. Workspace root | pass | `git rev-parse --show-toplevel` returned `.` (the main worktree). `--git-common-dir` equals `--git-dir`, both `./.git` |
| b. Storage root | pass | `.backlogit` exists and `.backlog` does not, so exactly one exists |
| c. Canonicalize | pass | No symlink or reparse-point component. `.backlogit` is a direct child of the workspace root |
| d. Bind: ID pattern | pass | `196-S` matches `^[0-9]+-S$` |
| d. Served-Root Attestation | pass | `workspace.root_path` is `.` and `workspace.storage_root` is `.backlogit`. `pragma_database_list` returned exactly one `main` row with file `.backlogit/backlogit.db` |
| d. Manifest location | pass | Present at `.backlogit/queue/196-S.md` and absent from `.backlogit/archive/` |
| d. Static check | pass | `id`, `status` (queued), `updated_at` (`2026-10-04T05:43:33.6526021Z`), and the ordered `custom_fields.items` (10 entries) match MCP `backlogit_get_shipment`. No sync was needed |
| e. Halt | not triggered | — |
| f. Pass | pass | Ship was re-dispatched with both served roots and the binding evidence, citing the plan path and R14 |

## Prior Halt

The first Ship dispatch halted with `SERVED_ROOTS_UNRESOLVED`, because it did not include the
handoff. See `docs/memory/2026-10-04/ship-196-S-preclaim-blocked-served-roots.md` and the
checkpoint `.backlogit/checkpoints/checkpoint-20261004-190203.json`. No claim or mutation
occurred.
