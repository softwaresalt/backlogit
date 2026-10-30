# Ship 207-S served-root handoff outcome

* Main worktree verified: `HEAD == origin/main == 45c2f19063fa06c990ee727404a486d706b3b8de`, `git status --porcelain` empty.
* Catalog and `pragma_database_list` attested `.backlogit` and `.backlogit/backlogit.db`.
* `.backlogit/queue/207-S.md` frontmatter matches `backlogit_get_shipment(id="207-S")`.
* `pre_claim` topology gate passed (`exit_code: 0`, `BRANCH_CREATE_ELIGIBLE`, `WORKTREE_CLEAN`).
* Condition (b) on `154-S` verified from `.backlogit/archive/154-S.md` (`archived_status: shipped`) and `.backlogit/logs/154-S.jsonl` (`CONDITION_B_ATTESTED` after `shipped`).
* Ship: `BRANCH_CREATED: feat/207-s-replan-queued-shipment-manifests-without-hand-edits-206-f`; shipment claimed; `CLAIM_VERIFY_OK` after one bounded re-verify (first post_claim read returned `CLAIM_NOT_OBSERVED` due to index lag).
