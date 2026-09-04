# Pre-067-S Memory Compaction — Repo-Wide Stale Checkpoint Sweep

- **Compacted:** 2026-06-27 (during 067-S post-merge closure, Step 6 item 8)
- **Trigger:** `docs/memory/` exceeded the mandatory 40-file threshold (164 files / 491.6 KB) per `.github/instructions/context-efficiency.instructions.md` and the ship agent's mandatory compact-context closure step.
- **Action (archive-only):** 150 verbose session/checkpoint files older than 14 days moved from `docs/memory/` to `docs/archive/memory/`, preserving relative paths (mirror mapping `docs/memory/<X>` -> `docs/archive/memory/<X>`). No file deleted. `docs/memory/` now holds 13 recent (<14d) checkpoints plus this `compacted/` index.
- **Durable knowledge preserved:** Every archived checkpoint belongs to an already-shipped, already-closed release unit. Durable decisions and learnings were graduated at each shipment closure into `docs/closure/`, `docs/compound/`, `docs/decisions/`, and `docs/design-docs/`. This sweep removes session verbosity, not substance.
- **Traceability:** Each archived original is retrievable at its mirror path under `docs/archive/memory/`, and in git history at its prior `docs/memory/` path.

## Counts by month (150 archived)

| Month | Files |
|---|---|
| 2026-04 | 111 |
| 2026-05 | 20 |
| undated | 19 |

_(Plus 4 pre-existing files already under `docs/archive/memory/`, untouched by this sweep.)_

## Archived file index

<details><summary>2026-04 (111)</summary>

- `docs/archive/memory/[20260406-191725]-groom-stash-triage-identity-cluster.md`
- `docs/archive/memory/[20260406-192757]-groom-deliberation-dl003-checkpoint.md`
- `docs/archive/memory/[20260406-195017]-groom-dl003-decisions-resolved.md`
- `docs/archive/memory/[20260406-195954]-015-shipment-validation-review-fixes-memory.md`
- `docs/archive/memory/[20260407-031000]-groom-dl003-harvest-complete.md`
- `docs/archive/memory/[20260407-032600]-ship-s001-harness-f016-t001.md`
- `docs/archive/memory/[20260407-033200]-ship-s001-f016-t001-complete.md`
- `docs/archive/memory/[20260407-034300]-ship-s001-progress-t001-t002.md`
- `docs/archive/memory/[20260407-040900]-ship-s001-f016-t003-complete.md`
- `docs/archive/memory/[20260407-054535]-ship-s001-pr-ready-awaiting-merge.md`
- `docs/archive/memory/[20260407-063839]-ship-s001-merged-and-shipped.md`
- `docs/archive/memory/[20260407-064823]-ship-s001-post-merge-pr9-ready.md`
- `docs/archive/memory/[20260407-065352]-ship-s001-awaiting-pr9-approval.md`
- `docs/archive/memory/[20260407-065825]-ship-s001-workflow-complete.md`
- `docs/archive/memory/[20260408-033915]-ship-002-s-shipped.md`
- `docs/archive/memory/[20260408-054000]-groom-group-a-backlog-shaped.md`
- `docs/archive/memory/[20260408-124500]-groom-group-g-data-quality.md`
- `docs/archive/memory/[20260408-210042]-pr-14-merge-stash-bom-fix.md`
- `docs/archive/memory/[20260408-233800]-ship-019-f-data-quality.md`
- `docs/archive/memory/[20260408-235900]-ship-019-f-post-merge-closure.md`
- `docs/archive/memory/[20260409-111705]-harness-scaffold-021-token-telemetry.md`
- `docs/archive/memory/[20260409-135417]-021-f-fix-ci-round2-pr15-ready.md`
- `docs/archive/memory/[20260409-140557]-021-f-shipped-009-s-closed.md`
- `docs/archive/memory/[20260409-142329]-pr16-review-comments-resolved.md`
- `docs/archive/memory/[20260409-164132]-pr16-merged-021-f-complete.md`
- `docs/archive/memory/[20260409-192100]-021-f-ci-fix-pr15-ready.md`
- `docs/archive/memory/[20260410-045522]-005-s-shipped.md`
- `docs/archive/memory/[20260423-160000]-042-s-pr-ready.md`
- `docs/archive/memory/[20260423-161726]-042-s-shipped-final-memory.md`
- `docs/archive/memory/[20260423-182610]-ship-043-s-pr-ready.md`
- `docs/archive/memory/[20260423-200000]-ship-043-s-final-memory.md`
- `docs/archive/memory/[20260424-174043]-043-s-archive-sync-pr-ready-memory.md`
- `docs/archive/memory/[20260424-175445]-043-s-archive-sync-complete-memory.md`
- `docs/archive/memory/[20260424-200000]-v1.1.0-release-complete-memory.md`
- `docs/archive/memory/[20260425-193400]-stage-telemetry-quality-harvest-memory.md`
- `docs/archive/memory/[20260428-100259]-plugin-distribution-ship-closure-memory.md`
- `docs/archive/memory/[20260428-120000]-harness-tune-memory.md`
- `docs/archive/memory/2026-04-10-ship-006-S-event-traceability.md`
- `docs/archive/memory/2026-04-10-ship-010-S-pr-ready.md`
- `docs/archive/memory/2026-04-10-stage-006-S-retroactive-gating.md`
- `docs/archive/memory/2026-04-10-stage-010-S-core-data-integrity.md`
- `docs/archive/memory/2026-04-10-stage-stash-triage.md`
- `docs/archive/memory/2026-04-11-ship-010-S-ci-remediation.md`
- `docs/archive/memory/2026-04-11-ship-010-S-execution.md`
- `docs/archive/memory/2026-04-11-ship-010-S-merged.md`
- `docs/archive/memory/2026-04-11-ship-011-S-pr-ready.md`
- `docs/archive/memory/2026-04-11-ship-012-S-pr-ready.md`
- `docs/archive/memory/2026-04-11-stage-shipment-grouping-triage.md`
- `docs/archive/memory/2026-04-12-ship-013-S-pr-ready.md`
- `docs/archive/memory/2026-04-12-stage-shipment-grouping-triage.md`
- `docs/archive/memory/2026-04-13-165321-ship-031-s-telemetry-pipeline.md`
- `docs/archive/memory/2026-04-13-183500-ship-031-s-review-remediation.md`
- `docs/archive/memory/2026-04-13-200000-ship-031-s-final-memory.md`
- `docs/archive/memory/2026-04-13-stage-telemetry-pipeline-enhancements.md`
- `docs/archive/memory/2026-04-13T2343-032-s-shipped-memory.md`
- `docs/archive/memory/2026-04-14-ship-033-s-complete.md`
- `docs/archive/memory/2026-04-14-ship-033-s-pr-ready.md`
- `docs/archive/memory/2026-04-14-stage-hooks-shipment-harvest.md`
- `docs/archive/memory/2026-04-14-stage-shipment-grouping-analysis.md`
- `docs/archive/memory/2026-04-19-153700-stage-shipment-b-no-op-stash-triage.md`
- `docs/archive/memory/2026-04-19-162500-stage-shipment-b-staged.md`
- `docs/archive/memory/2026-04-21-131500-ship-039-s-harness-ready.md`
- `docs/archive/memory/2026-04-21-132200-ship-039-s-build-complete.md`
- `docs/archive/memory/2026-04-21-133400-ship-039-s-review-gate.md`
- `docs/archive/memory/2026-04-21-134300-ship-039-s-pr-ready.md`
- `docs/archive/memory/2026-04-21-135600-ship-039-s-post-merge.md`
- `docs/archive/memory/2026-04-21-171500-ship-038-s-complete.md`
- `docs/archive/memory/2026-04-21-193300-stage-grouping-review.md`
- `docs/archive/memory/2026-04-21-194500-stage-039-s-ready.md`
- `docs/archive/memory/2026-04-21-215200-stage-triage-grouping.md`
- `docs/archive/memory/2026-04-21-223800-stage-shipment-a-harvest-complete.md`
- `docs/archive/memory/2026-04-21-224200-ship-040-s-harness-complete.md`
- `docs/archive/memory/2026-04-21-225000-ship-040-s-039009-complete.md`
- `docs/archive/memory/2026-04-21-231700-ship-040-s-release-workflow-slice-complete.md`
- `docs/archive/memory/2026-04-21-232200-ship-040-s-installer-plumbing-complete.md`
- `docs/archive/memory/2026-04-21-233100-ship-040-s-039013-complete.md`
- `docs/archive/memory/2026-04-21-233500-ship-040-s-039015-complete.md`
- `docs/archive/memory/2026-04-21-234000-ship-040-s-039016-complete.md`
- `docs/archive/memory/2026-04-21-234500-ship-040-s-039012-complete.md`
- `docs/archive/memory/2026-04-21-235500-ship-040-s-039014-complete.md`
- `docs/archive/memory/2026-04-21-ship-037-s-pr49-ready.md`
- `docs/archive/memory/2026-04-21-ship-038-s-pr-ready.md`
- `docs/archive/memory/2026-04-21-ship-040-s-post-merge.md`
- `docs/archive/memory/2026-04-22-ship-040-s-pr-ready-after-copilot-comments.md`
- `docs/archive/memory/2026-04-22-ship-041-s-pr-ready-memory.md`
- `docs/archive/memory/2026-04-22-stage-040-f-pipeline-complete.md`
- `docs/archive/memory/2026-04-22-stage-audit-harvest-memory.md`
- `docs/archive/memory/2026-04-22-stage-shipment-grouping-memory.md`
- `docs/archive/memory/2026-04-24-044-s-agent-session-disaster-recovery-closure.md`
- `docs/archive/memory/2026-04-24-044-s-closure-pr-merged-memory.md`
- `docs/archive/memory/20260419-193002-ship-034-s-pr-ready-memory.md`
- `docs/archive/memory/20260419-204036-ship-034-s-pr-green-awaiting-merge.md`
- `docs/archive/memory/20260419-214846-ship-034-s-pr-merge-ready.md`
- `docs/archive/memory/20260419-221911-ship-034-s-closure-complete.md`
- `docs/archive/memory/20260420-000323-stage-035-s-polish-staged-memory.md`
- `docs/archive/memory/20260420-094500-ship-035-s-pr-ready.md`
- `docs/archive/memory/20260420-143220-ship-035-s-final-memory.md`
- `docs/archive/memory/20260420-150100-stage-036-s-memory.md`
- `docs/archive/memory/20260420-153800-ship-036-s-memory.md`
- `docs/archive/memory/20260420-155758-pr46-copilot-comments-resolved-memory.md`
- `docs/archive/memory/20260420-162000-pr47-copilot-comments-resolved-memory.md`
- `docs/archive/memory/20260420-173500-ship-036-s-post-merge-closure.md`
- `docs/archive/memory/20260420-200750-ship-036-s-complete.md`
- `docs/archive/memory/20260420-201715-stage-triage-shipment-grouping.md`
- `docs/archive/memory/20260420-204007-stage-037s-harvest-complete.md`
- `docs/archive/memory/20260420-230000-ship-037-s-complete.md`
- `docs/archive/memory/20260420-merge-sync-mcp-deliberation-context.md`
- `docs/archive/memory/20260421-001400-stage-triage-shipment-grouping-memory.md`
- `docs/archive/memory/20260421-092300-stage-shipment-a-harvest-complete-memory.md`
- `docs/archive/memory/20260426-072000-ship-045-S-complete-closure-memory.md`
- `docs/archive/memory/20260428-210700-npm-publish-metadata-memory.md`

</details>

<details><summary>2026-05 (20)</summary>

- `docs/archive/memory/[20260505-182400]-stash-grouping-review-memory.md`
- `docs/archive/memory/[20260505-213200]-telemetry-gap-analysis-memory.md`
- `docs/archive/memory/[20260505-213800]-stash-spike-followup-memory.md`
- `docs/archive/memory/[20260505-215200]-046-f-stage-to-shipment-memory.md`
- `docs/archive/memory/[20260505-223300]-047-s-ship-execution-memory.md`
- `docs/archive/memory/[20260506-121100]-047-s-post-merge-closure-memory.md`
- `docs/archive/memory/[20260506-185900]-pr-85-post-merge-closure-memory.md`
- `docs/archive/memory/[20260506-224300]-group-a-cli-interop-staged-memory.md`
- `docs/archive/memory/[20260507-132800]-048s-cli-agent-interop-shipped-memory.md`
- `docs/archive/memory/[20260507-154900]-049s-telemetry-attribution-staged-memory.md`
- `docs/archive/memory/[20260508-112709]-stash-queue-grouping-review-memory.md`
- `docs/archive/memory/[20260508-121400]-stage-051-release-binary-readiness-memory.md`
- `docs/archive/memory/[20260508-140900]-ship-050-release-readiness-pr-memory.md`
- `docs/archive/memory/[20260508-210100]-stage-telemetry-stash-triage-memory.md`
- `docs/archive/memory/[20260508-214000]-ship-052s-telemetry-accuracy-memory.md`
- `docs/archive/memory/[20260509-001319]-053s-model-aware-telemetry-memory.md`
- `docs/archive/memory/[20260509-084548]-053s-closure-memory.md`
- `docs/archive/memory/[20260509-184500]-052-harvest-scanner-overflow-shipped-memory.md`
- `docs/archive/memory/20260507-171038-049s-telemetry-attribution-shipped-memory.md`
- `docs/archive/memory/20260522-post-merge-063-s-closure-complete-memory.md`

</details>

<details><summary>undated (19)</summary>

- `docs/archive/memory/[2026-04-22-232244]-041-s-pr-merge-ready-memory.md`
- `docs/archive/memory/[2026-04-22-234659]-041-s-copilot-round2-fixed-memory.md`
- `docs/archive/memory/[2026-04-23-071520]-041-s-shipped-final-memory.md`
- `docs/archive/memory/[2026-04-25-201945]-ship-045-pr-ready-awaiting-merge.md`
- `docs/archive/memory/[2026-04-26-045333]-ship-045-review-comments-resolved.md`
- `docs/archive/memory/005-S-review-gate-complete-2026-04-10.md`
- `docs/archive/memory/008-S-build-phase.md`
- `docs/archive/memory/008-S-review-remediation.md`
- `docs/archive/memory/2026-04-04/F013-complete-checkpoint.md`
- `docs/archive/memory/2026-04-05/harness-merge-install-memory.md`
- `docs/archive/memory/2026-04-05/T008-checkpoint.md`
- `docs/archive/memory/2026-04-05/two-agent-workflow-continuation-memory.md`
- `docs/archive/memory/2026-04-05/two-agent-workflow-design-session.md`
- `docs/archive/memory/2026-05-11/050-s-ship-closure-memory.md`
- `docs/archive/memory/2026-05-11/stage-planning-queue-harvest-memory.md`
- `docs/archive/memory/2026-05-18/pr-116-post-merge-closure-memory.md`
- `docs/archive/memory/2026-05-22/058-s-post-merge-closure-memory.md`
- `docs/archive/memory/2026-05-22/stage-new-bug-backlog-memory.md`
- `docs/archive/memory/2026-05-30/059-s-post-merge-closure-memory.md`

</details>

