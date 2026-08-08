---
title: "Default workspace directory rename from .backlogit to .backlog"
description: "Deliberation on stash 9370A18C: whether and how to change the default workspace storage directory name, and what backward-compatibility, conflict-precedence, and migration contract the change requires."
source: docs/decisions/2026-08-07-workspace-default-dir-rename-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Changing the default workspace install directory from .backlogit to .backlog with a backward-compatible resolution and migration contract (stash 9370A18C)"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-08-07-workspace-default-dir-rename-plan.md"
tags:
  - "feature"
  - "config"
  - "workspace"
  - "migration"
  - "compatibility"
  - "stage"
stash_ids:
  - "9370A18C"
---

## Problem Frame

Stash `9370A18C` (medium priority, kind `feature`): *"change the default install
directory for a workspace from `.backlogit` to `.backlog`."*

The stated benefit is ergonomic — `.backlog` is shorter, tool-agnostic, and
reads better as a workspace convention than a product-named directory. The stated
risk is that this is a **potentially breaking change to workspace discovery and
containment**, which are Constitution Principle III and IV surfaces.

### Constraints

- **Principle III / IV** — all filesystem operations must resolve inside the
  workspace root; path traversal must be rejected. Discovery is a security
  surface, not just an ergonomic one.
- **Operator policy** — reliability and security over feature; simplicity over
  complexity; the change must not degrade either.
- Every existing workspace on disk uses `.backlogit`, including **this
  repository's own backlog**.
- The pinned release binary (`C:\Tools\backlogit.exe`) that operates this repo's
  backlog does **not** change when a fix merges to `main`.

### Success criteria

- A newly initialized workspace uses `.backlog`.
- Every existing `.backlogit` workspace continues to resolve, indefinitely, with
  no operator action.
- When both directories exist, behavior is **deterministic and explicit** — never
  ordering-dependent, never silent.
- A supported, previewable, idempotent migration path exists.
- Path containment and the canonical-scan safety set are not weakened.

### Explicitly out of scope

Renaming the module, the binary, the MCP server name, or any published artifact.
Changing the config file name or the internal queue/archive/logs layout.

## Research Findings

### Where the name actually lives

| Concern | Location |
|---|---|
| The single hardcoded default | `internal/core/workspace.go:55-62` — `WorkspaceStorageRoot()` returns `filepath.Join(rootPath, ".backlogit")` |
| `init` directory creation | `internal/cli/root.go:297` — `dir := filepath.Join(root, ".backlogit")` |
| Migration config lookup | `internal/cli/migrate.go:127` |
| Everything else (queue, archive, logs, db, stash, templates, checkpoints, hooks, memories, telemetry) | derived from `WorkspaceStorageRoot`, not independently hardcoded |

The derived-path discipline is good news: there is **one** primary seam, plus two
secondary literals.

### How discovery actually works

`resolveWorkspaceRoot` (`internal/core/workspace.go:216-250`):

1. if `cwd/.backlogit/config.yaml` exists → `cwd` is the root;
2. else if `cwd` is itself named `.backlogit` and holds `config.yaml` → parent is
   the root;
3. else scan **immediate child directories** and pick the unique child whose
   `child/.backlogit/config.yaml` exists.

There is **no upward parent walk**. Discovery keys on **config-file presence**,
with the directory name supplying the path. That is important: the resolver
already has a "does this candidate hold a config" test that generalizes cleanly
to two candidate names.

### Blast radius

- Go source: one default constant plus two secondary literals; a large volume of
  help text and comments.
- **Tests: roughly 100 test files assert the literal `.backlogit`**, several with
  20+ occurrences (`internal/events/checkpoint_lifecycle_test.go` 27,
  `internal/core/archive_test.go` 25, `internal/core/doctor_test.go` 19,
  `internal/cli/migrate_import_test.go` 16).
- Non-Go surfaces: `README.md`, `docs/migration-guide.md`, `AGENTS.md`,
  `.github/instructions/*`, `.autoharness/*`, `.vscode/*`, `scripts/*`, `tests/*`.

### Compatibility-shim precedent exists in-repo

`upgradeLegacyTransitions` (`internal/config/loader.go:132-150,178-182`), the
legacy stash path fallback (`internal/core/stash.go:127-167`), read-time
self-heal for legacy archive paths (`internal/core/archive.go:704+`), and
backward-compatible aliases in `internal/core/hierarchy.go:13,16`. The house
style for "default changed, old value still honored" is established.

### Prior-learning warnings that directly shape the design

Confidence on direct prior art was **low**, but the adjacent warnings are sharp:

- Do **not** derive the safety-critical scan set (ID-collision guard, doctor,
  archive-destination checks) purely from mutable config. Precedent `5f86ee9d`
  had to force `.backlogit/archive` into the canonical scan unconditionally
  because a degraded registry reopened a data-loss path. **With two possible
  roots this failure mode roughly doubles.**
- Do **not** rely on CLI-only CI to validate discovery changes. The MCP server
  defaults `RootPath` to `"."` — a relative-root path the CLI never exercises,
  and exactly how a `filepath.Rel` defect once shipped green.
- Do **not** leave collision precedence implicit. An ordering-dependent winner is
  a silent data-loss vector.
- Do **not** combine the directory move with content rewrites in one commit —
  it can push git rename-similarity below threshold and break `git log --follow`.
- Do **not** assume merging makes the rename live for this repo's own backlog:
  the pinned release binary is unaffected.
- Do **not** audit the docs by path-existence alone; that shallow audit already
  passed factually wrong storage-contract prose once.

## Options Evaluated

### Option A — Hard rename, no compatibility

Change the constant, update tests and docs, done.

- **Pros:** trivially simple; one seam.
- **Cons:** every existing workspace breaks with no diagnostic. Directly violates
  reliability-over-feature. Non-starter.
- **Effort:** low. **Fit:** none.

### Option B — Dual-root resolution with explicit precedence, conflict refusal, and a migration command

`.backlog` becomes the default for `init`. Resolution accepts either name.
Precedence is explicit. Both present → deterministic hard error. A previewable,
idempotent `migrate` subcommand moves an existing workspace. `doctor` gains a
conflict check.

- **Pros:** existing workspaces never break; conflict is loud rather than silent;
  reuses the established shim style and the established migration UX
  (`--dry-run` → apply → idempotent re-run); the one-seam structure keeps the
  code change small; decomposes into clean width-isolated units.
- **Cons:** two candidate names must be threaded into the canonical scan set and
  the containment guards; permanently more surface than one name.
- **Effort:** medium. **Fit:** high.

### Option C — Fully config-driven storage directory name

Make the directory name a config key.

- **Pros:** maximum flexibility.
- **Cons:** **bootstrap paradox** — the config file lives *inside* the storage
  directory, so the name cannot be read from the thing it names. Worse, it makes
  the safety-critical scan set derive from mutable config, which is the exact
  data-loss reopening the `5f86ee9d` precedent warns about. Rejected on security
  grounds.
- **Effort:** high. **Fit:** low.

### Option D — Defer

Leave `.backlogit` as the default.

- **Pros:** zero risk.
- **Cons:** the operator asked for it and it is a real ergonomic improvement;
  deferring indefinitely just keeps the debt.
- **Effort:** none. **Fit:** low.

## Trade-off Comparison

| Criterion | A (hard rename) | B (dual-root + migrate) | C (config-driven) | D (defer) |
|---|---|---|---|---|
| Existing workspaces keep working | **no** | **yes** | yes | yes |
| Conflict behavior deterministic | n/a | **yes, refuses** | ambiguous | n/a |
| Safety scan set stays config-independent | yes | **yes** | **no** | yes |
| Bootstrap sound | yes | yes | **no** | yes |
| Migration previewable + idempotent | n/a | **yes** | partial | n/a |
| Reuses established shim style | no | **yes** | no | n/a |
| Width-isolated task decomposition | yes | **yes** | no | n/a |
| Delivers the requested outcome | yes | **yes** | yes | **no** |

## Decision

**Adopt Option B**, with a narrow environment escape hatch and a strict
precedence rule.

### Resolution precedence (fixed, documented, tested)

1. `BACKLOGIT_WORKSPACE_DIR` environment override, when set and non-empty. The
   value must be a **single path segment** — no separators, no `..`, no absolute
   path — validated with the existing `ensureContainedRelPath` discipline
   (`internal/config/loader.go:62-86`).
2. `.backlog`, when it holds `config.yaml`.
3. `.backlogit`, when it holds `config.yaml`.
4. Neither → "not a workspace", unchanged from today.

**Both `.backlog` and `.backlogit` hold `config.yaml`, and no env override is
set → hard error** (`ErrAmbiguousWorkspaceRoot`) naming both paths and the two
supported resolutions (set the env override, or complete the migration). Never
pick a winner silently. This is the f015 dual-reader rule applied literally.

### Default for new workspaces

`backlogit init` creates `.backlog`. `init` in a directory that already holds a
`.backlogit` workspace refuses rather than creating a second root.

### Safety-set rule (non-negotiable)

Both candidate names are added to the canonical scan set, the archive-destination
guard, and the ID-collision guard **unconditionally and in code** — never derived
from config or registry. The `5f86ee9d` precedent is explicit that a degraded
config-derived scan set reopens a data-loss path, and two roots double the
exposure.

### Migration

A `backlogit migrate --workspace-dir` mode following the established UX contract:
`--dry-run` preview first, apply only after review, idempotent on re-run,
refuses when the destination already exists. The move is `git mv` for tracked
content with a filesystem fallback, mirroring the existing git-aware artifact
move so `git log --follow` survives. **The pure move is its own commit**, with no
content rewrites, to keep rename similarity above threshold.

### Doctor

A new `--check-workspace-root-conflict` finding, registered through the existing
`DoctorOptions` boolean pattern (`internal/core/doctor.go:133-165`). Advisory,
never blocking.

### Verification rule

Discovery changes must be exercised through the **MCP** path as well as the CLI,
because the MCP server defaults `RootPath` to `"."`. A CLI-only test matrix is
declared insufficient.

### Sequencing

This release unit ships **after** all formal-gate work (F1, F4, F6, F5). It is a
feature-class change and the operator policy ranks reliability and security work
ahead of it.

### Residual exposure (must be recorded in closure)

Merging does not change this repository's own pinned `C:\Tools\backlogit.exe`.
This repo's `.backlogit` directory keeps working by design (it is precedence
level 3), and **no in-repo directory rename is performed by this release unit**.
Renaming this repository's own backlog directory is explicitly out of scope and
would be a separate, later, operator-initiated action.

## Rejected Alternatives

- **Option A (hard rename)** — breaks every existing workspace with no
  diagnostic.
- **Option C (config-driven name)** — bootstrap paradox, and it makes the
  safety-critical scan set config-derived, which is the documented data-loss
  reopening.
- **Silent precedence when both roots exist** — rejected explicitly; an
  ordering-dependent winner is a silent data-loss vector.
- **Renaming this repository's own `.backlogit` directory as part of this unit** —
  rejected: it mixes a product change with a repo-state change, and the pinned
  binary would not honor it anyway.

## Unresolved Questions

- Whether the legacy `.backlogit` name should ever be *removed* is deliberately
  left open; the decision here is "honored indefinitely". A deprecation timeline
  needs adoption data that does not exist yet.
- Whether the test corpus should be migrated wholesale to `.backlog` or keep
  deliberate coverage of both names: the plan keeps **both**, because dual-root
  resolution is precisely what needs coverage.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Two roots double the data-loss surface for scan/archive/collision guards | Both names hardcoded into the safety set in code, never config-derived; dedicated guard tests |
| Discovery defect ships green via CLI-only CI | MCP-path discovery tests required, including the relative `RootPath: "."` case |
| Silent winner when both roots exist | Deterministic `ErrAmbiguousWorkspaceRoot` refusal, tested |
| `git log --follow` breaks on migration | Pure-move commit with no content rewrites; `git mv` with filesystem fallback |
| ~100 test files assert the old literal | Test updates are their own width-isolated tasks; both names keep coverage rather than a blanket find-and-replace |
| Docs audited shallowly and left factually wrong | Docs task requires verifying prose against the read/write code paths, not path existence |
| Env override becomes a traversal vector | Single-segment validation via the existing containment discipline; rejected values are a hard error |
