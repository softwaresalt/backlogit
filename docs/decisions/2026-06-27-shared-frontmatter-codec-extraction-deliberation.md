---
chunk_strategy: h1-h2-h3
description: Deliberation on extracting the duplicated body-preserving frontmatter codec (and atomic-write helper) shared by internal/docline and internal/core into an import-cycle-free leaf package
doc_type: decision
docline:
    decision_status: decided
    depth: standard
    linked_artifacts:
        - docs/closure/2026-06-26-archived-from-migration-closure.md
        - docs/compound/2026-06-26-docline-frontmatter-contract.md
    promoted_to: plan
    stash_ids:
        - 8863C6C8
    tags:
        - tech-debt
        - refactor
        - docline
        - frontmatter-codec
        - import-cycle
        - atomic-write
        - width-isolation
    topic: Extract the shared frontmatter codec + atomic-write helper into a leaf package to remove the internal/docline <-> internal/core duplication (stash 8863C6C8)
ingested_at: "2026-06-28T05:42:20Z"
schema_version: "1.0"
source: docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md
title: 'Shared body-preserving frontmatter codec: extract a leaf package to end the docline/core duplication'
---

## Problem Frame

`internal/docline` owns a body-preserving Markdown frontmatter codec: `Decode`/`Encode`
plus the fence helpers `openingFenceLen` and `splitAtClosingFence` (`internal/docline/codec.go`).
The codec rewrites ONLY the YAML frontmatter block (deterministic sorted keys, CRLF normalized
inside the fence only) and slices the body verbatim — the closing fence is the first line that is
exactly `---` with no leading whitespace.

Because `internal/docline` imports `internal/core` (`docline/service.go` uses `core.SafeResolve`),
`internal/core` **cannot** import `internal/docline` without creating a package import cycle. During
067-S (PR #141, `docs/closure/2026-06-26-archived-from-migration-closure.md`), `internal/core/doctor.go`
needed the same body-preserving rewrite to repair `archived_from` fields and therefore **inlined a
second copy** of the codec:

| Concept | `internal/docline` | `internal/core/doctor.go` |
|---|---|---|
| opening fence length | `openingFenceLen` (codec.go:90) | `frontmatterOpenLen` (doctor.go:565) — byte-identical |
| split at closing fence | `splitAtClosingFence` (codec.go:106) | `splitAtFrontmatterFence` (doctor.go:581) — byte-identical logic |
| decode + re-encode | `Decode`/`Encode` + `Markdown` (codec.go) | re-inlined inside `rewriteArchivedFromField` (doctor.go:530) |
| atomic write | `atomicWrite` (service.go:320) | `atomicWriteArchiveFile` (doctor.go:619) — **different implementation** |

**The pain:** the fence/decode/encode logic is genuine, byte-identical duplication. A future fix or
correctness change to one copy (e.g. fence-detection edge cases, CRLF handling) will silently NOT
reach the other, producing divergent frontmatter handling between the docline migration path and the
doctor repair path. The `doctor.go` comment (lines 523-529) itself documents the duplication as a
deliberate workaround for the import cycle. This is the tech debt 8863C6C8 asks us to retire.

**Who cares:** maintainers of the docline contract and the doctor repair path; future reviewers who
must keep two codecs in lock-step by hand.

**Constraints / invariants that MUST survive (from the docline contract + 065-S adversarial review):**
- Body-byte stability: `body_bytes_changed` must remain false for every legitimate rewrite; the body
  is sliced verbatim (CRLF, trailing whitespace, horizontal rules preserved).
- Fence semantics: opening fence is `---\n` or `---\r\n` at byte 0; closing fence is the FIRST line
  that is exactly `---` (trailing CR tolerated, no leading whitespace).
- Deterministic encode: `yaml.Marshal` sorted keys, `---\n` … `---\n` framing, LF-terminated.
- `docline`'s public API (`docline.Decode`, `docline.Encode`, `docline.Markdown`) is consumed by
  `cmd/gen-docs/main.go` (calls `docline.Decode`) — it must NOT break.
- Idempotency: `Normalize(Normalize(x)) == Normalize(x)`; single-file re-apply is a byte-identical no-op.
- The CLI-Reference-Drift gate and `backlogit docs lint` must stay green.

**Success criteria:** one body-preserving codec, imported by both packages; zero behavior change
(proven by characterization tests + existing suites + `git diff` showing no content hunks on a
single-file migrate/repair); no import cycle; `docline`'s public codec API preserved for `gen-docs`.

**Out of scope:** the docline migration-hardening follow-ups L2 (`B349CBED`, full JSON-schema
validation) and L4 (`AE53BC5C`, apply-time TOCTOU re-read); any change to the docline taxonomy,
scope, or profiles; any change to WHAT frontmatter fields mean.

## Research Findings

- **Import direction confirmed:** `internal/docline` → `internal/core` (one-way, via `core.SafeResolve`
  in `service.go:14`). `internal/core` imports no docline. A new leaf package importing only the stdlib
  + `gopkg.in/yaml.v3` can be imported by BOTH without any cycle.
- **External docline importers** (blast radius of relocating the codec): `cmd/gen-docs` (low-level
  `docline.Decode`/`Normalize`), `internal/cli/docs.go` and `internal/mcp/docs_tools.go` (high-level
  service API only). Only `gen-docs` touches the low-level codec, so preserving `docline.Decode/Encode`
  as thin re-exports over the leaf package eliminates all external churn.
- **Atomic-write helpers diverge, they are NOT duplicates:** `docline.atomicWrite` uses `os.CreateTemp`
  (random temp name) + `Stat`-then-`Chmod` mode preservation + Windows-only pre-remove, but has **no
  explicit short-write guard**. `core.atomicWriteArchiveFile` uses a fixed `path+".tmp"` + `os.WriteFile`
  at a hard-coded `0644` + Windows-only pre-remove. Unifying them is a *behavior change* (mode
  preservation, temp-naming, short-write guard), not a mechanical move.
- **Prior learnings (confidence: HIGH)** — `docs/compound/best-practices/`:
  - Windows-safe atomic rename: gate the pre-rename `os.Remove(dst)` on `runtime.GOOS == "windows"`
    only (`windows-safe-atomic-rename-goos-gate-2026-04-23.md`).
  - Short-write guard: capture `n` from `f.Write` and treat `n != len(data)` as a hard error before
    `Sync` (`go-file-write-short-write-guard-2026-04-23.md`).
  - File-mode preservation (C1 fix) must not regress when refactoring `atomicWrite`.
- **Verification recipe (reuse from 065-S):** `backlogit docs lint --format json` (0 violations);
  single-file `docs migrate --apply --yes --path <file>` idempotency proof (`git diff` shows no content
  hunks); `go test -count=1 ./internal/docline/... ./internal/core/... ./cmd/gen-docs/...`.

## Options Evaluated

### Option A: Codec-only leaf extraction (defer atomic-write unification)

Create a leaf package (working name `internal/mdfront`) holding the body-preserving codec: the
`Markdown` type, `Decode`, `Encode`, and the fence helpers. `internal/docline` re-exports
`Decode`/`Encode`/`Markdown` over the leaf package (type alias + forwarding) so `gen-docs` is
untouched; `internal/core/doctor.go` rewrites `rewriteArchivedFromField` to call the leaf
`Decode`/`Encode`. Leave both atomic-write helpers exactly where they are.

- **Pros:** smallest blast radius; targets the *actual* divergence-risk surface (the byte-identical
  codec); zero change to either write path → near-zero behavior-change risk; cleanly ≤2h per task.
- **Cons:** does not fully satisfy the stash text, which names "codec **and** atomic-write helper";
  leaves two atomic-write implementations (one without a short-write guard).
- **Effort:** low. **Fit:** high on risk/scope, partial on stash scope.

### Option B: Codec + unified hardened atomic-write leaf extraction (full discharge)

Option A plus a single hardened `WriteFileAtomic` in the leaf package adopting the *superior* union of
both helpers: `CreateTemp` + mode preservation + **short-write guard** + Windows GOOS-gated rename.
Migrate `docline.atomicWrite` and `core.atomicWriteArchiveFile` to call it; delete both local copies.

- **Pros:** fully discharges 8863C6C8 (codec + atomic-write); removes ALL duplication; strictly
  improves `core`'s archive write (adds the missing short-write guard, mode preservation, collision-safe
  temp naming); one audited atomic writer for both paths.
- **Cons:** changes `core`'s archive-write behavior (0644-fixed → mode-preserving; `.tmp` → CreateTemp)
  — a real, if low-risk, behavior change that MUST be pinned by characterization tests; slightly larger
  blast radius (one extra task + a behavior-equivalence proof).
- **Effort:** medium. **Fit:** high on stash scope; risk managed by characterization-first protocol.

### Option C: Move the codec, update all call sites (no re-export shim)

Like Option A/B but relocate `Decode/Encode/Markdown` out of `docline` and update every caller
(including `gen-docs`) to import the leaf package directly; drop the docline re-export.

- **Pros:** one canonical home, no shim indirection.
- **Cons:** churns `gen-docs` + docline-internal call sites for no functional gain; larger diff, more
  review surface, higher chance of an import-path mistake; breaks anyone pinned to `docline.Decode`.
- **Effort:** medium-high. **Fit:** low (churn without value; violates least-change).

## Trade-off Comparison

| Criterion | Option A | Option B | Option C |
|---|---|---|---|
| Eliminates codec duplication | Yes | Yes | Yes |
| Eliminates atomic-write divergence | No | Yes | No |
| Behavior-change risk | Near-zero | Low (test-pinned) | Near-zero |
| Blast radius / diff size | Smallest | Small-medium | Largest |
| Satisfies stash 8863C6C8 fully | Partial | Yes | Partial |
| External call-site churn (gen-docs) | None | None | High |
| Per-task ≤2h decomposition | Clean | Clean | Strained |

## Decision

**Chosen: Option B — extract a leaf package containing the body-preserving codec AND a single hardened
atomic-write helper, with `docline` keeping its public `Decode`/`Encode`/`Markdown` API via re-export.**

Rationale: Option B fully discharges the stash's stated scope (codec + atomic-write helper) and retires
ALL of the duplication, while the only real risk — changing `core`'s archive-write semantics — is
narrow, strictly an improvement (adds the short-write guard and mode preservation core currently
lacks), and fully pinned by characterization tests that assert byte-identical output and preserved file
mode. Keeping `docline`'s public codec API as a thin re-export shim means `gen-docs` and the docline
service callers see no change, holding the diff small and review-friendly. The decomposition is clean:
a leaf package + tests, then two mechanical consumer migrations, each ≤2h, each independently
verifiable, characterization-first so behavior equivalence is proven before the duplicate copies are
deleted.

The leaf-package name is an implementation detail for `impl-plan` (candidates: `internal/mdfront`,
`internal/frontmatter`, `internal/mdcodec`); the binding decision is "a stdlib+yaml-only leaf package
both `docline` and `core` import, with `docline` re-exporting its existing public codec symbols."

## Rejected Alternatives

- **Option A** (codec-only): leaves the second atomic-write implementation and the missing short-write
  guard in `core`; does not fully close 8863C6C8. Retained as the fallback if plan-review judges the
  atomic-write unification too risky to bundle — the codec-only subset is independently shippable.
- **Option C** (move + update all call sites): pure churn over the re-export shim, larger review
  surface, breaks external `docline.Decode` callers for no functional gain. Violates least-change.

## Unresolved Questions

- Final leaf-package name (deferred to `impl-plan`; a bikeshed, not a blocker).
- Whether the unified `WriteFileAtomic` should additionally `fsync` the parent directory. Current repo
  atomic writers do not; out of scope unless plan-harden flags durability as in-scope. Default: match
  existing behavior (no new dir-fsync) to keep the change behavior-preserving.

## Risks and Mitigations

- **Risk:** unifying atomic-write silently changes `core` archive-write semantics (mode, temp naming).
  **Mitigation:** characterization test pinning byte-identical content + preserved mode for the
  `archived_from` repair path before deleting `atomicWriteArchiveFile`; keep the Windows GOOS gate.
- **Risk:** relocating the codec breaks `gen-docs` / docline service callers.
  **Mitigation:** `docline` retains `Decode`/`Encode`/`Markdown` as re-exports; run
  `go test ./cmd/gen-docs/... ./internal/docline/...` as a gate.
- **Risk:** subtle fence/CRLF regression in the moved codec.
  **Mitigation:** move the existing `codec_test.go` cases into the leaf package verbatim and add a
  cross-package characterization test asserting `docline.Decode == mdfront.Decode` on a corpus.
- **Risk:** scope creep into L2/L4 docline hardening.
  **Mitigation:** explicit out-of-scope boundary above; those remain separate active stash entries.
