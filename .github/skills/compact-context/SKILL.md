---
name: compact-context
description: "Scan .copilot-tracking/ for stale or oversized tracking artifacts, produce compacted summaries, and archive originals. Use when tracking files accumulate beyond thresholds that degrade agent context quality."
argument-hint: "[threshold-days=14] [max-files=40] [max-size-kb=500]"
---

# Compact Context

Reduce context noise by compacting old `.copilot-tracking/` artifacts into dense summaries. Originals are archived, never deleted, preserving full audit history.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[COMPACT] Starting context compaction` |
| Assessment complete | info | `[COMPACT] Assessment: {file_count} files, {total_size_kb} KB across {dir_count} subdirectories` |
| Stale batch identified | info | `[COMPACT] Stale batch: {count} files in {subdirectory} older than {threshold} days` |
| Files preserved | info | `[COMPACT] Preserved {count} files referenced by active backlog tasks` |
| Summary written | success | `[COMPACT] Summary written: {file_path}` |
| Archive complete | success | `[COMPACT] Archived {count} originals to .copilot-tracking/archive/{subdirectory}/` |
| Session complete | success | `[COMPACT] Complete: {archived_count} files archived, {summary_count} summaries written` |
| Nothing to compact | info | `[COMPACT] Tracking directory healthy — no compaction needed` |

## Subagent Depth Constraint

This skill is a leaf executor. It MUST NOT spawn additional subagents. Perform all work using direct tool calls.

## Inputs

* `${input:threshold-days:14}`: (Optional, defaults to 14) Files older than this threshold are candidates for compaction.
* `${input:max-files:40}`: (Optional, defaults to 40) File count threshold that triggers compaction when exceeded.
* `${input:max-size-kb:500}`: (Optional, defaults to 500) Total size threshold in KB that triggers compaction when exceeded.

## Execution Steps

### Phase 1: Assess

Scan `.copilot-tracking/` to determine whether compaction is needed and identify stale artifacts.

1. **Enumerate subdirectories**: List all subdirectories in `.copilot-tracking/` excluding `archive/`. Record the subdirectory name, file count, and total size for each.

2. **Check thresholds**: If the total file count (excluding `archive/`) is below `${input:max-files}` AND total size is below `${input:max-size-kb}` KB, broadcast `[COMPACT] Tracking directory healthy` and exit. No compaction needed.

3. **Identify stale files**: For each subdirectory, identify files with a last-modified date older than `${input:threshold-days}` days from today. Group stale files by subdirectory.

4. **Cross-reference active tasks**: Read `.backlog/tasks/` and scan task descriptions and implementation notes for file path references matching `.copilot-tracking/` paths. Any tracking file referenced by an active task (status: "To Do" or "In Progress") is excluded from compaction regardless of age.

5. **Build compaction manifest**: For each subdirectory with stale files, record:
   - Subdirectory name
   - Files to compact (stale and not referenced by active tasks)
   - Files to preserve (recent or referenced by active tasks)
   - Total size of files to compact

6. Broadcast the assessment summary.

### Phase 2: Compact

For each subdirectory batch in the compaction manifest:

1. **Read stale files**: Read each file in the batch. Extract key content:
   - Section headings (H2, H3 level)
   - Key decisions and their rationale
   - Outcomes and results (success/failure, commit hashes, task completions)
   - Error resolutions and workarounds
   - Discard verbose logs, raw command output, and intermediate debugging steps

2. **Write summary file**: Create a compacted summary at `.copilot-tracking/{subdirectory}/{YYYY-MM-DD}-compacted-summary.md` with this structure:

   ```markdown
   ---
   type: compacted-summary
   date: YYYY-MM-DD
   source_count: {number of files compacted}
   source_date_range: "{oldest_date} to {newest_date}"
   ---

   # Compacted Summary: {subdirectory}

   Compacted from {source_count} files spanning {source_date_range}.

   ## Key Decisions

   * {decision} — {rationale} (from {source_file})

   ## Outcomes

   * {outcome description} (from {source_file})

   ## Error Resolutions

   * {error} — {resolution} (from {source_file})

   ## Preserved Context

   * {any critical context that would be lost without the originals}
   ```

3. **Archive originals**: Move the compacted files to `.copilot-tracking/archive/{subdirectory}/`. Create the archive subdirectory if it does not exist. Use file move operations, not copy-then-delete.

4. **Broadcast**: Report the summary file path and the count of archived originals.

5. Repeat for each subdirectory batch in the manifest.

## Constraints

* **Never delete files.** All originals are moved to `.copilot-tracking/archive/`. The archive directory is the permanent home for compacted originals.
* **Preserve active task references.** Files referenced by active backlog tasks are never compacted, regardless of age or size.
* **One summary per batch.** Each subdirectory compaction produces exactly one summary file. Multiple compaction runs on the same subdirectory produce separate dated summaries.
* **Idempotent.** Running compaction twice on an already-compact directory produces no changes (threshold check exits early).
