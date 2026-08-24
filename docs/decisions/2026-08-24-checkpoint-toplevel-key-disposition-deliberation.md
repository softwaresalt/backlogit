---
title: "Deliberation: preserve or refuse on unmodeled top-level keys in checkpoint disposition rewrites"
description: "Design decision for stash D3CE9E81 — how core.AbandonCheckpoint and events.ResolveCheckpoint must treat checkpoint documents whose top-level namespace is not closed, including the resolve path's missing validity gate and the abandon/quarantine remediation deadlock"
source: "backlogit stash D3CE9E81 (scope-boundary follow-up 1 of 8, docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md)"
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Status

Decided. Depth: `deep`. Promotion: `plan`.

Session posture: dark factory mode (`DARK_MODE_ACTIVE`, P-017) with a bounded scope of stash
`D3CE9E81` only. `agent-intercom` tools are unavailable, so remote operator visibility is
**degraded**; all decisions below were taken autonomously under the pre-authorized dark-mode
scope and are recorded here in place of an intercom broadcast.

## Provenance

| Field | Value |
|---|---|
| Stash entry | `D3CE9E81` (kind `task`, priority `high`) |
| Origin | Scope boundary follow-up 1 of 8, `docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md` §"Scope boundaries and recorded follow-ups" |
| Parent work | Feature `146-F`, shipment `129-S` (both shipped and archived) |
| External source | `3C7AAC71` |
| Release-blocking | No — explicitly recorded as residual exposure, not release-blocking for `129-S` |

Two excluded surfaces in that table map to this single stash entry:

* `core.AbandonCheckpoint` (`internal/core/checkpoint_disposition.go`)
* `events.ResolveCheckpoint` (`internal/events/checkpoint_lifecycle.go`)

## Problem Frame

### The reported defect

Both surfaces perform the same `parse -> mutate -> re-marshal` round-trip on a **pre-existing**
file:

1. `events.ParseCheckpoint(data)` — a plain `json.Unmarshal` into `CheckpointV1` with no
   unknown-field handling, so any top-level key not modeled by the struct is silently discarded.
2. Mutate the struct (`Status`, `UpdatedAt`, and for abandon the four `disposition*` fields).
3. `jsonutil.MarshalReadable(cp)` — re-emits **only** the modeled fields.
4. Write those bytes over the original file.

`146-F` / `129-S` closed the nested `context` case by giving `CheckpointContext` an
`Extra map[string]json.RawMessage` carrier with a custom `UnmarshalJSON`/`MarshalJSON` pair
(`internal/events/checkpoint_schema.go`). The **top level** was explicitly deferred to this entry
because it is a different design question, not a mechanical repeat.

### What the codebase already decided

Three shipped contracts constrain the answer.

**C1 — The top level is a CLOSED namespace at create.** `checkClosedSchemaNamespace`
(`internal/events/checkpoint_strict.go`, 146.011-T / U4) walks the create payload as an ordered
token stream and rejects any top-level key that is not a modeled `CheckpointV1` JSON tag, plus the
four reserved `disposition*` keys and the reserved `status: "abandoned"` literal. The
`CheckpointContext` doc comment states the split in terms: the context namespace is
"intentionally OPEN", "distinct from the CheckpointV1 top level and the nested progress object,
both of which enforce a CLOSED schema namespace at the create boundary".

**C2 — A document that cannot be trusted to round-trip must be moved verbatim, never rewritten.**
`docs/design-docs/checkpoint-administrative-disposition.md`, "Malformed-Only vs Valid-Only Split
Rationale": *"Quarantine must never rewrite a malformed document — since the document cannot be
trusted to round-trip through parse/marshal, quarantine moves the original bytes verbatim
instead."* `AbandonCheckpoint` rewrites in place and is therefore validity-gated;
`QuarantineCheckpoint` moves bytes byte-identically and is therefore the malformed-only verb. The
two are disjoint by design and each names the other in its refusal error
(`ErrCheckpointUseQuarantine`, `ErrCheckpointUseAbandon`).

**C3 — Legacy (non-V1) dumps are written verbatim at create.** `CreateCheckpoint`
(`internal/events/memory.go:55-121`) probes for `schema_version == 1`. Only that branch parses,
namespace-checks, validates, and re-marshals. Any other dump is written through **unchanged**, and
its context keys are recovered by a best-effort `legacyContextKeys` scan. This is how arbitrary
top-level keys legitimately exist on disk.

### Ground truth on the nine live files

The stash text records "all nine live files under `.backlogit/checkpoints/` carry such keys". That
is accurate, and inspection sharpens it materially. The nine legacy files carry **no**
`schema_version`, **no** `agent`, **no** `session_id`, **no** `created_at`, and **no**
`updated_at`; only `phase` and (in six of them) `resume_hint` are modeled fields. Two additional
files (`checkpoint-20260822-064434.json`, `checkpoint-20260822-212617.json`) are conforming V1
documents with zero unmodeled top-level keys.

| File | Unmodeled top-level keys |
|---|---|
| `checkpoint-20260406-171334.json` | `branch`, `compaction`, `review_findings_fixed`, `fixes`, `validation`, `notes` |
| `checkpoint-20260411-051040.json` | `shipment_id`, `shipment_status`, `feature_id`, `feature_status`, `branch`, `pr_number`, `pr_url`, `tasks_completed`, `tasks_blocked`, `review_findings_resolved`, `commits`, `merge_status`, `memory_file` |
| `checkpoint-20260421-164238.json` | `session`, `shipment`, `feature`, `tasks`, `harness_files`, `server_changes`, `build`, `red_phase`, `structural_tests_passing`, `next` |
| `checkpoint-20260424-162622.json` | `consumer_id`, `shipment_id`, `feature_id`, `tasks_completed`, `tasks_blocked`, `branch`, `pr_number`, `pr_status`, `merge_sha`, `closure_status`, `memory_file`, `decisions`, `next_steps` |
| `checkpoint-20260424-174043.json` | `version`, `consumer_id`, `shipment_id`, `branch`, `pr_number`, `pr_url`, `pr_status`, `ci_status`, `copilot_review`, `items_completed`, `items_blocked`, `decisions`, `next_steps` |
| `checkpoint-20260424-204116.json` | `consumer_id`, `shipment_id`, `branch`, `pr_number`, `pr_url`, `merge_sha`, `items_completed`, `items_blocked`, `decisions`, `errors` (plus `status: "SHIPPED"`, not a legal enum value) |
| `checkpoint-20260426-031618.json` | `consumer_id`, `shipment_id`, `feature_id`, `task_ids`, `branch`, `pr_number`, `pr_url`, `pr_status`, `ci_status`, `items_completed`, `items_blocked`, `commits`, `review_artifact`, `review_gate`, `decisions`, `follow_up_tasks`, `next_steps` |
| `checkpoint-20260426-045333.json` | `version`, `consumer`, `shipment_id`, `feature_id`, `branch`, `pr_number`, `pr_url`, `ci_status`, `tasks_completed`, `tasks_queued`, `review_artifact`, `review_gate`, `copilot_comments_resolved`, `last_commit`, `decisions`, `next_steps` |
| `checkpoint-20260801-051014.json` | `consumer`, `command`, `mode`, `stash_consumed`, `feature`, `tasks`, `dependencies`, `shipment`, `shipment_status`, `deliberation`, `plan`, `review`, `deferred_stash`, `archived_duplicate`, `next` |

### Three findings that reshape the decision

**F1 — Abandon and resolve are NOT symmetric today.** `AbandonCheckpoint` runs
`ParseCheckpoint` **and** `ValidateCheckpoint` before any rewrite and refuses a failure with
`ErrCheckpointUseQuarantine`. All nine legacy files fail `ValidateCheckpoint` (`schema_version`
must equal 1; `agent`, `session_id`, `status`, `created_at`, `updated_at` are `required`).
**Abandon therefore already refuses all nine files.** Its residual loss surface is narrow: a
document that parses, validates, *and* carries extra top-level keys. Post-`146-F` that shape can
no longer be produced by `CreateCheckpoint`; it is reachable only by hand-editing or by an older
binary.

**F2 — `ResolveCheckpoint` has no validity gate at all, and its failure mode is far worse than
key loss.** It calls `ParseCheckpoint` only — never `ValidateCheckpoint`. Run against any of the
nine legacy files it does not merely drop the unmodeled keys: it *replaces the entire document*
with a fabricated near-empty V1 skeleton — `"schema_version": 0`, `"agent": ""`,
`"session_id": ""`, `"created_at": "0001-01-01T00:00:00Z"`, `"context": {}` — plus
`"status": "resolved"` and a fresh `updated_at`. Every decision, PR number, merge SHA, and
next-step record in the file is destroyed, and the replacement is itself schema-invalid. This is
the live, reachable, high-severity half of `D3CE9E81`, and it is strictly larger than the reported
"unmodeled top-level keys are dropped".

`CleanupCheckpoints` is not exposed to this: it validates before archiving and skips invalid files
with a recorded error. `ListCheckpoints` is read-only and already surfaces these files with
`needs_quarantine: true` plus a `RemediationCommand`. The reachable callers of the destructive
path are `backlogit checkpoint resolve` (`internal/cli/checkpoint.go:228`) and
`backlogit_resolve_checkpoint` (`internal/mcp/tools.go:1223`) — and both Stage and Ship
session-start recovery protocols instruct agents to call resolve on leftover checkpoints.

**F3 — A naive "refuse" creates an unremediable deadlock.** If abandon and resolve refuse a
parseable, schema-**valid** document that carries extra top-level keys, `QuarantineCheckpoint`
also refuses it, because its classification is `parse OK && validate OK -> ErrCheckpointUseAbandon`.
The operator would be told "use quarantine" by one verb and "use abandon" by the other, with no
way out. Any refusal decision must therefore also widen quarantine's malformed classification, or
it ships a trap.

## Research Findings

### Prior learnings retrieved (bounded search, `docs/compound/` only)

The delegated learnings-researcher stalled in the prior session; this retrieval was performed
directly and confined to `C:\Source\GitHub\backlogit\docs\compound\`.

| Path | Confidence | Bearing on this decision |
|---|---|---|
| `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | **high** | The same defect class in the artifact codec: a typed round-trip through `WriteArtifactFile` drops unmodeled top-level frontmatter keys (`archived_from`, `archived_status`), making archive records non-invertible. Its closing "Evidence" line names *this exact fork*: the path "should preserve unmodeled archive-only frontmatter keys (**or refuse to mutate** archived items) rather than silently dropping provenance." It also demonstrates the preserve half in production: the `--section`-only and `--size` paths reserialize the **raw** frontmatter map and therefore keep unmodeled keys, while the typed field-flag path does not. Precedent that both arms are viable and that mixing them in one entry point is the trap. |
| `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | **high** | Any re-persist seam must reload from the source of truth, not from a lossy projection, or fields absent from the projection vanish. The checkpoint file *is* the source of truth here, so the analogue is: do not let a struct that models less than the document become the write source. |
| `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md` | **high** | Enforce the invariant in the core mutation seam **before** schema resolution, not only in a downstream projection — "the setter protects the file (source of truth); the projection protects the index". Directly supports gating inside `AbandonCheckpoint`/`ResolveCheckpoint` rather than relying on the create boundary alone. |
| `docs/compound/2026-06-28-codec-extraction-leaf-packages.md` | **medium** | Prove a behavior-preserving change with **differential golden byte-equality** tests plus idempotency, and put shared logic in a stdlib-only leaf so no new import edge appears. Shapes the verification strategy and the placement of any shared conformance helper. |
| `docs/compound/2026-06-26-docline-frontmatter-contract.md` | **medium** | The body-preserving codec pattern: mutate only the intended field and leave every other byte untouched. This is the technique Option C would need. |
| `docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md` | **low** | Applied to this session's `compact-context` archival (use `git mv` for tracked artifacts), not to the code change. |

No compound learning contradicts the closed-namespace contract, and none records a prior decision
to preserve unmodeled **top-level** keys anywhere in the codebase.

### Codebase evidence

| Location | Fact |
|---|---|
| `internal/events/checkpoint_schema.go:22-58` | `CheckpointV1` field set; `Disposition*` fields are `omitempty`, `DispositionAt` is `*time.Time` so a never-abandoned checkpoint omits it |
| `internal/events/checkpoint_schema.go:60-108` | `CheckpointContext.Extra` carrier — the shipped precedent for an OPEN namespace, with `json:"-"` load-bearing |
| `internal/events/checkpoint_schema.go:400-408` | `ParseCheckpoint` = plain `json.Unmarshal`, no unknown-field handling |
| `internal/events/checkpoint_strict.go:57-70` | `checkpointV1TopLevelKeys` derived by reflection, minus the four reserved `disposition*` keys |
| `internal/events/checkpoint_strict.go:154-186` | `checkClosedSchemaNamespace` — ordered token-stream diff; returns typed `*CheckpointUnknownFieldError` |
| `internal/events/checkpoint_lifecycle.go:139-180` | `ResolveCheckpoint` — parse, no validate, mutate, re-marshal, atomic write |
| `internal/core/checkpoint_disposition.go:44-124` | `AbandonCheckpoint` — parse **and** validate gated, audit-before-rewrite, `MutationEnvelope` |
| `internal/core/checkpoint_disposition.go:143-241` | `QuarantineCheckpoint` — classify in memory, refuse valid targets, verbatim `moveNoReplace`, sidecar upsert |
| `internal/events/memory.go:55-121` | `CreateCheckpoint` — V1 branch is strict; non-V1 dumps written verbatim |
| `internal/events/checkpoint_lifecycle_test.go:19-25` | Test fixtures are built from a `CheckpointV1` struct via `json.Marshal`, so they are conforming by construction — existing resolve/abandon tests are unaffected by adding a conformance gate |

## Options Evaluated

### Option A — Raw top-level carrier (preserve)

Mirror `CheckpointContext.Extra` at the top level: add `Extra map[string]json.RawMessage` with
`json:"-"` to `CheckpointV1` plus a custom `UnmarshalJSON`/`MarshalJSON` pair that routes
unmodeled top-level keys into `Extra` and re-emits them after the modeled fields.

* **Pros** — symmetric with the shipped 146-F context fix; nothing is ever lost; abandon and
  resolve keep succeeding on every document they accept today; one recognizable pattern.
* **Cons** —
  * It **inverts C1**. The top level would be rejected-at-create but preserved-on-rewrite, giving
    one namespace two contradictory policies and quietly legitimising out-of-band schema
    extension: hand-inject a key, and every future rewrite faithfully carries it forward.
  * It does **not** fix F2. For a legacy file the loss is not only the unmodeled keys — the
    zero-valued modeled fields (`schema_version: 0`, `agent: ""`, `created_at:` zero time) are
    still fabricated and written. Option A preserves the payload while corrupting the envelope,
    and the result is *newly* schema-invalid in a way it was not before.
  * `MarshalJSON` on `CheckpointV1` interacts with `jsonutil.MarshalReadable` and with the
    reflection-derived `checkpointV1TopLevelKeys`; the `Extra` field must stay invisible to
    `encoding/json` exactly as the context carrier does, adding a second place where that subtlety
    must be right.
* **Effort** — medium.

### Option B — Refuse to mutate, and close the resolve gap (fail closed)

Treat "unmodeled top-level keys present" as *non-conforming* and refuse to rewrite, routing the
caller to the verbatim-move verb. Concretely: a shared read-boundary conformance check
(the same token-stream machinery as `checkClosedSchemaNamespace`, but over the **full** modeled
key set including the four `disposition*` keys, since abandon legitimately writes them); a
`ValidateCheckpoint` gate added to `ResolveCheckpoint`; and a widened
`QuarantineCheckpoint` classification so a valid-but-non-conforming document is quarantinable
(F3).

* **Pros** —
  * Preserves C1 with a single policy: the top level is closed everywhere, at create and at
    rewrite.
  * Directly instantiates C2 — a document that cannot be trusted to round-trip is moved verbatim,
    not rewritten. `QuarantineCheckpoint` already does exactly that, byte-identically, with an
    audit append and a disposition sidecar.
  * Fixes F2 completely and at the root: with a validity gate, resolve can no longer destroy a
    legacy file, regardless of which keys it carries.
  * No new carrier to keep synchronised with the struct; the reflection-derived key set already
    exists.
  * Matches the recommendation recorded in the closest prior learning
    (`2026-07-17-backlogit-update-drops-archive-provenance.md`) and the seam-level enforcement rule
    from `2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`.
* **Cons** —
  * Behaviour change on `ResolveCheckpoint`: calls that previously "succeeded" (destructively) now
    return a typed error. This is the point of the change, but it must be explicit, tested, and
    documented, and the nine live files become resolve-refusing.
  * Adds a third classification outcome to the abandon/quarantine pair, so the disjointness
    invariant and its design doc must be restated rather than assumed.
* **Effort** — medium.

### Option C — Byte-preserving surgical member rewrite

Replace `parse -> struct -> marshal` with a token-stream edit that rewrites only the `status`,
`updated_at`, and `disposition*` members in place, leaving every other byte untouched.

* **Pros** — the strongest preservation of all three options: unmodeled keys, key order, and
  formatting all survive, and no zero-valued modeled field is ever fabricated. Mirrors the
  body-preserving codec pattern already proven in `internal/mdfront`.
* **Cons** — hand-rolled JSON surgery on a governed write path is materially riskier than the
  existing typed round-trip; it still legitimises arbitrary top-level extension (same C1 inversion
  as Option A); and it silently blesses a non-conforming document as a first-class, mutable
  checkpoint rather than routing it to its designed disposition. It is a *bigger* change than the
  defect warrants and would need its own byte-equality harness across every document shape.
* **Effort** — high.

### Option D — Do nothing; document the exposure

* **Pros** — zero risk of regression.
* **Cons** — leaves F2 live. `backlogit checkpoint resolve` on any of the nine files silently
  destroys the record, and both agent session-start protocols instruct exactly that call. Not
  acceptable.
* **Effort** — none.

## Trade-off Comparison

| Criterion | A — Preserve carrier | B — Refuse + gate | C — Byte-preserving edit | D — Document only |
|---|---|---|---|---|
| Honours the closed top-level namespace (C1) | No — inverts it | **Yes** | No — inverts it | Yes (vacuously) |
| Honours "never rewrite an untrustworthy doc" (C2) | No | **Yes** | No | Yes (vacuously) |
| Fixes the reported key loss on abandon | Yes | **Yes** (by refusing) | Yes | No |
| Fixes the resolve envelope destruction (F2) | **No** | **Yes** | Partially | No |
| Avoids the abandon/quarantine deadlock (F3) | n/a | Yes, once quarantine widens | n/a | n/a |
| Nine live files made safe | No | **Yes** | Yes | No |
| New lossy-round-trip surface introduced | Yes (a second carrier to keep in sync) | No | Yes (custom JSON writer) | No |
| Behaviour change to existing green tests | Low | Low (fixtures are struct-built) | Medium | None |
| Blast radius | Medium | Medium | High | None |
| Effort | Medium | Medium | High | None |

## Decision

**Adopt Option B — refuse to mutate a non-conforming checkpoint, and close the `ResolveCheckpoint`
validity gap.**

The deciding argument is that this repository has already answered the general form of the
question, twice, in shipped contracts. C1 says the top level is a closed namespace. C2 says a
document that cannot be trusted to round-trip must be moved verbatim rather than rewritten. A
document carrying unmodeled top-level keys is, by C1, not a conforming `CheckpointV1`; therefore by
C2 the correct treatment is to refuse the rewrite and route it to `QuarantineCheckpoint`, which
already moves bytes byte-identically with an audit append and a disposition sidecar. Option A would
make the same namespace closed in one direction and open in the other, and — decisively — would
still leave the nine live files exposed to F2, which is the larger and more dangerous half of the
defect.

### Decided behaviour

1. **Shared conformance check.** Add a read-boundary conformance helper in `internal/events`
   alongside `checkClosedSchemaNamespace`, reusing `decodeTopLevelEntries`, `isFoldKeyIn`, and
   `modeledJSONTagKeys` so the key set stays reflection-derived and cannot desynchronise from the
   struct. It differs from the create-boundary check in exactly two ways: the four `disposition*`
   keys are **legal** (abandon writes them), and the reserved `status: "abandoned"` literal is
   **legal** (an already-abandoned document must remain readable and re-classifiable). Nested
   `progress` keys are diffed against `checkpointProgressKeys` exactly as at create.
2. **`AbandonCheckpoint` refuses non-conforming targets.** After the existing parse and validate
   gates and before the audit append, run the conformance check. On unknown keys, return a typed
   error naming the offending keys and directing the caller to quarantine. No audit event is
   appended and the file is not touched — matching the existing
   "audit append happens BEFORE any rewrite" ordering guarantee in reverse.
3. **`ResolveCheckpoint` gains a validity gate and the same conformance check.** It must run
   `ValidateCheckpoint` after `ParseCheckpoint`, refusing an invalid document with a typed error
   that names the quarantine remediation, and must then run the conformance check with the same
   refusal. The existing idempotent-already-resolved short-circuit and the
   `ErrCheckpointCannotResolveAbandoned` guard stay ahead of the new gates so their current
   semantics are unchanged.
4. **`QuarantineCheckpoint` widens its malformed classification (F3).** A target that parses and
   validates but fails the conformance check is classified **malformed** and is quarantinable. Only
   a target that parses, validates, **and** conforms is refused with `ErrCheckpointUseAbandon`.
   This keeps the two verbs disjoint and total: every document is dispositionable by exactly one
   of them.
5. **No preservation carrier is added to `CheckpointV1`.** The top-level namespace remains closed
   in both directions.
6. **The nine live legacy files are left untouched by this work.** They are already
   `needs_quarantine: true` in `ListCheckpoints` output, and after this change no rewrite path can
   reach them. Actually disposing of them is workspace hygiene, not a code change, and is out of
   scope (see below).

### Scope boundary

In scope: `internal/events/checkpoint_lifecycle.go`, `internal/events/checkpoint_strict.go`,
`internal/core/checkpoint_disposition.go`, `internal/errors/checkpoint_errors.go`, their tests, the
`checkpoint-administrative-disposition.md` design doc, and the CLI/MCP error surfacing for the new
refusals.

Explicitly **out of scope**:

* Disposing of the nine live legacy checkpoint files, and the stale active `129-S` checkpoint
  `checkpoint-20260822-212617.json` — workspace hygiene, recorded as a follow-up, not a code
  change.
* Any change to `CreateCheckpoint`'s legacy (non-V1) verbatim passthrough. Tightening it would
  sweep the existing on-disk corpus into rejection and is a separate decision.
* A structured JSON error envelope for CLI validation failures — already recorded as stash
  `63E810D9`.
* `CleanupCheckpoints`, `ListCheckpoints`, and hook checkpoints under `.backlogit/runtime/hooks/`.
* The seven sibling scope-boundary follow-ups (`EA1F5912`, `EC987334`, `1787FD85`, `5F4E0FC3`,
  `360A183F`, `63E810D9`, `6CE00B88`).

## Rejected Alternatives

* **Option A (preserve carrier)** — inverts the closed-namespace contract established one shipment
  ago, and leaves the higher-severity resolve envelope destruction (F2) unfixed. Preserving the
  payload while fabricating a corrupt envelope is not a fix.
* **Option C (byte-preserving surgical edit)** — the most preservation for the most risk. Hand-rolled
  JSON member surgery on a governed write path needs its own byte-equality harness across every
  document shape, and it still blesses non-conforming documents as mutable, contradicting C1 and C2
  just as Option A does.
* **Option D (document only)** — leaves a reachable, agent-invoked, silent data-destruction path
  live. Rejected outright.
* **Making `ParseCheckpoint` strict globally** — already rejected in
  `docs/exec-plans/2026-08-21-success-shaped-evidence-loss-plan.md`: it would sweep the entire
  existing on-disk corpus into quarantine candidacy. The conformance check is therefore a
  **caller-invoked** gate on the two rewrite paths, never a change to `ParseCheckpoint` itself.

## Unresolved Questions

1. Should the refusal reuse `ErrCheckpointUseQuarantine` or introduce a distinct sentinel (e.g.
   `ErrCheckpointNonConforming`) that wraps it? A distinct sentinel gives callers a precise
   `errors.Is` handle and lets the message name the offending keys, while wrapping keeps the
   existing "use quarantine instead" remediation string intact for callers already matching on it.
   Resolved in planning; the plan must pick one and pin it with a test.
2. Should the `ResolveCheckpoint` invalid-document refusal reuse `ErrCheckpointInvalid` or a new
   resolve-specific sentinel? Deferred to planning.
3. Does the widened quarantine classification need a matching change to the `RemediationCommand`
   text emitted by `ListCheckpoints` for valid-but-non-conforming files? `ListCheckpoints` sets
   `NeedsQuarantine` from `ValidateCheckpoint` only, so such a file would list clean today.
   Deferred to planning as a scoped decision, not an automatic yes.

## Risks and Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| `ResolveCheckpoint` becomes stricter and breaks a caller that relied on lenient resolve | Medium | Existing tests build fixtures from the struct, so they conform by construction. Add explicit red-phase tests for both the invalid and the non-conforming refusal, and confirm the CLI and MCP surfaces report the typed error rather than a bare failure. |
| The abandon/quarantine deadlock (F3) ships if the quarantine widening is dropped from the plan | **High** | Make the widening a first-class implementation unit with its own test asserting that a valid-but-non-conforming document is accepted by quarantine and refused by abandon — the two assertions in one table so neither can be removed alone. |
| Conformance key set drifts from `CheckpointV1` | Medium | Derive it via the existing reflection helper `modeledJSONTagKeys`, never a hand-written literal, and assert the derived set in a test. |
| A future modeled field is added and the create-boundary and read-boundary sets diverge | Medium | Both sets derive from the same reflection helper over the same struct; add a test asserting the read-boundary set equals the create-boundary set plus the reserved `disposition*` keys. |
| The nine live legacy files remain undisposed | Low | Out of scope by decision; recorded as a workspace-hygiene follow-up. They are already flagged `needs_quarantine: true`, and after this change no rewrite path can damage them. |
| Windows atomic-write regression on the changed rewrite paths | Low | No change to the write mechanism — `atomicfile.WriteFileAtomic` and `syncWriteFileAtomic` are untouched. |
