---
chunk_strategy: h1-h2-h3
description: Implementation plan to extract the body-preserving frontmatter codec into the internal/mdfront leaf package and a hardened atomic-write helper into the internal/atomicfile leaf package, retiring the internal/docline <-> internal/core duplication
doc_type: plan
ingested_at: "2026-06-28T05:45:29Z"
schema_version: "1.0"
source: docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md
title: 'Shared frontmatter codec extraction: internal/mdfront + internal/atomicfile leaf packages'
---

## Source

- Deliberation: `docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md` (Decision: Option B)
- Stash: `8863C6C8`
- Prior art: `docs/closure/2026-06-26-archived-from-migration-closure.md`, `docs/compound/2026-06-26-docline-frontmatter-contract.md`

## Problem Frame (technical)

Two copies of a body-preserving Markdown frontmatter codec exist because `internal/docline` imports
`internal/core` (via `core.SafeResolve`), so `core` cannot import `docline`:

- `internal/docline/codec.go`: `Markdown` type, `Decode`, `Encode`, `openingFenceLen`, `splitAtClosingFence`.
- `internal/core/doctor.go`: `frontmatterOpenLen` (≡`openingFenceLen`), `splitAtFrontmatterFence`
  (≡`splitAtClosingFence`), and a re-inlined decode/encode inside `rewriteArchivedFromField`.

Two divergent atomic-write helpers also exist:

- `internal/docline/service.go` `atomicWrite`: `os.CreateTemp` + `Stat`/`Chmod` mode preservation +
  Windows GOOS-gated rename; **no explicit short-write guard**.
- `internal/core/doctor.go` `atomicWriteArchiveFile`: fixed `path+".tmp"` + `os.WriteFile(0644)` +
  Windows GOOS-gated rename.

Fix: introduce **two** stdlib-only leaf packages (a plan-review refinement of the deliberation's
"leaf package", separating two distinct concerns for cohesion — see Decisions):

- `internal/mdfront` (`bytes`+`fmt`+`yaml.v3` only) — the single canonical body-preserving codec.
- `internal/atomicfile` (`fmt`+`io`+`os`+`path/filepath`+`runtime` only) — the single hardened atomic
  file writer (a generic filesystem primitive, NOT markdown-specific; siblings exist in
  `internal/telemetry/checkpoint.go` and `internal/core/archive.go`, deliberately left untouched here).

Both `docline` and `core` import both leaf packages. `docline` keeps `Decode`/`Encode`/`Markdown`
exported as thin re-exports so `cmd/gen-docs` (the only external low-level codec caller) is untouched.
Names are adjustable (`internal/frontmatter`/`internal/mdcodec`; `internal/fsutil`) but the binding
decision is two cohesive leaf packages, not one mixed-concern package.

## Requirements Trace

| Source requirement | Implementation action | Unit |
|---|---|---|
| One body-preserving codec imported by both packages | Create `internal/mdfront` codec; migrate both consumers | U1, U3, U4 |
| Atomic-write helper shared (stash scope) | Add hardened `WriteFileAtomic` to `internal/atomicfile`; migrate both write sites | U2, U3, U4 |
| No import cycle | Both leaf packages import only stdlib (+ `yaml.v3` for the codec) | U1, U2 |
| Preserve `docline` public codec API (gen-docs) | `docline.Decode` + the `Markdown` alias (which preserves the `(*Markdown).Encode()` method) become re-exports | U3 |
| Behavior preserved on body/fence/encode + on `docline` write path | Characterization tests + existing suites; `git diff` no content hunks | U1, U3, U4 |
| `core` archive-write mode/temp-naming is an INTENDED change, pinned | Characterization-pin `archived_from` self-ref repair byte output AND mode policy before swap | U4 |
| docline lint + CLI-Reference-Drift gate stay green | `backlogit docs lint`; `go test ./cmd/gen-docs/...` | U3, U4 |
| Fence-less / malformed record must NOT panic or synthesize frontmatter (F1) | `mdfront.Decode` nil-map + `HasFrontmatter` guard; explicit error→skip parity test | U1, U4 |

## Implementation Units

### U1 — Create `internal/mdfront` codec (characterization-first)

- **Changes:** new leaf package with the body-preserving codec, behavior-identical to
  `internal/docline/codec.go`.
- **Files:** `internal/mdfront/codec.go` (new), `internal/mdfront/codec_test.go` (new),
  `internal/mdfront/doc.go` (new, package doc).
- **Functions/types:** `Markdown` struct (`HasFrontmatter`, `Frontmatter map[string]any`, `Body []byte`);
  `Decode([]byte) (*Markdown, error)`; `(*Markdown) Encode() ([]byte, error)`; unexported
  `openingFenceLen`, `splitAtClosingFence`.
- **`doc.go` ownership note (P3 F10):** state package ownership explicitly so `mdfront` is not mistaken
  for a third frontmatter API: `mdfront` = byte-preserving RAW markdown/frontmatter codec;
  `internal/models` = artifact materialization / string serialization; `internal/parser` =
  higher-level file-to-artifact adapters. New low-level callers use `mdfront`, not `docline`.
- **Imports:** `bytes`, `fmt`, `gopkg.in/yaml.v3` only (leaf; no internal imports).
- **Tests (characterization — port the FULL `internal/docline/codec_test.go` corpus VERBATIM, F2):**
  port every existing case, and the enumeration MUST explicitly include the highest-value fence
  regression catchers: (a) block-scalar-containing-fence (an indented `---` inside a YAML block scalar
  is NOT the closing fence); (b) horizontal-rule-in-body / body starting with `---` (first real fence
  wins); (c) nested `docline:` map preservation; (d) round-trip idempotence `Encode(Decode(x))`;
  plus the basics (decode with/without frontmatter, CRLF-in-body preserved, deterministic sorted-key
  Encode, empty-frontmatter returns body unchanged). Add a **golden/differential byte-equality test**
  asserting `mdfront.Decode`/`Encode` output is byte-identical to captured PRE-refactor docline output
  over the corpus. (NOTE: "verbatim" scopes to the codec/fence cases; the existing
  `TestModelsArtifactSerialization_Unchanged` case imports `internal/models` and stays in
  `internal/docline` — it does not belong in this leaf package.)
- **Posture:** characterization-first. **Milestone:** `go test ./internal/mdfront/...` green; package
  compiles with no internal imports; golden bytes equal pre-refactor output.
- **2-hour check:** 3 files, 4 funcs + 1 type. Test count exceeds the 4-scenario heuristic because the
  port is mechanical (copy existing table cases); if it strains 2h, the test port may be split from the
  package scaffold into a sibling task without changing the design.

### U2 — Add hardened `WriteFileAtomic` to `internal/atomicfile` (test-first)

- **Changes:** one audited atomic writer in its OWN leaf package (cohesion fix F4) adopting the superior
  union of both existing helpers.
- **Files:** `internal/atomicfile/atomicfile.go` (new), `internal/atomicfile/atomicfile_test.go` (new),
  `internal/atomicfile/doc.go` (new).
- **Function:** `WriteFileAtomic(path string, data []byte) error` — `os.CreateTemp(dir, ...)` → write
  the data through an **`io.Writer` seam** (F5) with a **short-write guard** (`n != len(data)` is a hard
  error) → `Chmod` to a **clamped** mode: the existing file's mode with group/world write bits stripped
  (`perm &^ 0o022`), or `0644` for a new file (mode policy F9 — preserve, but never perpetuate an
  over-permissive 0666/0777 record) → `Close` → `os.Rename`, with the Windows-only
  (`runtime.GOOS == "windows"`) pre-remove-and-retry fallback. Temp file removed on any failure.
  **Deliberately Sync-free** (F10): rename gives atomic visibility; git is the durability/rollback
  mechanism for docs and archive records — document this in `doc.go`, do NOT add fsync (out of scope).
- **`doc.go` contract (F10/security):** `WriteFileAtomic` is path-agnostic and performs NO path
  containment — callers MUST pass a path already validated via `core.SafeResolve` (or equivalent). The
  temp prefix is NOT `*.md` so docline/doctor `*.md` scans cannot pick it up mid-write.
- **Imports:** `fmt`, `io`, `os`, `path/filepath`, `runtime`.
- **Tests (test-first):** (1) overwrite existing file → content byte-identical AND mode preserved
  (0600 source stays 0600); (2) create new file → exists at `0644` with exact content; (3) short-write
  surfaced as an error — driven deterministically via the `io.Writer` seam with a short-writing fake
  (this is the unit's NEW behavior and MUST be falsifiably tested); (4) over-permissive source (0666)
  is written back at the clamped mode (`0644`), not perpetuated.
- **Posture:** test-first. **Milestone:** `go test ./internal/atomicfile/...` green; the short-write
  branch is observed failing (red) before the guard is added.
- **Depends on:** none (independent leaf package). **2-hour check:** 3 files, 1 func, 4 scenarios. OK.

### U3 — Migrate `internal/docline` to consume the leaf packages (characterization-first)

- **Changes:** replace docline's local codec with re-exports over `mdfront`; delegate `atomicWrite`
  to `atomicfile.WriteFileAtomic`; delete the now-duplicate local fence helpers + atomic writer.
- **Files:** `internal/docline/codec.go` (replace impl with `type Markdown = mdfront.Markdown` — a TRUE
  alias — and a single forwarding `func Decode(raw []byte) (*Markdown, error) { return mdfront.Decode(raw) }`;
  remove local `openingFenceLen`/`splitAtClosingFence`); `internal/docline/service.go` (replace
  `atomicWrite` body with a call to `atomicfile.WriteFileAtomic`, or delete `atomicWrite` and call it
  directly at the call site).
- **CORRECTION (F8):** `Encode` is **inherited via the alias** and MUST NOT be re-declared in `docline`
  (Go forbids declaring a method on a non-local alias target — it will not compile). Forward ONLY the
  package-level `Decode` function. `normalize.go` (`out.Encode()`) and `codec_test.go` keep working
  through the inherited method automatically.
- **Path-guard checklist (F10/security):** confirm `docline.ValidateApplyPath` + `core.SafeResolve`
  in the apply preflight (`service.go`) are UNTOUCHED — the containment chain must not be dropped when
  swapping the writer.
- **Error-context note (F10):** verify no test/log/caller matches the old `docline.Decode:`/`atomicWrite`
  error prefixes; if any does, wrap the leaf error with docline-qualified context to keep traces stable.
- **Preserve:** the `docline` public codec surface — `docline.Decode` (package function) and
  `docline.Markdown` with its inherited `(*Markdown).Encode()` method — stays exported, behavior-identical.
- **Tests:** keep existing `internal/docline/codec_test.go` + `service_test.go` as the regression net
  (they now exercise the re-export); they must stay green unchanged.
- **Posture:** characterization-first (existing docline tests pin behavior across the swap).
- **Milestone:** `go test ./internal/docline/... ./cmd/gen-docs/...` green; `go build ./...` clean;
  `backlogit docs lint` 0 violations; single-file `docs migrate --apply` on a sample doc is a
  byte-identical no-op (`git diff` shows no content hunks).
- **Depends on:** U1, U2. **2-hour check:** 2 files changed, Decode→forward + Markdown alias + atomicWrite
  delegate. OK.

### U4 — Migrate `internal/core/doctor.go` to consume the leaf packages (characterization-first)

- **Changes:** rewrite `rewriteArchivedFromField` to decode/encode via `mdfront`; replace the
  `atomicWriteArchiveFile` call with `atomicfile.WriteFileAtomic`; delete `frontmatterOpenLen`,
  `splitAtFrontmatterFence`, and `atomicWriteArchiveFile`.
- **CRITICAL guard (F1 — P1):** the current `rewriteArchivedFromField` returns a HARD ERROR on a missing
  opening/closing fence (caller then SKIPS the record). `mdfront.Decode` instead returns
  `HasFrontmatter=false` with a **nil** `Frontmatter` map and **no error**. The rewrite MUST therefore
  add an explicit `if !md.HasFrontmatter { return nil, fmt.Errorf("no frontmatter fence") }` guard AND
  guarantee a non-nil map before `fm["archived_from"] = newValue` — otherwise a fence-less record
  panics on the nil-map assignment OR gets a synthetic frontmatter block wrapping the whole document
  (corruption) written atomically. This guard MUST exist and be tested BEFORE the local helpers are
  deleted.
- **Files:** `internal/core/doctor.go` (rewrite 1 func, delete 3 helpers); `internal/core/doctor_test.go`
  (or a new `internal/core/archived_from_codec_test.go`) — characterization tests.
- **Tests (characterization, pin BEFORE swapping impl):**
  (1) `rewriteArchivedFromField` on a SELF-REFERENTIAL record rewrites ONLY the `archived_from` field,
  body bytes byte-identical (CRLF/trailing-ws/horizontal-rule preserved), sorted-key frontmatter,
  output byte-identical to captured PRE-refactor bytes;
  (2) **malformed/fence-less record** (no opening fence and no closing fence) → returns an ERROR
  (asserting error→skip parity with current behavior, NOT a panic, NOT a synthetic-frontmatter write);
  (3) the archive-write path preserves the target file's existing mode (clamped) and writes
  byte-identical content; add a read-only-destination case for the Windows rename fallback.
- **Runtime verification fixture (F3 — P1):** end-to-end `--fix-archived-from` MUST use a
  **SELF-REFERENTIAL** `archived_from` record (the ONLY auto-repaired class; ~130 in the 067-S census),
  asserting byte-identical repaired output + preserved mode. Keep a SEPARATE assertion that a
  **malformed** record (e.g. `038-DL`/`039-DL`, value `done`) is left byte-UNCHANGED (flag-only) — this
  preserves the explicit 067-S operator decision that malformed records are never auto-repaired.
- **Out of scope (F7):** `scanArchivedFrom` (the READ-only detection path) keeps using
  `models.ParseFrontmatter` — it extracts a field value, a legitimately different contract from the
  body-preserving WRITE path; consolidating detection+repair onto one codec is a documented follow-up,
  not this unit.
- **Posture:** characterization-first — this changes `core`'s write semantics (mode clamp, CreateTemp),
  so byte-equivalence + mode policy must be proven before deleting the local helpers. "Behavior
  preserved" here scopes to body/fence/encode + the error→skip contract; mode/temp-naming is an
  INTENDED, pinned change.
- **Milestone:** `go test ./internal/core/...` green; `backlogit doctor --check-archived-from` and
  `--fix-archived-from` behave identically on the self-referential + malformed fixtures; `go vet` +
  `gofmt` clean.
- **Depends on:** U1, U2. **2-hour check:** 2 files, 1 func rewritten + 3 deleted, 3 scenarios. OK.

## Dependency Graph

```
U1 (internal/mdfront codec)      U2 (internal/atomicfile writer)
        │                                 │
        └────────────┬────────────────────┘
                     ├─> U3 (docline migration: consumes mdfront + atomicfile)
                     └─> U4 (core/doctor migration: consumes mdfront + atomicfile)
```

- U1 and U2 are **independent leaf packages** with no dependencies and no dependency on each other —
  they are fully parallelizable.
- U3 depends on U1 + U2. U4 depends on U1 + U2. U3 and U4 are independent of each other.
- No cycles.

Suggested execution order: {U1 ∥ U2} → {U3 ∥ U4}.

## Decisions and Rationale

- **Two cohesive leaf packages (not one mixed-concern package):** `internal/mdfront` owns the
  byte-preserving codec; `internal/atomicfile` owns the generic atomic file writer. They address two
  distinct concerns (markdown parsing vs. filesystem durability) and have different stdlib import sets;
  fusing them would create a package that is both a YAML codec and an fs primitive. The atomic writer is
  NOT markdown-specific (siblings exist in `telemetry/checkpoint.go` and `core/archive.go`).
- **Leaf package, re-export shim (not move-all-callers):** `docline` keeps its public codec symbols as
  re-exports so `gen-docs` and the docline service callers are untouched — smallest diff, least risk.
  `Markdown` becomes a TRUE alias (`type Markdown = mdfront.Markdown`); `Encode` is inherited via the
  alias and is NOT re-declared (Go forbids a method on a non-local alias target — F8); only the
  package-level `Decode` is forwarded.
- **Unify atomic-write onto the hardened union, with a mode CLAMP:** adopt `CreateTemp` + an `io.Writer`
  seam + short-write guard + mode preservation **clamped to strip group/world write bits** (`perm &^ 0o022`)
  + Windows GOOS gate. This strictly improves `core` (which lacked the short-write guard and mode
  preservation) WITHOUT perpetuating an over-permissive 0666/0777 source mode (F9). The `io.Writer` seam
  (F5) makes the short-write branch falsifiably testable (a real `os.File.Write` won't short-write).
- **Sync-free by design (F10):** the writer deliberately does NOT fsync — `os.Rename` provides atomic
  visibility, and git is the durability/rollback mechanism for docs + archive records. Documented in
  `internal/atomicfile/doc.go`; adding fsync is explicitly out of scope.
- **Path containment stays with the caller (F10/security):** `atomicfile.WriteFileAtomic` is path-agnostic
  and performs no containment; callers MUST pre-validate via `core.SafeResolve`. The docline apply
  preflight (`ValidateApplyPath` + `SafeResolve`) is preserved untouched in U3.
- **F1 nil-map / fence-less guard:** `mdfront.Decode` returns `HasFrontmatter=false` + nil map + no error
  on a missing fence (unlike `core`'s current hard error). U4's `rewriteArchivedFromField` MUST add an
  explicit `if !md.HasFrontmatter { return error }` guard + non-nil-map guard to preserve the error→skip
  contract and avoid a nil-map panic or synthetic-frontmatter corruption.
- **Characterization-first across all migration units:** behavior equivalence is the acceptance bar;
  existing test corpora are ported verbatim (U1) and retained (U3), and a golden self-referential fixture
  pins the doctor repair path (U4).
- **Names deferred to implementation:** `internal/mdfront` / `internal/atomicfile` are working names
  (`internal/frontmatter`/`internal/mdcodec`; `internal/fsutil` acceptable). Not a blocker.

## Risks and Caveats

- **Subtle codec regression** corrupting frontmatter across docs or archive records.
  *Mitigation:* port the existing `codec_test.go` corpus verbatim into `mdfront` (U1) + a
  golden/differential byte-equality assertion vs pre-refactor output; retain docline tests over the
  re-export (U3); golden-pin the doctor path (U4).
- **F1 nil-map panic / synthetic-frontmatter corruption** if `core`'s hard-error-on-missing-fence
  contract is dropped when swapping to `mdfront.Decode`.
  *Mitigation:* explicit `HasFrontmatter` + non-nil-map guard in U4, tested with a fence-less record
  BEFORE the local helpers are deleted (error→skip parity, not panic, not synthetic write).
- **F3 wrong-fixture false confidence:** validating repair on a malformed record would assert the wrong
  contract (malformed = flag-only/never repaired per the 067-S operator decision).
  *Mitigation:* the runtime fixture uses a SELF-REFERENTIAL record (the only auto-repaired class); a
  separate assertion pins malformed records as byte-UNCHANGED.
- **`core` atomic-write behavior change** (mode clamp/temp naming) is INTENDED, not incidental.
  *Mitigation:* characterization test asserting byte-identical content + clamped-mode policy (incl. an
  over-permissive-source case) before deleting `atomicWriteArchiveFile` (U4); keep the Windows GOOS gate.
- **`gen-docs` breakage** from relocating the codec.
  *Mitigation:* re-export `docline.Decode/Encode/Markdown`; gate on `go test ./cmd/gen-docs/...`.
- **Other in-tree atomic writers stay divergent (F7):** `telemetry/checkpoint.go` and `core/archive.go`
  keep their own writers; `scanArchivedFrom`/`models.ParseFrontmatter` (READ-only detection) keeps its
  own field-extraction contract. *Mitigation:* explicit out-of-scope; documented follow-up, not this unit.
- **Scope creep** into docline L2/L4 hardening. *Mitigation:* explicit out-of-scope; those stay active
  stash entries (`B349CBED`, `AE53BC5C`).

## Runtime Verification and Closure

Runtime surfaces touched (indirectly, behavior-preserving except where pinned as intended):

- `backlogit docs migrate --apply` (docline write path) — verify: dry-run reports `body_bytes_changed=0`;
  single-file `--apply` is a byte-identical no-op on an already-compliant doc (`git diff` no content hunks).
- `backlogit doctor --fix-archived-from` (core archive-repair write path) — verify on TWO fixtures:
  (a) a **SELF-REFERENTIAL** `archived_from` record (the only auto-repaired class) → repaired record is
  byte-identical to the pre-refactor output and file mode is preserved (clamped); (b) a **malformed**
  record (e.g. `038-DL`/`039-DL`, value `done`) → left byte-UNCHANGED (flag-only), confirming the 067-S
  never-auto-repair-malformed decision is intact.

Aggregate gates before PR: `go build ./...`, `go vet ./...`, `gofmt -l` clean, `golangci-lint run`,
`go test -count=1 ./internal/mdfront/... ./internal/atomicfile/... ./internal/docline/... ./internal/core/... ./cmd/gen-docs/...`,
`backlogit docs lint --format json` (0 violations).

Closure expectation: this is an internal refactor with no operator-facing surface change; rollback =
revert the PR. No monitoring window or runbook needed beyond the standard post-merge closure note.

## Plan Hardening Signals

- **Public API / schema / contract change:** ABSENT. `internal/mdfront` is new INTERNAL API. `docline`'s
  public codec API is preserved via re-export. No CLI/MCP/schema/contract surface changes.
- **Security / auth / permission / compliance:** ABSENT. The atomic-write change *preserves* file mode
  and additionally CLAMPS group/world write bits (`perm &^ 0o022`) — strictly tighter than `core`'s fixed
  `0644` and never looser than the source; no auth/permission semantics change. Path containment stays
  with callers via `core.SafeResolve` (preserved untouched).
- **Migration / backfill / destructive / irreversible:** ABSENT. No data migration or backfill. The
  doctor archived_from repair behavior is preserved and characterization-pinned; writes remain atomic
  (temp+rename) and reversible (revert the PR). No destructive step.
- **External integration / operator checkpoint / external dependency:** ABSENT. No new dependency
  (`yaml.v3` already vendored); no external integration.
- **High runtime / rollout / rollback risk:** ABSENT. Behavior-preserving refactor; characterization
  tests + ported corpora + existing suites are the safety net; rollback is a single PR revert; the two
  touched write paths are low-frequency and atomic.

Requires plan hardening: no

## Plan Review

<!-- plan-review-attempt: 2 -->

### Cycle 1 — verdict: FAIL (revised, re-review pending)

Six reviewer personas (Constitution, Go, Scope-Boundary, Architecture-Strategist, Security-Lens,
Learnings-Researcher) reviewed the initial plan. Verdict: **FAIL** on 3 P1 findings; 7 P2/P3 folded in.
The plan above is the revised version that closes them.

**P1 (blocking) — all addressed in the revision:**

- **F1 — nil-map panic / synthetic-frontmatter corruption (Go).** `core`'s current
  `rewriteArchivedFromField` returns a HARD ERROR on a missing fence (caller skips). `mdfront.Decode`
  returns `HasFrontmatter=false` + nil map + no error. A naive swap panics on `fm[...] = v` or writes a
  synthetic frontmatter block. *Resolution:* U4 adds an explicit `HasFrontmatter` + non-nil-map guard +
  fence-less error→skip parity test BEFORE deleting local helpers. Requirements Trace + Risks + Decisions
  updated.
- **F2 — corpus gap (Go + Learnings).** U1 originally enumerated only 4 scenarios; the canonical codec is
  protected by a richer corpus (block-scalar-containing-fence, HR-in-body first-fence-wins, nested
  `docline:` map, round-trip idempotence). *Resolution:* U1 now ports the FULL `codec_test.go` corpus
  verbatim + a golden/differential byte-equality assertion vs pre-refactor output.
- **F3 — wrong runtime fixture (Learnings + Architecture).** U4 originally validated repair on a
  "malformed" record, but malformed records are flag-only/never auto-repaired (067-S operator decision);
  only SELF-REFERENTIAL records are repaired. *Resolution:* U4 runtime fixture is now self-referential,
  with a separate assertion that malformed stays byte-unchanged.

**P2/P3 (folded into the revision):**

- **F4 (Architecture):** split the atomic writer into its own `internal/atomicfile` leaf package (cohesion;
  not markdown-specific). → Done: two leaf packages; Dependency Graph + Decisions updated.
- **F5 (Go):** add an `io.Writer` seam so the short-write guard is falsifiably testable. → Done in U2.
- **F6 (Scope):** reframe "zero behavior change" — `core`'s mode/temp change IS intended; assert vs
  PRE-refactor bytes. → Done: Requirements Trace + U4 posture + Risks reframed as INTENDED/pinned.
- **F7 (Scope):** explicit out-of-scope for other atomic writers + `scanArchivedFrom`/`ParseFrontmatter`.
  → Done in U4 out-of-scope + Risks.
- **F8 (Go):** Go forbids a method on a non-local alias target — `Encode` is inherited via
  `type Markdown = mdfront.Markdown`; forward only `Decode`. → Done: U3 corrected, Decisions updated.
- **F9 (Security-Lens):** clamp preserved mode (`perm &^ 0o022`) to avoid perpetuating over-permissive
  source modes. → Done in U2 + Plan Hardening security signal.
- **F10 (Security-Lens + Architecture):** `doc.go` ownership notes (mdfront vs models vs parser;
  atomicfile path-agnostic / caller-validates-path; Sync-free-by-design). → Done in U1 + U2 doc.go.

Re-review (cycle 2) targets the two P1 sources (Go Reviewer + Learnings Researcher) to confirm F1/F2/F3
closure before harvest.

### Cycle 2 — verdict: PASS

Focused re-review by the two P1-source personas (Go Reviewer + Learnings Researcher).

- **Go Reviewer: PASS.** F1 CLOSED (HasFrontmatter + non-nil-map guard + fence-less error→skip test
  before helper deletion; consistent with verified `mdfront.Decode` contract). F2 CLOSED (full corpus
  port maps 1:1 onto real `codec_test.go` cases; captured golden bytes avoid a test-time
  `docline→mdfront` cycle). F3 CLOSED (self-referential repair fixture matches
  `repairArchivedFrom` `doctor.go:468-470` `continue`-on-non-self-ref). F8 CLOSED (alias-inherited
  `Encode`, forward only `Decode` — legal Go; consumers compile through the inherited method).
  Mode-clamp `perm &^ 0o022` confirmed correct (0o666→0o644, 0o777→0o755, 0o600 unchanged).
  io.Writer short-write seam confirmed falsifiable. **No new P1 blockers.**
- **Learnings Researcher: PASS.** F3 + F2 confirmed faithful to prior art
  (`docs/closure/2026-06-26-archived-from-migration-closure.md`: 130 self-referential repaired, 2
  malformed `038-DL`/`039-DL` flag-only by operator decision;
  `docs/compound/2026-06-26-docline-frontmatter-contract.md`: body-preserving + idempotent contract).
  **No factual contradictions**; the "~130 census" count matches the closure doc exactly.
- **Non-blocking advisory (folded):** a strictly verbatim corpus port would pull
  `TestModelsArtifactSerialization_Unchanged` (imports `internal/models`) into the leaf — that case
  belongs in `docline`, not `mdfront`. U1's "port verbatim" scopes to the codec/fence cases; the
  models-serialization case stays in `internal/docline`. Placement nit only — does not block.

**Gate result: PASS at attempt 2 (1 re-entry cycle). Proceeding to harvest.**
