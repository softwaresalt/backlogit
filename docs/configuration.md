---
chunk_strategy: h1-h2-h3
description: Reference and examples for configuring backlogit workspaces
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - configuration
        - config.yaml
        - header-def.yaml
        - migration.yaml
        - registry.yaml
        - templates
        - toolchain.yaml
    ms.date: 2026-04-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/configuration.md
title: Configuration Reference
---

## Configuration Surfaces

backlogit stores workspace configuration inside `.backlogit/`. The current implementation reads and uses these files:

| File | Purpose | Used today |
|------|---------|------------|
| `.backlogit/config.yaml` | Defines artifact types, ID/name formats, allowed child types, shared field metadata, and queue hierarchy levels | Yes |
| `.backlogit/header-def.yaml` | Defines per-type field schemas, defaults, enum values, and immutable system fields | Yes |
| `.backlogit/registry.yaml` | Routes artifacts to relative directories based on status or type | Yes |
| `.backlogit/migration.yaml` | Remaps already-detected import classes to backlogit artifact types during source migration | Yes |
| `.backlogit/templates/*.md` | Defines template sections and markdown body scaffolds per artifact type | Yes |
| `.backlogit/hooks.yaml` | Configures the agent hook event system and built-in lifecycle hooks, including the pre-task-completion gate broker (`lifecycle.pre_task_completion_gate`) | Yes |
| `.backlogit/toolchain.yaml` | Declares the workspace Level 1 pinned tool inventory, companion MCP server pins, and host prerequisites | Yes |

> [!IMPORTANT]
> `backlogit init` writes a default `hooks.yaml` with a
> `lifecycle.pre_task_completion_gate` block. If the file or that block is absent, it
> normalizes to documented defaults. The
> pre-task-completion gate broker (`enabled: auto` by default) is active even without
> an explicit block. See [Pre-Task-Completion Gate Broker](pre-task-completion-gate.md)
> for the full config, CLI/MCP surface, and operator runbooks.

## Workspace Defaults

Running `backlogit init` creates a workspace rooted at `.backlogit/` with these defaults:

* Active artifacts live in `.backlogit/queue`
* Terminal artifacts live in `.backlogit/archive`
* Deferred planning ideas live in `.backlogit/stash.jsonl`
* Per-item history lives in `.backlogit/logs/{item-id}.jsonl`
* The disposable cache lives in `.backlogit/backlogit.db`
* Default artifact types are `feature`, `task`, `subtask`, and `deliberation`
* Default typed suffixes are `-F`, `-T`, `-ST`, and `-DL`

The default hierarchy yields IDs such as `001-F`, `001.001-T`, and `001.001.001-ST`.

## Agent Discovery Surfaces

backlogit now exposes a unified metadata catalog for agent workflows. This catalog combines:

* artifact types and hierarchy levels
* per-type field schemas and enum values
* template sections
* registry routing
* stash conventions
* CLI command summaries
* MCP tool summaries

For stash-aware planning flows, the catalog now includes:

* the stash path
* supported stash kinds
* supported stash priorities
* the default stash priority

Use the CLI when an agent is running inside the workspace:

```text
backlogit metadata catalog
backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
backlogit metadata export-command-map .github\instructions\backlogit-command-map.json --format json
```

Use the MCP tools when an agent is connected through the backlogit MCP server:

* `backlogit_get_metadata_catalog`
* `backlogit_export_command_map`

`backlogit_export_command_map` writes only to workspace-relative paths and rejects path traversal outside the project root.

## Stash conventions

Deferred planning work lives in `.backlogit/stash.jsonl`. The current stash implementation supports:

* kinds: `feature`, `task`, `bug`, `epic`
* priorities: `low`, `medium`, `high`, `critical`
* default priority: `medium`
* optional deliberation linkage to a first-class `deliberation` artifact

Each stash line is stored in a programmatically readable format:

```text
- [ ] [ABCD1234] [priority:high] [deliberation:001-DL] feature: Split audit dashboard into a later feature set
```

Older stash lines without a priority tag are still accepted and normalize to `medium` when read.

### CLI stash examples

```text
backlogit stash add "Capture follow-up hardening ideas" --kind feature --priority high
backlogit deliberate ABCD1234 --problem-frame "Clarify the trade-offs before this becomes a planned feature."
backlogit stash list --priority critical
backlogit stash list --group-by-priority
backlogit stash harvest ABCD1234 --type feature
backlogit stash harvest --priority high --type feature
```

### MCP stash examples

The matching MCP tools support the same capability set:

* `backlogit_stash` accepts `kind`, `text`, and optional `priority`
* `backlogit_fetch_stash` accepts optional `priority` and `group_by_priority`, and returns linked deliberation artifacts when present
* `backlogit_deliberate` accepts `stash_id`, optional `title`, and optional deliberation section content
* `backlogit_harvest_stash` accepts either `stash_id` or `priority`, plus the target artifact details

This is the stash metadata that also appears in `backlogit metadata catalog` and exported command maps, which lets agents reason about priority-aware planning without hardcoding enums.

## config.yaml

`config.yaml` defines the structural model for work items. The loader expects these top-level keys:

* `artifact_types`
* `fields`
* `max_slug_length`
* `queue_layout`

### Current default

```yaml
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children:
      - task
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children:
      - subtask
  subtask:
    prefix: ST
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
    allowed_children: []
  deliberation:
    prefix: DL
    suffix: "-DL"
    name_format: "{NNN}{suffix}"
    allowed_children: []
fields:
  status:
    type: enum
    values:
      - queued
      - active
      - blocked
      - review
      - done
      - accepted
      - rejected
      - archived
    default: queued
    optional: false
    external_map: {}
max_slug_length: 60
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types:
        - feature
        - deliberation
    - level: 2
      types:
        - task
    - level: 3
      types:
        - subtask
  name_format: "{NNN}"
```

### What each part does

`artifact_types` defines the available artifact types. Each type declares:

* `prefix`: the typed ID segment prefix
* `suffix`: the typed ID segment suffix used by the default numeric-first naming
* `name_format`: the pattern used when a non-hierarchical fallback name must be resolved
* `allowed_children`: documentation and future-policy metadata for parent-child structure

`fields` defines shared field metadata. Today, the most important shared field is `status`. The `external_map` property is accepted as YAML schema data, but no automatic outbound sync engine consumes it yet.

`max_slug_length` is still loaded, but with the current ID-only default naming it matters only for custom `name_format` patterns that use slug placeholders.

`queue_layout` defines the hierarchy levels used by typed hierarchical IDs. In the current implementation, `root_dir` controls the active-work directory, and `levels` determines which types are valid at each depth.

### Hierarchy examples

#### Two-level feature/task model

```yaml
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children:
      - task
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: []
fields:
  status:
    type: enum
    values: [queued, active, blocked, review, done, accepted, rejected, archived]
    default: queued
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task]
```

This produces IDs such as `001-F` and `001.001-T`.

#### Feature/task/subtask with an added bug type

```yaml
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, bug]
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  subtask:
    prefix: ST
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
    allowed_children: []
  bug:
    prefix: B
    suffix: "-B"
    name_format: "{NNN}{suffix}"
    allowed_children: []
fields:
  status:
    type: enum
    values: [queued, active, blocked, review, done, accepted, rejected, archived]
    default: queued
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task, bug]
    - level: 3
      types: [subtask]
```

This lets root features own either tasks or bugs while keeping subtasks nested below tasks.

> [!NOTE]
> `allowed_children` is part of the config schema and is useful for tooling and documentation, but the current runtime primarily relies on `queue_layout` and explicit parent IDs when assigning levels and IDs.

## header-def.yaml

`header-def.yaml` defines field schemas by artifact type. It complements `config.yaml` by declaring which fields are available, whether they are immutable, and which enum values are valid.

### Current default

```yaml
defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  feature:
    prefix: F
    suffix: "-F"
    id_format: "{NNN}{suffix}"
    fields:
      harness_status:
        type: enum
        values:
          - pending
          - scaffolded
          - passing
          - failing
        default: pending
        optional: true
      status:
        type: enum
        values:
          - queued
          - active
          - blocked
          - review
          - done
          - accepted
          - rejected
          - archived
        default: queued
  task:
    prefix: T
    suffix: "-T"
    id_format: "{NNN}{suffix}"
    fields:
      priority:
        type: enum
        values:
          - low
          - medium
          - high
          - critical
        default: medium
      status:
        type: enum
        values:
          - queued
          - active
          - blocked
          - review
          - done
          - accepted
          - rejected
          - archived
        default: queued
  subtask:
    prefix: ST
    suffix: "-ST"
    id_format: "{NNN}{suffix}"
    fields:
      status:
        type: enum
        values:
          - queued
          - active
          - blocked
          - review
          - done
          - accepted
          - rejected
          - archived
        default: queued
```

### Why both config.yaml and header-def.yaml exist

`config.yaml` defines the workspace model. `header-def.yaml` defines per-type field rules. In practice:

* `config.yaml` says a `task` exists and uses prefix `T` with suffix `-T`
* `header-def.yaml` says a `task` may have `priority` and `status`, and what values are valid

If you add a new artifact type in `config.yaml`, add the matching type block in `header-def.yaml` as well.

### Example: adding severity to bugs

```yaml
types:
  bug:
    prefix: B
    suffix: "-B"
    id_format: "{NNN}{suffix}"
    fields:
      severity:
        type: enum
        values:
          - low
          - medium
          - high
          - critical
        default: medium
      status:
        type: enum
        values:
          - queued
          - active
          - blocked
          - review
          - done
          - accepted
          - rejected
          - archived
        default: queued
```

## registry.yaml

`registry.yaml` controls where artifacts live under `.backlogit/`. The paths must be relative to the workspace storage root.

### Current default

```yaml
directories:
  - path: archive
    condition:
      status:
        - done
        - accepted
        - rejected
        - archived
      type: []
  - path: queue
    condition:
      status:
        - queued
        - active
        - blocked
        - review
      type: []
```

### How routing works

Routing rules are checked in order. The first matching status or type wins. If no rule matches, backlogit falls back to `queue`.

### Example: route review items into a separate folder

```yaml
directories:
  - path: archive
    condition:
      status: [done, accepted, rejected, archived]
      type: []
  - path: review
    condition:
      status: [review]
      type: []
  - path: queue
    condition:
      status: [queued, active, blocked]
      type: []
```

This creates a distinct `.backlogit/review` lane while preserving the default archive and queue behavior.

### Example: route bugs separately by type

```yaml
directories:
  - path: archive
    condition:
      status: [done, accepted, rejected, archived]
      type: []
  - path: bugs
    condition:
      status: []
      type: [bug]
  - path: queue
    condition:
      status: [queued, active, blocked, review]
      type: []
```

This works only if `bug` is also present in `config.yaml` and `header-def.yaml`.

## migration.yaml

`migration.yaml` influences source imports, especially the `backlog-md` adapter. It does **not** define a general-purpose parser. Instead, it remaps already-detected source classes to backlogit artifact types.

### Current default

```yaml
document_classes:
  - name: work_items
    glob_patterns:
      - tasks/*.md
      - drafts/*.md
      - completed/*.md
      - archive/*.md
    artifact_type: task
    keywords:
      - task
      - acceptance criteria
      - implementation plan
  - name: milestones
    glob_patterns:
      - milestones/*.md
    artifact_type: feature
    keywords:
      - plan
      - milestone
default_layout: structured
source_paths:
  - path: tasks/
    class: work_items
  - path: drafts/
    class: work_items
  - path: completed/
    class: work_items
  - path: archive/
    class: work_items
  - path: milestones/
    class: milestones
```

### What each part does

`document_classes` names the classes the migration config can remap.

`glob_patterns` matches source-relative paths such as `tasks/foo.md` or `milestones/bar.md`.

`artifact_type` says which backlogit type should be assigned to that class after the source adapter parses it.

`source_paths` documents the expected source folders for the structured layout.

### Migration examples

#### Map milestone files to epics

```yaml
document_classes:
  - name: work_items
    glob_patterns: [tasks/*.md, drafts/*.md, completed/*.md, archive/*.md]
    artifact_type: task
  - name: milestones
    glob_patterns: [milestones/*.md]
    artifact_type: epic
default_layout: structured
source_paths:
  - path: tasks/
    class: work_items
  - path: milestones/
    class: milestones
```

Use this only if `epic` exists in `config.yaml` and `header-def.yaml`.

#### Import task-like directories as subtasks

```yaml
document_classes:
  - name: work_items
    glob_patterns: [tasks/*.md, drafts/*.md, completed/*.md, archive/*.md]
    artifact_type: subtask
  - name: milestones
    glob_patterns: [milestones/*.md]
    artifact_type: feature
default_layout: structured
source_paths:
  - path: tasks/
    class: work_items
  - path: milestones/
    class: milestones
```

This is useful when the source system already has a higher-level planning layer outside the imported markdown workspace.

#### Legacy flat migration note

The runtime also supports an internal queue-layout migration for older backlogit workspaces. That flow is separate from source import. `migration.yaml` does not control the flat-to-hierarchical workspace rewrite.

## templates

Template files live in `.backlogit/templates/` and are parsed from markdown files with YAML frontmatter. Each template declares:

* `name`
* `type`
* `description`
* `sections`

Every declared section must have matching `<!-- BEGIN:section-name -->` and `<!-- END:section-name -->` markers in the markdown body.

### Default feature template

```markdown
---
name: feature-template
type: feature
description: "A feature-level work item"
sections:
  - name: description
    required: true
    description: "Detailed description of the feature"
  - name: goals
    required: false
    description: "Goals and intended outcomes"
  - name: dod
    required: false
    description: "Definition of Done for this feature"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Goals

<!-- BEGIN:goals -->
<!-- END:goals -->

## Definition of Done

<!-- BEGIN:dod -->
<!-- END:dod -->
```

### Template example: bug intake

```markdown
---
name: bug-template
type: bug
description: "Bug intake template"
sections:
  - name: description
    required: true
    description: "What is broken"
  - name: repro
    required: true
    description: "How to reproduce the issue"
  - name: expected-behavior
    required: false
    description: "What should have happened"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Reproduction

<!-- BEGIN:repro -->
<!-- END:repro -->

## Expected Behavior

<!-- BEGIN:expected-behavior -->
<!-- END:expected-behavior -->
```

## External system mapping

backlogit has **schema-level** support for external value translation through `external_map`, but it does **not** currently ship an active sync engine for Azure DevOps, GitHub Issues, or Jira.

### What exists today

* `FieldConfig` accepts `external_map` in `config.yaml`
* Docs and architecture intend this to support external-system translation
* Runtime hook orchestration and outbound connector execution are deferred

### What does not exist yet

* No external-sync rules or connectors defined in `hooks.yaml`
* No loader or executor for external sync rules
* No CLI command that pushes artifacts to Azure DevOps, GitHub Issues, or Jira
* No MCP tool that performs stakeholder-system synchronization

### Documentation example for future mapping design

The following examples show how teams can document intended mappings today, even though automatic sync is not yet implemented.

#### Azure DevOps field mapping example

```yaml
fields:
  priority:
    type: enum
    values: [low, medium, high, critical]
    default: medium
    external_map:
      azure_devops:
        low: "4 - Low"
        medium: "3 - Medium"
        high: "2 - High"
        critical: "1 - Critical"
```

#### GitHub Issues label mapping example

```yaml
fields:
  status:
    type: enum
    values: [queued, active, blocked, review, done]
    default: queued
    external_map:
      github_issues:
        queued: backlog
        active: in-progress
        blocked: blocked
        review: review
        done: done
```

#### Jira issue type mapping example

```yaml
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task]
    external_map:
      jira:
        issue_type: Story
```

> [!CAUTION]
> The last example documents intended behavior only. The current `ArtifactTypeConfig` schema does not support `external_map` on artifact types, and the runtime will not consume it.

## Recommended setup workflow

1. Run `backlogit init`
2. Edit `config.yaml` to define your artifact types and hierarchy
3. Edit `header-def.yaml` to define valid fields for those types
4. Edit `registry.yaml` to route active and terminal work
5. Edit templates in `.backlogit/templates/` to define the section structure
6. If you import from a markdown-backed source, edit `migration.yaml` to remap imported classes
7. Run `backlogit sync` after significant manual changes to the workspace

## Cache refresh: sync vs. merge-sync

`backlogit sync` performs a **full rehydration** — it deletes all rows from the
SQLite index, walks every `.backlogit/` Markdown file, and re-inserts all
artifacts. This guarantees consistency but is proportional to workspace size.

`backlogit_merge_sync` (MCP only) performs **incremental rehydration** — it diffs
the current `.backlogit/` file tree against an in-memory snapshot of paths and
modification times, upserts only changed artifacts, and deletes only removed
artifacts. A fallback to full rehydration triggers automatically when the diff is
too large or the manifest is stale.

Use `backlogit sync` (or `backlogit_sync_index`) after bulk manual edits to
`.backlogit/` files. Use `backlogit_merge_sync` in agent loops where only a few
artifacts change between tool calls and token efficiency matters.

The response from `backlogit_merge_sync` includes `upserted`, `deleted`,
`failed_paths`, and `fallback_triggered` fields so callers can detect and respond
to incremental sync anomalies.

## Level 1 Workspace Toolchain Configuration

backlogit uses `.backlogit/toolchain.yaml` (validated against `schemas/workspace-toolchain.schema.json`) to declare the workspace's Level 1 pinned toolchain inventory, companion MCP server entrypoints, workspace-state bindings, and host prerequisites.

> [!IMPORTANT]
> Level 1 workspace-local pinning (`containment_level: "level-1-workspace-local-pinned"`) establishes static executable, lockfile, checksum, and `--cwd` / state-directory pinning within the workspace. It does **not** provide an OS or container sandbox (`os_container_sandbox_deferred: true`).

### toolchain.yaml Schema and Containment Classes

`.backlogit/toolchain.yaml` requires these top-level fields:

* `schema_version`: must be `"1.0"`
* `containment_level`: must be `"level-1-workspace-local-pinned"`
* `os_container_sandbox_deferred`: must be `true`
* `workspace_root`: canonical or repo-relative workspace root (for example `"."`)
* `tools`: array of declared MCP servers, local tools, and built-in host tools
* `host_prerequisites`: array of explicit host runtime commands (`id`, `command`, `purpose`, `required`, `reason`)

Every entry in `tools` carries an explicit `containment_class` from the closed Level 1 vocabulary:

| Class | Meaning | Required pinning or metadata |
|------|---------|------------------------------|
| `workspace-local-pinned` | Executable and entrypoint live inside the workspace tree | `executable_path`, `pinned_version` and `sha256` (or `lockfile_path`), `entrypoint_path` (required for runner executables such as `bun`/`bunx`; disallowed on `backlogit`), and `workspace_binding_args` (`--cwd` required on `backlogit`; optional `COPILOT_HOME`, `ENGRAM_DATA_DIR`) |
| `host-prerequisite` | Required or optional external runtime on the host | Non-empty `reason` explaining why the host runtime is required |
| `remote-declared` | Remote HTTP/MCP service without a workspace-local binary | DNS-label-safe `id` and non-empty `remote_policy` |
| `built-in-host` | Capability supplied directly by the agent host | Non-empty `reason` |
| `unsupported` | Tool or shell construct outside the supported static verification profile | Rejected by provisioning helpers; reported as a verification violation with remediation guidance |
| `non-conformant` | Declared or discovered tool that violates workspace-local pinning | Rejected by provisioning helpers; reported as a verification violation with remediation guidance |

### Provisioning Helpers and CLI Surface

Package `internal/toolchain` (`internal/toolchain/launcher.go`) exposes Go library APIs (not CLI subcommands) for Level 1 auditing and provisioning:

* `WriteToolchainManifest(rootPath string, manifest *ToolchainManifest) error`: Go API that validates all `tools` and `host_prerequisites` containment rules, rejects `unsupported` or `non-conformant` classes and path-traversal or symlink escapes, and atomically writes `.backlogit/toolchain.yaml` (or `.backlog/toolchain.yaml`)
* `RenderPinnedMCPConfig(rootPath string, manifest *ToolchainManifest) ([]byte, error)`: Go API that validates the manifest and renders deterministic `.mcp.json` JSON binding local stdio servers to `./bin/...` executables and workspace-scoped arguments/environment variables
* `VerifyLauncherScripts(rootPath string) ([]ToolVerificationFinding, error)`: statically inspects `.mcp.json`, `start.ps1`, and `start.sh` for workspace-local pinning, terminating missing-file checks, workspace-scoped `COPILOT_HOME`/`ENGRAM_DATA_DIR`, and absence of ambient `PATH` lookups (`Get-Command`, `command -v`)
* `AuditRepositoryToolchain(ctx context.Context, rootPath string) (*ToolchainVerificationReport, error)`: combines inventory, MCP config, and launcher verification into a single report

Use these CLI commands to prewarm the multi-session daemon and audit workspace pinning:

```text
backlogit mcp prewarm --format json
backlogit mcp status --toolchain --format json
backlogit mcp status --toolchain --strict --format json
```

* `backlogit mcp prewarm` starts or attaches to the workspace background MCP daemon before client sessions connect.
* `backlogit mcp status --toolchain` reports daemon status together with the nested `"toolchain"` `ToolchainVerificationReport` object (`valid`, `violations`, `containment_level`, `os_container_sandbox_deferred`, `workspace_root`, `tools`, and `host_prerequisites`).
* `backlogit mcp status --toolchain --strict` (or `--strict` alone, which implies `--toolchain`) writes the full report and exits non-zero (wrapped validation error) when `violations > 0`.

### Operator-Owned Verified Workspace Toolchain Setup, Provisioning, and Custody Alignment Procedure

Follow this four-step procedure to provision and verify a workspace-local Level 1 toolchain:

1. **Confirm Ownership and Gitignore / Custody Boundary**:
   * Tracked repository artifacts (`schemas/workspace-toolchain.schema.json`, `internal/toolchain/*.go`, `.backlogit/toolchain.yaml`, and documentation under `docs/`) are committed in the repository.
   * Tracked/unignored launcher and dependency inputs remain under operator custody (`start.ps1`, `start.sh`, `.gitignore`, `tools/mcp/package.json`, `tools/mcp/bun.lock`, and `tools/mcp/install.ps1`), while local runtime artifacts remain gitignored (`./bin/*.exe`, `.copilot/graphtor-mcp-shim.cjs`, `tools/mcp/node_modules/`, and `.mcp.json`).
2. **Verify Per-Tool Source, Checksums, Lockfile Provenance, and Workspace Binding**:
   * `backlogit` (`workspace-local-pinned`): build from repository source (`go build -o ./bin/backlogit.exe ./cmd/backlogit`), record its executable SHA-256 in `.backlogit/toolchain.yaml`, invoke as `./bin/backlogit.exe mcp --cwd <workspace>` in `.mcp.json`, and ensure both `start.ps1` and `start.sh` use workspace-anchored paths, terminating missing-file checks, workspace-scoped `COPILOT_HOME` and `ENGRAM_DATA_DIR`, and straight-line syntax without ambient `PATH` lookups.
   * `engram` (`workspace-local-pinned`): provision `./bin/engram.exe`, pin its version and executable SHA-256 in `.backlogit/toolchain.yaml`, bind `ENGRAM_DATA_DIR=.engram`, and invoke `./bin/engram.exe shim` in `.mcp.json`.
   * `bun` and `bunx` (`workspace-local-pinned`): provision `./bin/bun.exe` and `./bin/bunx.exe`, pin their versions, executable SHA-256 checksums, and `entrypoint_path: "./tools/mcp/install.ps1"` in `.backlogit/toolchain.yaml`, and isolate caches via `BUN_INSTALL=<workspace>/bin/bun`, `BUN_INSTALL_CACHE_DIR=<workspace>/bin/bun-cache`, and `BUN_RUNTIME_TRANSPILER_CACHE_PATH=0`.
   * `context7` and `tavily` (`workspace-local-pinned`): pin versions in `tools/mcp/package.json` and `tools/mcp/bun.lock`, ensure `tools/mcp/install.ps1` resolves `$binDirectory = Join-Path $workspaceRoot "bin"` (not `.bin`), and run `pwsh -NoProfile -File tools/mcp/install.ps1` (`./bin/bun.exe install --cwd tools/mcp --frozen-lockfile --ignore-scripts --backend copyfile --no-progress`).
   * `github` (`workspace-local-pinned`): provision `./bin/github-mcp-server.exe`, pin its version and executable SHA-256 in `.backlogit/toolchain.yaml`, and invoke `["stdio"]` in `.mcp.json`.
   * `graphtor-docs` (`workspace-local-pinned`): provision `./bin/kbdb.exe`, pin its executable SHA-256 in `.backlogit/toolchain.yaml`, and invoke `"./bin/kbdb.exe"` with `["serve"]` in `.mcp.json` (extra hyphen-prefixed flags beyond `--cwd` or `--script=` are rejected by `ValidateMCPConfigPinning`).
   * `graphtor-mcp-shim` (`workspace-local-pinned`, `kind: "tool"`): provision `.copilot/graphtor-mcp-shim.cjs` paired with `executable_path: "./bin/bun.exe"` and the `bun.exe` SHA-256 in `.backlogit/toolchain.yaml`.
   * `built-in-host` (`agent-host` inside `tools`) and `host_prerequisites` (`pwsh`, `bash`, `git`, `gh`, `go`, `copilot` in the top-level `host_prerequisites` array): declare each entry explicitly in `.backlogit/toolchain.yaml`.
3. **Align `.mcp.json` with `RenderPinnedMCPConfig`**:
   * Use the `RenderPinnedMCPConfig` Go helper in `internal/toolchain` (or mirror its exact schema in `.mcp.json`) so every local MCP server `"command"` targets `./bin/...` with workspace-contained arguments (`["mcp", "--cwd", "<workspace>"]` for `backlogit`) and environment variables without ambient `PATH` resolution.
4. **Run the Runtime Acceptance Check**:
   * Execute the strict audit against the provisioned workspace and confirm zero violations:

```powershell
./bin/backlogit.exe mcp status --toolchain --strict --format json
```

The command must exit `0` and emit a JSON object whose nested `"toolchain"` report contains `"valid": true`, `"violations": 0`, `"containment_level": "level-1-workspace-local-pinned"`, `"os_container_sandbox_deferred": true`, and the resolved `"workspace_root"`.

## Accuracy notes

The current implementation supports:

* type and hierarchy configuration
* field schema validation
* template-based section modeling
* status/type routing
* class-to-type migration remapping
* stash-based deferred planning
* incremental cache refresh (`backlogit_merge_sync` MCP tool)
* Level 1 workspace-local toolchain pinning and strict audit (`backlogit mcp status --toolchain --strict`)

The current implementation does not yet support:

* live external-system sync
* active `hooks.yaml` processing
* arbitrary migration parser definition through YAML alone
