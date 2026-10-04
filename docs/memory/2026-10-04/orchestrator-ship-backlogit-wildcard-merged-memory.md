# Orchestrator Memory: Full backlogit Tool Surface Merged

## Outcome

* PR #476 merged into `main` at `2741626afdd92e16262c365c689a54545e298f6e` using a merge commit.
* Every mutating workspace agent now has `backlogit/*`: Ship, go-engineer, prompt-builder, adversarial-review, review/security-sentinel, and subagents/security-sentinel.
* Read-only reviewer personas are still exempt.
* `TestWorkspaceAgentsGrantFullBacklogitToolSurface` walks `.github/agents/**` dynamically. It fails if a mutating agent lacks `backlogit/*` or if any agent lists explicit `backlogit/backlogit_*` entries.
* Archived as superseded: stash 4A0B7BCF, C24BC82F, and EC43AB70, plus deliberation 071-DL.

## Next Steps

1. Restart the CLI so the updated agent definitions load.
2. Re-run `Run pipeline in dark mode` scoped to 196-S, which is still `queued`.
3. Follow-up: the upstream autoharness templates still lack `backlogit/*`. A later tune could reintroduce explicit allowlists.
