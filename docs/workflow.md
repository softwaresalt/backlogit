---
chunk_strategy: h1-h2-h3
description: Developer and agent workflows with backlogit
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - workflow
        - mcp
        - cli
        - agent
    ms.date: 2026-04-06T00:00:00Z
    ms.topic: tutorial
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/workflow.md
title: Workflow Guide
---

## Workspace Lifecycle

A backlogit workspace follows a predictable lifecycle. You initialize it once, then add artifacts, query them, update their status as work progresses, and eventually archive completed items. The workspace lives entirely within the `.backlogit/` directory at the root of your project.

The lifecycle has six stages:

1. Initialize the workspace with `backlogit init`
2. Create artifacts with `backlogit add`
3. Stash deferred work in `.backlogit/stash.jsonl`
4. List and query artifacts with `backlogit list` and `backlogit query`
5. Update status and metadata with `backlogit update` and `backlogit move`
6. Archive completed work with `backlogit archive`

## Primary repository workflow

This repository now uses a two-agent delivery path with progressive disclosure.
`Stage` owns the intake path from stash to reviewed backlog, and `Ship`
owns shipment execution from ready backlog to shipped pull request state. The
durable lifecycle is `STASH -> BACKLOG -> SHIPMENT -> SHIPPED`.

Use the agent files for operational detail:

* [Stage](../.github/agents/stage.agent.md): stash triage, deliberation,
  planning, review gating, and harvest
* [Ship](../.github/agents/ship.agent.md): shipment claim, harness,
  implementation, review, CI remediation, release cleanup, and pull request flow

Shipment is a first-class artifact. Use shipment-aware MCP tools to create,
inspect, and maintain branch scope:

Shipment IDs use the `S` prefix and normally move through `queued`, `active`,
and `shipped`, with `abandoned` available when execution stops permanently.

```text
backlogit_create_shipment
backlogit_get_shipment
backlogit_list_shipments
backlogit_claim_shipment
backlogit_ship_shipment
backlogit_return_blocked
backlogit_add_to_shipment
```

## Blocked Shipment Lifecycle

A shipment in `blocked` is paused for a recoverable reason. The status is
non-terminal and resumable. It preserves branch, checkpoint, reconciliation,
member-status, and lifecycle evidence, but it does not occupy the single
`active` shipment slot. A blocked shipment cannot be claimed, executed, shipped,
or abandoned until it is unblocked through the governed lifecycle operation.

### Governed transition matrix

The `blocked` edges are deliberately narrow:

| From | To | Governed operation | Result |
|---|---|---|---|
| `active` | `blocked` | `BlockShipment` | Captures the member-status snapshot, returns `active` or `review` members to `queued`, and records correlated lifecycle evidence |
| `blocked` | `queued` | `UnblockShipment` with confirmation | Leaves every member `queued`, preserves the snapshot for later resumption, and releases the shipment to the queue |
| `blocked` | `active` | `UnblockShipment` with confirmation | Requires a free active slot and restores each member to its exact captured status |

In compact form, the governed edges are `active -> blocked`,
`blocked -> queued`, and `blocked -> active`. There is no direct transition
from `blocked` to `shipped` or `abandoned`.

Only the shipment block and unblock commands, or their MCP equivalents, may
cross a `blocked` edge:

```bash
backlogit shipment block 001-S \
  --reason "waiting for prerequisite" \
  --by operator \
  --resume-checkpoint checkpoint.json

backlogit shipment unblock 001-S --to queued --confirm --by operator
backlogit shipment unblock 001-S --to active --confirm --by operator
```

The corresponding MCP tools are `backlogit_block_shipment` and
`backlogit_unblock_shipment`. General-purpose status mutation and direct
artifact writes refuse entry to or exit from `blocked` with
`ErrShipmentBlockedRequiresEnvelope`. Do not edit shipment frontmatter to work
around that refusal.

Both operations run under the workspace-global lifecycle lock and persist an
intent with a complete preimage before changing the shipment or its members.
Unblock uses compare-and-swap checks over the shipment status, manifest, and
every member. It refuses if the shipment is no longer blocked, membership
changed, a member drifted from the expected blocked-state projection, the
snapshot is incomplete, or another shipment occupies the active slot. Recovery
also compares the current aggregate with the durable preimage or proven target;
it rolls back or rolls forward only when that comparison succeeds.

### Normalize an out-of-band blocked shipment

An imported or externally written `blocked` record is not eligible for unblock
until it has canonical metadata, member disposition, and lifecycle evidence.
Use `backlogit_normalize_blocked_shipment` with a workspace-relative
`shipment-bootstrap-snapshot/v1` JSON document:

```json
{
  "schema_version": "shipment-bootstrap-snapshot/v1",
  "shipment_id": "001-S",
  "branch": "feat/preserved-shipment",
  "target": "blocked",
  "blocked_reason": "waiting for corrective shipment",
  "blocked_at": "2026-09-22T00:00:00Z",
  "blocked_by": "operator",
  "resume_checkpoint_ref": "checkpoint.json",
  "members": {
    "001-F": "active",
    "001.001-T": "active"
  }
}
```

Store the document under the workspace storage root, for example
`.backlogit/bootstrap/001-S.snapshot.json`, and pass
`bootstrap/001-S.snapshot.json` as `snapshot_ref`. The snapshot must identify
the shipment, target `blocked`, contain a valid RFC3339 `blocked_at`, and cover
the current manifest exactly with valid member statuses. Unknown fields,
trailing content, paths outside the workspace, missing members, membership
changes, or member states that cannot be proven from the snapshot cause a
refusal. Free-form notes or memory files are not accepted as recovery evidence.

### Branch-scoped bootstrap runbook for shipment 154-S

> [!CAUTION]
> This is an operator-owned recovery procedure. Do not run it while shipment
> `155-S` is executing, and do not modify the preserved Ship branch.

1. Ship `155-S` normally on `main`. During its execution, the repository has no
   `blocked` shipment token. Use the normal topology gate and do not use a
   topology override for `155-S`.
2. After the merge, complete the required local-main synchronization and create
   `chore/block-154` from that synchronized `main`. This branch is backlog-only;
   use the existing worktree and do not add another worktree.
3. Hydrate only this explicit allowlist from
   `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition` at
   immutable precondition commit `dd9f01a1`:

   ```text
   .backlogit/checkpoints/checkpoint-20260914-070735.json
   .backlogit/queue/154-S.md
   .backlogit/queue/173-F.md
   .backlogit/queue/173.006-T.md
   .backlogit/reconcile/154-S-pre-20260914T053515Z.md
   ```

   Record the expected hash of every source blob from `dd9f01a1`, import only
   those paths, and verify each resulting file hash before continuing. Reject
   any path outside the allowlist. In particular, import no Go files, tests,
   generated harnesses, agent definitions, or other source/configuration
   content. The hydration must reconstruct the authoritative active shipment
   and member provenance without changing the preserved branch.
4. Write the authoritative machine-readable bootstrap snapshot at
   `.backlogit/bootstrap/154-S.snapshot.json`. It must use
   `shipment-bootstrap-snapshot/v1`, enumerate the exact hydrated manifest,
   record the preserved branch, set target `blocked`, and reference
   `checkpoint-20260914-070735.json`.
5. Invoke governed `BlockShipment`, not a general status mutation:

   ```bash
   backlogit shipment block 154-S \
     --reason "blocked pending correction for 7AA35A39" \
     --by operator \
     --resume-checkpoint checkpoint-20260914-070735.json
   ```

   Confirm that `154-S` is `blocked`, `173.006-T` is `queued`, the active slot
   is free, and the governed metadata and correlated intent/commit evidence are
   present.
6. Commit the complete governed output on `chore/block-154` as one backlog-only
   change. The commit must include the shipment record, every changed member
   artifact, the durable intent and complete preimage under `.backlogit/ops/`,
   `.backlogit/bootstrap/154-S.snapshot.json`, the reconciliation state, and
   authoritative per-item event logs for the shipment and every dispositioned
   member. A commit containing only `154-S` and the snapshot is incomplete.
   Open a pull request from `chore/block-154` to `main`.
7. After that pull request merges, create and ship the corrective release unit
   for deferred defect `7AA35A39` on `main` while `154-S` remains blocked.
8. After the correction ships, merge current `main` into the preserved 154
   feature branch. Resolve its backlog view to the governed blocked provenance,
   then run:

   ```bash
   backlogit shipment unblock 154-S --to active --confirm --by operator
   ```

   Resume from `checkpoint-20260914-070735.json` only after the unblock succeeds.

### Topology compatibility

The current external topology gate may reject the otherwise valid `blocked`
token even though blocked shipments do not occupy the active slot. This does
not affect the normal `155-S` execution described above.

For the later corrective flow, prefer the upstream one-line allowlist change
that adds `blocked` to the accepted live shipment statuses. If that change is
unavailable, an audited per-phase `--force` is permitted only after the normal
gate proves that its sole failure is the unsupported `blocked` status. Never
use a standing override, and stop if any other topology finding is present.

### Verify in a fixture workspace

Run synchronization and integrity checks against a dedicated fixture workspace,
never the live backlog corpus:

```bash
backlogit --cwd <dedicated-fixture-workspace> sync
backlogit --cwd <dedicated-fixture-workspace> doctor --format json
```

The doctor check fails with error severity and exit code 1 for multiple active
shipments, a blocked shipment without a non-empty `blocked_reason` and valid
RFC3339 `blocked_at`, or a lifecycle intent without a correlated committed or
compensated event. Keep the fixture path outside the live workspace's
`.backlogit` storage root.

## Legacy orchestration path

The older multi-agent path remains available for migration and targeted
automation, but it is no longer the primary model for this repository.

```text
deliberate or spike
-> impl-plan
-> plan-review
-> backlog-harvester
-> harness-architect
-> build-orchestrator
-> review or pr-review
```

## Developer CLI Workflow

**Initialize a new workspace:**

```bash
backlogit init
```

This creates the `.backlogit/` directory with default `config.yaml`, `header-def.yaml`, `registry.yaml`, `migration.yaml`, and template files. It also creates `.backlogit/stash.jsonl` so deferred work can be captured before it is ready to become a formal work item. The SQLite cache (`backlogit.db`) is created on first use.

**Add artifacts:**

```bash
# Create a feature
backlogit add --type feature --title "User authentication flow" --status active

# Create a task under the feature
backlogit add --type task --title "Add rate limiting to API" --parent 001-F --status active

# Create a subtask under the task
backlogit add --type subtask --title "Write token validation tests" --parent 001.001-T --status queued
```

**List and filter artifacts:**

```bash
# List all active items
backlogit list --status active

# List features only
backlogit list --type feature

# List tasks
backlogit list --type task --status active
```

**Search by keyword:**

```bash
backlogit search "rate limiting"
```

**Run a SQL query against the index:**

```bash
backlogit query "SELECT id, title, status FROM items WHERE artifact_type='task' ORDER BY created_at DESC LIMIT 10"
```

**Get the work queue (prioritized active items):**

```bash
backlogit queue view
```

**Capture deferred work in the stash:**

```bash
# Stash an idea during planning or review
backlogit stash add "Split audit dashboard into a later feature set" --kind feature --priority high
backlogit deliberate ABCD1234 --options "- Keep the current feature set narrow\n- Pull the work into the next feature wave"

# Fetch active stash entries for grouping and planning
backlogit stash list --group-by-priority
backlogit stash list --priority critical

# Harvest a stash entry into a real work item, carrying any linked deliberation lineage
backlogit stash harvest ABCD1234 --type feature --description "Pulled into the current feature wave"

# Harvest every critical stash item into planned work
backlogit stash harvest --priority critical --type task --parent-id 001-F --description "Pulled forward from stash"
```

**Inspect a specific artifact:**

```bash
backlogit get 001.001-T
```

**Update fields on an artifact:**

```bash
backlogit update 001.001-T --status review
backlogit update 001.001-T --title "Add rate limiting to public API"
```

**Move an artifact to a new status:**

```bash
backlogit move 001.001-T --status done
```

**Add a dependency between artifacts:**

Dependencies are managed through the MCP tool surface
(`backlogit_add_dependency`) or with the CLI dependency commands such as
`backlogit dep add 001.002-T 001.001-T --type blocks` and `backlogit dep remove 001.002-T 001.001-T`.

**Archive a completed artifact:**

```bash
backlogit archive 001.001-T
```

**Force-rebuild the SQLite index from Markdown files:**

```bash
backlogit sync
```

## Agent MCP Workflow

AI agents connect to backlogit through the Model Context Protocol. The server exposes artifact, queue, stash, and planning tools over JSON-RPC 2.0 via stdio. Start the server with:

```bash
backlogit mcp
```

The server runs until terminated and communicates over standard input and output. Agents discover all tools automatically through the MCP `initialize` handshake.

### Connecting Claude Code

Add the following to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "backlogit": {
      "command": "backlogit",
      "args": ["mcp"]
    }
  }
}
```

### Connecting GitHub Copilot CLI

For GitHub Copilot CLI, add backlogit to `.copilot/mcp-config.json` in your
workspace:

```json
{
  "mcpServers": {
    "backlogit": {
      "type": "stdio",
      "command": "backlogit",
      "args": ["mcp"]
    }
  }
}
```

For VS Code, use the same server entry under `.vscode/mcp.json`'s `servers`
object.

### Connecting Cursor

Add the server entry to Cursor's MCP settings under Settings > MCP:

```json
{
  "backlogit": {
    "command": "backlogit",
    "args": ["mcp"]
  }
}
```

### Core Agent Operations

Once connected, agents call tools by name. Common patterns include:

```
backlogit_create_item  -- create a feature, task, or subtask
backlogit_list_items   -- list with optional status/type filters
backlogit_query_sql    -- run a read-only SELECT against backlogit.db
backlogit_update_item  -- change status, title, or other fields
backlogit_move_item    -- transition an artifact to a new status
backlogit_search_items -- full-text search across all artifacts
backlogit_get_queue    -- retrieve the prioritized work queue
backlogit_create_shipment -- create a shipment artifact
backlogit_get_shipment -- inspect a shipment by ID
backlogit_list_shipments -- list shipment artifacts
backlogit_claim_shipment -- move a queued shipment to active
backlogit_ship_shipment -- close a released shipment, archive released scope, and record merge commit traceability
backlogit_return_blocked -- return a blocked item from a shipment to backlog
backlogit_add_to_shipment -- attach backlog items to a shipment
backlogit_fetch_stash  -- retrieve active stash entries from stash.jsonl, optionally filtered or grouped by priority, with linked deliberations when present
backlogit_stash        -- add deferred work to the stash with kind and priority
backlogit_deliberate   -- create a deliberation artifact linked to a stash entry
backlogit_harvest_stash -- promote one stash entry or a whole priority band into planned work items
backlogit_save_memory  -- persist agent memory to memories.json
backlogit_create_checkpoint -- save a session state snapshot
backlogit_track_commit -- associate a git commit with an artifact
backlogit_poll_hook_events -- poll for unacknowledged hook events since the consumer's last checkpoint
backlogit_ack_hook_events -- acknowledge processing of hook events up to and including seq
```

The `backlogit_query_sql` tool only accepts `SELECT` statements. Write operations go through the dedicated mutation tools to preserve data integrity.

## Hook Event Consumption

Agents subscribed to workflow automation can poll a JSONL-backed event queue
for signals emitted by the backlogit lifecycle. Two MCP tools support this:

```
backlogit_poll_hook_events -- poll for unacknowledged events since the consumer's last checkpoint
backlogit_ack_hook_events  -- advance the consumer's checkpoint to the highest processed seq
```

At session start, an agent polls with its consumer ID:

```json
{
  "tool": "backlogit_poll_hook_events",
  "arguments": { "consumer_id": "stage" }
}
```

The response contains two arrays:

* `events` — durable queue entries, each with a monotonic `seq` field.
* `derived_signals` — ephemeral computed signals (always `seq: 0`, never acked).

After processing all `events`, acknowledge the highest seq:

```json
{
  "tool": "backlogit_ack_hook_events",
  "arguments": { "consumer_id": "stage", "seq": 7 }
}
```

Skip the ack call when `events` is empty. Derived signals are never acked.

Checkpoint files live at `.backlogit/runtime/hooks/{consumer_id}.checkpoint.json`
and are ephemeral (gitignored). Deleting them resets a consumer to seq=0 for
idempotent replay.

Supported v1 event types:

| Type | Emitted when |
|---|---|
| `feature_review_ready` | A feature clears the review gate and is ready for shipment |
| `post_merge_closure` | A shipment is merged and closure tasks are due |
| `blocked_stale` | A blocked item has exceeded `BlockedStaleDays` without resolution |

## CQRS in Practice

backlogit separates writes from reads at the storage level. Writes always update a Markdown file first, then update the SQLite cache. Reads always go to SQLite. If the cache is missing or stale, `backlogit sync` rebuilds it from the Markdown files in seconds.

This means you can safely delete `backlogit.db` at any time. Running `backlogit sync` or any read command will rebuild it. You can also edit Markdown files directly in your editor; the next sync or read operation will pick up the changes.

## Configuration Overview

Four YAML files currently control workspace behavior:

`config.yaml` defines artifact types, ID patterns, shared field metadata, and queue hierarchy. The default types are `feature`, `task`, and `subtask`.

`header-def.yaml` defines per-type field schemas, enum values, defaults, and immutable system-managed fields.

`registry.yaml` maps statuses to directory paths within `.backlogit/`. By default, active work stays in `.backlogit/queue` and terminal work moves to `.backlogit/archive`.

`migration.yaml` defines source-path classification and default artifact-type mappings for imports from external markdown-backed systems such as Backlog.md.

Templates in `.backlogit/templates/` define the section structure for each artifact type.

For a complete setup guide, examples, and current limitations, see [Configuration Reference](configuration.md).

## Git Integration

The `.backlogit/` directory is committed to your repository. Markdown artifact files are Git-friendly: they have stable field ordering in their YAML frontmatter, deterministic ID-based filenames, and no binary content. The only gitignored file is `backlogit.db`.

When multiple developers or agents make concurrent changes, Markdown files merge cleanly because each artifact is a separate file. Work-item history is appended to `.backlogit/logs/{item-id}.jsonl`, and the stash remains a single hidden planning surface in `.backlogit/stash.jsonl`.

Associate a commit with an artifact using the MCP tool or the CLI:

```bash
backlogit update 001.001-T --commit abc1234
```

The `backlogit_track_commit` MCP tool records commit SHAs against artifact IDs for traceability.
