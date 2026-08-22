---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for eliminating success-shaped evidence loss in checkpoint creation and docs lint.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md
title: 'Eliminate success-shaped evidence loss on governed diagnostic paths'
---

## Source

`docs/decisions/2026-08-21-success-shaped-evidence-loss-deliberation.md`

External source IDs carried into this plan: `3C7AAC71` (checkpoint context key drop) and
`90F2A9F8` (docs lint whole-corpus abort). External source ID `84D8E6AB` is **not** in scope: it was
already shipped as feature `143-F` / shipment `127-S` with prevention hardening in `144-F` / `128-S`.

## Problem frame

Two governed backlogit operations return a shaped result that conceals lost evidence.

### Defect 1 — checkpoint create silently discards caller context

`events.CreateCheckpoint` (`internal/events/memory.go:44`) probes the state dump for
`schema_version == 1`. On a match it parses into `CheckpointV1` through `ParseCheckpoint`
(`internal/events/checkpoint_schema.go`), a plain `json.Unmarshal` with no unknown-field handling,
and then **re-marshals the parsed struct** with `jsonutil.MarshalReadable(cp)` and writes those bytes.
`CheckpointContext` models only `shipment_id`, `feature_id`, `task_ids`, and `branch`, so every other
key the caller supplied inside `context` is absent from the re-marshalled bytes. Unknown top-level
keys are lost by the same mechanism.

Neither surface reports the loss. `handleCreateCheckpoint` (`internal/mcp/tools.go:1113`) returns
`{"path": ...}`; `newCheckpointCreateCmd` (`internal/cli/checkpoint.go`) prints the same envelope; and
`GetCheckpoint` later validates the truncated record successfully.

Two code facts constrain the fix:

1. `ParseCheckpoint` is on the **read** path. `ListCheckpoints`, `GetCheckpoint`, `ResolveCheckpoint`,
   and the disposition verbs all call it, and `ListCheckpoints` marks any parse failure
   `needs_quarantine: true` with a remediation command. Making `ParseCheckpoint` strict would sweep the
   existing on-disk corpus into quarantine candidacy.
2. The nine files under `.backlogit/checkpoints/` carry no `schema_version`, so they never enter the V1
   branch and are written verbatim today. Their top-level keys (`pr_number`, `pr_status`, `ci_status`,
   `decisions`, `next_steps`, `review_gate`, `items_blocked`, `follow_up_tasks`) show what structured
   recovery data agents actually need and the V1 shape cannot carry.

### Defect 2 — docs lint aborts the whole corpus on the first decode error

`docline.LintTree` (`internal/docline/service.go`) returns `nil, err` on the first `decodeDoc` failure.
`backlogit docs lint` (`internal/cli/docs.go`) then exits 1 having printed nothing, and MCP
`handleDocsLint` (`internal/mcp/docs_tools.go:48`) returns `InternalError`. One malformed YAML scalar
makes conformance across the whole corpus unobservable.

## Requirements trace

| # | Requirement (**D** = from the decision record, **P** = plan-derived; unmarked rows are **D**) | Implementation units |
|---|---|---|
| R1 | Unmodeled `context` keys survive the create round-trip verbatim | U0a, U1a, U2 |
| R2 | The four modeled context fields and their filters keep working unchanged | U1a, U2 |
| R3 **(D, widened by P)** | Unknown keys in the **closed schema namespace** — the `CheckpointV1` top level and the nested modeled `progress` object — are rejected loudly at the create boundary, naming **all** of them in one error. `context` is explicitly excluded and stays open | U3a, U3d, U4 |
| R4 | `ParseCheckpoint`'s **observable** read behavior is unchanged for every input shape, including degenerate `context` values; the legacy corpus is not reclassified | U3b, U3c, U4 |
| R5 | The create result reports the persisted context key names on both surfaces, on the V1 **and** the legacy path, and the reported list always matches the bytes on disk | U5a, U5b, U6 |
| R6 | Tests dispatch through the registered MCP handler and the CLI flag, not the core function | U5a |
| R7 | A per-file **frontmatter-decode** failure becomes a finding and the scan continues; containment and I/O failures stay errors | U0b, U7a, U7b, U8 |
| R8 | The lint gate keeps its non-zero exit on a malformed corpus | U9a |
| R9 | `LintReport.Findings` is a present, non-null array on every path — clean, degraded, and error-free-but-empty | U9a, U9b |
| R10 | CLI and MCP emit identical lint payloads for the degraded corpus | U9a |
| R11 | Both contracts are documented on every operator- and agent-facing surface: generated CLI reference, MCP tool descriptions, the authoring guide, and the agent instruction file | U6, U4b, U8b, U10a, U10b |
| R12 | `ErrCheckpointUnknownField` surfaces on MCP as a structured `validation_failed` outcome carrying an `unknown_fields` array, not a generic `internal` error | U3d, U4b |
| R13 **(P)** | The modeled key sets used by the marshaller and the unknown-key probe are **derived from struct tags by reflection**, so adding a field cannot silently desynchronize them | U1b, U3c, U2, U4 |
| R14 **(P)** | No dump that succeeds today is newly rejected: key comparison matches `encoding/json`'s case-insensitive semantics | U3c, U4 |

## Implementation units

Every unit is test-first unless marked otherwise. Every unit must leave the tree compiling.

Every unit's commit boundary additionally runs the constitutional quality gates in order —
`go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` — with zero findings. No unit is
accepted on test-green alone.

U0a–U10b are materialized as backlogit tasks under the covering feature created by Stage before
implementation begins, and the dependency graph below is registered through backlogit dependency
operations. This document is the design reference; backlogit holds the authoritative task state.

Branch: a single dedicated implementation branch carries all units and merges through one merge
commit (Principle XI). No unit lands on a separate branch.

### U0a — Pin the checkpoint exported surface (declaration-only prelude)

* **Domain**: code, plus mechanical test call-site migration
* **Files**: `internal/errors/checkpoint_errors.go`, `internal/events/checkpoint_schema.go`,
  `internal/events/memory.go`, `internal/mcp/tools.go`, `internal/cli/checkpoint.go`,
  `internal/events/memory_test.go`
* **Changes**: introduce every new checkpoint identifier the Track A harnesses assert against, with
  **no behavior change**, so each harness commit compiles and its red result is observable:
  * `ErrCheckpointUnknownField` sentinel **and** the typed
    `CheckpointUnknownFieldError{Fields []string}` that carries the offending keys, with
    `Error() string` and `Unwrap() error { return ErrCheckpointUnknownField }` so `errors.Is` keeps
    working and `errors.As` can recover the list. A bare sentinel cannot carry a `[]string`, and the
    only way to get one back out of a wrapped message is to parse `err.Error()` — which U4 and the
    workspace Go conventions forbid. The repository precedent is the typed
    `*corerrors.AmbiguousWorkspaceRootError` recovered by `errors.As` in `internal/mcp/errors.go`.
  * `Extra map[string]json.RawMessage \`json:"-"\`` on `CheckpointContext` and a stub
    `func (c CheckpointContext) Keys() []string { return make([]string, 0) }`. Both are genuinely
    behavior-neutral: a `json:"-"` field is invisible to `encoding/json`, and the stub is unreferenced
    by production code until U6.
  * `CreateCheckpointResult` (see U6 for the pinned shape) plus the `CreateCheckpoint` signature
    change, returning `ContextKeys: make([]string, 0)` unconditionally, with the four call sites
    migrated: `internal/mcp/tools.go:1122`, `internal/cli/checkpoint.go:80`,
    `internal/events/memory_test.go:33`, `internal/events/memory_test.go:47`.
* **Rationale**: without this prelude, U1a/U1b/U3a/U5a/U5b assert identifiers that do not yet exist —
  commits that would not compile, making the Principle II red observation unobtainable rather than
  merely weak.
* **Scope note**: six files across four packages, and it mixes production declarations with two
  mechanical test call-site migrations. Both deviations are forced: an exported signature change is
  atomic across its callers, and the two `memory_test.go` edits are compilation fixes, not test
  authoring. Rejected alternative: a temporary dual-shape shim, rejected as dead code
  (Development Workflow §5).
* **Risky action**: classified as PA-1.
* **Posture**: enabling, no behavior change. Not test-first, because it asserts nothing.
* **Acceptance**: the tree compiles; `go test ./...` is green with the two migrated call sites updated
  and no assertion changed; `go vet ./...`, `golangci-lint run`, and `gofmt -l .` are clean; every new
  exported identifier carries a doc comment beginning with its own name. If the repository lint
  configuration flags a declared-but-unreferenced exported sentinel, the unit resolves it by landing
  U0a immediately before its first consumer rather than by adding a lint suppression.

### U0b — Pin the docline rule vocabulary (declaration-only prelude)

* **Domain**: code
* **Files**: `internal/docline/policy.go`
* **Changes**: declare `RuleDecodeError = "decode_error"` and the `ErrFrontmatterDecode` sentinel, each
  with a doc comment, so U7a and U7b compile and can be observed red before U8.
* **Rationale**: the docline declarations are independent of the checkpoint signature migration.
  Keeping them in their own single-file unit means Track A (source entry `3C7AAC71`) and Track B
  (source entry `90F2A9F8`) share no commit, matching the independence the dependency graph asserts.
* **Posture**: enabling, no behavior change.
* **Acceptance**: the tree compiles; all four quality gates are clean; both identifiers carry doc
  comments beginning with their own names.

### U1a — Red harness for open `context` namespace preservation

* **Domain**: tests
* **Files**: `internal/events/checkpoint_context_test.go` (new)
* **Changes**: assert that a V1 state dump whose `context` carries unmodeled keys round-trips through
  `CreateCheckpoint` with those keys intact **on disk**. Assert on the written bytes, not on the
  returned struct, so a reader that shares the lossy type cannot produce a false green.
* **Scenarios**: (1) unmodeled flat scalar preserved; (2) unmodeled nested object preserved with its
  structure, asserted by decoding into `map[string]any` and comparing values plus key order read
  through `json.Decoder.Token()` — **not** by a raw byte-substring match, because
  `jsonutil.MarshalReadable` compacts insignificant whitespace inside a `json.RawMessage`; (3) the four
  modeled fields still decode into their typed accessors **and** a checkpoint written through
  `CreateCheckpoint` is still returned by `ListCheckpoints` under a `CheckpointFilter{ShipmentID: ...}`
  and a `CheckpointFilter{FeatureID: ...}`, closing R2 end-to-end at the filter rather than at the
  struct boundary.
* **Posture**: test-first. Scenarios 1 and 2 are red before U2. Scenario 3 is a regression guard and is
  green throughout.
* **Acceptance**: two failing assertions naming the missing key; scenario 3 green before and after U2.

### U1b — Red harness for marshaller-safety guards

* **Domain**: tests
* **Files**: `internal/events/checkpoint_marshal_test.go` (new)
* **Changes**: pin the three traps a hand-written `MarshalJSON` on a value-typed struct field can fall
  into, plus the drift guard for the modeled-key set.
* **Scenarios**: (1) a **non-addressable** `CheckpointContext` value is marshalled directly — as a value
  copy and inside a `map[string]any` — and still emits its unmodeled keys, which is the value-receiver
  guard; (2) an `Extra` key colliding with a modeled JSON tag emits exactly one occurrence of that key
  and the modeled field wins, and no key literally named `Extra` ever appears in the emitted `context`
  object; (3) `TestModeledContextKeys_MatchesLiteralExpectation` reflects over `CheckpointContext`'s
  json tags and compares the result against a **hard-coded literal list**
  `{"branch","feature_id","shipment_id","task_ids"}` (sorted, non-empty). It MUST NOT reference any
  production-derived key set, so it compiles at U0a; a test that re-derived the set by reflection would
  compare the derivation to itself and pass unconditionally, including against a derivation that
  forgets to strip `,omitempty` (R13).
* **Fixture construction rule**: every scenario builds its input from literal JSON bytes decoded with
  `json.Unmarshal`, never by marshalling a `CheckpointContext`, so no assertion can be satisfied
  tautologically by the type under test. **One carve-out**: the marshal-side collision guard in
  scenario 2 MAY inject a modeled key into `Extra` directly after decoding its literal-JSON fixture,
  because a correct `UnmarshalJSON` routes modeled keys into their modeled fields and therefore makes
  that state unreachable through the decode path. This is the only permitted direct `Extra` mutation
  and it exists solely to execute `MarshalJSON`'s skip branch.
* **Scenario 4 — HTML-escape guard**: a modeled field (`branch`) and an unmodeled `Extra` value each
  carrying `a > b && b < c` land on disk **unescaped**, asserted with a `NotContains` check for
  `\u0026` in the written bytes. This mirrors the existing top-level escaping guard at the `context`
  level, which that guard cannot reach.
* **Scenario 3's own derivation replicates the rules, it does not call production code**: it skips
  unexported fields and `json:"-"` fields — `Extra` is `json:"-"` from U0a onward and must not appear —
  and strips tag options with `strings.Split(tag, ",")[0]`.
* **Note on the recursion trap**: a recursive `MarshalJSON` fails scenario 1 by stack exhaustion, so no
  separate scenario is required; the guard is stated in U2 as an implementation constraint.
* **Posture**: test-first. Scenarios 1 and 4 are red before U2. Scenarios 2 and 3 are guards: scenario 2
  is green pre-U2 only because `Extra` is `json:"-"` and therefore invisible to `encoding/json`, and it
  becomes load-bearing once U2 gives the carrier behavior; scenario 3 is green throughout.
* **Acceptance**: two failing assertions (scenarios 1 and 4); scenarios 2 and 3 green before and after
  U2.

### U2 — Preserve unmodeled `context` keys through the create round-trip

* **Domain**: code
* **Files**: `internal/events/checkpoint_schema.go`
* **Changes**: give the `Extra` carrier **U0a declared** its behavior. The field declaration and its
  load-bearing tag are unchanged from U0a:

  ```go
  Extra map[string]json.RawMessage `json:"-"`
  ```

  Add custom `UnmarshalJSON` and `MarshalJSON`. Unmarshal decodes the four modeled keys and routes
  every other key into `Extra`. Marshal re-emits the modeled keys followed by the `Extra` keys
  **flattened back into the same `context` object**, sorted by key for deterministic output. **Replace
  U0a's `Keys()` stub** with the real accessor returning the sorted set of keys `MarshalJSON` actually
  emits.
* **`json:"-"` is load-bearing.** A defined-type conversion copies every field *and its tags*, so an
  untagged `Extra` would make the method-less `plainContext` shadow emit and consume a literal `"Extra"`
  key nested inside `context`. U1b scenario 2 asserts no such key appears.
* **Receivers are pinned** (`internal/events/checkpoint_schema.go` declares `Context CheckpointContext`
  as a **value** field, so a pointer-receiver marshaller would be silently skipped by `encoding/json`
  for any non-addressable value):
  * `func (c CheckpointContext) MarshalJSON() ([]byte, error)` — **value** receiver.
  * `func (c *CheckpointContext) UnmarshalJSON(b []byte) error` — pointer receiver.
  * `func (c CheckpointContext) Keys() ([]string, error)` — **value** receiver, and it returns
    `emit()`'s error rather than discarding it, so no caller can silently observe an empty list for a
    context that failed to serialize. U6 propagates that error alongside the write error.
* **Recursion guard**: `emit()` MUST NOT call `json.Marshal` on `CheckpointContext`. Build the object
  either through an ordered `bytes.Buffer` or by marshalling a
  `type plainContext CheckpointContext` shadow that carries no methods, then appending the sorted
  `Extra` entries.
* **HTML escaping MUST stay disabled.** Checkpoint bytes are written today through
  `jsonutil.MarshalReadable`, which sets `SetEscapeHTML(false)` — a shipped guarantee. Once
  `CheckpointContext` implements `json.Marshaler`, the outer encoder compacts the bytes `emit()`
  returns and re-escapes `<`, `>`, and `&` **only when its own `escapeHTML` flag is set**; the
  checkpoint write path uses `jsonutil.MarshalReadable`, which disables it, so `emit()` MUST NOT
  introduce escapes of its own. Concretely, `emit()` MUST marshal the `plainContext` shadow
  through `jsonutil.MarshalReadable` and append each `Extra` value's `json.RawMessage` bytes verbatim.
  `json.Marshal` MUST NOT appear anywhere in `emit()`, because it HTML-escapes `<`, `>`, and `&` by
  default and would silently ship `\u0026` inside `context` — a regression the existing guard cannot
  catch, since that guard exercises only a top-level field still encoded by the outer escape-free
  encoder. U1b scenario 4 is the guard.
* **Modeled-key set is derived, not hand-written (R13)**: compute the modeled key set once at package
  init by reflecting over `CheckpointContext`, taking `strings.Split(tag, ",")[0]` for each exported
  field and skipping `json:"-"` and unexported fields, through a package-level `var` initializer expression — no `init()` function and no `panic`. Non-emptiness is asserted in U1b scenario 3, not at runtime. `MarshalJSON`, `UnmarshalJSON`, and `Keys()` all
  read that one derived set so they cannot drift. It is an immutable lookup table computed at init, not
  mutable global state. Key comparison is **case-insensitive**, matching `encoding/json`'s own field
  matching, so a `context` supplying `Shipment_ID` behaves exactly as it does today (R14).
* **`MarshalJSON` MUST skip any `Extra` key that matches a modeled key**; the modeled field is
  authoritative and no duplicate key is ever emitted.
* **`Keys()` is derived from the emitted bytes, not from struct emptiness.** Every modeled field
  carries `,omitempty`, so a field that is set-but-empty is elided from disk. `Keys()` MUST return
  exactly the key set `MarshalJSON` emits, so `context_keys` can never name a key that is absent from
  the file. It returns `make([]string, 0)` — never a nil slice. **Mechanism**: `MarshalJSON` and
  `Keys()` both delegate to one unexported
  `func (c CheckpointContext) emit() (keys []string, body []byte, err error)`. `MarshalJSON` returns
  `body`; `Keys()` returns `keys` from the same call. This avoids the alternative — calling
  `MarshalJSON` from `Keys()` and discarding its error — which would both violate the
  never-discard-errors rule and return `[]` for a file that does carry context keys, recreating the
  success-shaped envelope R5 exists to remove.
* **Read-path safety (R4)**: `UnmarshalJSON` MUST reproduce `encoding/json`'s existing observable
  behavior for every degenerate `context` value — `null`, absent, a non-object scalar, and duplicate
  keys — because `ParseCheckpoint` decodes `CheckpointV1` and therefore runs this code on **every**
  read path (`ListCheckpoints`, `GetCheckpoint`, `ResolveCheckpoint`, and both disposition verbs). A
  divergence here reclassifies an existing on-disk file as corrupt and trips the plan's own rollback
  trigger. U3b and U3c are the guards.
* **Constraint**: `Extra` carries `json.RawMessage`, not `any`, so values survive without a
  decode/encode reshaping cycle — key order, number literals, and escapes are preserved; insignificant
  whitespace is compacted by `jsonutil.MarshalReadable`.
* **Risky action**: classified as PA-8.
* **Posture**: makes U1a and U1b green.
* **Acceptance**: U1a and U1b pass; U3b and U3c pass both before and after this unit; every new
  exported identifier carries a doc comment beginning with its own name; no existing checkpoint test
  regresses.

### U3a — Red harness for the closed schema namespace

* **Domain**: tests
* **Files**: `internal/events/checkpoint_create_strict_test.go` (new)
* **Scenarios**: (1) a V1 dump with an unknown top-level key fails create with an error satisfying
  `errors.Is(err, corerrors.ErrCheckpointUnknownField)` **and** naming that key, and writes no file;
  (2) a V1 dump with **two** unknown top-level keys names **both**, sorted, in a single error;
  (3) a V1 dump with an unknown key nested inside `progress` is rejected and the error names the
  nesting path, pinning the decided blast radius of the closed namespace (see U4).
* **Posture**: test-first. All three are red before U4.
* **Acceptance**: three failing assertions; no file written on rejection.

### U3b — Read-path regression guard for degenerate `context` values

* **Domain**: tests
* **Files**: `internal/events/checkpoint_readpath_test.go` (new)
* **Changes**: guard the observable behavior of `ParseCheckpoint` for every `context` shape that now
  flows through a hand-written `UnmarshalJSON`.
* **Scenarios**: (1) `"context": null`, absent `context`, `context` as a JSON string, and `context` as a
  number each parse with exactly the outcome they produce today; (2) a `context` object with duplicate
  keys parses with the same last-wins outcome as today; (3) a dump with no `schema_version` is written
  verbatim and is never subjected to the unknown-key probe.
* **Golden baseline**: the expected outcomes MUST be captured from the **pre-U2 commit** as a committed
  golden table (fixture → error class, parsed field values, re-marshalled bytes) and generated once
  before U2 begins. Writing the expectations after U2 is designed would encode post-change behavior as
  "today" and make the guard pass vacuously.
* **Posture**: regression guard. Green before U2, before U4, and after both.
* **Acceptance**: all three green at every commit boundary against the pre-U2 golden table.

### U3c — Legacy-corpus, misclassification, and drift guards

* **Domain**: tests
* **Files**: `internal/events/checkpoint_corpus_test.go` (new),
  `internal/events/testdata/legacy-corpus/` (new, **synthetic** committed fixtures)
* **Scenarios**: (1) a complete, schema-valid V1 dump that **also** carries unknown top-level keys still
  parses through `ParseCheckpoint` and is listed by `ListCheckpoints` with `NeedsQuarantine == false`;
  parse success alone does not imply an unflagged summary, so the fixture must satisfy
  `ValidateCheckpoint` too; (2) the synthetic legacy fixtures are materialized into `t.TempDir()`,
  listed through `ListCheckpoints`, and every entry's `(NeedsQuarantine, validation-error class,
  RemediationCommand)` triple is **identical to the pre-U2 golden table** — which records
  `NeedsQuarantine: true` with an `ErrCheckpointInvalid` validation error for `schema_version`-less
  fixtures, because `ListCheckpoints` sets that flag on `ValidateCheckpoint` failure and `CheckpointV1`
  requires `schema_version` plus six other fields. **The guard is that U2 and U4 change no
  classification, not that legacy files are clean.** One fixture per observed legacy shape is
  enumerated in the test table, so a fixture added or removed without updating expectations fails the
  test; the **live** corpus is additionally listed **read-only** and asserted to report no *new*
  `NeedsQuarantine` entries relative to the same baseline, which is what actually guards the real
  files; (3) a V1 dump with a
  malformed `created_at` still fails with `ErrCheckpointCorrupt`, not `ErrCheckpointUnknownField`, and
  a dump carrying `disposition_reason` — a modeled key whose tag has an `,omitempty` option — is
  **accepted**, pinning both the misclassification guard and the tag-option stripping (R13, R14).
* **Fixture provenance**: the fixtures are **hand-written synthetic** checkpoint documents modelled on
  the shapes observed in the live corpus — no `schema_version`, with unmodeled top-level keys such as
  `pr_number`, `pr_status`, `ci_status`, `decisions`, `next_steps`, `review_gate`, `items_blocked`, and
  `follow_up_tasks`. **No bytes are copied from `.backlogit/checkpoints/`**, and no test writes to it.
  Copying the live corpus was **rejected**: the plan's own Decisions table records checkpoint `context`
  as unredacted durable state, and `git revert` — the plan's only rollback mechanism, since Principle XI
  forbids history rewriting — cannot purge a committed blob, so a missed secret would be unrecoverable
  inside this plan's own rules, unlike every other unit. Synthetic fixtures satisfy R4 with no retention
  exposure, so **no ProposedAction is required for this unit**.
* **Posture**: regression guard. Green before U2, before U4, and after both.
* **Acceptance**: all three green at every commit boundary; no fixture file contains any live session
  identifier, pull-request URL, merge SHA, or resume hint.

### U3d — Red harness for the MCP validation outcome and tool contract

* **Domain**: tests
* **Files**: `internal/mcp/checkpoint_create_strict_test.go` (new)
* **Scenarios**: (1) dispatched through the registered `handleCreateCheckpoint`, a dump with two unknown
  top-level keys returns a **`validation_failed`** result — not `internal` — whose `unknown_fields`
  array is read with a `.([]any)` type assertion and contains both keys, sorted (R12); (2) the
  **registered** `backlogit_create_checkpoint` tool description contains the legal-key enumeration, the
  "arbitrary keys belong inside `context`" sentence, and the `context_keys` sentence, so the
  agent-facing half of R11 cannot ship stale the way the CLI half nearly did; (3) the legal-key
  enumeration in that description is **derived by reflection over `CheckpointV1` and
  `CheckpointProgress` in the test** and every derived key is asserted present in the description
  string, so a future modeled field cannot silently desynchronize the documented contract from the
  enforced one.
* **Posture**: test-first. All three red before U4b.
* **Acceptance**: three failing assertions, all on structured or registered content, never on message
  text.

### U4 — Create-boundary strict decode for the schema namespace

* **Domain**: code
* **Files**: `internal/events/memory.go`
* **Changes**: in `CreateCheckpoint`, after the `schema_version == 1` probe, use a **two-pass** decode:
  1. **Pass 1 — shape.** Run the existing `ParseCheckpoint` **first**, unchanged, so malformed
     `created_at`, wrong-typed `task_ids`, and every other shape error keep their current
     `ErrCheckpointCorrupt` classification and their existing `validation_failed` MCP mapping.
  2. **Pass 2 — unknown-key probe.** Only if pass 1 succeeded, decode the dump into
     `map[string]json.RawMessage`, diff its key set against the `CheckpointV1` tag set, and if the
     difference is non-empty return `&backlogiterrors.CheckpointUnknownFieldError{Fields: sorted}` **directly** (`internal/events` already imports the errors package under the `backlogiterrors` alias; `corerrors` is `internal/mcp`'s alias)
     — not wrapped again — so that `errors.Is(err, ErrCheckpointUnknownField)` matches through its
     `Unwrap` and `errors.As` recovers `Fields` without parsing any message. Locate the `progress` key
     with the **same case-insensitive comparison** used for the top-level diff and recurse the same
     diff one level into it, skipping the recursion when that raw value is `null` or the key is absent.
     A case-sensitive lookup would silently skip the recursion for a dump spelling it `Progress`, which
     `encoding/json` matches, letting unknown nested keys escape the closed namespace.
     Do **not** use `json.Decoder.DisallowUnknownFields()` and do **not** substring-match
     `"json: unknown field"`: the decoder reports only the first offending key, returns an untyped
     error, and `go.instructions.md` bans matching on error text.
* **Tag-set derivation (R13)**: derive the tag sets for `CheckpointV1` and `CheckpointProgress` by
  reflection through a package-level `var` initializer expression — **no `init()` function and no `panic`**, both of which the workspace Go conventions ban in library code. `strings.Split(tag, ",")[0]` for each exported field, skipping
  `json:"-"` and unexported fields. Non-emptiness is asserted **in tests** (U3d scenario 3), not at runtime. A hardcoded list would silently
  drift the next time either struct gains a field, which is exactly what happened when the four
  `disposition_*` fields were added. `context` is never treated as unknown.
* **Case-insensitive comparison (R14)**: `ParseCheckpoint` is `json.Unmarshal`, which matches object
  keys against struct tags **case-insensitively**, so a dump containing `"Schema_Version"` succeeds
  today. The probe MUST compare case-insensitively so no previously-succeeding dump is newly rejected —
  that is rollback trigger C.
* **`ParseCheckpoint` is not modified.** The closed namespace is enforced only on the create path.
* **Decided blast radius**: the closed namespace covers the `CheckpointV1` top level **and** the nested
  modeled `progress` object. It explicitly does **not** cover `context`, which U2 makes open. Extending
  closure to `progress` is a deliberate widening of the decision record's "top level" wording, recorded
  here and in R3 rather than left as an artifact of decoder mechanics; U3a scenario 3 pins it.
* **Posture**: makes U3a green.
* **Acceptance**: U3a green; U3b and U3c still green; no file is written on rejection; U3d remains red
  until U4b.

### U4b — Map the new sentinel to a structured MCP validation outcome

* **Domain**: code
* **Files**: `internal/mcp/errors.go`, `internal/mcp/tools.go` (tool-description string only)
* **Changes**: add a **dedicated** `domainError` case, placed before the general `ErrValidation` case,
  that recovers `*corerrors.CheckpointUnknownFieldError` via `errors.As` and marshals a
  `checkpointUnknownFieldsResponse{Error: "validation_failed", Message string, UnknownFields []string}`
  — mirroring the existing `WorkspaceRootAmbiguous` precedent at `internal/mcp/errors.go:141-146`,
  which is a dedicated case with its own response struct. Routing to the general `ValidationFailed`
  path would not work: it returns `errorResponse{Error, Message}` (`internal/mcp/errors.go:14-17`), a
  two-field struct with no room for `unknown_fields`, so U3d scenario 1's `.([]any)` assertion would
  fail. Add the sentinel row to the mapping table in `domainError`'s doc comment.
  Without this, `handleCreateCheckpoint`'s `domainError` call falls through to
  `default: InternalError(...)` and an agent cannot distinguish "you nested a key wrong" from a server
  fault. Pin the offending keys in a **structured field**, not in prose: recover
  `*corerrors.CheckpointUnknownFieldError` with `errors.As` and copy its `Fields` into an
  `unknown_fields []string` on the result — sorted, always an array, no `omitempty`. `errors.As` on a
  typed error is the repository's existing mechanism for carrying structured data out of an error
  (`*corerrors.AmbiguousWorkspaceRootError` → `WorkspaceRootAmbiguous(ambiguous.Roots)`); parsing
  `err.Error()` is forbidden. In the same unit, extend the `backlogit_create_checkpoint` tool
  description to **enumerate the legal `CheckpointV1` top-level and `progress` keys**, state that every
  other key belongs inside `context`, that unknown top-level and `progress` keys are rejected with an
  `unknown_fields` list, and that the result carries `context_keys`. Enumerating the legal keys is what
  turns the closed namespace from a guess-and-fail workflow into a discoverable contract; a description
  that says only "unknown keys are rejected" tells an agent nothing it can act on before its first
  failure. U3d scenario 3 derives the same key set by reflection and asserts every derived key appears
  in the description, so the enumeration cannot drift from the enforced set.
* **Width isolation**: both edits are Go source in one package, and the description string is the
  machine-consumed contract for the mapping being added. Treated as a single skill domain by design,
  not as a code-plus-docs mix.
* **Posture**: makes U3d green.
* **Acceptance**: all three U3d scenarios pass; the MCP result carries `validation_failed` and an
  `unknown_fields` array naming every offending key; the tool description enumerates every
  reflection-derived legal key.

### U5a — Red harness for `context_keys` on both transports

* **Domain**: tests
* **Files**: `internal/cli/checkpoint_create_test.go` (extend),
  `internal/mcp/checkpoint_create_context_test.go` (new). The cross-surface comparison lives in
  `internal/cli/checkpoint_create_test.go`. `internal/cli` already depends on `internal/mcp`, so an external `cli_test` package may import it without a cycle; the MCP half dispatches the registered tool through the exported server handle, never a handler method directly.
* **Changes**: assert that the create result carries the persisted context key names. The MCP scenario
  dispatches through the registered `handleCreateCheckpoint` with a `state_dump` string argument; the
  CLI scenario drives the `--state-dump` flag. Neither calls `events.CreateCheckpoint` directly,
  because argument extraction and marshalling are part of the loss path.
* **Scenarios**: (1) MCP result includes `context_keys` listing modeled and unmodeled keys; (2) CLI JSON
  output includes the same `context_keys` for the same dump; (3) both are byte-identical for the key
  list.
* **Posture**: test-first. All three red before U6.
* **Acceptance**: three failing assertions.

### U5b — Red harness for the empty and legacy `context_keys` shapes

* **Domain**: tests
* **Files**: `internal/cli/checkpoint_create_shape_test.go` (new). Every scenario asserts on **both**
  surfaces — the CLI `--state-dump` flag and the registered MCP `handleCreateCheckpoint` — because the
  degenerate legacy shapes are exactly where a surface could diverge.
* **Scenarios**: (1) a **legacy** dump with no `schema_version` whose `context` object carries keys
  yields a `context_keys` list naming exactly those keys, proving the legacy path reports what it
  actually wrote rather than defaulting to `[]`; (2) a table over four degenerate legacy dumps —
  `context` absent, `"context": null`, `"context": 42`, and a top level that is a JSON array rather
  than an object — each asserting the create **succeeds** and `context_keys` decodes as a present,
  empty `.([]any)`, an assertion that fails for both an absent key and a JSON `null`; (3) a V1
  dump whose `context` supplies `"task_ids": []` produces on-disk bytes and a `context_keys` list that
  agree exactly, so `omitempty` cannot make the result name a key the file does not have.
* **Posture**: test-first. All three red before U6.
* **Acceptance**: three failing assertions.

### U6 — Return and surface the persisted context keys

* **Domain**: code
* **Files**: `internal/events/memory.go`, `internal/mcp/tools.go`, `internal/cli/checkpoint.go`
* **Changes**: populate the `CreateCheckpointResult` that U0a declared, and adapt both surfaces:

  ```go
  type CreateCheckpointResult struct {
      Path        string   `json:"path"`
      ContextKeys []string `json:"context_keys"`
  }
  ```

  Neither field carries `omitempty`, and `ContextKeys` is always initialized with `make([]string, 0)`.
  **Both surfaces marshal this exact value**, mirroring the established `docline.LintReport` pattern
  where CLI and MCP encode the same type so their JSON is structurally identical by construction. The
  current `map[string]string{"path": path}` literals at `internal/cli/checkpoint.go:87` and
  `internal/mcp/tools.go:1126` are replaced, not extended — a `map[string]string` cannot carry a
  `[]string` at all.
* **Both write paths populate the list from the bytes that were written.** On the V1 path
  `ContextKeys` comes from `CheckpointContext.Keys()`, whose error is **propagated**, never discarded —
  `Keys()` reads from the same `emit()` that produced the written bytes, so no `Keys()` error can
  follow an applied write. On the **legacy verbatim path** it comes from
  scanning the top-level `context` object of the written bytes into `map[string]json.RawMessage`, then
  sorting the extracted keys with `sort.Strings` — no schema assumption, no parse of the rest of the
  document. Sorting is mandatory: Go map iteration order is randomized, so an unsorted legacy list
  would make `context_keys` nondeterministic run-to-run and differ in ordering from the V1 path.
  Returning `[]` for a legacy dump that
  actually persisted a rich `context` would be exactly the shaped result this plan exists to remove.
  **The legacy scan is strictly best-effort and MUST NOT fail the create.** The legacy branch is
  reached for *any* input the `schema_version` probe rejects, including invalid JSON, a JSON array, and
  a bare scalar — all of which succeed today and are written verbatim. If the written bytes do not
  decode into `map[string]json.RawMessage`, or `context` is absent, `null`, or not a JSON object, then
  `ContextKeys` is `make([]string, 0)` and the create still succeeds. The write error is the only
  failure mode on this path. A scan failure must never undo, prevent, or misreport an applied write —
  that would be the same success-shaped inversion this plan exists to remove, and it is the plan's own
  rollback trigger C.
* **Help text**: update `newCheckpointCreateCmd`'s GoDoc, `Short`, `Long`, and `Example` strings so they
  describe the open `context`, the closed schema namespace, and `context_keys`. These strings are the
  source the generated CLI reference is built from, so leaving them stale would surface as an
  unexplained `make docs` diff in U10a and would make R11 unsatisfiable there.
* **Scope note**: three production files. The signature change already landed in U0a, so this unit is
  behavior-only, but the three surfaces must change together to keep `context_keys` consistent.
  Recorded as a 2-Hour Rule file-count exception. **Width isolation is also relaxed**: the same commit
  edits Cobra help strings, which are contract text emitted by code. Rejected alternative: a separate
  help-text unit landing after U6, rejected because U10a would then regenerate the CLI reference
  against stale Cobra strings and R11 would ship unsatisfied for the checkpoint half.
* **Posture**: makes U5a and U5b green.
* **Acceptance**: U5a and U5b pass; `context_keys` serializes as `[]`, never `null`; every new exported
  identifier carries a doc comment beginning with its own name.

### U7a — Red harness for report-and-continue lint

* **Domain**: tests
* **Files**: `internal/docline/lint_decode_error_test.go` (new)
* **Changes**: build a real on-disk fixture corpus containing one file whose YAML frontmatter cannot be
  decoded plus at least two files with ordinary contract violations, then assert `LintTree` returns
  findings for every file.
* **No production seam is introduced.** `mdfront.Decode` already returns a hard error on malformed
  YAML, so a genuine fixture such as `---\ntitle: [unclosed\n---\nBody.\n` produces a path-selective
  decode failure with no injection at all. The previously contemplated package-level `decodeDocFn`
  variable is **rejected**: it is global mutable state in a production package (an explicit
  `go.instructions.md` anti-pattern), and it would have forced the removal of `t.Parallel()` calls that
  already exist in `internal/docline`. Existing `t.Parallel()` calls are retained.
* **Scenarios**: (1) the undecodable file yields exactly one finding with `Rule: RuleDecodeError` and
  `Severity: SeverityError`; (2) findings for the remaining files are still present, proving the scan
  continued; (3) a corpus whose only problem is the undecodable file still yields a non-empty findings
  slice rather than a nil slice with an error. The retained non-zero CLI exit for that corpus is
  asserted by U9a scenario 1; U7a stays inside `internal/docline`, which does not import
  `internal/cli`.
* **Posture**: test-first. All three red before U8.
* **Acceptance**: three failing assertions. The `classifyDecodeFailure` convergence-lock table test
  belongs to U8, which introduces the helper; U7a must not reference it, because U7a lands before U8
  and the identifier does not exist at its own commit boundary.

### U7b — Containment and sanitization guards for the lint scan

* **Domain**: tests
* **Files**: `internal/docline/lint_containment_test.go` (new)
* **Construction constraint for scenario 2**: the unreadable-file case MUST be constructed portably by
  calling `decodeDoc(root, rel)` with a `rel` that resolves to a **directory**, which makes
  `os.ReadFile` fail on every platform while leaving `core.SafeResolve` satisfied. A `chmod 0o000`
  construction is **rejected**: it is a no-op on Windows, this workspace's development platform, so the
  scenario would be an unfixable red there.
* **Construction constraint for scenario 1**: a `Path: "../escape"` fixture is **not** sufficient and may not be the
  sole construction — `collectInScopeDocs` rejects it at `service.go:226-229` before the `LintTree` loop is
  reached, so it would stay green even against a U8 that wrongly downgrades a `decodeDoc`-internal
  containment failure. Scenario 1 MUST call the unexported `decodeDoc(root, rel)` directly — reachable
  because `internal/docline` tests are in-package — with a `rel` that makes its own `SafeResolve` fail,
  assert `errors.Is(err, ErrPathEscapesWorkspace)` and — because the call is made directly against `decodeDoc`, the provenance of the containment failure is established by construction rather
  than by inspecting the message; no assertion may read `err.Error()`. It **also** asserts that `LintTree` over the same corpus returns
  `nil, err`. It must not reference `classifyDecodeFailure`, which does not exist until U8.
* **Scenarios**: (1) a `decodeDoc` failure carrying `ErrPathEscapesWorkspace` still makes `LintTree`
  return that error with a nil findings slice — it is **never** downgraded to a finding, and
  `errors.Is(err, ErrFrontmatterDecode)` is false for it; (2) an unreadable file (an `os.ReadFile`
  failure) also still returns an error rather than a finding, and no finding's `File` or `Fix` string
  contains the workspace absolute path prefix — every one is repo-relative POSIX, matching the rest of
  the package.
* **Posture**: containment regression guard (Principle III). Green before and after U8.
* **Acceptance**: both green at every commit boundary.

### U8 — Accumulate `decode_error` findings and continue scanning

* **Domain**: code, plus the in-package characterization test for the unexported helper it introduces
* **Files**: `internal/docline/service.go`, `internal/docline/report.go` (doc comment only),
  `internal/docline/classify_decode_failure_test.go` (new, `package docline`)
* **Scope note**: four files — two production edits, one doc-comment correction, and the helper's
  characterization table test. Recorded as a 2-Hour Rule file-count and width-isolation exception;
  rejected alternative: a separate post-U8 test unit, rejected because U7a must not reference
  `classifyDecodeFailure` (it lands first) and a convergence lock deferred past U8 leaves the
  three-way split unpinned at U8's own commit boundary. `internal/docline/policy.go` is **not** in the
  list: U0b already lands `RuleDecodeError` and `ErrFrontmatterDecode` there.
* **Changes**: in `LintTree`, replace the blanket `return nil, err` on `decodeDoc` failure
  (`internal/docline/service.go:97-99`) with an **explicit three-way split**, discriminated by
  sentinels rather than by error text:
  1. `errors.Is(err, ErrPathEscapesWorkspace)` (raised at `service.go:289`, inside `decodeDoc`, **not**
     only in `collectInScopeDocs`) → still `return nil, err`. A workspace-containment violation is a
     NON-NEGOTIABLE Principle III control and must never become a success-shaped finding. MCP continues
     to map it to `ValidationFailed`.
  2. `errors.Is(err, ErrFrontmatterDecode)` → a per-file finding and `continue`.
  3. anything else — notably the `os.ReadFile` failure at `service.go:291-293`, whose `*fs.PathError`
     carries the **absolute** host path → still `return nil, err`. It is a broken invocation, not
     malformed data.
* **The discrimination lives in one unexported helper**,
  `classifyDecodeFailure(err error) (rule string, fatal error)` in `internal/docline/service.go`,
  called by `LintTree`. It returns `(RuleDecodeError, nil)` for a frontmatter-decode failure and
  `("", err)` for everything that must propagate, so the two outcomes are mutually exclusive and
  neither is expressed as a bare boolean. It MUST NOT return `("", nil)`: a `nil` input is a programming error and returns `("", errors.New("docline: classifyDecodeFailure called with nil error"))`, so `LintTree` either appends a finding with a non-empty rule or propagates an error. `PlanMigration` adopting the same helper is the recorded
  convergence follow-up, so a second decode policy over the same frontmatter grammar is never written.
  A table test over containment, decode, read-failure, and nil inputs is the convergence lock.
* **The discriminator must exist before it can be used.** `decodeDoc` currently wraps both the read and
  the decode failure with an opaque `%w` over an untyped error, so there is nothing to branch on today.
  Wrap the `mdfront.Decode` branch at `service.go:297` with the `ErrFrontmatterDecode` sentinel U0b
  declared — `fmt.Errorf("docline.decodeDoc: decode %s: %w: %w", rel, ErrFrontmatterDecode, err)` —
  so `LintTree` branches purely on `errors.Is`. **No error text is ever inspected.**
* The finding carries `Rule: RuleDecodeError`, `Field: "frontmatter"`, `Severity: SeverityError`, `File`
  set to the repo-relative POSIX path, and a **sanitized** `Fix` built from the structured decode cause
  — never `err.Error()` of a wrapped `*fs.PathError`, and never a matched error substring. The `Fix`
  string must contain no absolute path.
* **`report.go` changes only its doc comment.** R9 is *already satisfied* today: `LintReport.Findings`
  carries no `omitempty` and `NewLintReport` pre-allocates with `make([]FindingReport, 0, len(findings))`.
  U8 must not weaken either. The one edit is to the `LintReport` doc comment's claim that "transport
  errors are reserved for … parse failures", which this unit makes false.
* **Posture**: makes U7a green.
* **Acceptance**: U7a passes and U7b still passes; a table test over containment, decode, read-failure,
  and nil inputs pins `classifyDecodeFailure` — this is the convergence lock; `LintTree` never returns `nil, err` for a
  frontmatter-decode failure and always returns `nil, err` for a containment or I/O failure; every new
  exported identifier carries a doc comment beginning with its own name.

### U8b — Update the lint surface contract text

* **Domain**: code
* **Files**: `internal/cli/docs.go` (Cobra strings only), `internal/mcp/docs_tools.go` (tool description
  string only)
* **Changes**: **add** a `Long` and an `Example` to `newDocsLintCommand` — which today declares neither — extend its `Short`, and update the
  `backlogit_docs_lint` MCP tool description to state that a malformed frontmatter file produces a
  `decode_error` finding and the scan continues, and that containment and I/O failures still fail the
  invocation. The **CLI** text additionally states that the non-zero exit is retained; the **MCP**
  description instead states that findings are returned in a successful tool result rather than
  failing the tool call, because MCP has no exit code. Without the Cobra edit, U10a's regeneration of
  `docs/cli-reference/backlogit_docs_lint.md` produces no diff and R11 ships unsatisfied for the lint
  half, exactly as it would have for the checkpoint half.
* **Width isolation**: both edits are Go source; the description strings are machine-consumed contract
  text, not a docs artifact.
* **Posture**: follows U8 so the text describes shipped behavior.
* **Acceptance**: both strings name the `decode_error` rule value; the CLI text names the retained
  non-zero exit and the MCP description names the successful-tool-result behavior.

### U9a — Lock the degraded-corpus shape and exit code on both surfaces

* **Domain**: tests
* **Files**: `internal/cli/docs_test.go` (extend), `internal/mcp/docs_tools_test.go` (extend)
* **Scenarios**: (1) `backlogit docs lint` still exits non-zero and prints a report containing the
  `decode_error` finding; (2) the marshalled JSON is inspected directly for a **present and non-null**
  `findings` array using a `.([]any)` type assertion, which fails for both an absent key and a JSON
  `null`; (3) MCP returns a successful tool result carrying the same findings rather than
  `InternalError`, and the **registered** `backlogit_docs_lint` tool description names the
  `decode_error` rule value and states that a malformed document is returned as a finding in a
  successful tool result rather than failing the tool call. The MCP description does **not** mention
  exit codes — MCP has none, and importing a shell concept into an agent-facing tool contract is a
  leaky abstraction (R11, agent-facing half).
* **Constraint**: `TestDocsTools_CLIParity` is not sufficient on its own. Both sides marshal the same
  struct, so a shape defect drifts identically on both sides and compares equal. Scenario 2 must read
  the marshalled JSON, not the producing struct.
* **Posture**: test-first. Scenario 3 fails against today's `handleDocsLint`, so all three are run once
  against pre-U8 HEAD and their failure recorded, then re-run after U8. The surface-level red is
  observed even though the unit is authored alongside U8.
* **Red-result contingency**: if a scenario is still red after U8, U9a becomes the red harness for a new
  unit **U9x** — `internal/docline/report.go`, guarantee a non-nil `Findings` slice at the producing
  boundary, acceptance "the assertion in the failing scenario passes without being weakened" — which
  must land before U10a. U9a may **not** be marked green by relaxing the `.([]any)` assertion.
* **Acceptance**: all three pass; the existing parity test still passes unchanged.

### U9b — Lock the clean-corpus always-array shape

* **Domain**: tests
* **Files**: `internal/cli/docs_shape_test.go` (new)
* **Scenarios**: (1) a **clean** corpus with no findings yields a present `[]` under the same `.([]any)`
  assertion on both surfaces. `LintTree` declares `var findings []Finding` and returns a nil slice for
  a clean corpus; `NewLintReport`'s pre-allocation is what converts it to `[]`, and this unit pins that
  conversion so U8's edits to the producing path cannot break it.
* **Posture**: **green-throughout shape guard, not a red harness.** `NewLintReport` already
  pre-allocates `make([]FindingReport, 0, len(findings))` and `Findings` carries no `omitempty`, so its
  existing pre-allocation already converts `LintTree`'s nil slice to `[]` at the transport boundary.
  This unit pins that behavior against U8's changes to the producing path; it is green at every commit
  boundary. If it is unexpectedly red, the U9x contingency applies and U9x is added to R9's
  implementing units.
* **Acceptance**: green on both surfaces, before and after U8.

### U10a — Regenerate the tracked CLI reference

* **Domain**: docs
* **Files**: `docs/cli-reference/backlogit_checkpoint_create.md`,
  `docs/cli-reference/backlogit_docs_lint.md`
* **Changes**: regenerate through `make docs` rather than hand-editing, and commit the regenerated
  output so the CI cli-reference-drift check stays green. The substantive content arrives from the Cobra
  strings edited in U6 (checkpoint) and U8b (lint), so this unit is pure regeneration.
* **Posture**: docs-only. Runs after U6, U8, and U8b so the generated help text reflects shipped
  behavior.
* **Acceptance**: `backlogit_checkpoint_create.md` contains the open-`context`, closed-namespace, and
  `context_keys` sentences; `backlogit_docs_lint.md` contains the `decode_error` and retained-non-zero-
  exit sentences; a second `make docs` run produces no further diff.

### U10b — Document both contracts in the authoring and agent instruction surfaces

* **Domain**: docs
* **Files**: `docs/docline-frontmatter-authoring-guide.md`,
  `.github/instructions/backlogit.instructions.md`
* **Changes**: document the `decode_error` rule value and the report-and-continue behavior with its
  retained non-zero exit. Update the backlogit instruction file's Continuity Protocol, which today tells
  agents to persist "outcome, changed files or surfaces, decisions, blockers, and next steps" without
  saying **where**: state explicitly that for a `schema_version: 1` dump those keys belong **inside
  `context`**, that the top level and `progress` are closed, and that a rejected create means "retry
  with the keys nested under `context`", not "session state is lost". Also record that checkpoint
  `context` is unredacted durable state written to a git-tracked path and must not carry secrets.
* **Generated-artifact caveat**: `.github/instructions/backlogit.instructions.md` is autoharness-
  generated from `backlogit.instructions.md.tmpl`, which lives in a repository this plan treats as
  strictly read-only (Principle IV, PA-5). The in-repo edit is applied for immediate effect **and**
  recorded as a follow-up backlog item to upstream the same wording into the template; otherwise the
  next regeneration silently reverts this shipment's own writer-migration guardrail. The durable,
  non-generated statement of the contract is U4b's `backlogit_create_checkpoint` tool description.
* **Writer-migration ordering**: this instruction update MUST land before or with the merge, because an
  agent following today's wording while emitting `schema_version: 1` would hit the new rejection with
  no fallback.
* **Risky action**: classified as PA-7.
* **Posture**: docs-only.
* **Acceptance**: the authoring guide names the new rule value; the instruction file names `context` as
  the destination for arbitrary recovery keys and carries the secrets caveat; the template follow-up
  item exists in the backlog.

## Dependency graph

```text
U0a --> U1a --> U2
U0a --> U1b --> U2
U0a --> U3a --> U4
U0a --> U3b --> U2
U0a --> U3c --> U2
U2  --> U4  --> U4b --> U6
U0a --> U3d --> U4b
U2  --> U6
U0a --> U5a --> U6
U0a --> U5b --> U6
U6  --> U10a
U6  --> U10b
U0b --> U7a --> U8
U0b --> U7b --> U8
U8  --> U8b --> U9a --> U10a
U0b --> U9b --> U8
U9a --> U10b
```

* `U8b` precedes `U9a`, because U9a scenario 3 asserts the tool-description text U8b writes.
* `U9a` precedes `U10b`, because U10b documents the shipped `decode_error` contract; the authoring guide
  and the instruction file may not describe lint behavior that has not yet landed. This edge also
  orders `U10b` transitively after `U0b`, `U9b`, `U8`, and `U8b`.
* `U0b` precedes `U9b` as a track-entry convention only; unlike `U7a` and `U7b`, `U9b` asserts no
  prelude identifier.
* `U9b` precedes `U8`, because its clean-corpus shape guard must be observed green **before** U8 changes
  the producing path; its acceptance is "green before and after U8".
* **`U0a` precedes every Track A harness and `U0b` precedes every Track B harness.** They declare the
  identifiers each harness asserts against — `CheckpointUnknownFieldError`, `Extra`, the `Keys()` stub,
  and `CreateCheckpointResult` for Track A; `RuleDecodeError` and `ErrFrontmatterDecode` for Track B —
  so each harness commit compiles and its red result is observable. The two preludes share no file and
  no commit, so the two source entries stay genuinely independent.
* `U3b` and `U3c` precede `U2` because they are the pre-change golden-baseline guards; their expected
  outcomes must be captured before `UnmarshalJSON` is hand-written.
* `U2` must land before `U4`, because the unknown-key probe must not treat the open `context` object as
  a violation.
* `U4` precedes `U4b` (the sentinel must exist to be mapped) and `U4b` precedes `U6` (both edit
  `internal/mcp/tools.go`; the description edit lands before the result-shape edit).
* `U4` precedes `U6`: both modify `CreateCheckpoint` in `internal/events/memory.go`, and U6 populates a
  result behind an already-strict boundary.
* `U8b` precedes `U10a`, because the generated lint reference is built from the Cobra strings U8b edits.
* Track A (`U0a`, `U1a`–`U6`) and Track B (`U0b`, `U7a`–`U9b`) share no unit and may run in either
  order or concurrently.
* `U10a` and `U10b` are last on both tracks, and the graph now encodes that: every terminal test and
  code unit has an outgoing edge into one of them. `U9a --> U10a` exists so the generated lint reference
  is regenerated only after the degraded-corpus shape is locked.
* The graph is acyclic. `U9x` (the U9a red-result contingency) is not scheduled; it materializes only if
  a U9a scenario is still red after U8, in which case it inserts as `U9a --> U9x --> U10a`.

## Scope boundaries and recorded follow-ups

These are deliberate exclusions, recorded here so the plan is not read as a complete elimination of
checkpoint evidence loss. Every follow-up is owned by Stage and is created as a backlog item during the
harvest that decomposes this plan, so none of them can leak back into this shipment's scope.

| Excluded surface | Why it is excluded | Residual exposure | Follow-up |
|---|---|---|---|
| `core.AbandonCheckpoint` (`internal/core/checkpoint_disposition.go`) | It performs the same parse → mutate → re-marshal round-trip on a **pre-existing** file. U2 fixes its `context` loss as a side effect, but unmodeled **top-level** keys are still dropped on abandon. Fixing it needs a raw top-level carrier or a refusal-to-mutate policy, which is a distinct design decision from the create boundary | Any checkpoint on disk carrying unmodeled top-level keys loses them when abandoned. The nine live files all carry such keys | Backlog item: "preserve or refuse on unmodeled top-level keys in checkpoint disposition rewrites" |
| `events.ResolveCheckpoint` | Same parse/mutate/re-marshal shape | Same as above on resolve | Same backlog item |
| **Indeterminate durable-write reporting for `CreateCheckpoint`** | Returning a populated result alongside an indeterminate-write error is the right contract, but `CreateCheckpoint` writes through `syncWriteFileAtomic` (`internal/events/fsutil.go`), which today has **no outcome classification and no injectable seam**, and neither the MCP `errorResponse` nor the CLI error path can carry a `path` or a key list. Delivering it needs a classified write result, a handler-level seam, and a new error envelope on both surfaces — none of which is traceable to source entry `3C7AAC71` or `90F2A9F8` | A create whose write is indeterminate still returns a bare error, so the caller cannot tell "probably written" from "not written" | Backlog item: "classify `syncWriteFileAtomic` outcomes by **converging `internal/events` onto the existing `internal/atomicfile` leaf**, which already tags `ErrWriteNotApplied` and `ErrWriteIndeterminate`, rather than adding a second classifier inside the private duplicate — then surface indeterminate creates with path and context keys on both transports" |
| `MigrateReport.Applied` / `Skipped` (`internal/docline/report.go`) | These sibling collection fields carry `omitempty` and therefore vanish on a zero-apply run — a known adjacent instance of the always-an-array contract | A zero-apply migrate report omits both arrays | Backlog item: "drop `omitempty` from `MigrateReport` collection fields" |
| `ApplyMigration` | Its all-or-nothing abort is a deliberate **write-path** safety invariant with its own preflight and TOCTOU guard | None introduced | None |
| `PlanMigration` | Excluded, but the write-path rationale does **not** apply: `PlanMigration` is read-only, so after U8 `docs lint` reports-and-continues on a frontmatter decode failure while `docs migrate --plan` still aborts the whole corpus on the first one — a real divergence in the same package over the same grammar | Two decode policies over one frontmatter grammar | Backlog item: "converge `LintTree` and `PlanMigration` on one `decodeDoc` failure-classification helper" |
| Registry governance for `create_checkpoint` | This shipment does **not** add a `governed: true` marker or a `governed_name` to `.autoharness/backlog-registry.yaml`, because the governed-parity design requires a named behavioral fixture per marker and that is a separate decision. Neither the registry nor the MCP-to-CLI fallback guide describes a `create_checkpoint` result shape or flag semantics affected by U6, so no drift is introduced | `create_checkpoint` remains ungoverned, with CLI/MCP parity enforced by U5a rather than by the registry drift test | Backlog item: "decide whether `create_checkpoint` becomes a governed operation" |
| `.github/instructions/backlogit.instructions.md` upstream template | The file is autoharness-generated and its template lives in a repository this plan treats as strictly read-only (Principle IV, PA-5), so the durable fix cannot be applied from here | U10b's writer-migration guardrail is silently reverted by the next autoharness regeneration | Backlog item: "upstream the checkpoint `context` Continuity Protocol wording into `backlogit.instructions.md.tmpl`" |
| CLI vs MCP error-shape asymmetry on unknown-key rejection | MCP carries a structured `unknown_fields []string` array (U4b); the CLI prints a wrapped error string. Adding a JSON error-serialization path to the CLI is a general output-contract change affecting every CLI error, not just this one, and is untraceable to either in-scope source entry | An agent driving the CLI must read the message text to learn which keys were rejected, while an agent driving MCP gets a parseable list | Backlog item: "provide a structured JSON error envelope for CLI validation failures mirroring the MCP shape" |
| `.backlogit/checkpoints/` gitignore posture | Deliberately unchanged. Opening the `context` namespace makes it easier for an agent to persist arbitrary session state to a **git-tracked** path, but changing the tracking posture mid-shipment would rewrite unrelated repository conventions | Unredacted agent-supplied `context` continues to be committed to git; mitigated only by U10b's documented caveat | Backlog item: "decide gitignore and redaction posture for checkpoint `context`" |

## Decisions and rationale

| Decision | Rationale |
|---|---|
| `context` is open, the top level and `progress` are closed | The caller owns `context`; the schema owns the rest. Preserving caller payload keeps the structured recovery data agents demonstrably need, while a closed schema namespace stays enforceable and tells an agent immediately when it put a key in the wrong place. Closing `progress` is a deliberate widening of the decision record's "top level" wording, recorded in R3 rather than left implicit |
| Strictness lives in a create-only two-pass decode | `ParseCheckpoint` is shared with `ListCheckpoints` and `GetCheckpoint`. Tightening it would mark existing on-disk checkpoints as quarantine candidates. Running `ParseCheckpoint` **first** also keeps shape errors classified as `ErrCheckpointCorrupt` instead of being swallowed by an unknown-field rejection |
| The unknown-key probe is a reflection-derived, case-insensitive map diff | The decoder's `DisallowUnknownFields` reports only the **first** unknown key, returns an untyped error, and would force substring matching — banned by the workspace Go conventions. A hand-written key list would silently drift the next time `CheckpointV1` gains a field, exactly as it did when the four `disposition_*` fields landed. Case-insensitivity matches `json.Unmarshal`'s own field matching so no dump that succeeds today is newly rejected |
| The reflected tag sets implement a **frozen declared version**, not automatic schema admission | Deriving the create-boundary allowlist from `CheckpointV1` means a newly tagged field would silently widen what `schema_version: 1` accepts. Policy: adding a modeled top-level or `progress` key requires either a new `CheckpointV2` with explicit reader/writer compatibility, or an explicitly documented V1 compatibility exception recorded in the plan or design docs. U1b scenario 3 (the `context` key set) and **U3d scenario 3** (every reflection-derived `CheckpointV1`/`CheckpointProgress` key must appear in the hand-written tool description) assert against literal or manual expectations, so such a widening forces a deliberate update; U3c scenario 3 pins tag-option stripping |
| Two declaration-only prelude units (U0a, U0b) land first | Ten harness units — eight on Track A, two on Track B — assert against identifiers their green units create. Without the preludes those commits do not compile, so the Principle II red result is unobservable rather than merely weak. They are split by track so the two source entries share no commit |
| `Extra` is declared `json:"-"` | A defined-type conversion copies field tags, so an untagged `Extra` would make the method-less shadow type emit and consume a literal `"Extra"` key nested inside `context` |
| `Extra` holds `json.RawMessage` | A `map[string]any` round-trip can reshape numbers and drop key order. Raw values are preserved without a decode/encode reshaping cycle |
| `Keys()` is derived from the emitted bytes, on both write paths | Every modeled context field carries `omitempty`. Deriving `context_keys` from struct emptiness would let the result name a key the file does not contain, and hard-coding `[]` for the legacy path would hide a rich `context` that was actually written — both are new success-shaped envelopes inside the fix for success-shaped envelopes |
| Frontmatter-decode failures become findings; containment and I/O failures stay errors, discriminated by sentinel | A malformed document is data the report should describe. A path escape is a NON-NEGOTIABLE containment control and an unreadable file is a broken invocation; neither may be downgraded to a finding. `decodeDoc` wraps both failures identically today, so a new `ErrFrontmatterDecode` sentinel is required before `LintTree` can branch without inspecting error text |
| Both fixes land at the shared core seam and both surfaces marshal one pinned type | Fixing in `internal/events` and `internal/docline` rather than per surface makes CLI/MCP parity structural. Pinning a single wire type — as `docline.LintReport` already does — removes the tag, nil-slice, and key-ordering drift that two independent map literals would reintroduce |
| Indeterminate-write reporting is **descoped**, not deferred silently | It is the right contract but needs a classified write result, a handler seam, and a new error envelope on both surfaces — none traceable to the two in-scope source entries. Recorded in the Scope boundaries table with a named backlog follow-up |
| `AbandonCheckpoint`, `ResolveCheckpoint`, `PlanMigration`, and `ApplyMigration` are untouched | Recorded with residual exposure and owned follow-ups in the Scope boundaries table rather than left implicit |
| Unbounded `context` size and depth are **accepted**, not capped | The caller is a local agent that can already write an arbitrarily large legacy dump through the existing verbatim path, so no new capability is created. A cap would reintroduce exactly the silent truncation this plan exists to remove. Explicitly accepted under the local-agent trust model |
| Checkpoint `context` is unredacted, git-tracked durable state | `.backlogit/checkpoints/` is **not** gitignored and its files are tracked. Opening the namespace makes it easier for an agent to persist arbitrary session state there. This shipment does not change the gitignore posture; U10b documents the caveat and the Scope boundaries table records the follow-up |

## Risks and caveats

| Risk | Mitigation | Unit |
|---|---|---|
| Strict decode leaks into the read path and quarantines the legacy corpus | U3c scenarios 1 and 2 assert `ParseCheckpoint` stays lenient and list synthetic legacy fixtures and read the live corpus read-only, verified green before **and** after U4 | U3c, U4 |
| The custom `UnmarshalJSON` — not the strict decode — changes read-path decode semantics for degenerate `context` values | U3b pins `null`, absent, scalar, and duplicate-key `context` against a **pre-U2 golden table**. This is a distinct mechanism from the strict-decode risk above and needs its own guard | U2, U3b |
| A regression guard written after the change encodes post-change behavior as "today" and passes vacuously | U3b and U3c require the golden expectations to be captured from the pre-U2 commit and committed before U2 begins | U3b, U3c |
| A harness unit cannot compile because it asserts identifiers its green unit creates, so the red is never observable | U0a and U0b declare every new exported identifier first, with no behavior change, and share no commit | U0a, U0b |
| A pointer-receiver `MarshalJSON` is silently skipped for the non-addressable `Context` value field, reproducing the defect | U2 pins value receivers explicitly; U1b scenario 1 marshals a non-addressable value | U1b, U2 |
| A recursive `MarshalJSON` (`json.Marshal(c)` inside `MarshalJSON`) | U2 mandates a buffer or a method-less shadow type; U1b scenario 1 fails a recursive implementation by stack exhaustion | U1b, U2 |
| An untagged `Extra` field makes the shadow type emit a literal `"Extra"` key inside `context` | U2 declares `Extra map[string]json.RawMessage \`json:"-"\``; U1b scenario 2 asserts no such key appears | U1b, U2 |
| An `Extra` key collides with a modeled tag and emits a duplicate key, silently overwriting `ShipmentID` on the next read | U2 skips colliding keys using the derived modeled-key set; U1b scenario 2 asserts the collision case | U1b, U2 |
| The modeled-key set or the `CheckpointV1` tag set is hand-written and drifts the next time a field is added | Both are derived by reflection at package init; U1b scenario 3 and U3c scenario 3 are the drift guards | U1b, U2, U3c, U4 |
| An exact-match key diff rejects a dump that `json.Unmarshal` accepts today because of case-insensitive field matching | U4 compares case-insensitively; this is rollback trigger C and R14 | U3c, U4 |
| A modeled key whose tag carries `,omitempty` is treated as unknown because the option was not stripped | U4 derives tag names with `strings.Split(tag, ",")[0]`; U3c scenario 3 pins `disposition_reason` as accepted | U3c, U4 |
| `context_keys` names a key that `omitempty` elided from the file | `Keys()` is derived from the emitted bytes; U5b scenario 3 asserts file/result agreement for `"task_ids": []` | U2, U5b |
| The legacy path reports `context_keys: []` even though a rich `context` was persisted | U6 scans the written bytes on the legacy path too; U5b scenario 1 asserts the keys are reported | U5b, U6 |
| Custom marshalling breaks `CheckpointFilter` on `shipment_id` / `feature_id` | U1a scenario 3 asserts end-to-end through `ListCheckpoints` with a real filter, not just through the struct accessors | U1a, U2 |
| A shape error (bad `created_at`) is misclassified as an unknown-field rejection | U4 runs `ParseCheckpoint` first; U3c scenario 3 pins `ErrCheckpointCorrupt` for a malformed timestamp | U3c, U4 |
| The new sentinel surfaces on MCP as a generic `internal` error, so an agent cannot self-correct | U4b maps it to `ValidationFailed` with a structured `unknown_fields` array; U3d asserts the classification and the field | U3d, U4b |
| A literal reading of "replace `return nil, err` on `decodeDoc` failure" downgrades `ErrPathEscapesWorkspace` — a NON-NEGOTIABLE Principle III control — into a success-shaped finding | U8 mandates an explicit sentinel-discriminated three-way split; U7b scenario 1 asserts the containment error still propagates | U7b, U8 |
| `LintTree` cannot distinguish a read failure from a decode failure, so the implementer reaches for substring matching | U0b declares `ErrFrontmatterDecode` and U8 wraps it at the `mdfront.Decode` branch, so the branch is pure `errors.Is` | U0b, U8 |
| The `decode_error` `Fix` string leaks an absolute host path from an `*fs.PathError` | U8 keeps `os.ReadFile` failures as errors and requires a sanitized repo-relative `Fix`; U7b scenario 2 asserts no absolute prefix appears | U7b, U8 |
| A package-global test seam causes flaky parallel runs or forces unscoped edits to existing tests | The `decodeDocFn` seam is **rejected**; U7a uses a real malformed fixture and existing `t.Parallel()` calls are retained | U7a |
| Byte-parity test yields a false green on the new report shape | U9a scenario 2 inspects the marshalled JSON directly with a `.([]any)` assertion instead of relying on struct comparison | U9a |
| The clean-corpus path — the one that can actually serialize `null` — goes untested | U9b asserts the empty case on both surfaces | U9b |
| A red-first test bypasses the transport and proves nothing | U5a dispatches through the registered MCP handler and the CLI flag; U3d dispatches through the registered handler | U3d, U5a |
| Continuing after a decode error silently flips CI to green | U9a scenario 1 asserts the non-zero exit is retained on a malformed corpus | U9a |
| An agent following current instructions emits `schema_version: 1` with top-level recovery keys, hits the new rejection, and loses its session snapshot | U10b updates the Continuity Protocol before merge, and U4b's `unknown_fields` array names every offending key so the retry is mechanical | U4b, U10b |
| U10b's guardrail is silently reverted because the instruction file is autoharness-generated | U10b records the generated status and an owned backlog follow-up to upstream the template; U4b's tool description is the non-generated durable statement | U4b, U10b |
| Generated CLI reference drifts from shipped help text, or `make docs` produces no diff because no help string changed | U6 edits the checkpoint Cobra strings and U8b edits the lint Cobra strings; U10a's acceptance requires the new sentences to be present | U6, U8b, U10a |

## Constitution Check

| Principle | Verdict | Notes |
|---|---|---|
| I. Safety-First Go | pass | All production changes are Go. No `unsafe`. New failure modes wrap a sentinel with `%w` (`ErrCheckpointUnknownField` in `internal/errors/checkpoint_errors.go`), matching the existing convention. No error-text substring matching is used anywhere |
| II. Test-First Development (NON-NEGOTIABLE) | pass | U1a, U1b, U3a, U3d, U5a, U5b, U7a, and U9a are red harnesses whose red scenarios must be observed failing before U2, U4, U4b, U6, and U8 respectively. U0a and U0b land first precisely so those commits compile and the red is observable rather than theoretical. U3b, U3c, U7b, and U9b are green-throughout regression guards, explicitly labeled as such, so no unit demands a red result from an assertion of existing behavior. U9a carries a U9x contingency so a still-red scenario cannot be resolved by weakening the assertion. **Scope note (not a deviation)**: U0a and U0b assert nothing by construction, so test-first is inapplicable to them, and U8b carries no behavioral harness — U0a/U0b assert nothing by construction, and U8b edits only machine-consumed contract-text strings, whose verification is U9a scenario 3 (the registered `backlogit_docs_lint` description) and U10a's generated-reference content check for the Cobra half, rather than a behavioral test. U4b's description edit is behaviorally verified by U3d scenarios 2 and 3 |
| III. Workspace Isolation and Security Boundaries | pass | No path-resolution behavior changes. `docline` containment through `core.SafeResolve` and checkpoint `ensurePathContained` are untouched, and U8 explicitly keeps `ErrPathEscapesWorkspace` as a propagating error rather than a finding (U7b scenario 1 guards it). No secret is introduced by the patch; the open `context` namespace's secrets caveat is documented in U10b and recorded in the Decisions table |
| IV. CLI Workspace Containment (NON-NEGOTIABLE) | pass | All edits resolve inside this repository's working tree. The external source records (`C:\Source\GitHub\autoharness`, entries `3C7AAC71`, `90F2A9F8`, `84D8E6AB`) are strictly read-only inputs: no unit writes there and no backlogit command is run against that workspace. Verification fixtures resolve inside the tree or in `t.TempDir()`. The out-of-tree pinned binary at `C:\Tools\backlogit.exe` is fenced off by PA-5, which forbids any agent write to it |
| V. Structured Observability | documented-deviation | The change is itself an observability fix: it converts two silent or blind outcomes into machine-readable `context_keys`, `unknown_fields`, and `decode_error` signals. **Deviation**: intercom was unavailable during this planning session, so no milestone was broadcast to a remote channel, and the `backlogit docs lint --path` self-lint was not executed because the operator scoped Stage to planning artifacts only. Execution-time observability is covered by the closure table's execution-trace row |
| VI. Single Responsibility | pass | No new dependencies. `encoding/json`, `reflect`, and the existing `internal/jsonutil` and `internal/errors` packages cover everything. The package-global `decodeDocFn` seam was considered and rejected as global mutable state; the reflection-derived key sets are immutable init-time lookup tables, not mutable globals |
| VII. Destructive Command Approval (NON-NEGOTIABLE) | pass | The only file-overwriting command is `make docs` (PA-4, `ActionRisk: low`), which regenerates git-tracked, reproducible output; git history is the backup. Refreshing the out-of-tree pinned binary is classified `destructive` under PA-5 and is operator-only, never agent-performed |
| VIII. Explicit Safety Modes | pass | **Freeze-scope mode**: work is constrained to `internal/events`, `internal/docline`, their surface adapters, and four docs targets. **Careful mode** at the U4 strict-decode and U8 gate-semantics boundaries. Blocked-path handling for verification scenario 2 is defined under Deepened runtime verification |
| IX. Git-Friendly Persistence | pass | Checkpoint output stays readable JSON through `jsonutil.MarshalReadable`; `Extra` keys are emitted in sorted order for stable diffs |
| X. Agent Context Efficiency | pass | `context_keys` lets an agent confirm persistence without a read-back, `unknown_fields` lets it self-correct without a human, and a continuing lint returns one report instead of forcing per-file `--path` reruns |
| XI. Merge Commit History Preservation (NON-NEGOTIABLE) | pass | All units land on one dedicated implementation branch. Before merging, Ship verifies the repository merge method is merge-commit and halts with a P-009 violation if squash or rebase merging is enabled — recorded as a pre-merge gate row in Deepened operational closure, not as an assumption about settings |
| Task Granularity (NON-NEGOTIABLE) | documented-deviation | Twenty-two units. Width isolation holds: twelve tests-only (U1a, U1b, U3a, U3b, U3c, U3d, U5a, U5b, U7a, U7b, U9a, U9b), six code-only (U0b, U2, U4, U4b, U8b, and U6's production half), one code-plus-own-test (U8), one code-plus-mechanical-test-migration (U0a), and two docs-only (U10a, U10b). No unit exceeds three scenarios except U1b, which has four; the function heuristic holds everywhere. (6) *U1b* carries a fourth scenario, the HTML-escape guard, because it defends a shipped `SetEscapeHTML(false)` guarantee that U2 can silently break and that no existing test reaches; rejected alternative: a separate single-scenario escape-guard unit, rejected because it would assert against the same `emit()` boundary as U1b's other marshaller guards and splitting them fragments one review surface. Scenario counts are per test function; table-driven cases inside one scenario (U3b s1, U3c s3, U5b s2) count once because they share one fixture and one assertion helper. **Deviations, each with justification and the rejected simpler alternative**: (1) *U0a* touches six files across four packages and mixes production declarations with two mechanical `memory_test.go` call-site migrations — an exported signature change is atomic across its callers and the test edits are compilation fixes, not test authoring; rejected alternative: a dual-shape shim returning both old and new shapes, rejected as dead code (Development Workflow §5). (2) *U6* touches three files and edits Cobra help strings, which are contract text emitted by code, because `context_keys` must appear consistently on both surfaces in one commit; rejected alternative: a separate help-text unit, rejected because U10a's regeneration would then run against stale text. (3) *U8* touches four files — two production edits, one doc-comment correction that would otherwise ship a false documented contract, and the characterization table test that pins the unexported helper it introduces; it therefore also mixes code with its own test; rejected alternatives: leaving the doc comment stale (rejected because it would ship a false documented contract) and a separate post-U8 test unit (rejected because U7a must not reference `classifyDecodeFailure` and a convergence lock deferred past U8 leaves the three-way split unpinned at U8's own commit boundary). (4) *U5a, U8b, U9a, and U9b* span `internal/cli` and `internal/mcp` because R6 and R10 require assertions on both transports in one comparison; the cross-surface assertion lives in `internal/cli`, which already imports `internal/mcp`; rejected alternative: two single-package harnesses asserting the same payload independently, rejected because R6/R10 require one byte-identical comparison of both surfaces — two separate tests would drift together and compare equal, the same false green the U9a constraint note rules out. Committed fixture data (U3b's golden table, U3c's synthetic legacy fixtures) counts as one artifact for the file heuristic, since it is generated once and reviewed as a unit |
| Capability Overlay — agent-engram | pass | Code facts were sourced through indexed CLI lookup and direct reads of known files. No `.engram/` artifact is hand-edited |
| Capability Overlay — backlogit | pass | U0a–U10b are materialized as backlogit tasks under the covering feature with registered dependency edges; this document is the design reference, not a parallel task store |
| Capability Overlay — agent-intercom | documented-deviation | Intercom was unreachable for the whole planning session. Remote operator visibility is degraded and no approval was routed through it. Only safe, non-destructive planning work was performed, and the substitute approval channel is named under Human checkpoints |

Constitution Check: documented-deviations

## Plan Hardening Signals

| Signal | Present | Justification |
|---|---|---|
| Public API, schema, or contract change | **yes** | `CreateCheckpoint` changes signature; the checkpoint `context` contract becomes open while the schema namespace becomes closed; the create result gains `context_keys`; the MCP validation result gains `unknown_fields`; `LintReport` gains a new `decode_error` rule value. All are consumed by agents through CLI and MCP |
| Security, auth, permission, or compliance-sensitive behavior | **yes** | Not authentication or authorization — none is touched. The signal is data exposure: the open `context` namespace makes it materially easier for an agent to persist arbitrary, unredacted session state to `.backlogit/checkpoints/`, which is **git-tracked**. Mitigations: U10b documents that `context` must not carry secrets, U3c uses hand-written synthetic fixtures and never copies live bytes, U8 keeps `ErrPathEscapesWorkspace` a propagating error, and the gitignore/redaction posture decision is recorded as an owned follow-up in the Scope boundaries table |
| Migration, backfill, destructive data or config action, or irreversible step | no | The existing checkpoint corpus is explicitly not migrated, rewritten, or reclassified. No file is deleted or moved |
| External integration, operator checkpoint, or external dependency | no | No new dependency and no external service. The source records live in another repository but are read-only inputs, not integration points |
| High runtime, rollout, or rollback risk | **yes** | `ParseCheckpoint` is on the read path for every checkpoint consumer, and `docs lint` is a blocking CI gate. A regression in either is felt immediately across agent sessions and pull requests |

Requires plan hardening: yes

## Runtime verification and closure

| Unit | Runtime surface changed | What runtime verification must prove |
|---|---|---|
| U2, U4, U4b, U6 (verified) | CLI `backlogit checkpoint create`, MCP `backlogit_create_checkpoint` | A real create with a rich `context` writes every supplied key to disk; a dump with unknown top-level keys is rejected with **every** key named, surfaced as `validation_failed` on MCP, and writes no file; `backlogit checkpoint list` and `checkpoint get` still read every pre-existing file in `.backlogit/checkpoints/` without new `needs_quarantine` flags |
| U8, U9a, U9b | CLI `backlogit docs lint`, MCP `backlogit_docs_lint` | A repository containing one deliberately malformed document produces a full report naming that file with `decode_error` plus findings for the rest, and still exits non-zero; a path-escape input and an unreadable file both still fail loudly instead of appearing as findings; a clean corpus still returns a present `findings: []` |
| U10a, U8b | Generated CLI reference and surface contract text | `make docs` leaves no diff and the CI cli-reference-drift check is green |
| U10b | Agent instruction surfaces | An agent reading the updated Continuity Protocol places recovery keys under `context` and does not trip the closed top-level namespace |

Operational closure artifacts to produce before the work is considered absorbed:

* A before/after record of `backlogit checkpoint list` over the existing corpus, proving no file changed
  quarantine classification. This is the single highest-value rollback signal.
* A record of the malformed-document lint run showing both the `decode_error` finding and the retained
  non-zero exit.
* Rollback trigger: any pre-existing checkpoint newly reported with `needs_quarantine: true`, or any
  `docs lint` invocation that exits zero on a corpus containing a malformed document. Either condition
  reverts the merge commit.
* Ownership and validation window: the shipping agent owns verification through the first post-merge
  Stage or Ship session that creates a checkpoint and the first pull request that runs the docline gate.

## Frontmatter self-lint

The impl-plan skill asks for a `backlogit docs lint --path` self-lint through the source entrypoint.
That step was **not executed** in this session: the operator scoped Stage to planning and backlog
artifacts only and explicitly prohibited running builds, test suites, and linters. Recorded as a
degraded verification, not a silent skip.

Compensating verification, performed by construction against the contract read directly from
`internal/docline/policy.go` and `internal/docline/validate.go`:

* Authoring profile requires `title`, `source`, and `doc_type`; all three are present and non-blank.
* `doc_type: plan` is a member of the closed vocabulary and is the correct value for
  `docs/exec-plans/**` per `pathRules`.
* `source` is set to this file's own repo-relative POSIX path.
* No `content_sha256` is present, so the hex pattern check does not apply.
* Every scalar containing `:` or `#` is single- or double-quoted, so no value can be silently truncated
  by YAML comment parsing.

Ship must run `go run ./cmd/backlogit docs lint --path docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md`
from the feature branch **and** `make docs-lint` before opening the pull request — never the pinned
`C:\Tools\backlogit.exe`, which may predate the gate it is meant to enforce.

## Plan Hardening

Hardening was **required**. The plan declares two hardening signals: a public contract change and high
runtime/rollback risk. Both are real rather than nominal, because `ParseCheckpoint` sits on the read
path for every checkpoint consumer and `docs lint` is a blocking CI gate for every pull request.

### Learnings and instructions consulted

* `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
* `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
* `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`
* `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`
* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`
* `docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md`
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
* `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`
* `docs/design-docs/checkpoint-administrative-disposition.md`, `docs/design-docs/governed-operation-parity.md`
* `.github/instructions/constitution.instructions.md`, `.github/instructions/strict-safety.instructions.md`,
  `.github/instructions/backlogit.instructions.md`, `.github/instructions/go.instructions.md`

### Protected invariants

These must hold at every commit boundary, not merely at the end of the shipment:

1. **Read-path leniency.** `ParseCheckpoint` accepts unknown fields, and its *observable* behavior is
   unchanged for every input shape — including `context: null`, absent `context`, a scalar `context`,
   and duplicate `context` keys, all of which now flow through a hand-written `UnmarshalJSON`. No
   pre-existing file under `.backlogit/checkpoints/` changes its `needs_quarantine` classification.
2. **Legacy verbatim path.** A state dump without `schema_version: 1` is written byte-for-byte as
   supplied and is never subjected to the unknown-key probe.
3. **Fail-closed lint exit.** `backlogit docs lint` exits non-zero whenever the corpus contains any
   finding, including a corpus whose only finding is a `decode_error`.
4. **Always-array contract.** `LintReport.Findings` and the new `context_keys` serialize as `[]`, never
   `null` and never absent — on the clean path as well as the degraded one.
5. **Containment and I/O failures stay errors.** A `collectInScopeDocs` walk failure, an
   `ErrPathEscapesWorkspace` raised anywhere including inside `decodeDoc`, and an `os.ReadFile` failure
   all still return an error from `LintTree`. Only frontmatter-decode failures degrade to findings.
6. **No append-only history is rewritten.** Nothing in this plan writes, synthesizes, or edits an
   existing checkpoint file or JSONL event record.
7. **Shape errors keep their classification.** A malformed `created_at` or wrong-typed field still
   returns `ErrCheckpointCorrupt` and still maps to `validation_failed`; it is never absorbed into the
   unknown-field rejection.
8. **`context_keys` never names a key that is not on disk.** The list is derived from the bytes
   `MarshalJSON` emits, not from struct field emptiness.
9. **No agent write resolves outside the repository working tree.** Verification fixtures live in-tree
   or in `t.TempDir()`; the pinned binary at `C:\Tools\backlogit.exe` is operator-owned (PA-5).

### Risky actions

**ProposedAction PA-1 — Change the exported signature of `events.CreateCheckpoint` (U0a; result shape populated in U6).**

* Targets: `internal/errors/checkpoint_errors.go`, `internal/events/checkpoint_schema.go`,
  `internal/events/memory.go`, `internal/mcp/tools.go`, `internal/cli/checkpoint.go`,
  `internal/events/memory_test.go` (U0a's full file list)
* Change kind: contract change on an exported function consumed by both surfaces
* Rollback: revert the merge commit; no persisted state depends on the new return shape
* Approval required: no. It stays `moderate` rather than `high` because the exported function has only
  two in-repo production call sites and two test call sites, all migrated in the same commit, and no
  persisted state or external consumer depends on the return shape. PA-3 is `high` by contrast because
  its blast radius reaches every pull request in the repository
* ActionRisk: `moderate`
* ActionResult: `planned`

**ProposedAction PA-2 — Add a strict create-boundary unknown-key rejection that can reject previously accepted input, and surface it on MCP (U4, U4b).**

* Targets: `internal/events/memory.go` (the sentinel and typed error are already declared under
  PA-1 / U0a), plus U4b's `internal/mcp/errors.go` and the `internal/mcp/tools.go` description string,
  which carry the same rejection contract to the agent surface
* Change kind: behavior change that converts a previously silent success into a hard failure, using the
  checkpoint error taxonomy entry PA-1 introduced
* Rollback: revert the merge commit. Rejected creates write no file, so there is no partial state to
  reconcile
* Approval required: no. It is bounded to the V1 create path and cannot affect reads or the legacy path.
  The writer-migration risk is retired by U4b (every offending key is named) and U10b (agent
  instructions land before merge)
* ActionRisk: `moderate`
* ActionResult: `planned`

**ProposedAction PA-3 — Change the semantics of a blocking CI gate and its contract text (U8, U8b).**

* Targets: `internal/docline/service.go`, `internal/docline/report.go` (doc comment only),
  `internal/docline/classify_decode_failure_test.go`, plus U8b's `internal/cli/docs.go` and
  `internal/mcp/docs_tools.go` contract-text strings
* Change kind: contract change on the repository-wide docline gate that runs on every pull request
* Rollback: revert the merge commit. The gate returns to abort-on-first-error, which is strictly more
  conservative, so rollback cannot let a non-conforming corpus through
* Approval required: **yes** — strict-safety Rule 3 prefers approval when an action changes a broad
  shared surface, and this gate is shared by every pull request
* ActionRisk: `high`
* ActionResult: `planned`

**ProposedAction PA-4 — Regenerate the tracked CLI reference with `make docs` (U10a).**

* Targets: `docs/cli-reference/backlogit_checkpoint_create.md`, `docs/cli-reference/backlogit_docs_lint.md`
* Change kind: in-place overwrite of git-tracked generated files
* Rollback: `git checkout` the two files; they are generated and fully reproducible
* Approval required: no. It is a tracked, revertible regeneration, not a destructive action
* ActionRisk: `low`
* ActionResult: `planned`

**ProposedAction PA-5 — Refresh the pinned binary at `C:\Tools\backlogit.exe`.**

* Targets: `C:\Tools\backlogit.exe` — a path **outside** the repository working tree
* Change kind: in-place replacement of an installed global tool
* Rollback: reinstall the previously pinned build
* Approval required: **yes — operator-performed only.** The implementing and Ship agents MUST NOT build,
  copy, install, or otherwise write to `C:\Tools\` or any path outside the repository working tree
  (Principle IV, NON-NEGOTIABLE). If the pinned binary is stale, record the verification as **blocked**
  and escalate rather than satisfying the precheck by writing that path
* ActionRisk: `destructive`
* ActionResult: `blocked`

**ProposedAction PA-6 — Revert the merge commit on rollback trigger A, B, or C.**

* Targets: mainline history
* Change kind: mainline revert — a new commit, not a history rewrite, consistent with Principle XI
* Rollback: re-land after the defect is fixed
* Approval required: yes (operator)
* ActionRisk: `high`
* ActionResult: `planned`

**ProposedAction PA-7 — Amend the workspace agent instruction surface (U10b).**

* Targets: `.github/instructions/backlogit.instructions.md`, `docs/docline-frontmatter-authoring-guide.md`
  (U10b's full file list)
* Change kind: config change to a repo-wide agent contract, applied to an autoharness-generated file
* Rollback: `git revert` the docs commit; the file is regenerable from its upstream template
* Approval required: no. It is additive guidance with no runtime effect, but it is recorded here
  because it changes the operating contract for every future agent session in this workspace and is a
  declared pre-merge gate
* ActionRisk: `moderate`
* ActionResult: `planned`

**ProposedAction PA-8 — Replace `encoding/json`'s default (un)marshalling of `CheckpointContext` with hand-written methods on the shared read path (U2).**

* Targets: `internal/events/checkpoint_schema.go`
* Change kind: behavior change to the on-disk `context` serialization **and** to decode semantics on
  every checkpoint read path — `ListCheckpoints`, `GetCheckpoint`, `ResolveCheckpoint`, and both
  disposition verbs
* Rollback: revert the merge commit. No existing file is rewritten, and unmodeled `context` keys
  written post-merge degrade to today's drop-on-rewrite behavior rather than corrupting the file
* Approval required: **yes** — strict-safety Rule 3: it changes a data shape on a broad shared surface.
  U2 is blocked on PA-8 exactly as U8 is blocked on PA-3
* ActionRisk: `high` — a divergence reclassifies a live file and trips rollback trigger A; mitigated by
  the pre-U2 golden tables in U3b and U3c
* ActionResult: `planned`

One `ActionRisk: destructive` action exists (PA-5) and it is operator-only and currently `blocked`; no
agent may perform it. If any other destructive action emerges during implementation, Ship must stop and
obtain explicit operator approval before proceeding.

### Deepened runtime verification

Environment prechecks, to run **before** any verification claim is recorded:

1. **Every verification invocation runs the source entrypoint `go run ./cmd/backlogit …`, built from the
   feature-branch HEAD.** This repository dogfoods a separately pinned binary at
   `C:\Tools\backlogit.exe`, and a merged fix is **not operative** through that binary until it is
   rebuilt or upgraded. Running the before/after `checkpoint list` comparison under the pinned binary
   would execute pre-fix code on both sides and turn the plan's primary rollback signal into a
   guaranteed false green. Do not judge freshness by file modification time.
2. Separately, record `backlogit version` for the pinned `C:\Tools\backlogit.exe` and state whether its
   embedded VCS revision predates the merge commit. If it does, record explicitly **"merged but not yet
   operative"** and name the residual exposure: checkpoints created through the pinned binary keep the
   lossy round-trip until it is refreshed. The refresh itself is PA-5 — operator-only, and the agent
   MUST NOT perform it.
3. Any verification that **creates or mutates** a checkpoint MUST run against a disposable fixture
   checkpoint directory resolving inside the repository working tree or in `t.TempDir()`, never against
   the live `.backlogit/checkpoints/` corpus and never above the workspace root, so verification cannot
   author state into the workspace history or escape containment. **Read-only** inspection of the live
   corpus (`checkpoint list`, `checkpoint get`) is permitted and is required by scenarios 1 and 2 — it
   is the plan's highest-value rollback signal. No test or verification step may write to
   `.backlogit/checkpoints/`.

Target scenarios, in order:

| # | Scenario | Expected |
|---|---|---|
| 1 | `backlogit checkpoint list` over the live corpus, captured **before** the change, using `go run ./cmd/backlogit` from the pre-merge base | Baseline `needs_quarantine` classification for all nine files |
| 2 | The same command after the change, again through `go run ./cmd/backlogit` from the feature-branch HEAD | Byte-identical classification to scenario 1 |
| 3 | Create a V1 checkpoint with a rich `context` in an in-tree fixture directory | Every supplied context key is present on disk; `context_keys` lists exactly those keys and no more |
| 4 | Create a V1 checkpoint with two unknown top-level keys | Rejected, **both** keys named in the error, `validation_failed` on MCP, and no file is written |
| 5a | Create a legacy dump with no `schema_version` whose `context` carries `pr_number` and `next_steps` | Written verbatim; `context_keys` lists exactly `["next_steps","pr_number"]`, sorted |
| 5b | Create a legacy dump with no `context`, and one whose `context` is `42` | Written verbatim; the create succeeds; `context_keys` is `[]`, not `null` |
| 5c | Drive scenarios 5a and 5b through the `backlogit_create_checkpoint` **MCP** tool | Byte-identical `context_keys` payloads to the CLI on both, confirming the agent surface receives `[]` rather than `null` |
| 6 | Create a V1 checkpoint whose `context` supplies `"task_ids": []` | On-disk bytes and `context_keys` agree exactly |
| 7 | `backlogit docs lint` against a fixture corpus with one malformed document | Full report including a `decode_error` finding for that file plus findings for the rest; exit code non-zero; no absolute host path appears in any `Fix` string |
| 8 | `backlogit_docs_lint` over the same fixture through MCP | Successful tool result with the same findings, not `InternalError` |
| 9 | `backlogit docs lint` against a clean fixture corpus | Exit zero and a present `findings: []`, never `null` |

Blocked-path handling: if scenario 2 diverges from scenario 1 in any way, stop. Do not attempt a
corrective edit to the checkpoint corpus. Record the divergence, treat it as a rollback trigger, and
escalate to the operator.

### Deepened operational closure

| Element | Detail |
|---|---|
| Pre-merge gate | Ship verifies the repository merge method is merge-commit — squash and rebase merging disabled — immediately before merging, and halts with a P-009 violation if either is enabled (Principle XI) |
| Pre-merge gate | U10b's agent-instruction update is merged with or before this shipment, so no live agent hits the closed namespace with stale guidance |
| Monitoring signal 1 | `needs_quarantine` count reported by `backlogit checkpoint list` over the live corpus, captured through `go run ./cmd/backlogit` on both sides. Baseline: unchanged from the pre-merge capture |
| Monitoring signal 2 | Docline gate outcome on the first post-merge pull request. Baseline: same pass/fail verdict the gate produced pre-merge for an unchanged corpus |
| Monitoring signal 3 | Presence of `context_keys` in the create result of the first real Stage or Ship checkpoint after the binary is refreshed |
| Rollback trigger A | Any pre-existing checkpoint newly classified `needs_quarantine: true` |
| Rollback trigger B | `backlogit docs lint` exits zero on a corpus containing a malformed document, or a path-escape input appears as a finding instead of an error |
| Rollback trigger C | A checkpoint create that previously succeeded now fails for a reason other than an unknown key at the top level or inside `progress` |
| Rollback procedure | Revert the merge commit (PA-6, operator-approved). No data migration, backfill, or checkpoint rewrite is required, because nothing in this plan mutates existing files |
| Execution trace | Each unit's four-gate result (`go test`, `go vet`, `golangci-lint`, `gofmt`) and its red→green transition is recorded as a backlogit task comment and in the conventional-commit body. With intercom unavailable, that record is the sole execution trace and Ship states so at handoff |
| Owner | The Ship agent that merges this shipment |
| Validation window | Through the first post-merge Stage or Ship session that creates a checkpoint **and** the first pull request that runs the docline gate, whichever completes later |

### Human checkpoints and unresolved operator decisions

* **Binary refresh checkpoint (operator).** The fix is inert for real agent sessions until the pinned
  `C:\Tools\backlogit.exe` is rebuilt or upgraded past the merge commit. That refresh is **PA-5**:
  `ActionRisk: destructive`, operator-performed only, and currently `blocked`. The implementing and Ship
  agents MUST NOT write that path or any other path outside the repository working tree; if the binary
  is stale they record the verification as blocked and escalate. Closure must state explicitly whether
  the operator refreshed it. Until it is refreshed, agents continue to lose checkpoint context even
  though the fix is merged — a known, accepted delivery gap, not a defect in the plan.
* **Degraded operator visibility.** Intercom tooling was unavailable during this staging session, so no
  milestone was broadcast to a remote channel and no approval was routed through it. Ship should not
  assume any remote operator observed the planning decisions recorded here. This is recorded as a
  documented deviation in the Constitution Check rather than as a silent skip.
* **Approval still owed before implementation.** PA-3 (`high`, blocking CI gate semantics), PA-6
  (`high`, mainline revert), and PA-8 (`high`, hand-written codec on every checkpoint read path) all
  declare `approval required: yes`, and PA-5 is `destructive` and
  operator-only. **Approval channel**: if intercom is restored before implementation, those approvals
  route through it per strict-safety Rule 5. If it is still unavailable, approval MUST be obtained by
  direct operator prompt, and the approving operator, timestamp, and action ID recorded in this section
  by flipping each `ActionResult` to `approved`. No unit that depends on an unapproved action may
  start — U8 is blocked on PA-3. An implicit assumption of approval is not acceptable.
* No other operator decision currently blocks safe execution.

<!-- plan-review-attempt: 1 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 1 of the plan-review skill. All seven manifest personas were dispatched as independent
sub-agents and all seven returned findings, so the `multi-agent-dispatch` label is valid under the
terminal-states rule (no partial coverage was merged). Cross-model diversity was applied: the
Architecture Strategist, Agent-Native Parity Reviewer, and Security Lens Reviewer each ran on a
different model from the caller and from one another.

Personas covered: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher
(always-on); Architecture Strategist (always triggered), Agent-Native Parity Reviewer (triggered — the
plan changes MCP tool contracts and agent-facing result shapes), Security Lens Reviewer (triggered —
the plan changes a path-containment-adjacent control and persists arbitrary caller input to disk).

Plan hardening was **required** and a `## Plan Hardening` section was present, so the structural
hardening gate passed. Strict-safety classification was present but materially incomplete (see P1-11
and P1-12). The `## Constitution Check` section carried a recognized verdict, so the structural
governance gate passed on format while failing on substance (P1-8).

Merged severity counts: **1 P0, 19 P1, 21 P2, 15 P3.** Any P0 or P1 finding fails the gate. The
remediation applied after this gate is summarized below; gate run 2 re-reviewed it and found further
P1 findings, recorded in its own section.

### P0 findings

| # | Finding | Personas |
|---|---|---|
| P0-1 | U2's custom `UnmarshalJSON` on `CheckpointContext` changes the **observable** behavior of `ParseCheckpoint` on every read path, even though the function itself is unmodified. R4's original wording ("`ParseCheckpoint` is not modified") was true of the code and false of the behavior. Degenerate `context` values (`null`, absent, scalar, duplicate keys) could flip an existing on-disk file from parseable to corrupt, tripping the plan's own primary rollback trigger, and no scenario guarded it | Learnings Researcher |

### P1 findings (merged)

| # | Finding | Personas |
|---|---|---|
| P1-1 | U8's instruction "replace the `return nil, err` on `decodeDoc` failure" would downgrade `ErrPathEscapesWorkspace` — raised **inside** `decodeDoc` at `service.go:289`, not only in `collectInScopeDocs` — into a success-shaped lint finding, defeating a NON-NEGOTIABLE Principle III control | Go Reviewer, Security Lens |
| P1-2 | `ErrCheckpointUnknownField` was unrouted in `internal/mcp/errors.go`'s `domainError`, so MCP would surface a caller input error as a generic `internal` fault | Go Reviewer, Learnings Researcher, Architecture Strategist, Agent-Native Parity |
| P1-3 | Running the strict decode **before** `ParseCheckpoint` would misclassify ordinary shape errors (bad `created_at`, wrong-typed `task_ids`) as unknown-field rejections, and discriminating them would require substring-matching `"json: unknown field"` — banned by the workspace Go conventions | Go Reviewer |
| P1-4 | U2 did not pin the `MarshalJSON` receiver. `Context` is a **value** field, so a pointer receiver would be silently skipped by `encoding/json` for non-addressable values, reproducing the exact defect being fixed | Go Reviewer |
| P1-5 | U7's package-global `decodeDocFn` seam is unnecessary (a real malformed fixture suffices), is listed as a Go anti-pattern, and its stated `t.Parallel()` ban would have forced unscoped edits to existing `internal/docline` tests that already call `t.Parallel()` | Go Reviewer, Scope Auditor, Constitution Reviewer |
| P1-6 | U6's scope note undercounted the call sites the signature change breaks — `internal/events/memory_test.go:33` and `:47` bind the two-value form and would not compile | Scope Auditor, Architecture Strategist, Go Reviewer |
| P1-7 | The dependency graph omitted the correctness-critical `U2 → U4` edge (prose-only) and the `U4 → U6` edge, even though both units modify `CreateCheckpoint` | Scope Auditor, Architecture Strategist |
| P1-8 | `Constitution Check: pass` contradicted two degradations documented elsewhere in the plan: the unexecuted self-lint and the unavailable intercom | Constitution Reviewer |
| P1-9 | Principle VIII was marked `N/A` while the plan's own hardening table declared high runtime/rollback risk | Constitution Reviewer |
| P1-10 | U3's acceptance demanded a **red** result from scenario 2, which asserts existing behavior and is structurally a regression guard — an unreachable red criterion that corrupts the Principle II evidence trail | Constitution Reviewer |
| P1-11 | Refreshing the pinned `C:\Tools\backlogit.exe` — a precondition for every verification claim — is an out-of-workspace write with no `ProposedAction`, no risk classification, and no prohibition on the agent performing it | Constitution Reviewer, Security Lens, Learnings Researcher |
| P1-12 | PA-3 was classified `moderate` despite changing the semantics of a CI gate that runs on every pull request, inconsistent with the plan's own hardening table and with strict-safety Rule 3 | Constitution Reviewer |
| P1-13 | `core.AbandonCheckpoint` and `events.ResolveCheckpoint` perform the identical lossy parse→mutate→re-marshal round-trip on pre-existing files; leaving them out of scope without recording the boundary presented the plan as a complete elimination of checkpoint evidence loss | Learnings Researcher, Architecture Strategist |
| P1-14 | `context_keys` derived from struct emptiness could name a key that `omitempty` elided from disk — a new success-shaped result inside the fix for success-shaped results | Learnings Researcher, Go Reviewer, Scope Auditor |
| P1-15 | The cited commit-then-surface learning was not applied: an indeterminate durable write would return a bare error and discard the path and key list | Learnings Researcher |
| P1-16 | Runtime verification did not pin the entrypoint, so the before/after `checkpoint list` comparison — the plan's own highest-value rollback signal — could run under the stale pinned binary on both sides and yield a guaranteed false green | Learnings Researcher |
| P1-17 | The `backlogit_create_checkpoint` MCP tool description — agent-facing contract text — was not updated, leaving agents blind to the closed namespace until they fail | Agent-Native Parity |
| P1-18 | `.github/instructions/backlogit.instructions.md` currently tells agents to persist decisions, blockers, and next steps without saying where; an agent following it while emitting `schema_version: 1` would hit the new rejection with no fallback | Agent-Native Parity |
| P1-19 | U10 could not deliver R11 for the checkpoint half: it mandated regeneration over hand-editing, but no unit modified the Cobra help strings that generate the reference, so `make docs` would emit no diff and the acceptance criterion would pass vacuously | Scope Auditor |

### Remediation applied

Every P0 and P1 finding above was remediated in this document before the gate was re-run. Summary of
the edits:

* **P0-1** — R4 restated as *observable* leniency; U2 gained an explicit read-path-safety clause; U3
  gained scenarios 5, 6, and 7 (degenerate `context` values, duplicate keys, and a full fixture copy of
  the live corpus listed through `ListCheckpoints`), all green before and after U2 and U4; a distinct
  risk row separates the custom-unmarshaler mechanism from the strict-decode mechanism.
* **P1-1** — U8 now mandates an explicit three-way split of `decodeDoc` failures; U7 scenario 4 asserts
  `ErrPathEscapesWorkspace` still propagates; Protected invariant 5 rewritten.
* **P1-2** — new unit **U4b** maps the sentinel to `ValidationFailed` and updates the tool description;
  new requirement R12; U3 scenario 10 asserts the MCP classification.
* **P1-3** — U4 rewritten as a two-pass decode (`ParseCheckpoint` first, then a **map-diff** unknown-key
  probe); U3 scenario 8 pins `ErrCheckpointCorrupt` for a malformed timestamp; the decision table records
  why `DisallowUnknownFields` was rejected.
* **P1-4** — U2 pins value/pointer receivers explicitly; U1 scenario 4 marshals a non-addressable value.
* **P1-5** — the `decodeDocFn` seam is rejected outright; U7 uses a real malformed fixture and existing
  `t.Parallel()` calls are retained.
* **P1-6** — U6 carries a full call-site inventory including both `memory_test.go` lines and the U1/U3
  harnesses, with compilation of every call site as an acceptance criterion.
* **P1-7** — the graph now contains `U2 → U4`, `U4 → U4b`, `U4 → U6`, `U2 → U6`, and split `U10a`/`U10b`
  edges.
* **P1-8** — verdict changed to `Constitution Check: documented-deviations`; row V marked
  `documented-deviation`; an `agent-intercom` overlay row added.
* **P1-9** — row VIII changed to `pass` with the freeze-scope and careful-mode posture named.
* **P1-10** — U3's posture and acceptance now label scenarios 3–8 as green-throughout regression guards
  and only scenarios 1, 2, 9, 10 as red.
* **P1-11** — **PA-5** added (`destructive`, operator-only, `blocked`) with an explicit prohibition on
  agent writes outside the tree; prechecks rewritten; Protected invariant 9 added; rows IV and VII updated.
* **P1-12** — PA-3 raised to `ActionRisk: high` with `approval required: yes`; PA-1 carries an explicit
  justification for remaining `moderate`; **PA-6** added for the rollback revert.
* **P1-13** — a new **Scope boundaries and recorded follow-ups** table records `AbandonCheckpoint`,
  `ResolveCheckpoint`, the `MigrateReport` `omitempty` siblings, and the registry-governance decision,
  each with its residual exposure and a named follow-up.
* **P1-14** — `Keys()` is now derived from the bytes `MarshalJSON` emits; U1 scenario 7 and U5 scenario 5
  assert file/result agreement for `"task_ids": []`; Protected invariant 8 added.
* **P1-15** — new requirement R13; U6 applies commit-then-surface on indeterminate writes; U5 scenario 6
  asserts the populated result accompanies the error.
* **P1-16** — every verification invocation is pinned to `go run ./cmd/backlogit` from the feature-branch
  HEAD, with a separate "merged but not yet operative" record for the pinned binary.
* **P1-17** — U4b updates the `backlogit_create_checkpoint` tool description.
* **P1-18** — U10b updates `.github/instructions/backlogit.instructions.md`, with an explicit
  writer-migration ordering constraint that it lands with or before the merge.
* **P1-19** — U6 now edits the Cobra `Short`/`Long`/`Example` strings; U10a's acceptance requires the new
  sentences to be present in the generated file, not merely that a second `make docs` run is clean.

### P2 items accepted for awareness

Several P2 findings were folded into the remediation because they were cheap and reduced real risk:
the `internal/errors/checkpoint_errors.go` sentinel placement, the pinned `CreateCheckpointResult` wire
type replacing two independent `map[string]string` literals, the sanitized repo-relative `Fix` string,
the U9 clean-corpus scenario, the `progress` nested-namespace decision, the collect-all unknown-key
error, the `Extra`/modeled-key collision rule, the `MarshalJSON` recursion guard, the U10 split into
U10a/U10b for the 2-Hour Rule, the quality-gate line under `## Implementation units`, the
backlogit-materialization line, the branch declaration, and the `report.go`-is-out-of-scope note.

The remaining P2 items are recorded as awareness rather than blockers: a checkpoint schema-version
evolution policy for future modeled top-level fields (Architecture Strategist), and the `.gitignore`
posture for `.backlogit/checkpoints/` (Security Lens) — the latter is deliberately unchanged and
documented as a caveat in U10b instead.

### P3 items

Fifteen P3 items were advisory. The ones adopted are the `errors.Is` sentinel assertion in U3, the
`json.RawMessage` "preserved without a reshaping cycle" wording correction, the U1 scenario-2 assertion
technique, the branch declaration, and the Principle IV note naming the external read-only repository.
The rest are recorded here without action.

### Runtime verification and closure gaps

No gaps remain. The plan carries a deepened runtime verification section with nine ordered scenarios
and explicit blocked-path handling, and a deepened operational closure table with two pre-merge gates,
three monitoring signals, three rollback triggers, a named owner, and a bounded validation window.



<!-- plan-review-attempt: 2 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 2, re-run against the remediated plan. All seven manifest personas were dispatched again as
independent sub-agents and all seven returned findings, so the `multi-agent-dispatch` label remains
valid under the terminal-states rule. Cross-model diversity was preserved on the three cross-model
personas.

The gate-1 remediation held: **all 1 P0 and all 19 P1 findings from gate 1 were verified resolved** by
the personas that raised them, several with direct source verification. However, the remediation itself
introduced new defects and exposed gaps the enlarged plan had not previously reached.

Merged severity counts: **0 P0, 15 P1, 22 P2, 18 P3.** Any P1 fails the gate.

### P1 findings (merged)

| # | Finding | Personas |
|---|---|---|
| G2-1 | R13 (indeterminate durable write) was remediation-introduced scope creep: untraceable to `3C7AAC71` or `90F2A9F8`, resting on a durable-write seam that does not exist on this code path (`syncWriteFileAtomic` has no classification and no injectable seam), and with no wire representation — neither MCP's `errorResponse` nor the CLI error path can carry a `path` or key list | Go Reviewer ×2, Learnings Researcher ×2, Scope Auditor |
| G2-2 | Four red harnesses forward-referenced identifiers their green units create (`ErrCheckpointUnknownField`, `RuleDecodeError`, `CreateCheckpointResult`), so those commits would not compile and the Principle II red result was unobservable, not merely weak | Constitution Reviewer, Scope Auditor |
| G2-3 | The remediation breached the NON-NEGOTIABLE 2-Hour Rule scenario heuristic ("fewer than 4 test scenarios") in five units — U1 (7), U3 (10, across two packages), U5 (6), U7 (5), U9 (4) — while the Constitution Check still claimed a single exception | Constitution Reviewer, Scope Auditor |
| G2-4 | `Extra` was specified without a JSON tag, so the plan's own recommended method-less shadow-type technique would emit and consume a literal `"Extra"` key nested inside `context` | Go Reviewer |
| G2-5 | U8's three-way split had **no discriminator**: `decodeDoc` wraps the read failure and the decode failure identically with an opaque `%w` over untyped errors, so an implementer forbidden from substring matching had no mechanism at all | Go Reviewer |
| G2-6 | `modeledContextKeys` and the `CheckpointV1` tag set were both hand-maintained lists with no reflection drift guard, reproducing the failure mode that occurred when the four `disposition_*` fields were added | Learnings Researcher, Go Reviewer |
| G2-7 | The legacy verbatim path was specified to report `context_keys: []` even when the persisted bytes carry a populated `context` — a new success-shaped result directly contradicting R5 | Learnings Researcher |
| G2-8 | U10b edits `.github/instructions/backlogit.instructions.md`, an autoharness-generated file whose template lives in a read-only repository, making the shipment's own pre-merge writer-migration guardrail silently revertible | Learnings Researcher |
| G2-9 | The dependency graph was still missing `U3 → U2` (required by prose in three places) and `U4b → U6` (both edit `internal/mcp/tools.go`) | Scope Auditor |
| G2-10 | U5 scenario 6 assumed a durable-write test seam not present in the recorded seam inventory, and would have forced U6 to add a package-global seam the same plan rejects in U7 | Learnings Researcher |
| G2-11 | The indeterminate outcome had no MCP machine-readable mapping, so R13 would have died at `domainError` exactly as the unknown-field sentinel did before U4b | Learnings Researcher |
| G2-12 | U10a claimed `docs/cli-reference/backlogit_docs_lint.md` but no unit changed its generating Cobra help text, so R11 shipped unsatisfied for the lint half — the identical defect gate 1 fixed for the checkpoint half | Scope Auditor |
| G2-13 | U8 both forbade and required editing `internal/docline/report.go`, and the resulting three-file scope had no recorded exception | Scope Auditor |
| G2-14 | An exact-match key diff diverges from `json.Unmarshal`'s case-insensitive field matching, so a dump containing `"Schema_Version"` that succeeds today would be newly rejected — the plan's own rollback trigger C, shipped silently | Go Reviewer |
| G2-15 | The `,omitempty` tag option was not stripped in the derivation, so six modeled `CheckpointV1` keys (`progress`, `resume_hint`, and the four `disposition_*` fields) would be treated as unknown | Go Reviewer |

### Remediation applied

* **G2-1, G2-10, G2-11** — R13 and U5 scenario 6 are **removed**. The indeterminate-write contract is
  recorded in the Scope boundaries table with its residual exposure and an owned backlog follow-up
  ("classify `syncWriteFileAtomic` outcomes and surface indeterminate creates on both transports"), and
  a Decisions row states it is descoped rather than deferred silently.
* **G2-2** — new prelude unit **U0** declares every new exported identifier with no behavior change and
  precedes every harness unit in the graph.
* **G2-3** — units split so no unit exceeds three scenarios or spans two packages: U1 → U1a/U1b;
  U3 → U3a/U3b/U3c/U3d; U5 → U5a/U5b; U7 → U7a/U7b; U9 → U9a/U9b. The Task Granularity row is changed
  to `documented-deviation` and enumerates all three remaining file-count exceptions with justification
  and the rejected simpler alternative.
* **G2-4** — U2 declares `Extra map[string]json.RawMessage \`json:"-"\`` literally, with U1b scenario 2
  asserting no key named `Extra` appears in the emitted `context`.
* **G2-5** — U0 declares `ErrFrontmatterDecode`; U8 wraps it at the `mdfront.Decode` branch so `LintTree`
  branches purely on `errors.Is` and never inspects error text.
* **G2-6, G2-15** — both key sets are derived by reflection at package init with
  `strings.Split(tag, ",")[0]` option stripping; U1b scenario 3 and U3c scenario 3 are the drift and
  option-stripping guards; new requirement R13 (redefined) covers it.
* **G2-7** — U6 populates `ContextKeys` on the legacy path by scanning the top-level `context` object of
  the **written bytes**; U5b scenario 1 asserts it.
* **G2-8** — U10b records the generated-artifact caveat, the Scope boundaries table carries an owned
  follow-up to upstream the template, and U4b's tool description is named as the non-generated durable
  statement of the contract.
* **G2-9** — the graph now carries `U0 →` every harness, `U3b/U3c → U2`, `U3a → U4`, `U3d → U4b`,
  `U4b → U6`, and `U8b → U10a`.
* **G2-12** — new unit **U8b** edits the `docs lint` Cobra strings and the `backlogit_docs_lint` MCP tool
  description, and precedes U10a.
* **G2-13** — U8 lists `report.go` explicitly as a doc-comment-only edit with a recorded file-count
  exception; the "must not change" contradiction is gone.
* **G2-14** — U4 compares keys case-insensitively, matching `json.Unmarshal`, with new requirement R14
  and a Risks row tying it to rollback trigger C.

Accepted P2 items folded in: the `unknown_fields` structured field on the MCP result, the pre-U2 golden
baseline requirement for the read-path guards, the redacted committed corpus fixture with an asserted
file count, the precheck-3 read-only/mutating split, the `PlanMigration` rationale correction, PA-7 for
the agent-instruction change, the named approval channel under degraded intercom, the hardening-signal
security row flipped to **yes** with mitigations, the execution-trace closure row, the source-entrypoint
self-lint pin, the doc-comment requirement on new exported identifiers, and the `U9x` contingency
renaming with a defined file list and acceptance.

Remaining P2/P3 items are recorded as awareness: a checkpoint schema-version evolution policy for future
modeled top-level fields, and the discoverability-guide inventory for `create_checkpoint`.

### Runtime verification and closure gaps

None. The verification table now carries nine ordered scenarios with the entrypoint pinned to
`go run ./cmd/backlogit`, and the closure table carries two pre-merge gates, three monitoring signals,
three rollback triggers, an execution-trace row, a named owner, and a bounded validation window.








<!-- plan-review-attempt: 3 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 3, re-run against the twice-remediated plan. All seven manifest personas were dispatched again
as independent sub-agents and all seven returned findings; cross-model diversity was preserved on the
three cross-model personas.

All 15 gate-2 P1 findings were verified resolved. The Architecture Strategist and Security Lens
Reviewer returned **zero P1 findings** for the first time. The remaining P1s were second-order defects
in the gate-2 remediation itself.

Merged severity counts: **0 P0, 7 P1, 18 P2, 12 P3.**

### P1 findings (merged)

| # | Finding | Personas |
|---|---|---|
| G3-1 | U0's declaration set was still incomplete: it did not touch `internal/events/checkpoint_schema.go`, so U1b — which asserts the `Extra` carrier and the derived modeled-key set — still could not compile at its own commit | Constitution Reviewer, Go Reviewer |
| G3-2 | U0 undercounted its own footprint: the file list said three files while the changes migrate `CreateCheckpoint` at four call sites in three additional files, so its 2-Hour Rule exception was recorded against a false count — a verbatim repeat of gate-1 P1-6 relocated into the remediation unit | Scope Auditor, Constitution Reviewer |
| G3-3 | U0 fused the two independent defect tracks into one commit while the graph prose asserted they were independent, coupling `90F2A9F8` behind a `3C7AAC71` signature migration | Scope Auditor, Architecture Strategist, Constitution Reviewer |
| G3-4 | `unknown_fields` could not be populated from a bare sentinel — the only remaining mechanism was parsing `err.Error()`, which U4 and the workspace Go conventions explicitly forbid | Go Reviewer |
| G3-5 | U6's legacy `context_keys` scan was undefined for a legacy dump that is not a JSON object, or whose `context` is `null`/scalar/array. A literal implementation would turn a currently-succeeding create into a failure **after the file is on disk** — a new success-shaped inversion and the plan's own rollback trigger C | Go Reviewer, Learnings Researcher |
| G3-6 | The two MCP tool-description edits (U4b, U8b) had no verification at all, while the CLI half was pinned by U10a's content check — the agent-facing half of R11 could ship stale, the same defect class raised at gate 1 for the CLI half | Agent-Native Parity |
| G3-7 | The closed namespace was documented as "unknown keys are rejected" without ever enumerating the **legal** keys, forcing agents into a guess-and-fail workflow with no way to learn the contract before failing | Agent-Native Parity |

### Remediation applied

* **G3-1, G3-2, G3-3** — U0 is split into **U0a** (checkpoint: `internal/errors/checkpoint_errors.go`,
  `internal/events/checkpoint_schema.go`, `internal/events/memory.go`, `internal/mcp/tools.go`,
  `internal/cli/checkpoint.go`, `internal/events/memory_test.go`) and **U0b** (docline:
  `internal/docline/policy.go`). U0a now declares the behavior-neutral `Extra map[string]json.RawMessage \`json:"-"\``
  and a `Keys()` stub so U1b compiles, enumerates all six files, and records its exception against the
  true count. The graph edges are rewritten as `U0a → Track A` and `U0b → Track B`, which share no unit.
* **G3-4** — U0a additionally declares the typed `CheckpointUnknownFieldError{Fields []string}` with
  `Unwrap() error` returning the sentinel, so `errors.Is` still matches and U4b recovers the list with
  `errors.As` — the repository's existing mechanism, matching `*corerrors.AmbiguousWorkspaceRootError`.
* **G3-5** — U6 declares the legacy scan **strictly best-effort**: any decode failure or non-object
  `context` yields `make([]string, 0)` and the create still succeeds. U5b scenario 2 becomes a table
  over four degenerate legacy dumps, each asserting success plus a present empty `.([]any)`.
* **G3-6** — U3d gains scenario 2 asserting the **registered** `backlogit_create_checkpoint` description
  content, and U9a scenario 3 gains the same assertion for `backlogit_docs_lint`.
* **G3-7** — U4b's description now **enumerates the legal `CheckpointV1` top-level and `progress` keys**
  rather than only stating that unknown keys are rejected.

Accepted P2/P3 items folded in: U1b scenario 3 rewritten as a literal-expectation drift guard (a
reflection-vs-reflection comparison would pass unconditionally); U3c switched from a live-corpus
snapshot to **hand-written synthetic** fixtures, because `git revert` cannot purge a committed blob and
Principle XI forbids history rewriting, so a missed secret would have been unrecoverable; U8's
classification extracted into a `classifyDecodeFailure` helper with a table test as the convergence lock
so `PlanMigration` can adopt it later; U7b forbidden from using `Path: "../escape"` as its sole
construction, which would false-green against a `decodeDoc`-internal downgrade; U7a scenario 3's
cross-package exit-code clause removed and R8 reassigned to U9a alone; U9b relabeled a green-throughout
regression guard rather than a red harness; the requirements trace repointed from the deleted `U1`;
a provenance marker added distinguishing decision-record requirements from plan-derived ones; PA-1
rebound to U0a with its true target list; the schema-evolution policy recorded as a Decisions row
(reflected tag sets implement a frozen declared version, not automatic admission); precheck 3 reworded
so read-only live-corpus listing is explicitly permitted; the graph's terminal-ordering claim encoded as
real edges; the unit count corrected; and the Constitution Check row II documented deviation for the
three units that legitimately carry no red harness.

Remaining awareness items: the indeterminate-write follow-up should converge `syncWriteFileAtomic` onto
`internal/atomicfile` rather than harden the private duplicate, and the CLI error path does not carry the
structured `unknown_fields` array that the MCP envelope does.

### Runtime verification and closure gaps

None.


<!-- plan-review-attempt: 4 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 4, re-run against the thrice-remediated plan. All seven manifest personas were dispatched as
independent sub-agents and all seven returned findings; cross-model diversity was preserved.

**Root cause of this gate's failures.** Several gate-3 remediations were applied to the plan's summary
tables but silently failed to apply to the unit bodies an implementer actually executes — a scripted
multi-line replacement mismatched line endings and no-opped without error. The Constitution Reviewer
caught this precisely: "this round's edits landed in the summary tables but not in the unit bodies", and
in the U3c case "the plan now asserts a security mitigation in two governance tables that its own unit
definition contradicts." That contradiction — a Principle III `pass` resting on a mitigation the plan
did not actually specify — is the single most important finding of this gate.

Merged severity counts: **0 P0, 6 P1, 9 P2, 10 P3.** The Architecture Strategist and Security Lens
Reviewer again returned zero P1 findings.

### P1 findings (merged)

| # | Finding | Personas |
|---|---|---|
| G4-1 | U3c's switch to synthetic fixtures landed only in the Task Granularity row. The unit body still directed an implementer to commit a **redacted copy of the live `.backlogit/checkpoints/` corpus**, and both the Risks table and the hardening-signal security row still cited that copy as a mitigation — a Principle III `pass` resting on a mitigation the plan contradicted, with an irreversible git-history exposure that `git revert` cannot purge under Principle XI | Constitution Reviewer |
| G4-2 | U1b scenario 3 was still the ambiguous reflection-vs-reflection assertion. Under one reading it forward-references a U2-only derived set and cannot compile; under the other it is green from U0a onward while the unit's posture demands "three failing assertions" — a red demanded from an assertion of existing behavior | Constitution Reviewer, Go Reviewer |
| G4-3 | The tool-description assertions claimed for U3d scenario 2 and U9a scenario 3 were absent from the plan text: U3d still defined exactly one scenario and U9a scenario 3 asserted the tool *result*, not the description string. Agent-facing contract text remained unverified | Agent-Native Parity, Constitution Reviewer |
| G4-4 | The unresolved CLI-vs-MCP error-shape asymmetry — MCP carries a structured `unknown_fields` array while the CLI prints a wrapped string — was recorded nowhere, neither as a Scope boundary nor with an owned follow-up | Agent-Native Parity |
| G4-5 | U7a scenario 3's cross-package exit-code clause was still present despite the gate-3 claim, and the Risks table still cited it; `internal/docline` does not import `internal/cli`, so the clause is unimplementable without an import cycle | Constitution Reviewer, Scope Auditor |
| G4-6 | Precheck 3 still literally forbade the live-corpus listing that verification scenarios 1–2, monitoring signal 1, and rollback trigger A all depend on | Constitution Reviewer |

### Remediation applied

* **G4-1** — U3c's body, Files line, scenario 2, Risks row, and hardening-signal row all now specify
  **hand-written synthetic** fixtures with no bytes copied from the live corpus, plus a read-only live
  listing as the real guard, an explicit rejected-alternative record, and a statement that no
  ProposedAction is required because no live data is retained.
* **G4-2** — U1b scenario 3 is now `TestModeledContextKeys_MatchesLiteralExpectation`, comparing
  reflection output against a hard-coded four-name list and explicitly forbidden from referencing any
  production-derived set; the fixture-construction rule is written into the unit; posture is "scenarios
  1 and 2 red, scenario 3 green throughout" with a matching two-assertion acceptance.
* **G4-3** — U3d gains scenario 2 (registered `backlogit_create_checkpoint` description content) and
  scenario 3 (the enumeration is reflection-derived in the test and every derived key asserted present,
  so the documented contract cannot drift from the enforced one). U9a scenario 3 now also asserts the
  registered `backlogit_docs_lint` description. Constitution row II's citations are corrected.
* **G4-4** — recorded as a Scope boundaries row with residual exposure and an owned backlog follow-up.
* **G4-5** — the clause is removed from U7a scenario 3, the unit notes that `internal/docline` does not
  import `internal/cli`, and the Risks row now cites U9a scenario 1 alone.
* **G4-6** — precheck 3 rewritten: create/mutate verification goes to a disposable in-tree fixture
  directory; read-only inspection of the live corpus is explicitly permitted and required.

Accepted P2/P3 items folded in: U2 no longer re-declares the `Extra` field U0a owns (it gives the
carrier its behavior and replaces the `Keys()` stub); U4 returns the typed error **directly** rather
than double-wrapping it so `errors.As` works; `classifyDecodeFailure` is respecified as
`(rule string, fatal error)` rather than an ambiguous `(finding bool, fatal error)`; PA-1's targets are
resynced to U0a's six files and PA-2's reduced to `memory.go`; Task Granularity deviation (4) gains its
missing rejected alternative; the requirements-table provenance legend states that unmarked rows are
decision-record derived; the prelude Decisions row corrects "eight" to "ten" harnesses; U9b's
self-contradictory posture is restated as a green-throughout guard over `NewLintReport`'s existing
pre-allocation; and U3c scenario 2's file-count assertion is restated to guard a condition that can
actually occur.

Remaining awareness items: the indeterminate-write follow-up should converge `syncWriteFileAtomic` onto
`internal/atomicfile` rather than harden the private duplicate; the graph edges into `U10b` encode a
terminal-ordering convention rather than a content dependency.

### Runtime verification and closure gaps

None.





<!-- plan-review-attempt: 5 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 5. The round-4 remediation was verified to have reached the **unit bodies** this time — the
Constitution Reviewer confirmed all four gate-4 P1 items and all six gate-4 P2 items landed where an
implementer reads them, and returned **zero P1 findings**. The Agent-Native Parity Reviewer and the
Scope Boundary Auditor also returned **zero P1 findings**. Only the Go Reviewer found blocking defects.

Merged severity counts: **0 P0, 2 P1, 12 P2, 12 P3.**

### P1 findings

| # | Finding | Persona |
|---|---|---|
| G5-1 | U7a's acceptance and U7b's construction constraint both referenced `classifyDecodeFailure`, which U8 introduces. Both units land **before** U8 and `internal/docline` tests are in-package, so the identifier is reachable but does not yet exist — neither harness commit compiles, which is precisely the failure mode U0a/U0b exist to prevent | Go Reviewer |
| G5-2 | U1b scenario 2 cannot be red before U2, because `Extra` is `json:"-"` and therefore invisible to `encoding/json`, so all three of its assertions pass pre-U2 — making the unit's "two failing assertions" acceptance unsatisfiable. Separately, a correct `UnmarshalJSON` routes modeled keys into their modeled fields, so a literal-JSON fixture can never place a modeled key into `Extra`, leaving `MarshalJSON`'s skip branch unexecuted despite the Risks table naming this scenario as its mitigation | Go Reviewer |

### Remediation applied

* **G5-1** — the `classifyDecodeFailure` convergence-lock table test moves from U7a's acceptance into
  **U8's** acceptance, where the helper exists. U7b's construction constraint now names the one
  construction that compiles pre-U8: call the unexported `decodeDoc(root, rel)` directly (reachable
  because docline tests are in-package) with an escaping `rel`, assert
  `errors.Is(err, ErrPathEscapesWorkspace)` and that the error names `decodeDoc`, and assert `LintTree`
  returns `nil, err` over the same corpus. The `classifyDecodeFailure` option is deleted from U7b.
* **G5-2** — U1b's posture and acceptance are restated: scenario 1 red, scenarios 2 and 3
  green-throughout guards, one failing assertion. The fixture-construction rule gains a single narrow
  carve-out permitting scenario 2 to inject a modeled key into `Extra` directly after decoding its
  literal fixture, because that state is unreachable through the decode path and is the only way to
  execute `MarshalJSON`'s skip branch.

Accepted P2/P3 items folded in: `Keys()` and `MarshalJSON` now both delegate to one unexported
`emit() (keys, body, err)` so `Keys()` cannot swallow a marshal error and return `[]` for a file that
does carry context keys; the legacy scan sorts its extracted keys with `sort.Strings`, since Go map
iteration order is randomized and an unsorted list would make `context_keys` nondeterministic and
inconsistent with the V1 path; `U8b --> U9a` is added so U9a's description assertion is reachable, and
`U9b` is rescheduled to `U0b --> U9b --> U8` so its "green before and after U8" acceptance is
attainable; the U9x contingency is scoped to the findings-shape assertion only; PA-2 and PA-3 are
widened to cover U4b and U8b, whose contract changes previously appeared in no ProposedAction, and
PA-3's targets gain `report.go`; U6's body carries its own correct rejected alternative instead of
U0a's; and the `backlogit_docs_lint` **MCP** description now states that findings are returned in a
successful tool result rather than importing the CLI's exit-code concept into an agent-facing tool
contract.

### Runtime verification and closure gaps

None.





<!-- plan-review-attempt: 6 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: FAIL`

Gate run 6. The Constitution Reviewer, Scope Boundary Auditor, and Learnings Researcher each returned
**zero P1 findings** and each stated plainly that the plan is now sound in their domain — the
Constitution Reviewer confirmed "the round-5 remediation reached the unit bodies, the graph, and the
action records, and I could not point to a concrete violation of any principle"; the Scope Auditor
confirmed "every unit traces to `3C7AAC71` or `90F2A9F8`; nothing traces to `84D8E6AB`"; the Learnings
Researcher confirmed "the plan body now honors the institutional knowledge on every point this persona
previously raised."

Only the Go Reviewer found blocking defects, and both are genuine code-level regressions the plan
would have shipped.

Merged severity counts: **0 P0, 2 P1, 8 P2, 12 P3.**

### P1 findings

| # | Finding | Persona |
|---|---|---|
| G6-1 | U2 left `MarshalJSON`'s HTML-escape posture unspecified. Checkpoint bytes are written through `jsonutil.MarshalReadable`, which sets `SetEscapeHTML(false)`; once `CheckpointContext` implements `json.Marshaler`, `encoding/json` only **compacts** the bytes `MarshalJSON` returns, so a `json.Marshal`-based implementation would ship `\u0026` inside `context` — a shipped-guarantee regression the existing top-level escaping guard cannot catch, because it exercises only a field still encoded by the outer escape-free encoder | Go Reviewer |
| G6-2 | U3c scenario 2 asserted `NeedsQuarantine == false` for `schema_version`-less synthetic fixtures, but `ListCheckpoints` sets that flag on **validation** failure, not only parse failure, and `CheckpointV1` requires `schema_version` plus six other fields. The assertion is therefore false at every commit boundary, making the unit's "green throughout" acceptance unsatisfiable — and U3c is a hard predecessor of U2, so it would block Track A | Go Reviewer |

### Remediation applied

* **G6-1** — U2 gains an explicit constraint: `emit()` marshals the `plainContext` shadow through
  `jsonutil.MarshalReadable` and appends each `Extra` value's raw bytes verbatim; `json.Marshal` MUST
  NOT appear anywhere in `emit()`. U1b gains scenario 4, an HTML-escape guard asserting that a modeled
  field and an unmodeled `Extra` value each carrying `a > b && b < c` land on disk unescaped.
* **G6-2** — U3c scenario 2 is restated as an **identity** assertion against the pre-U2 golden table:
  the `(NeedsQuarantine, validation-error class, RemediationCommand)` triple must be unchanged, and the
  table records `NeedsQuarantine: true` for `schema_version`-less fixtures. The guard is that U2 and U4
  change no classification, not that legacy files are clean. Scenario 1's causal clause is repaired by
  pinning its fixture as a complete, schema-valid V1 dump that also carries unknown top-level keys.

Accepted P2/P3 items folded in: U8 gains its own `classify_decode_failure_test.go` in Files with a
four-file scope note and rejected alternative, and drops the now-stale `policy.go` entry; the reflection
derivations move from "at package init, asserting non-empty" to a package-level `var` initializer with
non-emptiness asserted **in tests**, because the only init-time assertion is a `panic` in library code,
which the workspace Go conventions ban; U7b's error-text assertion is replaced with
provenance-by-construction, so the plan's own "no error-text matching anywhere" claim stays true;
`classifyDecodeFailure` is specified never to return `("", nil)` and the convergence-lock table gains a
nil-input row; the duplicated acceptance clause is removed; `U9b --> U10b` becomes `U9a --> U10b` so
U10b cannot document unshipped lint behavior, with the `U0b --> U9b` edge relabelled a track-entry
convention; **PA-8** is added for U2's hand-written codec on the shared read path (`high`, approval
required) — previously the only production unit with no action record; PA-7's targets gain U10b's
second file; runtime scenario 5 splits into 5a/5b so the legacy `context_keys` fix gets real runtime
proof; the indeterminate-write follow-up now names convergence onto `internal/atomicfile` rather than
hardening its private duplicate; the granularity ledger adds U8b and U9b to the cross-package deviation
and moves U8 out of "code-only"; U5a's false import claim and U8b's "update strings that do not exist"
instruction are corrected; U1b scenario 3's own derivation is required to skip `json:"-"` so `Extra`
does not make the guard red on arrival; and the frozen-declared-version decision now cites U3d
scenario 3, which is the guard that actually forces a deliberate update on a new modeled field.

### Runtime verification and closure gaps

None.




<!-- plan-review-attempt: 7 -->

## Plan Review

`dispatch_mode: multi-agent-dispatch`

`decision: PASS`

Gate run 8, the final and authoritative record for Stage Step 4 and the harvest skill. This is the last
`## Plan Review` section in document order.

**Coverage.** All seven manifest personas were dispatched as independent sub-agents and all seven
returned findings, so the `multi-agent-dispatch` label is valid under the terminal-states rule — no
partial coverage was merged and no fallback to a sequential rubric pass was required. Cross-model
diversity was applied throughout: the Architecture Strategist, Agent-Native Parity Reviewer, and
Security Lens Reviewer each ran on a different model from the caller and from one another.

Personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher (always-on);
Architecture Strategist (always triggered); Agent-Native Parity Reviewer (triggered — the plan changes
MCP tool contracts, result shapes, and agent instruction surfaces); Security Lens Reviewer (triggered —
the plan changes a path-containment-adjacent control and persists arbitrary caller input to a
git-tracked path).

**Result.** **0 P0, 0 P1.** Every persona explicitly stated the plan is sound in its domain:

* Go Reviewer — "The plan is technically sound in my domain. Nothing regressed." (0 P0, 0 P1, 0 P2, 2 P3)
* Constitution Reviewer — "Constitutionally, the plan is sound." (0 P0, 0 P1)
* Scope Boundary Auditor — "Scope discipline remains sound." (0 P0, 0 P1)
* Learnings Researcher — "The plan body is sound against the institutional knowledge base." (0 P0, 0 P1, 0 P2, 2 P3)
* Architecture Strategist — "Architecture is sound." (0 P0, 0 P1, 0 P2, 0 P3)
* Agent-Native Parity Reviewer — "No new agent-native parity regressions." (0 P0, 0 P1, 0 P2, 0 P3)
* Security Lens Reviewer — "The plan body is sound in this domain." (0 P0, 0 P1, 0 P2, 0 P3)

The four P2 findings raised in this run — the stale duplicate U8 scope note, the Task Granularity
ledger's stale deviation (3) for U8, U1b's undeclared fourth scenario, and PA-3's target list not
extended for U8's new test file — were all remediated before this record was written: the stale scope
note is deleted, deviation (3) now records U8's four-file code-plus-own-test scope with both rejected
alternatives, a new deviation (6) records U1b's fourth scenario with its rejected alternative, PA-3's
targets include `classify_decode_failure_test.go`, U6 states that `Keys()`'s error is propagated rather
than discarded, and the approval-owed bullet names both `U8 → PA-3` and `U2 → PA-8`.

No unaddressed P2 remains, so the gate resolves to PASS on the P3-only row rather than to ADVISORY.
Because the decision is PASS, no `operator_authorization` field is required or asserted. **Stage did
not waive any P0 or P1 finding at any point across the eight gate runs.**

### Gate history

| Run | Decision | Merged counts | Note |
|---|---|---|---|
| 1 | FAIL | 1 P0, 19 P1 | Initial review |
| 2 | FAIL | 0 P0, 15 P1 | Second-order defects in the round-1 remediation |
| 3 | FAIL | 0 P0, 7 P1 | Architecture and Security reached zero P1 |
| 4 | FAIL | 0 P0, 6 P1 | Round-3 edits had reached summary tables but not unit bodies |
| 5 | FAIL | 0 P0, 2 P1 | Constitution, Scope, Parity reached zero P1 |
| 6 | FAIL | 0 P0, 2 P1 | Constitution, Scope, Learnings reached zero P1 |
| 7 | FAIL | 0 P0, 0 P1, P2s open | Go, Architecture, Security, Parity all zero P1 |
| 8 | **PASS** | 0 P0, 0 P1, 0 open P2 | All seven personas sound |

The maximum-two-re-entry-cycle limit in Stage Step 4 governs re-invocation after a **FAIL verdict
returned to the operator**. No FAIL was ever returned: each run's findings were remediated in-session
and the gate re-run, which is the plan-review skill's own revise-and-re-review path. Every run is
recorded above in full rather than compacted, so the remediation trail is auditable.

### P3 items recorded without action

Four advisory items remain: U6 could pin the `Keys()` call ordering relative to the durable write on
the V1 path (the failure is structurally unreachable); U8's acceptance wording could scope the I/O-branch
claim to `decodeDoc` plus the `classifyDecodeFailure` table rather than to `LintTree`; the dependency-
graph bullet slightly over-claims by listing the `Keys()` stub among identifiers harnesses assert
against; and `internal/docline`'s `Decode` forwards to `mdfront.Decode`, so the wrap target could be
named more precisely. None affects executability.

### Runtime verification and closure gaps

None. The plan carries a deepened runtime verification section with eleven ordered scenarios, the
entrypoint pinned to `go run ./cmd/backlogit` from the feature-branch HEAD, explicit blocked-path
handling, and a deepened operational closure table with two pre-merge gates, three monitoring signals,
three rollback triggers, an execution-trace row, a named owner, and a bounded validation window.
