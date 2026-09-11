---
chunk_strategy: h1-h2-h3
description: "Executable harness contract supplement that unblocks shipment 140-S / feature 158-F by making the S6 seq3 task contracts concrete without changing product scope"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-10-s6-140s-harness-contract-supplement.md
supersedes_review_of: docs/exec-plans/2026-09-03-s6-seq3-compat-corpus-plan.md
title: "S6 Seq3 — Executable Harness Contract Supplement (unblocks 140-S)"
---

# S6 Seq3 — Executable Harness Contract Supplement

**Purpose.** Make the eleven S6/seq3 task contracts (`158.001-T`..`158.011-T`,
after the P-002.1/P-002.6 declaration/behavior split — see §Declaration/Behavior
Split (attempt-5 amendment)) executable by the harness-architect **without
expanding product scope**. The
governing plan
(`docs/exec-plans/2026-09-03-s6-seq3-compat-corpus-plan.md`) remains the scope
authority; this supplement adds only the concrete API surfaces, fixture schemas,
diagnostic IDs, analyzer boundaries, RED/GREEN selectors, dependency ordering,
and check/build integration ownership that the prior plan review found missing.
It authors **no production or test code** — it declares the contract the RED
harness will encode.

**Blocker resolved.** The harness-architect halted (`gate_token:
HARNESS_CONTRACT_UNDERSPECIFIED`) because the governing plan was decision `FAIL`
and the task descriptions lacked executable detail. Inventing these details
during Ship would violate P-002/P-004. This supplement supplies them so Ship can
generate RED tests and resume on the existing feature branch.

**Boundary.** This is Stage-owned planning. It does NOT modify the `158.*-T`
backlog artifacts, shipment `140-S` membership/status, or any task status. Ship
consumes this document by task ID. The wave/dependency ordering declared here is
a **planning declaration for harness generation**, not a backlog dependency-edge
mutation.

## Prior-Review Findings This Supplement Resolves

The governing plan's `## Plan Review` attempt-2 record (decision `FAIL`) carried
these controlling P1 findings. Each is resolved below and cross-referenced.

| # | Finding | Resolution |
|---|---------|-----------|
| P1-a | Fuzzing advertised but no target/seed/budget/unit. | Governing plan added `U-fuzz` + `158.008-T`; this supplement pins the exact fuzz API, canonical seed location, budget, and a **valid** seed-regression RED (see §158.008-T). |
| P1-b | `success-after-audit-warning` and `uncancellable-lock timeout` analyzers likely need CFG/data-flow/SSA. | Both re-scoped to **AST + type-info, intra-block, declaration-driven allowlists** with conservative under-approximation and explicit exclusions — **no SSA/CFG/dataflow** (see §158.006-T, §158.007-T, §Analyzer Framework). |
| P1-c | Analyzer source/sink and wrap-boundary definitions missing (noise/unsafe risk). | Every analyzer declares source, sink, AST/type boundary, and false-positive exclusions explicitly. |
| P1-d | Runner output path into the S4-U4 evidence contract and S10 DAG unstated. | Runner emits a stable-sorted `Report` JSON (schema below). Consumption by the S10 evidence DAG is S10's node-executor responsibility and is **explicitly out of 140-S scope**; 140-S guarantees only the stable output surface (see §Corpus Runner Output Contract). |

## Second-Pass Review Findings Resolved (attempt-3 remediation)

The attempt-3 multi-agent review of this supplement surfaced four additional
blocking issues that are now fixed inline; recorded here for auditability.

| # | Finding | Resolution |
|---|---------|-----------|
| R3-1 | Declared corpus outcomes (duplicate-key, case-folded-key, unclosed-frontmatter, bad-path) are **unreachable** through raw `encoding/json`/`yaml.v3`/`mdfront`, which do not reject them. | Adapters are re-declared as **decode-under-test wrappers** that combine the real decoder with the corpus's own **declared strict-validation checks** for each fault class, returning **adapter-defined sentinel errors** (see §158.001-T). No core parser is changed. |
| R3-2 | The fuzz RED selector (`go test -run FuzzX`) passes on an empty `-run` match and does not prove committed native seeds. | RED is now a normal unit test `TestFuzzSeedCorpusCommitted` that asserts the native seed files exist and replays them deterministically; `-fuzz` retained only for bounded execution (see §158.008-T). |
| R3-3 | Analyzer tasks share a hidden intra-wave dependency (need `golang.org/x/tools` + scaffolding first), but the wave partition listed them as flat-parallel. | `158.003-T` is promoted to a **wave-1 prerequisite** owning the pinned `x/tools` dependency + scaffolding; analyzer tasks `158.004-T`..`158.007-T` declare a harness-ordering dependency on it (see §Wave Partition, §Shared Analyzer Scaffolding). |
| R3-4 | The `check` Makefile recipe was a non-executable placeholder; analysistest `testdata/src` fixtures are not multichecker-runnable. | `check` is concretely defined as `go test ./internal/faultline/analyzer/...` plus a build of the multichecker binary; repo-wide `./...` enforcement deferred (see §Shared Analyzer Scaffolding). |

Advisory (P2) refinements also applied: pinned `x/tools` version; dropped the
unused `DecodeResult.Warnings` field; pinned the lock-contention fixture to a
single deterministic outcome; honest acknowledgement of the one shared
`main.go` enumeration edit with pre-reserved sorted slots; reserved per-task
Wave-2 helper identifiers.

## Declaration/Behavior Split (attempt-5 amendment)

**Governing finding (P-002.1/P-002.6).** Test-first RED→GREEN does not permit an
in-task two-stage harness: a single task may not bundle *new package/symbol
declarations* with a *behavior harness* that cannot compile until those
declarations exist. Two original units bundled exactly that and are split by this
Stage manifest amendment (only a Stage amendment may change the frozen work
graph):

* **Compat corpus.** `158.001-T` (originally corpus + runner) is converted to a
  **declaration-only prerequisite** owning the real, compilable exported
  declarations of `internal/faultline/compatcorpus` plus a **source-shape go/ast
  harness**. A new behavior task **`158.009-T`** owns `TestCorpusRunner`, the
  runner/adapters, `testdata/corpus` fixtures, sentinel behavior, and
  `compatcorpus.report/v1`, inheriting **all** original `158.001-T` behavior
  acceptance. `158.009-T` depends on `158.001-T`.
* **Analyzer framework / FL001.** `158.003-T` (originally the FL001 analyzer +
  shared scaffolding) is narrowed to the **shared prerequisite** owning the
  minimum analyzer declaration + build scaffolding (x/tools `v0.39.0` direct
  promotion, `cmd/faultline-analyze` skeleton with the FL001 slot filled against
  the declared skeleton `Analyzer` var, the `Makefile` `check` target, and a
  source-shape harness). A new behavior task **`158.010-T`** owns the FL001
  scanner-discipline analyzer **behavior** + `analysistest` fixtures.
  `158.010-T` depends on `158.003-T`.

**Backlog dependency edges (now real, not prose-only).** The declaration/behavior
split promotes the previously prose-only harness ordering into real backlog
edges, and rewires the two runner consumers onto the behavior task that supplies
the implemented `Run`/adapters:

| edge | rationale |
|------|-----------|
| `158.009-T → 158.001-T` | runner behavior needs the corpus declarations |
| `158.010-T → 158.003-T` | FL001 behavior needs the analyzer scaffolding + declaration |
| `158.004-T → 158.003-T` | analyzer needs x/tools + scaffolding to compile analysistest |
| `158.005-T → 158.003-T` | " |
| `158.006-T → 158.003-T` | " |
| `158.007-T → 158.003-T` | " |
| `158.002-T → 158.009-T` | consumes the implemented runner (rewired off `158.001-T`) |
| `158.008-T → 158.009-T` | fuzz reuses the implemented runner/adapters (rewired off `158.001-T`) |
| `158.011-T → 158.004-T` | multichecker wiring needs the FL002 analyzer var |
| `158.011-T → 158.005-T` | needs the FL003 analyzer var |
| `158.011-T → 158.006-T` | needs the FL004 analyzer var |
| `158.011-T → 158.007-T` | needs the FL005 analyzer var |

The two runner consumers (`158.002-T`, `158.008-T`) keep their transitive
ordering after `158.001-T` through `158.009-T`; the direct `→ 158.001-T` edges
are removed because their genuine dependency is the *implemented* runner, not the
declarations alone.

**Sizing/complexity semantics preserved.** Both behavior tasks are strictly
**smaller** than the previously plan-reviewed single units they were carved from
(each original unit passed review as one ≤2h unit; removing the declaration/
scaffolding portion into a prerequisite can only shrink the residual behavior
task). All eleven member tasks remain **unsized** — matching the existing shipment
composition (`unsized`) — so no partial-sizing inconsistency is introduced; this
preserves the workspace's current size/complexity semantics for `140-S`.

## Cross-Cutting Decisions

### Analyzer Framework (applies to 158.003-T..158.007-T)

* **Framework:** `golang.org/x/tools` (`go/analysis` + `analysistest` +
  `multichecker`), promoted to a **direct** dependency and **pinned** to the
  version **already selected by the current module graph**
  (`golang.org/x/tools v0.39.0`, Go 1.24-compatible — its `go.mod` declares
  `go 1.24.0`, matching this module; already present in `go.sum` as a pruned
  transitive requirement; the exact version is locked in `go.mod`/`go.sum` —
  not `@latest`). Reusing the already-selected `v0.39.0` adds the direct pin
  **without upgrading or downgrading any existing dependency chain**, and it
  provides the identical stable `go/analysis` + `analysistest` + `multichecker`
  API surface these analyzers use. This is the conventional, minimal way to
  build and test Go analyzers; reimplementing the analysis driver/loader would
  be strictly larger scope.

  > **Dependency-pin correction (supersedes the original `v0.28.0` mandate).**
  > The original contract mandated an exact direct pin `golang.org/x/tools
  > v0.28.0`. The repository's module graph already MVS-selects
  > `golang.org/x/tools v0.39.0` (verified: `go list -m golang.org/x/tools` →
  > `v0.39.0`; the module is already recorded in `go.sum`). Forcing `v0.28.0`
  > would require an **unreviewed downgrade** of an already-selected chain;
  > retaining `v0.39.0` while the contract said `v0.28.0` would violate the
  > exact pin — this is the `HARNESS_CONTRACT_UNDERSPECIFIED` halt Ship
  > correctly raised. This correction reuses the already-selected `v0.39.0`
  > and authorizes **no** other dependency upgrade or downgrade. `v0.39.0`
  > exposes the same `analysis.Analyzer`, `analysistest.Run`, and
  > `multichecker.Main` API this contract depends on. See §158.003-T for the
  > exact, reproducible dependency operation and verification command.
* **Analysis depth:** AST (`go/ast`) plus type information (`pass.TypesInfo`,
  `pass.Pkg`) **only**. **No SSA, no CFG, no cross-function data-flow.** Where a
  property is not decidable syntactically within a single function/block, the
  analyzer **under-approximates** (may miss a true positive) rather than
  emitting a false positive. This is the deliberate resolution of finding P1-b.
* **Confidence posture:** report only high-confidence, syntactically-decidable
  violations. Every analyzer defines explicit exclusions and honours a
  per-analyzer suppression comment (`// faultline:<rule>-ok`).
* **Test harness:** `analysistest.Run` with `testdata/src/<pkg>/` fixtures using
  `// want "<regexp>"` seeded-violation markers plus a clean fixture with no
  markers.
* **Package layout:** each analyzer is its own subpackage
  `internal/faultline/analyzer/<name>/` exporting `var Analyzer *analysis.Analyzer`.
  **Subpackages import nothing from each other or from a parent registry**
  (decoupled — see scaffolding below).
* **Diagnostic IDs (frozen):** `FL001` Scanner discipline, `FL002` %w wrapping,
  `FL003` fail-open branch, `FL004` success-after-audit-warning, `FL005`
  timeout-vs-uncancellable-lock. Each ID is embedded in the analyzer's `Name`
  and diagnostic message.

### Shared Analyzer Scaffolding & Check-Target Integration (ownership)

Ownership is partitioned to prevent duplicate responsibilities and to make the
one unavoidable shared edit explicit and low-conflict.

* **`158.003-T` is the wave-1 analyzer prerequisite and owns:**
  * The pinned `golang.org/x/tools` addition to `go.mod`/`go.sum` (so every
    analyzer task's `analysistest` compiles from a base checkout).
  * `cmd/faultline-analyze/main.go` — a single `multichecker.Main(...)` call
    that **explicitly enumerates** the analyzers (no reflection, no parent
    `init()` registry). `158.003-T` authors the file with **FL001 wired live**
    and **FL002..FL005 as pre-reserved, alphabetically-sorted COMMENTED
    placeholder slots** (the file compiles referencing FL001 only). The
    FL002..FL005 slots are filled later by the single serialized integration
    task **`158.011-T`** — see below — so `main.go` is never edited by parallel
    wave tasks.
  * The `Makefile` `check` target (concrete, executable — see below) and its
    `.PHONY` entry.
  * Its own FL001 analyzer subpackage **declaration only** (the skeleton
    `var Analyzer *analysis.Analyzer` + compilable no-op run func); the FL001
    scanner-discipline **behavior** and its `analysistest` fixtures are owned by
    `158.010-T` (see §158.010-T). Because the FL001 `Analyzer` var is declared
    here, `158.003-T` also fills the FL001 slot in `main.go` against it.
* **Each analyzer task (`158.004-T`..`158.007-T`) owns ONLY:** its self-contained
  subpackage (`analyzer.go` + `analyzer_test.go` + `testdata/`). It does **not**
  edit `cmd/faultline-analyze/main.go` — that avoids parallel wave-2 tasks
  sharing one Go source file. Each analyzer's acceptance is proven by its own
  `analysistest` (`go test ./internal/faultline/analyzer/<name>/`), independent of
  multichecker wiring.
* **`158.011-T` (single serialized integration owner) owns:** the one `main.go`
  edit that fills the four reserved FL002..FL005 slots with the analyzer vars
  declared by `158.004-T`..`158.007-T`. It carries backlog `blocks` edges on all
  four analyzer tasks, runs in the final wave, and is the **sole** owner of the
  FL002..FL005 wiring. `main.go` is therefore touched only by `158.003-T`
  (wave 1) then `158.011-T` (final wave) — strictly serialized, never concurrent
  (resolves the duplicate/parallel shared-file-ownership finding).
* **Harness-ordering dependency (now a REAL backlog edge after the attempt-5
  declaration/behavior split):** `158.004-T`, `158.005-T`, `158.006-T`,
  `158.007-T`, and `158.010-T` each carry a backlog `blocks` edge on `158.003-T`
  (x/tools in `go.mod` + `cmd` scaffolding + FL001 declaration must land before
  their harness compiles); `158.011-T` carries edges on `158.004-T`..`158.007-T`.
  `158.001-T` has **no** such dependency (the corpus
  needs neither x/tools nor analyzer scaffolding) and runs fully parallel to
  `158.003-T`. The corpus behavior task `158.009-T` depends on `158.001-T`; the
  runner consumers `158.002-T`/`158.008-T` depend on `158.009-T`.
* **`check` target (concrete):**

  ```make
  .PHONY: check
  check:
  	go build ./cmd/faultline-analyze
  	go test ./internal/faultline/analyzer/...
  ```

  This proves each analyzer via its `analysistest` fixtures (the full 140-S
  acceptance surface) and proves the multichecker binary builds. **Repo-wide
  enforcement is deferred (declaration-only, no production edits now):** pointing
  the multichecker at `./...` may surface pre-existing real violations whose
  remediation is production work outside this shipment. Flipping `check` to
  blocking repo-wide enforcement (and remediating any real findings) is an
  explicit follow-up, NOT part of 140-S.

### Harness-First Sequencing

Per task the order is **declare contract (this doc) → RED test (harness) → GREEN
implementation (Ship execution)**. Stage authors no production code or
declaration. Where a later task references a symbol (fuzz/concurrency reusing the
corpus runner; analyzers needing x/tools + scaffolding), the depended-upon task
lands first per the wave partition, so the symbol exists as real code — not a
stub — before the dependent RED test compiles.

### Wave Partition (refined for harness generation)

This partition is now **backed by real backlog `blocks` edges** (attempt-5
declaration/behavior split); the harness-generation waves and the backlog edges
agree.

* **Wave 1 (prerequisites, mutually parallel):**
  * `158.001-T` — compat corpus **declarations** (source-shape go/ast harness);
    no x/tools/analyzer dependency.
  * `158.003-T` — analyzer **scaffolding + FL001 declaration**: pinned `x/tools`
    `v0.39.0` + `cmd/faultline-analyze` skeleton (FL001 wired live, FL002..FL005
    reserved as commented slots) + `check` target + skeleton FL001 `Analyzer`
    var.
* **Wave 2 (parallel, each after its wave-1 prerequisite):**
  * `158.009-T` — corpus runner + adapters **behavior** (depends `158.001-T`).
  * `158.010-T` — FL001 scanner-discipline **behavior** + `analysistest`
    (depends `158.003-T`).
  * `158.004-T`, `158.005-T`, `158.006-T`, `158.007-T` — FL002..FL005 analyzer
    subpackages (each depends `158.003-T`; none edits `main.go`).
* **Wave 3 (parallel, after `158.009-T`):**
  * `158.002-T` — concurrency/cancellation fixtures (depends `158.009-T`).
  * `158.008-T` — bounded fuzz target (depends `158.009-T`).
* **Wave 4 (serialized integration, after `158.004-T`..`158.007-T`):**
  * `158.011-T` — fills the FL002..FL005 multichecker slots in `main.go` (sole
    owner of that edit).

---

## 158.001-T — Compatibility corpus package declarations (U1a, decl-only) · Wave 1 (independent) · domain: code

**Split note (attempt-5).** This task authors the **declarations only**; the
runner/adapter/fixture **behavior** is `158.009-T` (which depends on this task).
The signatures below are exactly the compilable declarations this task lands
(stub bodies, e.g. `panic("not implemented")`, that carry no behavior).

**Target package:** `internal/faultline/compatcorpus` (new).

**Public API (Go declarations this task authors — compilable stubs, no behavior):**

```go
package compatcorpus

type Category string

const (
    CatMalformedJSON    Category = "malformed_json"
    CatTruncatedJSON    Category = "truncated_json"
    CatMalformedYAML    Category = "malformed_yaml"
    CatTruncatedYAML    Category = "truncated_yaml"
    CatDuplicateKey     Category = "duplicate_key"
    CatCaseFoldedKey    Category = "case_folded_key"
    CatCRLF             Category = "crlf"
    CatLF               Category = "lf"
    CatOversizedToken   Category = "oversized_token"
    CatOldIndexVersion  Category = "old_index_version"
    CatWindowsSemantics Category = "windows_semantics"
)

type Expectation string

const (
    ExpectRejected   Expectation = "rejected"   // Decode returns non-nil error
    ExpectAccepted   Expectation = "accepted"   // Decode returns nil error
    ExpectNormalized Expectation = "normalized" // accepted AND DecodeResult.Normalized == canonical
)

// Adapter-defined sentinel errors. The corpus asserts outcomes with errors.Is
// against THESE (not against library message text), so a stdlib/yaml.v3 message
// change never spuriously flips an entry.
var (
    ErrTruncated        = errors.New("compatcorpus: truncated input")
    ErrMalformed        = errors.New("compatcorpus: malformed input")
    ErrDuplicateKey     = errors.New("compatcorpus: duplicate key")
    ErrCaseFoldCollision = errors.New("compatcorpus: case-folded key collision")
    ErrOldVersion       = errors.New("compatcorpus: unsupported/old index version")
    ErrTokenTooLong     = errors.New("compatcorpus: token exceeds bound")
    ErrInvalidPath      = errors.New("compatcorpus: invalid/reserved path")
    ErrUnclosedFront    = errors.New("compatcorpus: unclosed frontmatter fence")
)

type Entry struct {
    ID             string      // stable, unique, sortable (e.g. "json-dupkey-01")
    Category       Category
    Adapter        string      // adapter Name() this entry targets
    Input          []byte      // exact bytes fed to Decode
    Expect         Expectation
    WantErr        error       // sentinel required when Expect==ExpectRejected (nil = any error)
    WantNormalized []byte      // required Normalized bytes when Expect==ExpectNormalized
}

type DecodeResult struct {
    Normalized []byte // canonicalized form when the adapter normalizes; else nil
}

type ParserAdapter interface {
    Name() string
    Decode(ctx context.Context, input []byte) (DecodeResult, error)
}

type Result struct {
    EntryID string
    Adapter string
    Passed  bool
    GotErr  string // error string (empty if nil), for the report
    Reason  string // why it failed the expectation (empty when Passed)
}

type Report struct {
    SchemaVersion string   // "compatcorpus.report/v1"
    Total, Passed, Failed int
    Deterministic bool     // true when two Run() calls on identical inputs are byte-identical
    Results       []Result // sorted by (EntryID, Adapter)
}

func Run(ctx context.Context, entries []Entry, adapters map[string]ParserAdapter) Report
func DefaultCorpus() ([]Entry, error)            // loads testdata/corpus via embedded FS
func DefaultAdapters() map[string]ParserAdapter  // frontmatter, events_jsonl, scanner
func LoadCorpus(fsys fs.FS) ([]Entry, error)     // deterministic loader from manifest
func (r Report) JSON() ([]byte, error)           // stable-sorted machine-readable output
```

**Acceptance (158.001-T, declaration-only):**
* The package compiles with every symbol above declared at the contracted shape
  (names, kinds, const values, interface method set, function/method
  signatures).
* The **source-shape go/ast harness** `TestCompatcorpusDeclShape` parses the
  package source with `go/parser` + `go/ast` and asserts every contracted symbol
  is declared with the contracted shape. It executes **no** runner behavior.
* No dependency on `golang.org/x/tools` or any analyzer scaffolding.

**RED selector:** `go test ./internal/faultline/compatcorpus/ -run TestCompatcorpusDeclShape -count=1`
(fails until the declarations exist).
**GREEN selector:** same command passes.

---

## 158.009-T — Compat corpus runner + adapters behavior (U1b) · Wave 2 (depends 158.001-T) · domain: tests

**Split note (attempt-5).** Owns **all** original `158.001-T` behavior acceptance
(runner, adapters, fixtures, sentinel behavior, `compatcorpus.report/v1`). Fills
in the stub bodies declared by `158.001-T`; introduces no new exported
declaration shape. Depends on `158.001-T`.

**Runner semantics (deterministic — the acceptance backbone):**

* `Run` is pure: no goroutines, no wall-clock, no filesystem, no globals. For
  each `Entry` it calls the named adapter's `Decode`, compares the outcome to
  `Entry.Expect` (+ `WantErr` via `errors.Is` / `WantNormalized` via
  `bytes.Equal`), and records a `Result`.
* Output ordering is a total sort by `(EntryID, Adapter)`, so `Report.JSON()` is
  byte-stable across runs (`Deterministic` proven by running `Run` twice and
  asserting identical JSON).
* An adapter panic is caught and converted to a failing `Result` (never
  propagates); this keeps the runner robust and feeds 158.008-T's no-panic
  invariant.

**Parser adapter set — decode-under-test wrappers (real decoder + declared
strict-validation checks).** The adapters are the corpus's own harness, not the
unmodified core parsers; they exist to *catch* the fault classes the raw
decoders tolerate. No `internal/core` parser is changed.

| `Name()` | Real decoder wrapped | Declared strict-validation checks (return sentinel) |
|----------|----------------------|-----------------------------------------------------|
| `frontmatter` | `internal/mdfront` split + `gopkg.in/yaml.v3` into `yaml.Node` | `ErrUnclosedFront` when the opening `---` has no closing fence; `ErrDuplicateKey` / `ErrCaseFoldCollision` via a normalized-key map walked over the decoded mapping `yaml.Node` (case-fold = `strings.ToLower` collision); `ErrInvalidPath` for path-typed fields containing a backslash or a reserved Windows device name; `ErrMalformed`/`ErrTruncated` surfaced from yaml.v3 decode errors; CRLF→LF normalization emitted in `Normalized`. |
| `events_jsonl` | `encoding/json` on one JSONL record | `ErrDuplicateKey` via a `json.Decoder.Token()` object-key scan (counts repeated keys — because `encoding/json` itself keeps last-wins and does NOT reject duplicates); `ErrOldVersion` when a `schema_version` field is below the supported floor; `ErrMalformed`/`ErrTruncated` mapped from decode errors. |
| `scanner` | `bufio.Scanner` with an explicit `Buffer(buf, max)` bound | `ErrTokenTooLong` mapped from `bufio.ErrTooLong` on oversized tokens; surfaces `Scanner.Err()`. Directly exercises the FL001 discipline. |

**Fixture schema (git-friendly, deterministic):**

* `internal/faultline/compatcorpus/testdata/corpus/<category>/<id>.input` — raw
  input bytes (one file per entry). CRLF/LF fixtures are committed with a
  `.gitattributes` `-text` guard so bytes survive checkout unmodified.
* `internal/faultline/compatcorpus/testdata/corpus/manifest.json` — array of
  `{ "id", "category", "adapter", "expect", "want_err", "want_normalized_file" }`
  where `want_err` names one of the sentinel constants above.
* `LoadCorpus` reads the manifest, joins the `.input` bytes, maps `want_err`
  names to sentinels, and returns a slice sorted by `id`. Missing file, unknown
  adapter, or unknown sentinel name is a load error.

**Representative entries (exact cases + deterministic expected outcomes):**

| id | adapter | input (essence) | Expect | WantErr sentinel |
|----|---------|-----------------|--------|------------------|
| `json-malformed-01` | events_jsonl | `{"id":"x", }` (trailing comma) | rejected | `ErrMalformed` |
| `json-truncated-01` | events_jsonl | `{"id":"x"` (no close) | rejected | `ErrTruncated` |
| `json-dupkey-01` | events_jsonl | `{"id":"a","id":"b"}` | rejected | `ErrDuplicateKey` |
| `json-oldver-01` | events_jsonl | `{"schema_version":0,...}` | rejected | `ErrOldVersion` |
| `yaml-malformed-01` | frontmatter | `key: : val` | rejected | `ErrMalformed` |
| `yaml-truncated-01` | frontmatter | opening `---` never closed | rejected | `ErrUnclosedFront` |
| `yaml-dupkey-01` | frontmatter | `id: a\nid: b` | rejected | `ErrDuplicateKey` |
| `yaml-casefold-01` | frontmatter | `Id: a\nid: b` | rejected | `ErrCaseFoldCollision` |
| `crlf-01` | frontmatter | valid frontmatter with `\r\n` endings | normalized | (Normalized has `\n` only) |
| `lf-01` | frontmatter | same content, `\n` endings | accepted | — |
| `token-oversized-01` | scanner | one line longer than the max-token bound | rejected | `ErrTokenTooLong` |
| `token-ok-01` | scanner | line within bound | accepted | — |
| `windows-semantics-01` | frontmatter | path field with backslash + reserved device name | rejected | `ErrInvalidPath` |

(The committed corpus MAY add more entries; the above are the minimum
representative set the RED test asserts.)

**Acceptance (158.009-T, behavior):**
* With `ctx := context.Background()` and `entries, err := DefaultCorpus()`
  (`err == nil`), `Run(ctx, entries, DefaultAdapters())` returns `Failed == 0`,
  every entry's actual outcome matching its `Expect`/`WantErr` (via `errors.Is`).
* `Report.Deterministic == true` (two runs byte-identical).
* `Report.JSON()` validates against the `compatcorpus.report/v1` shape.

**RED selector:** `go test ./internal/faultline/compatcorpus/ -run TestCorpusRunner -count=1`
(fails: runner/corpus not yet implemented).
**GREEN selector:** same command passes.

## Corpus Runner Output Contract (resolves finding P1-d)

* The runner's machine-readable surface is `Report` serialized by
  `Report.JSON()` under schema id `compatcorpus.report/v1` (stable-sorted).
* When executed in CI, the report is written to
  `artifacts/faultline/compatcorpus-report.json` (fixed here so downstream
  consumers have a stable contract). Writing the file is a thin CI/harness
  concern, not a runner responsibility (the runner stays pure).
* **S4-U4 evidence contract / S10 DAG:** consuming this report as an evidence
  node executor is **S10's** responsibility (S10 seq7 "integrates sequence 1-6
  detectors as node executors rather than duplicating their logic"). 140-S
  guarantees only the stable output surface (schema id + fixed path). No DAG
  wiring, node declaration, or evidence edge is in 140-S scope. The versioned
  schema/path is deliberately minimal (a stable-sorted JSON with a version tag),
  reserved for S10; per-entry pass/fail is the only in-scope acceptance.

---

## 158.002-T — Concurrency & cancellation fixtures (U2) · Wave 3 (depends 158.009-T) · domain: tests

**Target:** `internal/faultline/compatcorpus/concurrency.go` (small harness seam)
+ `concurrency_test.go`. Reuses the runner/adapters implemented by `158.009-T`
(`Run`/`DefaultAdapters`). Package-level test helpers introduced here are prefixed `conc*` to avoid collision with 158.008-T's helpers.

**Seam (keeps fixtures deterministic — no real OS locks / no wall-clock):**

```go
type Locker interface {
    Acquire(ctx context.Context) error // honours ctx cancellation/deadline
    Release()
}
```

**Fixtures & exact expected safe outcomes:**

| fixture | setup | assertion (single deterministic SAFE outcome) |
|---------|-------|-----------------------------------------------|
| lock-contention | `Locker` test-double configured in **blocking mode**; goroutine A holds the lock, goroutine B calls `Acquire`, test releases A via an explicit gate channel | B's `Acquire` **blocks until Release, then returns nil** (single pinned outcome). Invariants: never deadlocks, never both-hold. (A separate sub-case configures the double in **busy mode** and pins B to `ErrLockBusy` — each sub-case asserts exactly one outcome.) |
| context-cancellation | adapter/`Locker` given an already-cancelled `ctx` | returns promptly with `errors.Is(err, context.Canceled)`; `Run` marks the entry failed-cancelled, never hangs (bounded by a test `context.WithTimeout`). |
| ambiguous-gate-input | a corpus entry admitting two readings (duplicate key with conflicting values) | resolves **fail-closed**: `Expect == ExpectRejected` (`ErrDuplicateKey`). MUST NOT be silently accepted (Constitution VIII). |

**Determinism rule:** no `time.Sleep`-based synchronization; use channels / the
double's explicit gates. All goroutines joined (no leak).

**Acceptance:** each fixture asserts its single expected safe outcome; the
cancellation fixture proves no-hang under a bounded `context.WithTimeout`.

**RED selector:** `go test ./internal/faultline/compatcorpus/ -run TestConcurrencyFixtures -count=1`.
**GREEN selector:** same passes.

---

## 158.003-T — Analyzer framework scaffolding + FL001 declaration (U3a-scaffold) · Wave 1 (prerequisite) · domain: code+build

**Split note (attempt-5).** This task owns the shared analyzer **scaffolding +
FL001 declaration** only. The FL001 scanner-discipline **behavior** and its
`analysistest` fixtures are `158.010-T` (which depends on this task). Analyzer
tasks `158.004-T`..`158.007-T` and `158.010-T` all carry a real backlog `blocks`
edge on this task.

**Also owns (wave-1 prerequisite):** pinned `golang.org/x/tools` in
`go.mod`/`go.sum`; `cmd/faultline-analyze/main.go` (explicit
`multichecker.Main(...)` with pre-reserved sorted slots FL001..FL005, FL001
filled against the declared skeleton `Analyzer` var); `Makefile`
`check` target. See §Shared Analyzer Scaffolding. (Width note: the scaffolding is
thin — a single `main.go`, a two-line Makefile recipe, the dependency promotion,
and a skeleton FL001 `Analyzer` declaration — and stays within the Go/build skill
domain and the 2-hour bound. Removing the FL001 behavior into `158.010-T` makes
this strictly smaller than the previously-reviewed combined `158.003-T` unit.)

**Exact dependency operation (reproducible; no version change to any chain).**
`golang.org/x/tools` is currently a **pruned transitive** requirement: it is
recorded in `go.sum` at `v0.39.0` but is **not** listed in `go.mod`, and the
main module does not yet import it (`go mod why golang.org/x/tools` → "main
module does not need package golang.org/x/tools"). Adding the `go/analysis`
import in this task promotes it to a **direct** requirement. The harness MUST
pin it to the **already-selected** version and MUST NOT change any other
dependency:

* **Command:** `go get golang.org/x/tools@v0.39.0` (records the explicit direct
  require at the version the graph already selects — a promotion, not an
  upgrade/downgrade), then `go mod tidy` to normalize `go.mod`/`go.sum`.
* **Expected `go.mod` delta:** one added **direct** `require` line,
  `golang.org/x/tools v0.39.0` (no `// indirect` marker). `go mod tidy` may
  ALSO add new `// indirect` entries for modules that `go/analysis` transitively
  imports (e.g. `golang.org/x/mod`, `golang.org/x/sync`), each recorded at the
  version the graph **already selects** (present in `go.sum` today) — these are
  additions at already-selected versions, **not** version moves. **No** existing
  requirement's version changes (notably `golang.org/x/text v0.32.0` stays put).
* **Verification:** `go list -m golang.org/x/tools` prints
  `golang.org/x/tools v0.39.0`; `go build ./cmd/faultline-analyze` builds;
  `make check` is green.
* **Prohibited / halt condition:** any `x/tools` version other than `v0.39.0`,
  or any **version move** of an existing requirement (an upgrade/downgrade of
  `x/text` or any other already-selected module). Newly recorded `// indirect`
  entries at their already-selected versions are expected and are NOT a
  violation. If `go mod tidy` would MOVE any existing version, **halt** — that
  is outside this contract's scope.

**Analyzer package (declaration only):**
`internal/faultline/analyzer/scannerdiscipline` declares
`var Analyzer *analysis.Analyzer` (`Name: "FL001scannerdiscipline"`) with a
**compilable skeleton run func carrying no discipline logic**, so every dependent
`analysistest` harness compiles from a base checkout and `main.go` can enumerate
FL001. The seeded-violation / clean fixtures and the discipline detection are
**not** authored here (see §158.010-T).

**Acceptance (158.003-T, scaffold + declaration):**
* `go list -m golang.org/x/tools` prints `golang.org/x/tools v0.39.0`; no
  existing requirement version moved.
* `go build ./cmd/faultline-analyze` builds (multichecker enumerates
  FL001..FL005 slots; FL001 wired to the skeleton `Analyzer`).
* A source-shape harness `TestScaffoldShape` asserts the scaffolding + FL001
  declaration exist (skeleton `Analyzer` var, enumerated slots, `check` target).

**RED selector:** `go build ./cmd/faultline-analyze && go test ./internal/faultline/analyzer/scannerdiscipline/ -run TestScaffoldShape -count=1`
(fails until scaffolding + declaration land).
**GREEN selector:** same passes; `make check` green.

## 158.010-T — Analyzer FL001 Scanner discipline behavior + analysistest (U3a-behavior) · Wave 2 (depends 158.003-T) · domain: code

**Split note (attempt-5).** Implements the FL001 discipline **behavior** by
filling the skeleton run func declared by `158.003-T`, plus its `analysistest`
fixtures. Does **not** edit `cmd/faultline-analyze/main.go` (the FL001 slot is
already filled by `158.003-T` against the declared `Analyzer` var). Depends on
`158.003-T`.

**Analyzer package:** `internal/faultline/analyzer/scannerdiscipline`
(`Name: "FL001scannerdiscipline"`).

* **Message:** `FL001: bufio.Scanner used without Buffer() bound and/or Err() check`.
* **Source:** an assignment whose RHS is `bufio.NewScanner(...)` (confirmed via
  `pass.TypesInfo` result type `*bufio.Scanner`), bound to a local variable `v`.
* **Sink / rule:** within the **same function body**, require BOTH a selector
  call `v.Buffer(...)` before the scan loop AND a `v.Err()` check after it.
  Report if either is absent.
* **AST/type boundary:** intra-function, syntactic + type-checked receiver.
* **Exclusions:** `_test.go` files; scanners that escape the function (assigned
  to a struct field, returned, or passed to another call — cannot prove
  locally); a `// faultline:scanner-ok` comment on the `NewScanner` statement.
* **Seeded** (`testdata/src/fl001bad/bad.go`): `s := bufio.NewScanner(r); for
  s.Scan() { _ = s.Text() }` with no `Buffer`/`Err` → `// want "FL001"`.
* **Clean** (`testdata/src/fl001good/good.go`): adds `s.Buffer(make([]byte, 0,
  64*1024), 1<<20)` and `if err := s.Err(); err != nil { return err }`.

**RED selector:** `go test ./internal/faultline/analyzer/scannerdiscipline/ -run TestAnalyzer -count=1`
(fails: skeleton analyzer reports nothing on the seeded `// want` line).
**GREEN selector:** same passes; `go build ./cmd/faultline-analyze` builds; `make check` green.

## 158.004-T — Analyzer: error wrapping with %w (U3b, FL002) · Wave 2 (depends 158.003-T) · domain: code

**Analyzer package:** `internal/faultline/analyzer/errwrap` (`Name: "FL002errwrap"`).

* **Message:** `FL002: error argument to fmt.Errorf should use %w, not %v/%s`.
* **Source:** an `error`-typed argument (identifier or call whose result type
  implements `error`, via `pass.TypesInfo`) passed **directly** to the sink.
* **Sink:** a call to `fmt.Errorf` with a **constant** format string.
* **Rule:** parse the format verbs; for each argument position whose type
  implements `error`, require the verb `%w`. Flag `%v`/`%s`/`%+v` in an error
  position.
* **AST/type boundary:** single call expression; format must be a string literal
  (dynamic formats excluded).
* **Exclusions:** non-constant format strings; non-error positions;
  `// faultline:errwrap-ok` on the call; `errors.New` (not a wrap site).
* **Seeded:** `return fmt.Errorf("load: %v", err)` → `// want "FL002"`.
* **Clean:** `return fmt.Errorf("load: %w", err)`.

**RED selector:** `go test ./internal/faultline/analyzer/errwrap/ -run TestAnalyzer -count=1`.
**GREEN selector:** same passes. (The FL002 multichecker slot is filled by the
serialized integration task `158.011-T`, not here; this task owns only its
subpackage + `analysistest`.)

## 158.005-T — Analyzer: fail-open error branches (U3c, FL003) · Wave 2 (depends 158.003-T) · domain: code

**Analyzer package:** `internal/faultline/analyzer/failopen` (`Name: "FL003failopen"`).

* **Message:** `FL003: error branch returns success (fail-open); safety-mode requires fail-closed`.
* **Source:** an `if err != nil { ... }` inside a function whose signature's last
  result is `error`.
* **Sink / rule (narrow):** flag ONLY the classic fail-open terminal shapes as
  the block's terminating statement: `return nil`, or `return <zero-values>, nil`
  where the trailing result is the nil error.
* **AST/type boundary:** intra-block, syntactic; enclosing func result types via
  `pass.TypesInfo`.
* **Exclusions:** blocks that `return err`/wrapped error, `panic`, or `os.Exit`;
  functions with no error result; `// faultline:fail-open-ok` on the `if`;
  `defer`/cleanup closures.
* **Seeded:** `if err != nil { return nil }` in an error-returning func →
  `// want "FL003"`.
* **Clean:** `if err != nil { return err }`.

**RED selector:** `go test ./internal/faultline/analyzer/failopen/ -run TestAnalyzer -count=1`.
**GREEN selector:** same passes. (FL003 slot filled by `158.011-T`, not here.)

## 158.006-T — Analyzer: success returns after audit warnings (U3d, FL004) · Wave 2 (depends 158.003-T) · domain: code

**Analyzer package:** `internal/faultline/analyzer/auditsuccess` (`Name:
"FL004auditsuccess"`), plus a **declaration-only** allowlist file
`auditsuccess/sinks.go`. Resolves P1-b for this case: declaration-driven,
intra-block only — no data-flow/CFG.

* **Message:** `FL004: success returned in same block after an audit warning without failing closed`.
* **Audit-warning definition (declared, not inferred):** a call whose selector
  matches the declared allowlist in `sinks.go`. Default: `slog.Warn`, `*.Warn`
  on a `*slog.Logger`, `audit.Warn`, any selector named `AuditWarn`. The
  allowlist is the single source of truth.
* **Source:** an audit-warning call at statement index `i` within a block.
* **Sink / rule:** a later statement in the **same block** (index `> i`) that is
  a success terminal (`return nil` / `return <zero>, nil`) with **no intervening
  error return**.
* **AST/type boundary:** single block, straight-line order; selector match by
  name + (where cheap) receiver type.
* **Exclusions:** a `return err`/wrapped error between warn and terminal; warn
  and return in different blocks/closures; `// faultline:warn-nonfatal` on the
  warn call.
* **Seeded:** `slog.Warn("audit: soft fail"); return nil` in the same block →
  `// want "FL004"`.
* **Clean:** `slog.Warn("audit: soft fail"); return errAudit`.

**RED selector:** `go test ./internal/faultline/analyzer/auditsuccess/ -run TestAnalyzer -count=1`.
**GREEN selector:** same passes. (FL004 slot filled by `158.011-T`, not here.)

## 158.007-T — Analyzer: timeout claims reaching uncancellable locks (U3e, FL005) · Wave 2 (depends 158.003-T) · domain: code

**Analyzer package:** `internal/faultline/analyzer/locktimeout` (`Name:
"FL005locktimeout"`), plus a **declaration-only** allowlist file
`locktimeout/locks.go`. Resolves P1-b for this case: declaration-driven,
intra-function only — no SSA/data-flow.

* **Message:** `FL005: deadline/timeout context in scope but lock acquired via a non-ctx (uncancellable) call`.
* **Timeout claim (source):** a local `context.WithTimeout`/`context.WithDeadline`
  result in scope within the function (detected syntactically).
* **Lock sink (declared, not inferred):** a call whose selector matches the
  declared lock-acquire allowlist in `locks.go` AND takes **no
  `context.Context` argument**. Default: `(*sync.Mutex).Lock`,
  `(*sync.RWMutex).Lock`, `(*sync.RWMutex).RLock`, any selector named
  `Lock`/`Acquire` with a zero-`ctx` signature.
* **Rule:** flag when a deadline-bearing context is in scope AND a declared
  non-ctx lock-acquire call executes in the same function.
* **AST/type boundary:** intra-function, syntactic; selector/receiver types via
  `pass.TypesInfo`. No cross-call ctx tracing.
* **Exclusions:** lock-acquire calls that DO take a `ctx` argument; functions
  with no timeout/deadline context in scope (no false claim → not flagged);
  `// faultline:lock-nonctx-ok` on the lock call.
* **Seeded:** `ctx, cancel := context.WithTimeout(parent, d); defer cancel();
  ...; mu.Lock()` → `// want "FL005"`.
* **Clean:** no timeout context in scope, or a ctx-aware acquire
  (`acq.Acquire(ctx)`).

**RED selector:** `go test ./internal/faultline/analyzer/locktimeout/ -run TestAnalyzer -count=1`.
**GREEN selector:** same passes. (FL005 slot filled by `158.011-T`, not here.)

---

## 158.008-T — Bounded compatibility fuzz target (U-fuzz) · Wave 3 (depends 158.009-T) · domain: tests

**Target:** `internal/faultline/compatcorpus/fuzz_test.go` — the **single owning
package** required by `go test -fuzz`. Package-level test helpers introduced here
are prefixed `fuzz*` to avoid collision with 158.002-T's `conc*` helpers.

**Fuzz function:**

```go
func FuzzCompatibilityCorpusDecode(f *testing.F) {
    for _, e := range fuzzMustDefaultCorpus(f) { f.Add(e.Input) } // runtime seeds from 158.009-T corpus
    f.Fuzz(func(t *testing.T, input []byte) {
        for _, a := range compatcorpus.DefaultAdapters() {
            _, _ = a.Decode(context.Background(), input) // property: never panic, bounded
        }
    })
}
```

* **Invariant / property:** for any input, every adapter's `Decode` **must not
  panic** and must return within the bounded scanner/token limits (no unbounded
  allocation). Returning an error is acceptable; crashing/hanging is not.
* **Canonical native seed location:**
  `internal/faultline/compatcorpus/testdata/fuzz/FuzzCompatibilityCorpusDecode/`
  (Go's built-in package-local fuzz corpus), committed, augmented at runtime via
  `f.Add` from `DefaultCorpus()`.
* **Execution budget (exact):**
  `go test -run=^$ -fuzz=^FuzzCompatibilityCorpusDecode$ -fuzztime=30s ./internal/faultline/compatcorpus`
  (single package — never `./...`). If CI cannot run Go fuzzing, an equivalent
  fixed-count local harness replays the seed corpus deterministically.
* **Acceptance:** committed package-local seed corpus; crash-free over the
  budget; any crasher is minimized, committed to the seed corpus, and converted
  into a deterministic `compatcorpus` regression entry before close.

**RED selector (valid seed-regression gate — does NOT rely on `-run` matching a
fuzz target):** a normal unit test
`TestFuzzSeedCorpusCommitted` that (1) asserts the native seed directory exists
and is non-empty via `os.ReadDir`, and (2) replays each committed seed and each
`DefaultCorpus()` input through every adapter's `Decode`, asserting no panic:
`go test ./internal/faultline/compatcorpus/ -run TestFuzzSeedCorpusCommitted -count=1`
(fails until the seed files + target exist).
**GREEN selector:** that test passes; the budgeted `-fuzz` run is crash-free.

---

## 158.011-T — Wire FL002..FL005 into faultline-analyze multichecker (U3-wire) · Wave 4 (depends 158.004-T..158.007-T) · domain: code+build

**Split note (attempt-5, Go-anchor remediation).** Added to remove parallel
shared-file ownership of `cmd/faultline-analyze/main.go`. `158.003-T` authors
`main.go` with FL001 wired live and FL002..FL005 as pre-reserved commented slots;
the analyzer tasks `158.004-T`..`158.007-T` own **only** their subpackages and do
not touch `main.go`. This task is the **single serialized owner** of the FL002..
FL005 wiring and runs after all four analyzer subpackages exist.

**Target:** `cmd/faultline-analyze/main.go` (fill the four reserved slots with
`errwrap.Analyzer` (FL002), `failopen.Analyzer` (FL003), `auditsuccess.Analyzer`
(FL004), `locktimeout.Analyzer` (FL005)) + `cmd/faultline-analyze/main_test.go`.

**Rule:** the `multichecker.Main(...)` enumeration references each of the five
analyzer vars exactly once, alphabetically sorted, no reflection/registry.

**Acceptance:** `go build ./cmd/faultline-analyze` builds with all five analyzers
enumerated; `make check` green; the enumeration test asserts FL001..FL005 present
exactly once each.

**RED selector:** `go test ./cmd/faultline-analyze/ -run TestMulticheckerEnumeratesAll -count=1`
(fails while FL002..FL005 remain commented).
**GREEN selector:** same passes; `go build ./cmd/faultline-analyze` builds.

---

## Plan Hardening

**Signals re-evaluated for this supplement:**

* public API/schema/contract change: **absent** — new internal analyzer/corpus
  packages only; no user-facing API/schema/CLI change. `compatcorpus.report/v1`
  is a new internal contract, versioned from day one.
* security/auth/permission/compliance-sensitive: **absent**.
* migration/backfill/destructive/irreversible: **absent**.
* external integration/operator checkpoint/external dependency: **one new dev
  dependency** (`golang.org/x/tools`, pinned to the **already-selected**
  `v0.39.0` — a promotion of an existing pruned transitive requirement to a
  direct one, with **no** version change to any chain) — build/test-time only,
  no runtime or distribution impact.
* high runtime/rollout/rollback risk: **absent** — no production behavior change;
  analyzers and corpus run in test/CI only.

**Requires plan hardening: no.** The single pinned dev dependency is the only
non-trivial signal; it is bounded, conventional, and version-locked. The
attempt-5 declaration/behavior split introduces **no new hardening signal** — it
only re-partitions existing scope into declaration prerequisites + behavior tasks
and promotes prose ordering into real backlog edges (no product-scope, API,
schema, security, migration, or rollout change). Residual risks and bounded
mitigations:

| Risk | Mitigation (in this contract) |
|------|-------------------------------|
| In-task two-stage harness (P-002.1/P-002.6) | Split: `158.001-T`/`158.003-T` are declaration-only prerequisites; `158.009-T`/`158.010-T` own the behavior; behavior harnesses compile only after declarations land (real backlog edges enforce order). |
| Analyzer false positives | AST+type only; declaration-driven allowlists; explicit exclusions + suppression comments; under-approximate over false-positive. |
| SSA/CFG scope creep (prior FAIL) | Hard boundary: intra-function/intra-block syntactic detection; no SSA/CFG/dataflow. |
| Unreachable corpus outcomes (R3-1) | Adapters declared as decode-under-test wrappers with strict-validation + sentinel errors; outcomes provably reachable. |
| Library message drift | Outcomes asserted via `errors.Is` sentinels, not message substrings. |
| Shared-file coupling in parallel waves | Explicit enumeration with pre-reserved sorted slots; honest acknowledgement, not conflict-free claim. |
| Hidden intra-wave dependency (R3-3) | `158.003-T` is the wave-1 prerequisite; `158.004-T`..`158.007-T` + `158.010-T` carry **real backlog `blocks` edges** on it (no longer prose-only). |
| Repo-wide enforcement surfacing production violations | Deferred out of 140-S; acceptance is fixture-scoped analysistest only. |
| Non-deterministic corpus / fuzz seeds | Pure runner, total sort, twice-run determinism assertion, `-text` fixtures, committed native seeds asserted by a real unit test. |

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

personas (attempt-3, genuine multi-agent dispatch over this supplement + the
governing plan):
* Go Reviewer, anchor (`gpt-5.6-terra`, effort high)
* Correctness Reviewer (`claude-sonnet-5`)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Architecture Strategist (`grok-4.6`)
* Constitution Reviewer (`claude-opus-4.8`)
* Security Reviewer — evaluated for risk-trigger; NOT triggered (no
  security/auth/secret/permission surface; test/CI-only, no runtime change).

Attempt-3 controlling findings and dispositions (all resolved inline, see
§Second-Pass Review Findings Resolved):
* P1 (Go, Correctness) R3-1 — corpus outcomes unreachable through raw decoders:
  RESOLVED by decode-under-test wrappers + sentinel errors.
* P1 (Go) R3-2 — invalid fuzz RED selector: RESOLVED by
  `TestFuzzSeedCorpusCommitted`.
* P1 (Go, Architecture) R3-3 — hidden intra-wave x/tools + scaffolding
  dependency: RESOLVED by promoting 158.003-T to wave-1a prerequisite + declared
  ordering + decoupled explicit enumeration.
* P1 (Go) R3-4 — non-executable `check` recipe: RESOLVED by concrete
  `go test ./internal/faultline/analyzer/...` + `go build`.
* P2 items (pin x/tools version; drop unused `Warnings`; pin lock-contention
  outcome; honest shared-file acknowledgement; reserved Wave-2 helper names;
  errors.Is over message substrings): all APPLIED.
* Scope Auditor: NO blocking scope violations — 1:1 mapping to the 8 governing
  units; S10/repo-wide/production-remediation explicitly out of scope; no
  backlog-artifact or shipment mutation.
* Constitution Reviewer: NO blocking violations — II Test-First (declare→RED→
  GREEN, no production code now), VIII fail-closed (FL003/FL004 + ambiguous
  fixture), VI single-responsibility, I `%w` all preserved.

No open P0/P1 findings remain. Gate: **PASS** — executable, harness-first,
scope-bounded. Ready for Ship to integrate this reviewed planning commit into the
existing implementation branch and resume harness generation.

## Plan Review

<!-- plan-review-attempt: 4 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

**Scope of this attempt:** a NARROWLY SCOPED re-review of the single
dependency-pin correction that unblocks shipment `140-S` — changing the mandated
`golang.org/x/tools` pin from an exact `v0.28.0` (which would force an unreviewed
downgrade of the already-MVS-selected `v0.39.0`) to reuse the repository's
already-selected `v0.39.0`, promoting it from a pruned transitive requirement to
a direct one. No unrelated plan scope was reopened. This attempt was explicitly
authorized by the operator as one additional narrowly scoped review correction to
resolve Ship's `HARNESS_CONTRACT_UNDERSPECIFIED` halt.

personas (genuine multi-agent dispatch over the amended contract surface):
* Go Reviewer, anchor (`gpt-5.6-terra`, effort high) — dependency/version
  semantics + analyzer API compatibility.
* Scope Boundary Auditor (`gemini-3.7-flash`) — scope-creep / collateral
  dependency / backlog-mutation guard.
* Security Reviewer — evaluated for risk-trigger; NOT triggered (build/test-only
  dev dependency; no runtime, auth, secret, or permission surface).

Verified facts (read-only):
* `golang.org/x/tools v0.39.0` is already MVS-selected — recorded in `go.sum`,
  absent from `go.mod`, `go mod why` → "main module does not need package".
  `go list -m golang.org/x/tools` → `v0.39.0`.
* `v0.39.0`'s `go.mod` declares `go 1.24.0`, matching this module — Go
  1.24-compatible, no `go` directive bump forced.
* `analysis.Analyzer`, `analysistest.Run`, and `multichecker.Main` exist at
  `v0.39.0` with the exact signatures the contract depends on; the API surface is
  stable across `v0.28.0`..`v0.39.0` (no incompatibility risk).

Findings and dispositions:
* Go Reviewer P2 — the original "exactly one added require line / no other
  requirement changes" understated the real `go mod tidy` result (new
  `// indirect` entries for `x/mod`/`x/sync` at their already-selected versions).
  RESOLVED inline: §158.003-T now expects those `// indirect` additions at
  already-selected versions and redefines the halt condition as any **version
  move** of an existing requirement, not any added line.
* Go Reviewer verdict: PASS (operation is a promotion, not an upgrade/downgrade;
  verification command sound).
* Scope Boundary Auditor: NO findings — doc-only, confined to the x/tools pin
  surface, authorizes no collateral dependency change, mutates no backlog /
  shipment `140-S` membership-status / task status / Ship checkpoint. Verdict:
  PASS.

No open P0/P1 findings remain. Gate: **PASS** — the contract now reuses the
already-selected `golang.org/x/tools v0.39.0`, is reproducible (exact command +
verification + bounded halt condition), and authorizes no unrelated dependency
change. This supersedes the attempt-3 record for the x/tools pin surface only;
all other attempt-3 dispositions remain in force. Ready for Ship to integrate
this reviewed planning commit into the existing implementation branch and resume
wave-1 harness generation.

## Plan Review

<!-- plan-review-attempt: 5 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

**Scope of this attempt:** the P-002.1/P-002.6 **declaration/behavior split** that
converts `158.001-T` and `158.003-T` into declaration-only prerequisites, adds
behavior tasks `158.009-T` (corpus runner) and `158.010-T` (FL001 behavior) plus
the serialized multichecker-wiring task `158.011-T`, promotes the analyzer
harness ordering into **real backlog `blocks` edges**, and rewires the runner
consumers `158.002-T`/`158.008-T` onto `158.009-T`. No product scope reopened;
all attempt-3/attempt-4 dispositions remain in force.

personas (genuine multi-agent dispatch over the amended supplement + governing
plan + the mutated backlog topology):
* Constitution Reviewer (`claude-opus-4.8`) — II Test-First / P-002 RED→GREEN
  ordering, VI single-responsibility, VIII fail-closed.
* Go Reviewer, anchor (`gpt-5.6-terra`, effort high) — declaration/behavior
  compile ordering, x/tools promotion, shared-file ownership.
* Architecture Strategist (`grok-4.6`) — dependency DAG + wave partition.
* Scope Boundary Auditor (`gemini-3.7-flash`) — dark scope `[140-S]` only.
* Correctness Reviewer (`claude-sonnet-5`) — edge/prose consistency, acceptance
  completeness.
* Template/Backlog Integrity (`gpt-5.6-terra`) — frontmatter, docs lint, manifest.

Controlling findings and dispositions (all resolved inline, re-reviewed clean):
* **P1 (Go anchor) — `158.009-T` acceptance called `Run(DefaultCorpus(),
  DefaultAdapters())`, which does not match the declared signature
  `Run(ctx, entries, adapters)`.** RESOLVED: the §158.009-T acceptance now uses
  `ctx := context.Background()` + `entries, err := DefaultCorpus()` then
  `Run(ctx, entries, DefaultAdapters())`. Go re-review: PASS.
* **P1 (Go anchor) — `158.004-T`..`158.007-T` each edited shared
  `cmd/faultline-analyze/main.go` in parallel (duplicate/parallel file
  ownership).** RESOLVED: added serialized integration task **`158.011-T`** as the
  sole owner of the FL002..FL005 wiring; `158.003-T` authors `main.go` (FL001
  live + FL002..FL005 commented reservations); analyzer tasks own only their
  subpackages. `main.go` is touched only by `158.003-T` (wave 1) then `158.011-T`
  (wave 4) — strictly serialized. Go re-review: PASS.
* **P1 (Correctness) — `158.008-T` body prose still said "depends 158.001-T".**
  RESOLVED: `158.008-T` and `158.002-T` descriptions rewritten to reference
  `158.009-T`. Correctness re-review: PASS.
* Constitution / Architecture / Scope / Template-Integrity: PASS with no
  blocking findings (acyclic DAG rooted at `158.001-T`/`158.003-T`; dark scope
  `[140-S]` only; docs lint clean; manifest = `158-F` + 11 tasks = 12 items;
  behavior tasks strictly smaller than the units they were carved from; sizing
  semantics preserved — all members unsized).

No open P0/P1 findings remain. Gate: **PASS** — this is the authoritative final
verdict for the declaration/behavior split; it supersedes the attempt-3/attempt-4
records for the task-partition surface only. Ready for Ship to re-claim `140-S`
and resume wave-1 harness generation on the existing implementation branch.
