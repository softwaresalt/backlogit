---
title: "Compacted memory: 2026-09-27 autoharness tune pass and PR 452 merge with 153-S halt"
date_range: "2026-09-27 to 2026-09-27"
doc_type: compacted-memory
source_count: 2
---

# Compacted memory: 2026-09-27

Compacts 2 source files. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

Autoharness tune (Auto-Tune 1.5.0, branch chore/autoharness-tune-2026-09-27):
- Lock scripts were refreshed to the capability-token and containment model, with matching updates to the concurrency instruction, the file-lock skill, and the Stage and Ship lock commands.
- model_routing tier routes were propagated into the frontmatter of 18 agents. The anchor route became gpt-6-sol. The escalation route was first set to claude-opus-5.5 / anthropic / xhigh, then the operator approved gpt-6-sol / openai / xhigh as a cross-family escalation route.
- Staged renders were not adopted wholesale. They dropped model_tier, demoted the H1 heading, and left QUALITY_GATE placeholders unresolved.
- The 79 checksum user-modified hits were CRLF false positives, confirmed by LF re-hashing.
- Installing the binary to the operator-owned install location outside the workspace stayed an operator action. The executable was locked by running MCP servers.

Orchestrator (PR 452 and 153-S):
- Operator-suggested order: step 1, Stage resumed checkpoint 005049 and harvested CB8887AF into shipments 177-S to 181-S. Step 2, the operator's three model-routing files were committed on stage/cb8887af-test-budget-ensembles, and staging PR 452 merged at 505b1bd3 (merge commit). Local main was then fast-forward synced. Step 3, ship 153-S halted at the pipeline-topology gate.
- 153-S halt: the pre-claim gate returned UNSEQUENCED_SHIPMENT. Decision condition (b) forbids claiming unrelated multi-member shipments until the 154-S marker exists and the external scheduler consumes it. Stage recorded a real edge, 153-S blocked on 154-S (commit 66431e46, branch stage/153-s-dag-sequencing).
- The 154-S shipped-provenance rule was recorded: live status shipped, or archived with archived_status shipped. Terminal statuses such as abandoned do not count.
- Routing hold: 141-S, 152-S, 147-S, 177-S, 178-S, and 179-S must not be routed until the 154-S waiver and marker conditions are met.

PR 452 review handling:
- Local review returned READY_WITH_FOLLOWUPS with no P0 or P1. Two P2s and one P3 about routing drift were captured as FB0A850B.
- Copilot thread: CB8887AF lacked source_stash_id. Stage made 176-F the single canonical carrier (commit 80f06b1e).
- Copilot thread: the constitution bump was reclassified from PATCH to MINOR (1.0.0 to 1.1.0) in commit 80f06b1e. A stale PATCH note in the harvest memory was fixed in commit c451d922.
- The P-018 Copilot review gate returned SATISFIED before the merge.

## Files modified or created

- .autoharness/harness-manifest.yaml (LF checksums, variables, tuned_at, tuning_history)
- .autoharness/tuning-reports/2026-09-27-tuning-report.md (full report) and .autoharness/backups/2026-09-27/ (pre-edit backups)
- Lock scripts: scripts/acquire_lock.* and scripts/release_lock.*; concurrency instruction; file-lock skill; Stage and Ship lock commands
- 18 agent frontmatters (model routes); orchestrator routing table; Ship closure-sequence phrase; backlog-integration registry rows; security-reviewer Scope Disposition section; constitution P-016 and P-020 text; spike docline and graphtor-docs guidance
- docs/decisions/2026-09-27-autoharness-upstream-fix-proposals.md (upstream proposals)
- bin\backlogit-main.exe built from main with a Go pseudo-version stamp
- .backlogit/queue/153-S blocked on 154-S edge (commit 66431e46)

## Key learnings

- Multiline PowerShell regex with .*$ consumed carriage returns. Use [^\r\n] instead.
- An ordinal sort misplaced a backtick-prefixed table row. Strip backticks from the sort key.
- The artifact indent in the manifest is 4 spaces, not 6.
- The backlogit registry parity test rejects bracketed alternatives in cli_command. Use plain {{param}} flags and document alternatives in rationale.
- The verifier requires the literal label "Source artifact cleanup" in operational-closure.
- The P3 lock-script fix had dropped 11 stderr messages. The restore was verified under PowerShell 5.1 and 7.
- A git describe version stamp sorts as a prerelease and falsely reports an update. A Go pseudo-version stamp avoids that.
- Pre-claim topology enforcement relies on the gate. Prose-only rules are not enforced by the queue.

## Failed or rejected approaches

- Adopting staged renders wholesale (rejected).
- Claiming multi-member shipments before the 154-S marker (rejected by condition b).
- A git describe build stamp (rejected in favor of a pseudo-version).
- Relying on the queue filter alone to prevent routing behind 154-S. Edges alone were not sufficient enforcement.

## Outcomes and open items

- The auto-tune diff needed review and a PR against main. TUNE-009 to TUNE-012 needed a hand-merge. Several were applied in the second session.
- Operator decisions carried: the cross-family escalation route (decided: gpt-6-sol), and custom model keys in sqlite-reviewer and go-mcp-expert.
- Halted: 153-S routing. 141-S, 152-S, 147-S, 177-S, 178-S, and 179-S were not routed.
- Open: FB0A850B (routing drift) to triage; 513E62AB (condition b enforcement) to triage.

## Sources

- docs/memory/2026-09-27/autoharness-tune-memory.md
- docs/memory/2026-09-27/orchestrator-pr452-merge-and-153-s-halt-memory.md
