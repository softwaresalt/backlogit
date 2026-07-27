---
chunk_strategy: h1-h2-h3
description: 'Harden docline.collectInScopeDocs against a relative workspace root: the walk computes filepath.Rel(root, p) where p is always absolute (SafeResolve -> WalkDir) but root is passed through unresolved, so a relative root (the MCP server RootPath is "." by default) fails with `Rel: can''t make <abs> relative to "."`. Fix by resolving the Rel base to the same absolute root as the walked paths; landed test-first (failing relative-root test, then the two-line fix).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-26-docline-collectinscopedocs-relative-root-rel-fix-plan.md
title: 'Harden docline collectInScopeDocs against a relative root (filepath.Rel absolute-base fix)'
---

## Source

- Stash: `EF4C0EC6` (kind=bug, priority=medium) — "backlogit_docs_lint MCP tool
  (and `docs lint`) fails on Windows with an absolute workspace root:
  `docs lint: docline.collectInScopeDocs: walk: Rel: can't make
  C:\Source\GitHub\backlogit relative to "."`." Discovered during a Stage
  session for feature 125-F on 2026-07-24 (backlogit v1.7.0, go1.24.13).
- Deliberation: intentionally right-sized out (operator-authorized). Root cause
  and fix are small and well understood; the decision framing is folded into the
  Problem Frame and Decisions and Rationale sections below rather than a separate
  deliberation artifact.
- Prior art (compound library):
  - `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md` —
    never mutate process-global state (cwd, `time.Local`) inside a parallel-test
    package; drive environment-sensitive behavior without a global mutation.
    Shapes the RED test design (avoid `os.Chdir` / `t.Chdir`).
  - `docs/compound/2026-06-26-docline-frontmatter-contract.md` — docline scope
    and authoring-profile context.
  - `docs/compound/go-implementation/feature-001-core-implementation.md` §4
    ("SafeResolve Absolute Path Comparison") — the established repo pattern for
    this exact class of defect: a relative workspace root breaks `filepath`
    prefix/relative operations, and the remedy is to `filepath.Abs` the root
    FIRST. U2 applies the same absolutize-first pattern to `filepath.Rel`, so this
    fix is consistent with prior art rather than novel.
  - CLI/MCP surface-parity family
    (`docs/compound/2026-05-07-mcp-cli-config-parity.md`,
    `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`):
    parity bugs hide on the surface you did not touch — this is why the CLI docline
    gate stayed green while the MCP path failed. Fixing the shared function
    (`collectInScopeDocs`) is the single-source-of-truth remedy this family favors.

## Problem Frame

`internal/docline/service.go` walks the in-scope documentation surface in
`collectInScopeDocs(root, subPath string)`:

```go
base, err := core.SafeResolve(root, subPath)      // ALWAYS absolute
...
walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
    ...
    rel, err := filepath.Rel(root, p)             // root passed through unresolved
    ...
})
```

`core.SafeResolve` (`internal/core/workspace.go`) calls `filepath.Abs` on the
workspace root and returns an absolute path, so `base` — and therefore every `p`
yielded by `filepath.WalkDir(base, ...)` — is absolute. But the Rel **base** is
the raw `root` argument. When `root` is relative, `filepath.Rel(root, p)` fails
because `filepath.Rel` requires both operands to be either both absolute or both
relative. The first walk entry is `base` itself (the root directory), so the
error fires immediately, before any `d.IsDir()` filtering.

The relative root reaches the function through the MCP server, not the CLI:

- CLI: `internal/cli/docs.go` `resolveDocsRoot` calls `filepath.Abs(root)`, so the
  CLI (`make docs-lint`, the Linux "Docline frontmatter gate" CI job) passes an
  absolute root and never triggers the bug.
- MCP: `backlogit mcp` defaults `--cwd` to `"."`. `openMCPServer(ctx, ".")`
  (`internal/cli/root.go`) calls `core.NewWorkspace(".")`, whose
  `resolveWorkspaceRoot` returns `filepath.Clean(rootPath)` (`"."`, **not**
  absolutized) when `.backlogit/config.yaml` is found directly under the root.
  So `ws.RootPath == "."`, `NewServer(ws)` sets `s.RootPath == "."`, and
  `backlogit_docs_lint` calls `LintTree(Options{Root: "."})`.

This is a genuine, live defect on current `main` (`v1.7.0-57-g535b150d`),
reproduced end-to-end through the MCP server with `--cwd .`:

```text
{"error":"internal","message":"docs lint: docline.collectInScopeDocs: walk:
Rel: can't make C:\\Source\\GitHub\\backlogit relative to ."}
```

The failing primitive (`filepath.Rel(".", <abs>)`) was confirmed directly on this
platform; `filepath.Rel(<abs>, <abs>)` succeeds. The defect is cross-platform
(`Rel(".", "/abs")` fails on Linux too); it was observed on Windows because that
is where the operator ran the MCP server. The CI docline gate stays green because
it exercises the CLI (absolute-root) path.

## Requirements Trace

| Requirement (from stash EF4C0EC6) | Implementation action |
|---|---|
| `docs lint` / `backlogit_docs_lint` must succeed with a relative root | U2: absolutize the Rel base in `collectInScopeDocs` |
| Fix located in `docline.collectInScopeDocs`, resolving the Rel base to the same absolute root as the walked paths | U2: `filepath.Rel(absRoot, p)` where `absRoot = filepath.Abs(root)` |
| Windows-path-safe (platform-agnostic) regression test, added test-first | U1: failing relative-root test in `internal/docline/service_test.go` |
| Do not weaken the workspace-containment guarantee | U2: `base` still comes from `SafeResolve`; only the Rel base is normalized |

## Implementation Units

Two units, test-first (P-002 / P-004: a failing harness lands before its
production code). Each satisfies the 2-hour rule (< 3 files, < 5 functions,
< 4 test scenarios) and single-domain width isolation.

### U1 — Failing relative-root test (test-first, RED)

- **Execution posture**: test-first (RED). Lands and fails before U2.
- **Domain**: tests.
- **File**: `internal/docline/service_test.go` (in-package `docline` test, so it
  can call the unexported `collectInScopeDocs` directly, as the stash names).
- **What it proves**: `collectInScopeDocs` returns without error when `root` is a
  **relative** path that resolves to a real workspace directory. Before U2 the
  test fails with the `filepath.Rel` error; after U2 it passes.
- **RED-signal integrity (acceptance criterion — Go Reviewer P2 /
  Constitution P3)**: U1 MUST be *observed to fail* (a genuine RED), not
  `t.Skip`, before U2 lands — a skipped test provides no test-first evidence. On
  the standard single-volume dev/CI setup (repo and `t.TempDir()` share a volume)
  the relative root resolves and the test runs live, failing on the `filepath.Rel`
  error, so RED is real. Only the rare cross-volume Windows case degrades to a skip
  (see below); if a runner would otherwise skip, witness the RED evidence via the
  `t.Chdir` demonstration form before committing so the failure is actually seen.
- **Committed design (single, parallel-safe — per the compound prior art; do NOT
  use `os.Chdir`, and avoid relying on `t.Chdir` in the committed test)**:
  1. `dir := t.TempDir()`; seed a minimal in-scope doc (e.g. `docs/x.md`) so the
     post-fix assertion also covers a non-empty result set. (Even an empty dir
     triggers the RED error, because the first walk entry is the root itself; the
     seeded doc adds a positive GREEN assertion.)
  2. Derive a **relative** root without mutating process cwd:
     `rel, err := filepath.Rel(mustGetwd(t), dir)`. Add a short comment noting the
     test reads process cwd (via `os.Getwd`) to construct the relative root, so the
     cwd-dependence is explicit for future readers.
  3. On the rare Windows cross-volume case (`t.TempDir()` and cwd on different
     drives) `filepath.Rel` cannot express a relative path; guard with `t.Skip`
     **only** as a hermetic fallback. This does not weaken CI: the Linux runner
     (single volume) always exercises the assertion. Per the acceptance criterion
     above, if the primary dev box would skip, witness RED via the `t.Chdir` form
     first.
  4. Call `collectInScopeDocs(rel, "")` and assert `err == nil` and that the seeded
     doc appears in the returned slice.
- **Scope confirmation (Go Reviewer P3)**: before implementing, run a package-wide
  grep to re-confirm `collectInScopeDocs` is the sole call site with a raw
  `filepath.Rel(root, …)` (non-`SafeResolve`d base); planning already verified
  this, and the implementer re-confirms so no sibling regression hides.
- **Test scenarios**: one focused test function, e.g.
  `TestCollectInScopeDocs_RelativeRootDoesNotErrorOnRel`.

### U2 — Absolutize the Rel base in collectInScopeDocs (GREEN)

- **Execution posture**: minimal production fix that turns U1 green.
- **Domain**: source (core docline service).
- **File**: `internal/docline/service.go`, function `collectInScopeDocs`.
- **Change**: resolve the Rel base to an absolute path once, and use it for every
  `filepath.Rel`:

  ```go
  absRoot, err := filepath.Abs(root)
  if err != nil {
      return nil, fmt.Errorf("docline.collectInScopeDocs: resolve root: %w", err)
  }
  // inside WalkDir callback:
  rel, err := filepath.Rel(absRoot, p)
  ```

  `base` is already `SafeResolve(root, subPath)`, which is
  `filepath.Abs(filepath.Join(filepath.Abs(root), subPath))`, so `absRoot`
  (`filepath.Abs(root)`) is guaranteed to be a prefix of every walked `p`. The
  computed `rel` values (and the downstream `filepath.ToSlash` POSIX paths,
  `relPosix == "."` root check, `isExcludedDir`, `inScope`) are byte-for-byte
  identical to today's output for the already-working absolute-root callers.
- **Error handling**: wrap with `%w` per Safety-First Go; do not introduce
  `panic`, `unsafe`, or a second SafeResolve call.
- **No API/signature change**: `collectInScopeDocs(root, subPath string)` keeps
  its signature; only the internal Rel base is normalized.
- **Verification of sibling call sites** (already confirmed during planning): the
  other root consumers in `service.go` — `decodeDoc(opts.Root, rel)` and
  `core.SafeResolve(opts.Root, ...)` — route through `SafeResolve`, which
  absolutizes internally, so they are already robust to a relative root. Only
  `collectInScopeDocs` had a raw `filepath.Rel(root, ...)`; fixing it closes the
  walk-path defect end to end.
- **Accepted redundancy (Go Reviewer P3 / Architecture Strategist P3)**: for
  already-absolute-root callers, `filepath.Abs(root)` duplicates normalization that
  `SafeResolve` also performs. This is accepted as the narrowest correct fix; a
  shared canonical-root helper (so normalization lives at one boundary) is captured
  as the RootPath follow-up in Risks rather than introduced here.

## Dependency Graph

- U1 → U2. U1 lands first and fails (RED). U2 makes U1 pass (GREEN). No cycles.
- No dependency on any other in-flight shipment. Touches only
  `internal/docline/service.go` and `internal/docline/service_test.go`.

## Decisions and Rationale

- **Fix in `collectInScopeDocs`, not `resolveWorkspaceRoot`.** The stash scopes
  the fix to `docline.collectInScopeDocs`, and the defensive fix belongs at the
  function that makes the invalid assumption. `collectInScopeDocs` already assumes
  `p` is absolute (it comes from `SafeResolve` → `WalkDir`), so it must use an
  absolute Rel base. Absolutizing `RootPath` inside `resolveWorkspaceRoot` would
  be a broader change with wide blast radius (every MCP tool's path handling) and
  is out of scope here. See Risks for the follow-up note.
- **"Absolute base" over "make both relative".** The stash offers two directions.
  Absolutizing the Rel base (`filepath.Abs(root)`) is the smaller, lower-risk edit
  and preserves the existing absolute-root behavior byte-for-byte, versus
  reworking the walk to keep `base`/`p` relative (which would ripple through
  `SafeResolve`'s absolute contract).
- **Test targets `collectInScopeDocs` directly** (in-package test), matching the
  stash's ask and giving the tightest RED signal, rather than only the exported
  `LintTree` wrapper.
- **Ceremony right-sizing (operator-authorized).** For a ~1–2h, well-understood,
  low-blast-radius bug the separate deliberation artifact is skipped and hardening
  is not required (see Plan Hardening Signals). This is recorded explicitly, not
  silently skipped.

## Risks and Caveats

- **Behavior drift for absolute-root callers**: none expected —
  `filepath.Abs(absPath) == absPath`, so `filepath.Rel(absRoot, p)` equals the
  current `filepath.Rel(root, p)` whenever `root` was already absolute. U1 plus the
  existing docline suite guard against regression.
- **Windows cross-volume temp dir**: the preferred RED test may `t.Skip` on a dev
  box where `t.TempDir()` and cwd are on different drives; the Linux CI runner
  keeps the assertion live, so coverage is not lost in CI.
- **Follow-up (out of scope here — capture as stash; Architecture Strategist P2)**:
  `RootPath` is currently an undefined shared invariant. `resolveWorkspaceRoot`
  returns a relative root when invoked with a relative `--cwd`, so the MCP server
  can run with `RootPath == "."`; and the server can also be built directly via
  `NewServerForRoot(rootPath)`, which bypasses `resolveWorkspaceRoot` normalization
  entirely. This fix makes docline robust to a relative root, but other MCP tools
  reading `s.RootPath` may carry latent relative-root assumptions. The explicit
  follow-up is to **define one `RootPath` contract** (a canonical absolute root)
  and audit **both** construction paths — `resolveWorkspaceRoot` and
  `NewServerForRoot` — plus every path-sensitive MCP consumer, so normalization
  happens once at a common boundary instead of defensively per call site. This is
  intentionally not bundled here (YAGNI / width isolation) and is deliberately
  broader than the original `resolveWorkspaceRoot`-only framing.

## Constitution Check

Mapped against `.github/instructions/constitution.instructions.md` (all
principles I–XI):

- **I. Safety-First Go** (NON-NEGOTIABLE): pass. Production code stays in Go; the
  new error path wraps with `%w`; no `panic`, no `unsafe`.
- **II. Test-First Development** (NON-NEGOTIABLE): pass. U1 lands a failing test
  (observed RED, not skip) before U2's production change (P-002 / P-004).
- **III. Workspace Isolation and Security Boundaries** (NON-NEGOTIABLE): pass.
  `base` still comes from `core.SafeResolve`, which enforces the workspace-boundary
  / path-escape guard; only the Rel base used to report relative POSIX paths is
  normalized. No containment weakening, no secrets.
- **IV. CLI Workspace Containment** (NON-NEGOTIABLE): pass / N/A. No files are
  created, modified, or deleted outside the working tree; the change is an internal
  function fix plus its in-package test.
- **V. Structured Observability**: N/A. No logging, telemetry, or event surface
  changes; the corrected error path already wraps with `%w`.
- **VI. Single Responsibility**: pass. The fix is confined to the one function that
  makes the invalid relative-root assumption. The `filepath.Abs(root)` call
  duplicates normalization `SafeResolve` also performs, accepted as the narrowest
  correct fix; a shared canonical-root helper is deferred to the RootPath follow-up
  in Risks.
- **VII. Destructive Command Approval** (NON-NEGOTIABLE): N/A. No destructive or
  irreversible steps.
- **VIII. Explicit Safety Modes for Elevated Risk**: N/A. No elevated-risk mode;
  low-blast-radius internal fix.
- **IX. Git-Friendly Persistence**: pass. Only Go source/test and markdown
  artifacts change; no binary or non-diffable persistence introduced.
- **X. Agent Context Efficiency**: pass. Two-line production change plus one
  focused test; right-sized ceremony (no separate deliberation artifact).
- **XI. Merge Commit History Preservation** (NON-NEGOTIABLE): pass. Ships through a
  merge commit; no squash or rebase (P-009).

Constitution Check: pass

## Plan Hardening Signals

- Public API, schema, or contract change: **absent**. Internal function fix; no
  exported signature, schema, or CLI/MCP contract change (output is preserved for
  existing callers and corrected for relative-root callers).
- Security, auth, permission, or compliance-sensitive behavior: **absent**. The
  workspace-containment boundary (SafeResolve) is unchanged; the fix does not touch
  auth, permissions, or secrets.
- Migration, backfill, destructive data/config action, or irreversible step:
  **absent**.
- External integration, operator checkpoint, or external dependency: **absent**.
- High runtime, rollout, or rollback risk: **absent**. Two-line change plus a test;
  fully reversible via revert.

Requires plan hardening: no

## Runtime Verification and Closure

- **Runtime surface changed**: the docline walk used by `backlogit docs lint`
  (CLI) and `backlogit_docs_lint` (MCP). U2 changes runtime behavior for the
  relative-root path only.
- **Verification before absorption**:
  1. `go test ./internal/docline/...` — U1 fails before U2 (RED), passes after
     (GREEN); the full docline suite stays green.
  2. Repo gates: `gofmt -l .` clean, `go vet ./...` clean, `go test ./...` green.
  3. End-to-end MCP re-check: **rebuild from HEAD first** (use `go run
     ./cmd/backlogit …` or reinstall the binary — a stale installed binary will not
     reflect U2, per the post-merge fresh-binary learning), then run
     `backlogit mcp --cwd .` and call `backlogit_docs_lint`; expect a valid lint
     result (no `Rel` error). This reproduced the bug during planning and is the
     acceptance probe.
  4. `make md-lint` (P-008, repo-wide blocking gate) reports 0 violations for every
     new or changed markdown file in this Stage shipment — i.e. this plan plus the
     backlog items harvested for it. Repo-wide scope is inherent to the P-008 gate;
     this fix does not otherwise pull backlog authoring into its own runtime scope.
- **Operational closure**: no monitoring/rollback infrastructure needed for a
  contained internal fix; closure is the merged PR plus the passing regression
  test (U1) which stands as the durable guard. The Windows docline self-lint that
  motivated this stash becomes usable via the MCP tool again after merge.

## Plan Review

- **dispatch_mode**: multi-agent-dispatch
- **reviewers**: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist (5 independent personas, parallel).
- **decision**: PASS

### Findings by severity (pre-incorporation)

- **P0**: none.
- **P1**: 1 — Learnings Researcher: plan should cite the most on-point prior art,
  `docs/compound/go-implementation/feature-001-core-implementation.md` §4
  (SafeResolve absolutize-first). Corroborating, non-blocking (plan was already
  *consistent* with the pattern; the gap was the citation). **Resolved**: citation
  added to Source → Prior art, plus the CLI/MCP surface-parity family.
- **P2**: 2 — (a) Go Reviewer: the RED test could `t.Skip` on a cross-volume
  Windows box, and a skip is not a RED; ensure U1 is observed to *fail* before U2.
  **Resolved**: added an explicit RED-signal-integrity acceptance criterion to U1
  (observe RED live on the single-volume common case; witness via `t.Chdir` if a
  runner would otherwise skip). (b) Architecture Strategist: `RootPath` is an
  undefined shared invariant; make the follow-up explicit and broader than
  `resolveWorkspaceRoot`. **Resolved**: Risks follow-up rewritten to define one
  `RootPath` contract and audit both `resolveWorkspaceRoot` and `NewServerForRoot`.
- **P3**: several — completed the Constitution Check across all principles I–XI
  with numerals (Constitution P3 x2); committed U1 to a single parallel-safe design
  with `t.Chdir` only as RED-evidence fallback (Scope P3, Constitution P3); added a
  package-wide-grep scope-confirmation step and a cwd-read comment note to U1 (Go
  P3 x2); documented the accepted redundant `filepath.Abs` normalization in U2 and
  the Constitution Check (Go P3, Architecture P3); scoped the md-lint closure
  wording to this shipment's own artifacts (Scope P3); added a rebuild-from-HEAD
  step before the MCP acceptance probe (Learnings advisory).

### Reviewer verdicts

- Constitution Reviewer: Constitution Check verdict justified — **yes** (0 P0/P1/P2).
- Go Reviewer: **APPROVE** (byte-for-byte parity for absolute-root callers
  confirmed; `%w` idiom mirrors `ValidateApplyPath`).
- Scope Boundary Auditor: **APPROVE** (no scope creep; deferral of the
  `resolveWorkspaceRoot`/RootPath audit is legitimate YAGNI).
- Learnings Researcher: confidence medium; consistent with prior art.
- Architecture Strategist: chosen locus **architecturally sound** with the explicit
  RootPath follow-up now recorded.

### Gate rationale

Pre-incorporation tally was 0 P0, 1 corroborating P1, 2 P2. All high-value findings
(the P1 and both P2s) were incorporated into the plan text/units above, leaving no
outstanding P0/P1/P2. Per the plan-review gate (P3-only or none → PASS), the
post-incorporation decision is **PASS**. Operator pre-authorized autonomous
execution to merge-readiness; no separate ADVISORY authorization is required
because the blocking-tier findings were resolved rather than waived.
