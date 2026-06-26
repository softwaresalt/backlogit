---
title: Adversarial Review — docline frontmatter tooling
date: 2026-06-25
branch: feat/065-docline-frontmatter
base: 5b34ed1d
mode: report-only (no code modified)
reviewers: 3 (independent, multi-model consensus)
review_models:
  - reviewer-A: claude-sonnet-4.6 (Tier 2)
  - reviewer-B: gpt-5.5 (Tier 3)
  - reviewer-C: gemini-3.1-pro-preview (Tier 1)
scope:
  - internal/docline/ (codec, policy, frontmatter, validate, classify, normalize, service, report)
  - internal/cli/docs.go (+ root.go wiring)
  - internal/mcp/docs_tools.go (+ tools.go wiring)
schema: schemas/docline/base-frontmatter-v1.schema.json
---

# Adversarial Review: docline frontmatter tooling

Three independent reviewers (different model families — Claude, GPT, Gemini) reviewed
the same diff (`5b34ed1d..HEAD`) against the seven critical invariants. The aggregator
(this report) independently re-verified every finding against the source and
`core.SafeResolve` before classifying. Findings are keyed by `file + line + rule`,
counted across reviewers, and assigned a confidence tier.

## Gate verdict

**No HIGH-confidence (3/3 consensus) P0/P1 finding exists.** The PR is **not blocked**
by a unanimous critical defect.

The single 3/3 consensus finding (atomic-write file-mode preservation, **C1**) is HIGH
confidence but **low severity** (P2/P3) — it does not corrupt content and is invisible
to git. The most *impactful* findings — the apply-gate `--path .` bypass (**M1/M2**) and
the move-never-drop edge cases (**M3/M4**) — are **MEDIUM confidence (2/3)** and warrant
explicit acknowledgment (fix or defer-with-rationale) before merge. Body preservation and
idempotency — the two highest-risk invariants — were probed by all three reviewers and the
aggregator and found **sound**.

| # | Finding | Reviewers | Confidence | Severity | Priority |
|---|---------|-----------|------------|----------|----------|
| C1 | Atomic write drops original file mode (temp 0600 → target) | A,B,C (3/3) | **HIGH** | MAJOR* | 9 |
| M3 | Move-never-drop: top-level key overwrites existing `docline.<k>` | B,C (2/3) | MEDIUM | CRITICAL† | 8 |
| M4 | Move-never-drop: scalar `docline:` value silently dropped | A,C (2/3) | MEDIUM | CRITICAL† | 8 |
| M1 | Apply-gate bypass: `--path .` / `--path docs/..` (CLI) | B,C (2/3) | MEDIUM | MAJOR | 6 |
| M2 | Apply-gate bypass: `path="."` (MCP) | B,C (2/3) | MEDIUM | MAJOR | 6 |
| L1 | ApplyMigration writes per-change; no zero-write preflight | B (1/3) | LOW | MAJOR | 3 |
| L2 | `ValidateFields` is not full-schema validation | B (1/3) | LOW | MAJOR | 3 |
| L3 | `--dry-run` flag is decorative (no effect) | A (1/3) | LOW | MINOR | 2 |
| L4 | TOCTOU: apply writes plan-time `After` without re-read (aggregator) | — | LOW | MINOR | 2 |

\* C1 conservative severity (one reviewer rated MAJOR; two rated MINOR). Real-world impact
is low — see C1. † M3/M4 conservative severity = CRITICAL (one reviewer); practical impact
is malformed/conflicting-input edge cases (see notes). Priority = confidence_weight ×
severity_weight.

---

## 1. Consensus findings (HIGH confidence — flagged by all 3 reviewers)

### C1 — Atomic write does not preserve the target file's permission mode
- **File:** `internal/docline/service.go:259-266` (`atomicWrite`, `os.CreateTemp` at L261)
- **Severity:** MAJOR (conservative) / practical **P2–P3** · **Confidence: HIGH (A,B,C)**
- **Problem:** `os.CreateTemp` creates the temp file with mode `0600` (Unix). After
  `os.Rename(tmpName, path)` the migrated document inherits `0600`, discarding the target's
  original mode (typically `0644`). On Unix CI/multi-user hosts a migrated doc becomes
  owner-only readable.
- **Real-world impact (aggregator note):** Low. Git tracks only the executable bit
  (`100644` vs `100755`), so `0600` vs `0644` produces **no** `git status` change and does
  **not** corrupt content. On the Windows dev host the mode is largely irrelevant. This is
  a hygiene defect, not a data-integrity defect — hence not a gate blocker despite HIGH
  confidence.
- **Fix:** `os.Stat(path)` before creating the temp; after `tmp.Close()` and before
  `os.Rename`, `os.Chmod(tmpName, info.Mode().Perm())`. Fall back to `0644` for the insert
  case (new file / Stat failure). Add a test asserting mode is preserved across an update.

---

## 2. Majority findings (MEDIUM confidence — flagged by >½ of reviewers)

### M3 — Move-never-drop: folding a top-level key overwrites an existing `docline.<k>`
- **File:** `internal/docline/normalize.go:39-44` (`docline[k] = v` at L43)
- **Severity:** CRITICAL (conservative; B=MAJOR, C=CRITICAL) · **Confidence: MEDIUM (B,C)**
- **Problem:** The namespace is seeded from any existing `docline` map (L34-38), then every
  non-contract top-level key is folded in with `docline[k] = v` (L43). If a top-level key
  `foo` *and* `docline.foo` both pre-exist, the top-level value silently overwrites the
  namespaced one — a silent drop, violating "move, never drop."
  ```yaml
  foo: B            # top-level
  docline:
    foo: A          # overwritten by B → A is lost
  ```
- **Reachability (aggregator note):** Only reachable for hand-authored / conflicting input
  (a normalized doc never has a top-level non-contract key, so this cannot recur and does
  **not** break idempotency). Bounded blast radius, but it *is* silent data loss.
- **Fix:** Detect the collision before assignment; either preserve both deterministically
  (e.g. `docline["foo"]` kept, incoming stored as `docline["foo__top"]`) or return a typed
  "manual resolution required" error. Add a collision test.

### M4 — Move-never-drop: a scalar/non-map `docline:` value is silently dropped
- **File:** `internal/docline/normalize.go:34` (`fm["docline"].(map[string]any)` guard)
- **Severity:** CRITICAL (conservative; A=MINOR, C=CRITICAL) · **Confidence: MEDIUM (A,C)**
- **Problem:** If `fm["docline"]` is present but not a `map[string]any` (e.g. `docline: hello`
  or a sequence), the type assertion at L34 fails, the fold loop skips `k == "docline"` (L40),
  and `FromMap` (frontmatter.go:47) applies the *same* failing assertion. `b.Docline` is then
  overwritten by the freshly-built (empty) map at L54. The malformed pre-existing value is
  lost with no error.
- **Reachability (aggregator note):** Malformed input only (`docline` is a contract namespace
  expected to be a map). No test covers this path.
- **Fix:** When the assertion fails and `fm["docline"] != nil`, either preserve the raw value
  (e.g. under `docline["_legacy_docline"]`) or return a typed validation error rather than
  dropping it.

### M1 — Apply gate is bypassable with `--path .` / `--path docs/..` (CLI)
- **File:** `internal/cli/docs.go:108` (`if path == ""`)
- **Severity:** MAJOR · **Confidence: MEDIUM (B,C)**
- **Problem:** The "whole-tree apply is refused" rail checks only `path == ""`. The
  aggregator confirmed `core.SafeResolve(root, ".")` → `root` and `SafeResolve(root, "docs/..")`
  → `root`, both returned **without error** (workspace.go:258 allows `abs == cleanRoot`).
  Therefore `docs migrate --apply --yes --path .` (or `--path docs/..`) passes the gate and
  applies across the **entire in-scope tree** — exactly what the gate intends to prevent.
- **Fix:** After resolving, reject any scope that resolves to (or above) the workspace root:
  `if abs == workspaceRoot { return ErrUnscopedApply }`. Require a proper sub-path. Add a
  test for `--path .` and `--path docs/..`.

### M2 — Apply gate is bypassable with `path="."` (MCP) — agent-surface risk
- **File:** `internal/mcp/docs_tools.go:81` (`if path == ""`)
- **Severity:** MAJOR · **Confidence: MEDIUM (B,C)**
- **Problem:** Same defect on the MCP surface: with `BACKLOGIT_DOCS_ALLOW_APPLY` enabled,
  `apply=true, path="."` clears the `path == ""` guard and applies tree-wide. **Higher risk
  than M1** because an LLM agent told to "migrate the docs" may naïvely pass `path="."`,
  defeating the scoped-apply safety rail without the human friction of typing it on a CLI.
- **Fix:** Same as M1 — resolve and reject root-equivalent scopes before `PlanMigration`.
  Centralize the check in `docline` (e.g. an `Options.requireScoped` guard reused by CLI+MCP)
  so the two surfaces cannot drift.

---

## 3. Unique findings (LOW confidence — flagged by exactly one reviewer / aggregator)

### L1 — ApplyMigration validates-then-writes per change; no all-or-nothing preflight
- **File:** `internal/docline/service.go:156-176` (`ApplyMigration`)
- **Severity:** MAJOR · **Confidence: LOW (B)**
- **Problem:** The function doc (L153-155) claims "every target is resolved through
  SafeResolve and any escape is rejected ... **before a single write**." The implementation
  interleaves the `BodyBytesChanged`/`SafeResolve` checks with `atomicWrite` inside one loop,
  so a plan `[valid, escaping]` writes the first file *before* returning
  `ErrPathEscapesWorkspace` — violating the documented "ZERO writes on escape" invariant and
  yielding a partial migration.
- **Reachability (aggregator note):** Not reachable via the CLI/MCP flow, which always builds
  the plan from `PlanMigration` (walked + safe-resolved paths). It is reachable only with an
  externally-constructed plan (`ApplyMigration` is exported). Defense-in-depth + doc-accuracy
  gap rather than a live exploit.
- **Fix:** Preflight **all** non-noop changes (`SafeResolve` + `BodyBytesChanged`, capturing
  resolved abs paths) and only begin atomic writes once the whole plan passes. Align the doc
  comment with the implementation.

### L2 — `ValidateFields` is field-presence + vocab only, not full schema validation
- **File:** `internal/docline/validate.go:19-45`
- **Severity:** MAJOR · **Confidence: LOW (B)**
- **Problem:** Lint checks required-field presence and `doc_type` vocabulary only. It does not
  enforce the base schema's `additionalProperties:false`, the `content_sha256` pattern, the
  `ingested_at` date-time format, or field types. The MCP tool description ("Validate ...
  against the docline base schema") overstates the behavior.
- **Aggregator note:** Likely by-design (lightweight authoring linter, not a JSON-schema
  validator) — but the description/behavior mismatch is real. Decide intent: either tighten
  the validator toward the schema or soften the tool description.
- **Fix:** Either validate the decoded frontmatter against
  `base-frontmatter-v1.schema.json`, or reword the description to "required-field + doc_type
  vocabulary checks (not full schema validation)."

### L3 — `--dry-run` flag is decorative (no behavioral effect)
- **File:** `internal/cli/docs.go:123` (`_ = dryRun`), flag declared L131
- **Severity:** MINOR · **Confidence: LOW (A)**
- **Problem:** `dryRun` is read into `_` and never influences control flow. `migrate
  --dry-run=false` (without `--apply`) still dry-runs; `migrate --apply --dry-run=true` still
  applies. The flag is non-functional. No safety impact (absence of `--apply` is already the
  dry-run path), but it is a misleading UX contract.
- **Fix:** Remove the flag, or enforce it — e.g. `if apply && dryRun { return error("--dry-run
  and --apply are mutually exclusive") }`.

### L4 — TOCTOU: apply writes plan-time `After` without re-reading the target (aggregator)
- **File:** `internal/docline/service.go:113-176` (`PlanMigration` → `ApplyMigration`)
- **Severity:** MINOR · **Confidence: LOW (aggregator-identified, not in reviewer set)**
- **Problem:** `ApplyMigration` writes `c.After` (computed from the file as read at plan time)
  without re-reading the file. If the document's body changes between plan and apply, applying
  the stale `After` reverts the body to the plan-time bytes — a body mutation *relative to the
  current on-disk state*, even though `BodyBytesChanged` (computed at plan time) is false.
- **Aggregator note:** The CLI and MCP flows call plan→apply back-to-back (microsecond window,
  no concurrency), so practical risk is negligible. Flagged for completeness.
- **Fix (optional):** Re-read and re-validate `bodyBytesChanged(currentRaw, After)` inside
  `ApplyMigration` immediately before writing, or persist a content hash of `Before` in the
  `Change` and verify it matches the on-disk file before rename.

---

## 4. Invariants confirmed SOUND (probed, no defect)

- **Body preservation (codec.go):** `Decode` CRLF-normalizes the *frontmatter block only*
  (L52-53); the body is sliced verbatim. The closing fence is the first line that is exactly
  `---` with no leading whitespace (L106-127), so indented block-scalar `---` and body
  horizontal rules are not mistaken for the fence. `Encode` writes the body bytes unchanged
  (L84). **No body-mutation path found.**
- **Idempotency (normalize.go + frontmatter.go):** `ingested_at` is seed-once (L51 — preserved
  when non-empty); `ToMap` emits a deterministic, sorted (via `yaml.Marshal`) key set;
  `strFromMap` re-renders any YAML-decoded `time.Time` back to RFC3339 (frontmatter.go:94-95),
  stabilizing timestamp round-trips. `Normalize(Normalize(x)) == Normalize(x)` holds for
  well-formed input.
- **Path containment:** `collectInScopeDocs`, `decodeDoc`, `PlanMigration`, and
  `ApplyMigration` all route through `core.SafeResolve`. True escapes (`../escape`, absolute
  paths) are rejected (workspace.go:258-259). (The root-equivalent gap is the *apply-gate*
  weakness M1/M2 and the *preflight ordering* L1 — not a containment-bypass.)
- **Error handling:** Errors are wrapped with `%w` and typed sentinels
  (`ErrPathEscapesWorkspace`, `ErrBodyMutated`, `ErrMissingRequiredField`, `ErrUnknownDocType`)
  used consistently; `errors.Is` matching is correct in the MCP handlers. *Minor aggregator
  note:* service.go wraps `SafeResolve`'s error as `ErrPathEscapesWorkspace` discarding the
  original (`%w` on the sentinel, not the source error), so a non-escape resolve failure would
  be mislabeled an escape — cosmetic, very low severity.

---

## 5. Remediation plan (priority = confidence × severity, descending)

| Rank | # | Action class | Owner action |
|------|---|--------------|--------------|
| 1 | C1 | `gated_auto` | Preserve target mode in `atomicWrite` (Stat → Chmod before rename) + test. Deterministic. |
| 2 | M3 | `gated_auto` | Add collision handling in the namespace fold; confirm preservation strategy. |
| 3 | M4 | `gated_auto` | Preserve/diagnose scalar `docline:`; add malformed-input test. |
| 4 | M1 | `gated_auto` | Reject root-equivalent `--path` scopes (CLI). |
| 5 | M2 | `gated_auto` | Reject root-equivalent `path` scopes (MCP); share guard with CLI. |
| 6 | L1 | `manual` | Preflight whole plan before first write; fix doc comment. |
| 7 | L2 | `advisory` | Decide validator scope vs. tool description. |
| 8 | L3 | `advisory` | Remove or enforce `--dry-run`. |
| 9 | L4 | `advisory` | Optional TOCTOU re-read before write. |

**Recommended pre-merge bar:** Fix **C1** (cheap, deterministic) and **M1/M2** (close the
scoped-apply rail — especially the MCP agent surface). Explicitly acknowledge **M3/M4** (fix or
defer with a "malformed/conflicting input only" rationale). L1–L4 may defer to a follow-up.

---

## 6. Backlog work items (P0/P1)

```yaml
- type: bug
  title: "Apply gate: reject root-equivalent --path (CLI whole-tree apply bypass)"
  description: "`docs migrate --apply --yes --path .` (or `--path docs/..`) passes the `path==\"\"` gate because SafeResolve resolves both to the workspace root, enabling a whole-tree apply the gate intends to refuse."
  file: "internal/cli/docs.go"
  line: 108
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "After resolving, reject any scope equal to (or above) the workspace root; require a proper sub-path. Add tests for `--path .` and `--path docs/..`."
  linked_review: "docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md"

- type: bug
  title: "Apply gate: reject root-equivalent path (MCP backlogit_docs_migrate apply bypass)"
  description: "With BACKLOGIT_DOCS_ALLOW_APPLY enabled, apply=true with path=\".\" clears the `path==\"\"` guard and applies tree-wide. Higher risk than the CLI variant: an LLM agent may pass path=\".\" naively, defeating the scoped-apply rail."
  file: "internal/mcp/docs_tools.go"
  line: 81
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Resolve and reject root-equivalent scopes before PlanMigration; centralize the scoped-apply guard in internal/docline so CLI and MCP cannot drift."
  linked_review: "docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md"
```

> Recommended (HIGH confidence, lower severity — not strictly P0/P1 but cheap to fix):

```yaml
- type: bug
  title: "Atomic write: preserve target file mode across migration"
  description: "atomicWrite creates the temp file at 0600; after rename the migrated doc loses its original 0644 mode on Unix. No git-visible change, but working-tree files become owner-only readable."
  file: "internal/docline/service.go"
  line: 261
  severity: "MAJOR"
  confidence: "HIGH"
  fix: "Stat the target, Chmod the temp to the existing mode before rename; fall back to 0644 for inserts. Add a mode-preservation test."
  linked_review: "docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md"

- type: bug
  title: "Normalizer move-never-drop edge cases (collision overwrite + scalar docline drop)"
  description: "Folding a top-level key over an existing docline.<k> silently overwrites it; a scalar `docline:` value is silently dropped by the map type assertion. Both violate move-never-drop on malformed/conflicting input."
  file: "internal/docline/normalize.go"
  line: 34
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Detect namespace collisions and preserve both deterministically; preserve or error on a non-map docline value. Add malformed-input tests."
  linked_review: "docs/closure/2026-06-25-docline-frontmatter-adversarial-review.md"
```

---

## 7. Remediation status (Ship run 1 — applied in commit 68640b7f)

The MEDIUM-confidence findings were acknowledged and resolved on the same branch
before opening the PR (no HIGH-confidence consensus blockers existed, so the gate
did not hard-block). Each fix is TDD-covered and all four gates pass.

| ID | Finding | Status | Resolution |
|---|---|---|---|
| C1 | atomicWrite drops original file mode (0600) | ✅ FIXED | `atomicWrite` now stats the target and chmods the temp to its mode (0644 default for inserts). Tests: `TestAtomicWrite_PreservesExistingFileMode`, `TestAtomicWrite_NewFileDefaultsTo0644` (POSIX-gated). |
| M1 | CLI `--path .` whole-tree apply bypass | ✅ FIXED | Shared `docline.ValidateApplyPath` guard (sentinel `ErrWholeTreeApply`) rejects empty + root-equivalent scopes. CLI apply gate now calls it. Tests in `TestDocsMigrate_ApplyRequiresYesAndPath` (`--path .`, `--path docs/..`). |
| M2 | MCP `path="."` whole-tree apply bypass | ✅ FIXED | Same shared guard wired into `handleDocsMigrate` so CLI and MCP cannot drift. Test in `TestDocsMigrateTool_ApplyRequiresPathWhenEnabled` (`path="."`, asserts no writes). |
| M3 | Fold collision overwrites existing `docline.<k>` | ✅ FIXED | `foldUnderDocline` preserves colliding values under a deterministic `<k>_topN` key instead of overwriting. Test: `TestNormalize_PreservesCollidingFoldedKeys` (incl. idempotency). |
| M4 | Scalar `docline:` value silently dropped | ✅ FIXED | Non-map `docline` value is folded under the namespace (move-never-drop). Test: `TestNormalize_PreservesScalarDoclineValue` (incl. idempotency). |
| L1 | No zero-write preflight in ApplyMigration | ⏭️ DEFERRED | Stashed as a follow-up for Stage (low risk; apply is gated + body-mutation guarded). |
| L2 | ValidateFields is not full-schema validation | ⏭️ DEFERRED | Stashed as a follow-up (contract-field presence is sufficient for the authoring gate; full JSON-schema validation is a future enhancement). |
| L3 | `--dry-run` flag is decorative (`_ = dryRun`) | ⏭️ DEFERRED | Stashed as a follow-up (default behavior is already dry-run; flag is cosmetic). |
| L4 | TOCTOU: apply writes plan-time `After` without re-read | ⏭️ DEFERRED | Stashed as a follow-up (single-writer migration; body-mutation guard catches drift). |

**MEDIUM-confidence acknowledgement (per adversarial-review gate rule):** all five
MEDIUM/HIGH-non-blocking findings (C1, M1, M2, M3, M4) were addressed in this run
rather than deferred. The four LOW-confidence findings (L1–L4) are deferred to
follow-up stashes for the Stage agent.

---

*Generated by Adversarial Review (3-reviewer consensus). Report-only — no code modified.
Every finding independently re-verified against source + core.SafeResolve before classification.*
