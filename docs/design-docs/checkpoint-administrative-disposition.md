---
chunk_strategy: h1-h2-h3
description: "Design rationale for the checkpoint abandon and quarantine verb pair (136-F)"
doc_type: design
schema_version: "1.0"
source: docs/design-docs/checkpoint-administrative-disposition.md
title: Checkpoint Administrative Disposition
---

# Checkpoint Administrative Disposition

## Summary

This design introduces two administrative disposition verbs for session
checkpoints: `AbandonCheckpoint` and `QuarantineCheckpoint`. Both are exposed
through the CLI (`backlogit checkpoint abandon`, `backlogit checkpoint
quarantine`) and the MCP server (`backlogit_abandon_checkpoint`,
`backlogit_quarantine_checkpoint`), and both route through the same core
implementation so the two surfaces produce identical observable state.

## Total Four-Class Disposition Contract

The classification below is **total only over `status: "active"` documents with
no administrative disposition** (I3 scoping, pinned by 147.009-T / U5). A
document already carrying an administrative disposition (`disposition:
abandoned` or `disposition: quarantined`) is governed by the precedence rule
in the next section, not by this table.

| Class | abandon | resolve | quarantine |
|---|---|---|---|
| valid + conforming | accept | accept | refuse (`ErrCheckpointUseAbandon`) |
| valid but non-conforming | refuse (`ErrCheckpointNonConforming`) | refuse (`ErrCheckpointNonConforming`) | **accept** |
| parses but schema-invalid | refuse (`ErrCheckpointUseQuarantine`) | refuse (`ErrCheckpointUseQuarantine`) | accept |
| does not parse | refuse (`ErrCheckpointUseQuarantine`) | refuse (`ErrCheckpointUseQuarantine`) | accept |

The "parses but schema-invalid" and "does not parse" classes are named
separately rather than folded into a single "malformed" row: they are
distinct failure modes at the `ParseCheckpoint` boundary versus the
`ValidateCheckpoint` boundary, and the schema-invalid-but-parseable shape is
the exact shape of the nine live legacy checkpoint files this feature exists
to handle. A table that only named parse failure would not be total over
active checkpoints and would silently omit the class most in need of a
disposition path.

The split exists because the three verbs have different safety requirements:

* `AbandonCheckpoint` and `ResolveCheckpoint` both rewrite the checkpoint in
  place. They can only do so safely once the document is known to parse,
  validate, and conform to the closed top-level (and nested `progress` /
  `context`) key namespace — hence both refuse the non-conforming class in
  addition to the two malformed classes.
* `QuarantineCheckpoint` must never rewrite a document it cannot fully trust
  to round-trip through parse/marshal. Since a non-conforming document is
  schema-valid but carries structure outside the enforced namespace, it is
  safer to move it verbatim than to risk losing or reshaping the unmodeled
  data during a rewrite — so quarantine **accepts** it rather than refusing.
  The invariant preserved is C2: a document that cannot be trusted to
  round-trip is moved verbatim, never rewritten.

Combining these behaviors into a single verb would require guessing which
safety contract applies to a given target; keeping the three verbs' accept/
refuse boundaries explicit and machine-checkable (via `ErrCheckpointUseAbandon`,
`ErrCheckpointUseQuarantine`, and `ErrCheckpointNonConforming`) removes that
guesswork at the boundary.

## Administrative Disposition Precedence

`ResolveCheckpoint` checks the document's existing administrative disposition
**before** applying the four-class validity/conformance classification above.
A document that has already been administratively abandoned
(`disposition: abandoned`) returns `ErrCheckpointCannotResolveAbandoned` from
`resolve` regardless of which of the four validity/conformance classes it
would otherwise fall into — the disposition guard fires first, and resolve
never reaches the classification table for such a document.

This precedence guard is a regression test for the implemented check
ordering, not a new feature: it existed before the four-class table above was
written and is recorded here only so the table's scope — "active documents
with no administrative disposition" — is not read as silently overriding it.


## Metadata Fields and Allowlist

`CheckpointV1` (`internal/events/checkpoint_schema.go`) carries four
disposition fields, populated only by `AbandonCheckpoint`:

* `disposition` — validates against the closed allowlist `abandoned` |
  `quarantined` (`events.DispositionAbandoned`, `events.DispositionQuarantined`);
  any other value fails closed.
* `disposition_reason` — the caller-supplied justification.
* `disposition_operator` — the resolved operator identity.
* `disposition_at` — the UTC timestamp the disposition was applied.

`Status` itself is widened from `oneof=active resolved` to `oneof=active
resolved abandoned` so an abandoned checkpoint is unambiguously terminal.

## Sidecar Filename Convention

`QuarantineCheckpoint` never rewrites the checkpoint's bytes, so its
disposition record is written as a sidecar file rather than inline: given a
quarantined checkpoint at `<archive>/checkpoints/<filename>`, the sidecar is
written at `<archive>/checkpoints/<filename>.disposition.json`
(`events.CheckpointDispositionSidecarPath`).

**153.002-T (S1 U2) — the sidecar write is CREATE-ONLY (not an idempotent
upsert)**: re-running quarantine on the same filename while a sidecar already
exists at the destination returns `ErrCheckpointDestinationOccupied`, not a
silent success. A post-link fsync failure returns `ErrWriteIndeterminate`,
meaning both the move and sidecar are committed but durability is uncertain;
do NOT blindly retry on that outcome. Prior semantics ("idempotent upsert")
are superseded by this contract.

## Audit Ordering and Failure Classes

Both verbs append a `checkpoint_disposition` audit event
(`internal/core/checkpoint_audit.go`) to a single cross-cutting audit trail
(`checkpoint-disposition-audit.jsonl`) **before** any move or rewrite of the
checkpoint file. The audit append is classified against the two-class
durable-write contract (`internal/errors/durability_errors.go`):

* A definite non-application (`ErrWriteNotApplied` class, or a plain error)
  maps to `ErrCheckpointAuditNotApplied`. Nothing was appended and nothing was
  moved or rewritten — the operation is safe to retry.
* An indeterminate outcome (`ErrWriteIndeterminate` class) maps to
  `ErrCheckpointAuditIndeterminate`. The disposition verb refuses and leaves
  the target untouched; callers must reconcile state before retrying rather
  than blindly retrying (a retry could double-append the audit event).

## Exact-Target Guarantee

`ResolveDispositionTarget` (`internal/core/checkpoint_target.go`) confines
the caller-supplied filename to a bare basename inside the workspace's
checkpoints directory, rejecting path separators, `..` traversal, absolute
paths, volume-qualified paths, UNC paths, and symlinked targets. Both verbs
verify (via hash comparison in the test suite) that exactly one file changes
per call: `AbandonCheckpoint` rewrites only the named target in place;
`QuarantineCheckpoint` moves only the named target, byte-identical, to the
archive.

## Operator Provenance

Every disposition surface requires an explicit, non-empty operator identity
and never defaults to a fixed string such as `"backlogit"`:

* CLI: `--operator` flag, falling back to `BACKLOGIT_OPERATOR`, falling back
  to the current OS user. An unresolvable operator refuses with
  `ErrCheckpointOperatorRequired`.
* MCP: `operator` is a required tool parameter. It is never inferred — an
  agent caller must always supply it explicitly.

## Read-Only Listing Change

Prior to this change, `ListCheckpoints` physically quarantined any
unparseable checkpoint file as a side effect of listing. This coupling meant
an agent merely inspecting the checkpoint directory could silently mutate it.
`ListCheckpoints` is now strictly read-only: a malformed file is surfaced with
`NeedsQuarantine: true` and a `RemediationCommand` (e.g. `backlogit checkpoint
quarantine checkpoint-bad.json --reason <reason>`) so a caller can explicitly
invoke `QuarantineCheckpoint` when ready. Any agent or workflow that relied on
the old auto-quarantine side effect must now call the remediation command
explicitly.

## Naming Boundary Against Artifact `abandoned` Status

The `abandoned` value written to a checkpoint's `disposition` and `status`
fields is scoped entirely to the checkpoint disposition domain
(`internal/events/checkpoint_schema.go`, `internal/events/checkpoint_disposition_types.go`).
It is unrelated to any `abandoned` status a backlog artifact (feature, task,
subtask) may carry in its own WIT lifecycle. The two domains share a word,
not a state machine — a checkpoint disposition never reads or writes artifact
frontmatter, and an artifact status transition never reads or writes
checkpoint disposition fields.

## Residual Exposure

This repository dogfoods a separately pinned `C:\Tools\backlogit.exe` binary
for its own harness operations (backlog queue management, MCP server
invocation for this very session). That pinned binary predates this feature
and does not yet expose the abandon/quarantine verbs; it is rebuilt and
re-pinned independently of this repository's release cadence. This is a known,
accepted residual exposure and out of scope for 136-F.

## Explicit Out-of-Scope Items

Per the implementation plan (`docs/exec-plans/2026-08-07-checkpoint-administrative-disposition-plan.md`),
the following are explicitly out of scope for this feature: `SetEscapeHTML`,
any change touching stash `B5D7E401`, `CheckpointDelete`, recovery/resume
flows, and hook checkpoints under `.backlogit/runtime/hooks/`.

