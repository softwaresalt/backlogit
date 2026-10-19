# Served-Root Handoff Procedure outcome — 196-S (R8 record)

- Authorization: P-017 dark-mode bounded scope naming `196-S` (operator, 2026-10-04); plan `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md` at commit `a3110460` (carries Amendment 1).
- Checkout: `main` at `2e89bfb6` (PR #475 merge).
- a. Workspace root: PASS — main worktree (`--git-common-dir` == `--git-dir` == `<root>/.git`).
- b. Storage root: PASS — exactly one of `.backlog` / `.backlogit` exists (`.backlogit`).
- c. Canonicalize: PASS — no reparse-point component; storage root is a direct child of the workspace root.
- d. Bind: PASS — `196-S` matches `^[0-9]+-S$`; catalog `workspace.root_path` == `<root>`, `workspace.storage_root` == `<root>/.backlogit`; `pragma_database_list` main row == `<root>/.backlogit/backlogit.db` (exactly one row); manifest only in `queue/` (not `archive/`), no reparse component; static `id`/`status`(queued)/`updated_at`(2026-10-04T05:43:33.6526021Z)/ordered items (10) match MCP `backlogit_get_shipment` with no sync needed.
- e. Halt: not triggered.
- f. Pass: served roots and binding evidence passed to Ship; served binary commit `131577c` (v1.11.0).
