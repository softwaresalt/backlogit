---
chunk_strategy: h1-h2-h3
description: Migrating from Backlog.md to backlogit
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - migration
        - backlog.md
        - migrate
    ms.date: 2026-04-01T00:00:00Z
    ms.topic: how-to
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/migration-guide.md
title: Migration Guide
---

## Prerequisites

Before migrating, confirm you have:

- backlogit installed (`backlogit --version` should print the version)
- Go 1.22 or later if you plan to build from source
- A current Backlog.md workspace under `backlog/` or `.backlog/`, or a legacy checklist-style backlog file
- A project directory where the backlogit workspace will live
- Git initialized in the project (strongly recommended so you can review and revert imports easily)

## What the Current Importer Supports

The `backlog-md` adapter now works with the current structured Backlog.md layout rather than only with a monolithic checklist file.

For the latest upstream Backlog.md, the importer reads the work-item side of a source workspace rooted at `backlog/` or `.backlog/`. It is designed around these directories:

- `tasks/`
- `drafts/`
- `completed/`
- `archive/`
- `milestones/`

The importer focuses on work-item content first. It preserves task metadata such as IDs, labels, dependencies, assignees, priority, milestone, references, and documentation links. It also keeps source-trace fields in `custom_fields` so you can audit where each imported item came from.

Under the current backlogit methodology, queue-driven migration is intentionally limited to work items and milestones. Repository documentation and decision records belong in their own documentation concern, not inside queue-driven backlog flows. For that reason, the importer leaves Backlog.md `docs/` and `decisions/` directories alone.

`migration.yaml` lets you remap the imported classes that are already parsed, but it does not make the importer discover new Backlog.md directory classes on its own.

## Migration Steps

### Step 1: Initialize the backlogit workspace

Run `backlogit init` in the directory where the imported backlog should live. This creates `.backlogit/` and writes the default workspace and migration configuration files.

```bash
cd your-project
backlogit init
```

Review these files before importing if your project needs custom mapping:

- `.backlogit/config.yaml` for artifact types and naming
- `.backlogit/registry.yaml` for routing
- `.backlogit/migration.yaml` for source-path to artifact-type mapping

The generated `migration.yaml` defaults to the current structured Backlog.md layout and maps:

- task-like directories to `task`
- milestone files to `feature`

### Step 2: Preview the migration with --dry-run

Always run a dry run before writing any files. The `--dry-run` flag parses the source, applies status and type mapping, and prints the migration plan without creating backlogit artifacts.

```bash
backlogit migrate \
  --source ./.backlog \
  --adapter backlog-md \
  --dry-run \
  --format text
```

If your project uses `backlog/` instead of `.backlog/`, substitute that path in the examples below.

Use `--format json` if you want machine-readable output for scripting or inspection:

```bash
backlogit migrate \
  --source ./.backlog \
  --adapter backlog-md \
  --dry-run \
  --format json
```

### Step 3: Detect and validate the source format

Use `--detect` to confirm the adapter recognizes the source path, and `--validate` to check that all parsed items can map into your current backlogit configuration without writing files.

```bash
backlogit migrate --source ./.backlog --detect
backlogit migrate --source ./.backlog --adapter backlog-md --validate
```

If `--validate` reports errors, resolve them before proceeding. Common issues include unsupported target artifact types after custom remapping, malformed source frontmatter, or source files that are not task-like items.

### Step 4: Run the migration

Once the dry run and validation pass, run the migration without `--dry-run`:

```bash
backlogit migrate --source ./.backlog --adapter backlog-md
```

backlogit will create one Markdown artifact per imported work item inside `.backlogit/`, assign new backlogit IDs, preserve key source metadata, and then rehydrate the SQLite index.

Imported active work lands under `.backlogit/queue/`. Imported terminal work such as archived items stays under `.backlogit/archive/`. Migration does not create top-level `queue/`, `tasks/`, `epics/`, or `archive/` directories in the repository root.

### Step 5: Sync the index and verify

The import command rehydrates the index automatically. After it finishes, verify the imported artifacts:

```bash
backlogit list
backlogit status
backlogit query "SELECT id, title, artifact_type, status FROM items ORDER BY created_at DESC LIMIT 20"
```

Compare the results against your source task count and spot-check a few imported artifacts to confirm that parent links, dependencies, references, and statuses came across correctly.

## Before and After Example

**Backlog.md source (`.backlog/tasks/back-101 - Example-task.md`):**

```markdown
---
id: BACK-101
title: Example task
status: In Progress
assignee:
  - '@alice'
labels: ["backend"]
dependencies: ["BACK-100"]
priority: medium
milestone: M1
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement rate limiting on the public API.
<!-- SECTION:DESCRIPTION:END -->
```

**Resulting backlogit artifact (`001-T.md`):**

```markdown
---
id: 001-T
title: Example task
type: task
status: active
assigned_to: '@alice'
labels:
  - backend
sprint: M1
custom_fields:
  backlog_md_id: BACK-101
  backlog_md_source_path: .backlog/tasks/back-101 - Example-task.md
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement rate limiting on the public API.
<!-- SECTION:DESCRIPTION:END -->
```

The imported artifact receives a new backlogit ID, keeps the migrated body, and stores source-trace metadata so you can map it back to the original Backlog.md file. Nested imports use typed hierarchical IDs such as `001-F`, `001.001-T`, and `001.001.001-ST`, and the filename matches the ID exactly.

## Status Mapping

The structured-workspace importer maps current Backlog.md statuses and directory context to backlogit statuses using the following rules:

| Backlog.md State              | backlogit Status |
| ----------------------------- | ---------------- |
| `To Do`                       | queued           |
| `In Progress`                 | active           |
| `Blocked`                     | blocked          |
| `Review`                      | review           |
| `Done`                        | done             |
| Files under `completed/`      | done             |
| Files under `archive/`        | archived         |
| Draft items with empty status | queued           |

Status values are matched case-insensitively and normalized across spaces, underscores, and hyphens. Unknown statuses default to `queued`.

Legacy checklist-style Backlog.md files still use the older section-and-checkbox mapping rules.

## Metadata Mapping

The importer preserves or translates the most useful Backlog.md fields:

| Backlog.md Field | backlogit Destination |
|------------------|-----------------------|
| `title` | `title` |
| `status` | `status` |
| `assignee` | `assigned_to` (first assignee) |
| `labels` | `labels` |
| `dependencies` | dependency links, remapped to new backlogit IDs when possible |
| `milestone` | `sprint` |
| `references` | `references` |
| `documentation` | merged into `references` for agent-friendly context |
| `task_type` / `type` | `artifact_type` when it maps cleanly, otherwise preserved in `custom_fields` |
| source ID, path, reporter, dates | preserved in `custom_fields` with `backlog_md_*` keys |

Subtasks identified by decimal source IDs such as `BACK-217.02` are linked to their parent item when the parent is imported in the same run.

## Using --dry-run Safely

The `--dry-run` flag is your safety net. It performs format detection, structured parsing, status mapping, and target-type selection, but writes nothing to disk. You can run it as many times as needed without side effects.

Pipe the dry-run output to a file for review:

```bash
backlogit migrate \
  --source ./.backlog \
  --adapter backlog-md \
  --dry-run \
  --format json > migration-preview.json
```

Inspect `migration-preview.json` to verify that every item was mapped correctly before committing to the full migration.

## Customizing Artifact Mapping

If you want different target types for the currently supported imported classes, edit `.backlogit/migration.yaml` before importing. For example, you can remap milestone files to `feature` instead of `epic`, or change task-like directories to use a custom type that exists in your `config.yaml`.

The migration command reads `.backlogit/migration.yaml` automatically when it exists.

## Configuring Artifact Types for Future Imports

Artifact types are assigned during import. backlogit does not support changing an
artifact's type afterward with `backlogit update`, so type mapping decisions
should be made in `.backlogit/migration.yaml` before you run `backlogit migrate import`.

If an imported item lands on the wrong type, update the migration mapping and
re-import into a clean workspace, or recreate the artifact with the desired
type through the normal CLI or MCP create flow.

To configure custom artifact types for future use, edit `.backlogit/config.yaml`:

```yaml
artifact_types:
  feature:
    prefix: F
    name_format: "{prefix}{NNN}"
    allowed_children: [task]
  task:
    prefix: T
    name_format: "{prefix}{NNN}"
    allowed_children: [subtask]
  subtask:
    prefix: ST
    name_format: "{prefix}{NNN}"
    allowed_children: []
```

Run `backlogit sync` after editing the configuration to refresh the index.

## Troubleshooting

**`no items found` after migration**

The import command rehydrates the SQLite index automatically. If the cache still looks stale, run `backlogit sync` once to force a rebuild and verify you are inspecting the same target workspace where the import ran.

**Items mapped to wrong status**

Check the source item's `status` field and the directory it lives in. The importer normalizes status names, but unknown values fall back to `queued`. If needed, adjust the imported artifact with `backlogit update <id> --status <status>` after import.

**`--detect` reports unknown format**

Confirm the source path points at one of these:

- a structured Backlog.md workspace directory such as `./backlog` or `./.backlog`
- a legacy checklist-style markdown file such as `./backlog.md`

The adapter does not treat arbitrary markdown directories as importable just because they contain `.md` files.

**Documentation and decisions did not import**

That is expected with the current importer and with the current backlogit methodology. It focuses on tasks, drafts, completed/archive task files, and milestones. Docs and decisions are treated as a separate documentation concern rather than part of the queue migration flow.

**Duplicate IDs after migration**

This can happen if you run the import multiple times into the same workspace. Use Git to revert the generated artifact files, or delete the imported artifact directories and rerun from a clean state.

## Safe Revert Procedure

Source import does not currently use the `--rollback` flow. That flag belongs to the separate internal layout migration used for queue reorganization. For Backlog.md imports, the safest revert path is Git.

If the import produces unexpected results and you want to start over:

1. Revert the import in Git if possible:

```bash
git restore .
git clean -fd
```

2. If you are not using Git, remove the imported artifact files from the workspace directories created by backlogit:

```bash
# Linux / macOS
find .backlogit/queue .backlogit/archive -name '*.md' -delete

# Windows PowerShell
Get-ChildItem -Path .backlogit\queue,.backlogit\archive -Recurse -Filter '*.md' -ErrorAction SilentlyContinue | Remove-Item
```

3. Delete the index to clear the cache:

```bash
rm .backlogit/backlogit.db
```

4. Rehydrate from the remaining Markdown source:

```bash
backlogit sync
```

Your original Backlog.md source workspace is not modified or deleted during import, so the source data remains intact.
