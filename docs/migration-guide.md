---
title: Migration Guide
description: Migrating from Backlog.md to backlogit
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: how-to
keywords:
  - backlogit
  - migration
  - backlog.md
  - migrate
---

## Prerequisites

Before migrating, confirm you have:

- backlogit installed (`backlogit --version` should print the version)
- Go 1.22 or later if you plan to build from source
- A `Backlog.md` file in checklist format
- A project directory where the backlogit workspace will live
- Git initialized in the project (optional but recommended for rollback)

## Migration Steps

### Step 1: Initialize the backlogit workspace

Run `backlogit init` in the directory where your `Backlog.md` file lives. This creates the `.backlogit/` directory with default configuration files.

```bash
cd your-project
backlogit init
```

Review `.backlogit/config.yaml` and adjust artifact types if your project uses categories that do not map directly to `task`, `story`, `bug`, or `epic`.

### Step 2: Preview the migration with --dry-run

Always run a dry run before writing any files. The `--dry-run` flag prints the migration plan to standard output without creating any artifacts.

```bash
backlogit migrate \
  --source ./Backlog.md \
  --adapter backlog-md \
  --dry-run \
  --format text
```

Use `--format json` if you want machine-readable output for scripting or inspection:

```bash
backlogit migrate \
  --source ./Backlog.md \
  --adapter backlog-md \
  --dry-run \
  --format json
```

### Step 3: Detect and validate the source format

Use `--detect` to confirm the adapter recognizes your file's format, and `--validate` to check that all items would convert successfully without errors.

```bash
backlogit migrate --source ./Backlog.md --detect
backlogit migrate --source ./Backlog.md --adapter backlog-md --validate
```

If `--validate` reports errors, resolve them in the source file before proceeding. Common issues include checklist items with no title text and malformed section headings.

### Step 4: Run the migration

Once the dry run and validation pass, run the migration without `--dry-run`:

```bash
backlogit migrate --source ./Backlog.md --adapter backlog-md
```

backlogit will create one Markdown file per checklist item in the appropriate subdirectory of `.backlogit/`, assign IDs, and set frontmatter fields based on the source item's status and section.

### Step 5: Sync the index and verify

After migration, rebuild the SQLite index and verify the artifacts were created correctly:

```bash
backlogit sync
backlogit list
backlogit status
```

The `status` command shows a summary of artifact counts by type and status. Compare this against your original `Backlog.md` item count to confirm completeness.

## Before and After Example

**Backlog.md source:**

```markdown
## In Progress

- [ ] Implement rate limiting on the public API
- [x] Add JWT authentication

## Backlog

- [ ] Write integration tests for the payment service
- [ ] Migrate database to PostgreSQL
```

**Resulting backlogit artifact (T001.md):**

```markdown
---
id: T001
title: Implement rate limiting on the public API
type: task
status: active
created_at: 2026-04-01T00:00:00Z
updated_at: 2026-04-01T00:00:00Z
---

Migrated from Backlog.md section: In Progress.
```

Each checklist item becomes an independent Markdown file. Completed items (checked boxes) receive a `done` status. Items in an "In Progress" section receive an `active` status.

## Status Mapping

The migration adapter maps Backlog.md states to backlogit statuses using the following rules:

| Backlog.md State             | backlogit Status |
|------------------------------|------------------|
| Checked item `[x]`           | done             |
| Item in "In Progress" section | active           |
| Item in "Backlog" section    | queued           |
| Item in "Blocked" section    | blocked          |
| Item in "Review" section     | in_review        |
| Item in "Done" / "Completed" | done             |
| Item in "Archive" section    | archived         |
| Unchecked item (no section)  | queued           |

Sections are matched case-insensitively. Items in sections not listed in the table default to `queued`.

## Using --dry-run Safely

The `--dry-run` flag is your safety net. It performs the full migration pipeline, including format detection, status mapping, ID assignment, and frontmatter generation, but writes nothing to disk. You can run it as many times as needed without side effects.

Pipe the dry-run output to a file for review:

```bash
backlogit migrate \
  --source ./Backlog.md \
  --adapter backlog-md \
  --dry-run \
  --format json > migration-preview.json
```

Inspect `migration-preview.json` to verify that every item was mapped correctly before committing to the full migration.

## Configuring Artifact Types Post-Migration

After migration, you may want to refine the artifact types assigned to migrated items. The migration adapter assigns `task` as the default type for all items. To change the type of a specific artifact:

```bash
backlogit update T042 --type bug
```

To configure custom artifact types for future use, edit `.backlogit/config.yaml`:

```yaml
artifact_types:
  - task
  - story
  - bug
  - epic
  - spike
  - chore
```

Run `backlogit sync` after editing the configuration to refresh the index.

## Troubleshooting

**`no items found` after migration**

Run `backlogit sync` to rebuild the SQLite index. The migration writes Markdown files but does not automatically refresh the cache.

**Items mapped to wrong status**

Check that your `Backlog.md` section headings match the expected names in the status mapping table. The adapter matches headings case-insensitively, but unusual section names fall back to `queued`. Rename the sections in the source file, or update the status manually after migration with `backlogit update <id> --status <status>`.

**`--detect` reports unknown format**

Confirm the file uses standard Backlog.md checklist syntax (`- [ ]` and `- [x]` items under Markdown `##` section headings). Other checklist formats are not currently supported by the `backlog-md` adapter.

**Duplicate IDs after migration**

This can happen if you run the migration twice. Delete the `.backlogit/` contents (except config files) and re-run the migration once from a clean state.

## Rollback Procedure

If the migration produces unexpected results and you want to start over:

1. Remove the migrated artifact files from `.backlogit/`:

```bash
# Linux / macOS
find .backlogit -name '*.md' -not -path '*.backlogit/config*' -delete

# Windows PowerShell
Get-ChildItem -Path .backlogit -Recurse -Filter '*.md' | Remove-Item
```

2. Delete the index to clear the cache:

```bash
rm .backlogit/index.db
```

3. If you committed the migration to Git, revert the commit:

```bash
git revert HEAD
```

Your original `Backlog.md` file is not modified or deleted during migration, so the source data remains intact.
