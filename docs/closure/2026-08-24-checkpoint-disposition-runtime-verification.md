---
chunk_strategy: h1-h2-h3
description: "Runtime verification of the checkpoint top-level-key disposition contract (147-F, shipment 130-S): refusal path, acceptance, quarantine evidence integrity, recovery-sweep discrimination, context-duplicate parity, and abandoned-resolve handler mapping."
doc_type: closure
docline:
    date: 2026-08-27T00:00:00Z
    status: accepted
    tags:
        - runtime-verification
        - checkpoint
        - disposition
        - 147-F
        - 130-S
schema_version: "1.0"
source: docs/closure/2026-08-24-checkpoint-disposition-runtime-verification.md
title: "147-F / 130-S — Checkpoint Disposition Runtime Verification"
---

# Runtime Verification: Checkpoint Top-Level-Key Disposition (147-F, shipment 130-S)

**Surface**: `cli` and `mcp` (backlogit CLI — `checkpoint list`, `checkpoint get`,
`checkpoint resolve`, `checkpoint abandon`, `checkpoint quarantine`; MCP tool —
`backlogit_resolve_checkpoint`).
**Mode**: manual — a branch-built binary exercised against an in-tree, git-ignored scratch
workspace, seeded with synthetic fixtures modeling the shapes of the live legacy corpus. No
bytes are copied from `.backlogit/checkpoints/`; that directory is read only for a live,
enumerated SHA-256 before/after comparison and is never a mutation target.
**Context**: branch `feat/147-f-checkpoint-toplevel-key-disposition`, worktree HEAD at the time
of this run `f0acf3cd`. Units 147.019-T / U10, 147.026-T / U10b, and 147.041-T / U10c share one
scratch workspace and one evidence artifact per the plan's execution contract.

## Binary Provenance

Built from the branch under test with the repository's own version ldflags
(`Makefile:6-8`), placed at `.copilot/scratch/checkpoint-verification/backlogit-verify.exe`
(git-ignored, in-tree; confirmed via `git check-ignore -v .copilot/scratch/checkpoint-verification/x.json`
→ `.gitignore:5:.copilot/`).

```json
{
  "version": "v1.10.0-180-gf0acf3cd",
  "commit": "f0acf3cd",
  "build_date": "2026-08-28T02:29:11Z"
}
```

`commit` (`f0acf3cd`) equals `git rev-parse --short HEAD` at build time.

## Scratch Workspace

Created inside the repository working tree at `.copilot/scratch/checkpoint-verification/`
(never `%TEMP%`), initialized via `backlogit-verify.exe init` (which materializes the current
`.backlog` storage-root naming convention — functionally equivalent to `.backlogit` for every
disposition behavior this verification exercises). The mirror used by U10b is a child of this
same root at `.copilot/scratch/checkpoint-verification/mirror/`, confirmed ignored by the same
`.gitignore:5:.copilot/` rule before its first write. No `.gitignore` or other configuration
file is committed by this verification.

## U10 — Refusal Path

### Row 1 — `list` read verdict (legacy-shaped, schema-invalid fixture)

`checkpoint list` reports `needs_quarantine: true` with a structured `remediation_intent`
object — `verb: "quarantine"`, `approval_class: "A4c"`, `reason: "schema_invalid"` — carrying no
shell string of its own (the sibling, deprecated `remediation_command` field is unrelated to
this structured object).

### Row 2 — `get` read verdict (valid-but-non-conforming fixture)

`checkpoint get` on a schema-valid document carrying one unmodeled top-level key (`review_gate`)
reports `conforming: false`, `needs_quarantine: true`, and
`non_conforming_fields.paths: ["review_gate"]` in raw, unquoted form.

### Row 3 — refused `resolve` (legacy-shaped, schema-invalid)

Refused with `checkpoint_use_quarantine`-class error naming quarantine as the required verb.
Fixture bytes SHA-identical before and after.

### Row 4 — refused `abandon` (valid-but-non-conforming)

Refused with `checkpoint_non_conforming`-class error naming the offending key `"review_gate"`
and quarantine as the required verb. Fixture bytes SHA-identical before and after.

### Row 5 — accepted `quarantine` (valid-but-non-conforming)

Destination confirmed absent before the call. Accepted; archived bytes byte-identical to the
pre-quarantine original; source removed from the active checkpoints directory.

```text
evidence_row: unit=U10 row=1 filename=checkpoint-u10-legacy-resolve.json sha256=91c26923e80e4025a2c37d6570dff2a0f4a5bbf22d265b1dbfddd5b76e71cbfa state=schema_invalid destination=none outcome=unchanged
evidence_row: unit=U10 row=2 filename=checkpoint-u10-nonconforming-get.json sha256=418bd21e2573edba86cbec0a558a5bf6bd8a791ab27398457360f401d5f52304 state=non_conforming destination=none outcome=unchanged
evidence_row: unit=U10 row=3 filename=checkpoint-u10-legacy-resolve.json sha256=91c26923e80e4025a2c37d6570dff2a0f4a5bbf22d265b1dbfddd5b76e71cbfa state=schema_invalid destination=none outcome=refused
evidence_row: unit=U10 row=4 filename=checkpoint-u10-nonconforming-abandon.json sha256=418bd21e2573edba86cbec0a558a5bf6bd8a791ab27398457360f401d5f52304 state=non_conforming destination=none outcome=refused
evidence_row: unit=U10 row=5 filename=checkpoint-u10-nonconforming-quarantine.json sha256=418bd21e2573edba86cbec0a558a5bf6bd8a791ab27398457360f401d5f52304 state=non_conforming destination=archive/checkpoints/checkpoint-u10-nonconforming-quarantine.json outcome=accepted
evidence_scalar: unit=U10 live_corpus_sha_unchanged=true
evidence_scalar: unit=U10 binary_commit=f0acf3cd
```

## U10b — Acceptance, Quarantine Evidence Integrity, and the Recovery Sweep

### Row 1 — accepted `abandon` (conforming active fixture)

Accepted; response reports `disposition: "abandoned"`.

### Row 2 — accepted `resolve` (conforming active fixture)

Accepted; response reports `status: "resolved"`.

### Row 3 — quarantine evidence integrity

The archived payload from U10 row 5 and its `<filename>.disposition.json` sidecar both exist;
the sidecar's `filename` field names the original filename exactly.

### Row 4 — refused second quarantine of the same filename (declared intentional collision)

A fresh source file was recreated under the same basename as the already-archived U10 row 5
target and quarantined again. Refused with `checkpoint_destination_occupied`. The archived
payload and its sidecar are both byte-unchanged after the refused attempt (payload no-clobber;
no claim is made that the sidecar write is itself no-replace — the sidecar upsert is not
reached because the payload move is refused first).

### Row 5 — recovery sweep against the mirror

Mirror seeded with the nine enumerated legacy filenames (synthetic legacy-shaped content, not
copied live bytes) plus three additional conforming fixtures. `checkpoint list` against the
mirror reports `needs_quarantine: true` for **exactly** the nine enumerated filenames and
`false` for all three others; `checkpoint resolve` independently succeeds on all three others.

```text
evidence_row: unit=U10b row=1 filename=checkpoint-u10b-conforming-abandon.json sha256=e396d1e1e1c2f021eeed263ca359f6512ba2710e0f2e0f6d2298760e60781938 state=conforming destination=none outcome=accepted
evidence_row: unit=U10b row=2 filename=checkpoint-u10b-conforming-resolve.json sha256=9e34f41101305d6893f5b52d9b35fa9e46ac57baa58ed1540da83f8d86fe81da state=conforming destination=none outcome=accepted
evidence_row: unit=U10b row=3 filename=checkpoint-u10-nonconforming-quarantine.json sha256=418bd21e2573edba86cbec0a558a5bf6bd8a791ab27398457360f401d5f52304 state=archived destination=archive/checkpoints/checkpoint-u10-nonconforming-quarantine.json outcome=unchanged
evidence_row: unit=U10b row=4 filename=checkpoint-u10-nonconforming-quarantine.json sha256=c2cc2192f2d81093ae3ec111d0a9fbe1b14802672ca918d1bd5927f286031641 state=non_conforming destination=archive/checkpoints/checkpoint-u10-nonconforming-quarantine.json outcome=refused
evidence_row: unit=U10b row=5 filename=checkpoint-20260406-171334.json sha256=fbb0e73a3c568751f10e5e058915073b02c91888c0d14fefb9323b6fb8838aa8 state=schema_invalid destination=none outcome=refused
evidence_scalar: unit=U10b sweep_refused_count=9
evidence_scalar: unit=U10b sidecar_pair_intact=true
evidence_scalar: unit=U10b live_corpus_sha_unchanged=true
```

## U10c — Context-Duplicate Parity and Abandoned-Resolve

### Row 1 — exact-duplicate `context` member (cross-surface)

A document carrying `"context":{"foo":"a","foo":"b"}` (a literal duplicate JSON key) is refused
by both `resolve` and `abandon`, naming `duplicate:context.foo`; `list` and `get` both report
`needs_quarantine: true` with a matching `non_conforming_fields.paths: ["duplicate:context.foo"]`;
`quarantine` accepts it. Bytes are SHA-identical (`6ec38583...`) across the resolve refusal, the
abandon refusal, and the pre-quarantine original.

### Row 2 — fold-variant aliasing and the open-namespace guard

* `shipment_id` + `Shipment_Id` (aliasing the modeled `CheckpointContext.ShipmentID` field):
  refused, naming `duplicate:context.shipment_id`.
* `foo` + `Foo` (distinct unmodeled fold variant): `resolve` **succeeds**; the resolved file's
  `context` object still carries both `"foo"` and `"Foo"` — neither is silently dropped.
* A fixture whose sole routing member is spelled `Context` (capital C, a fold match on the
  top-level routing key) with an inner exact duplicate `bar`/`bar`: refused, naming
  `duplicate:context.bar` — pinning 147.033-T / U2h's fold-match routing at runtime.

### Row 3 — abandoned-resolve handler mapping

Invoking the MCP `backlogit_resolve_checkpoint` tool on an already-abandoned document (via a
scratch-only verification program bound to this workspace, never the dirty primary worktree)
returns `{"error":"validation_failed", ...}`, not `{"error":"internal"}`.

```text
evidence_row: unit=U10c row=1 filename=checkpoint-u10c-ctxdup-resolve.json sha256=6ec3858387cb5725aa2022413aefc3a14a5f92630fe11e54a5eec54e0fc99254 state=non_conforming destination=none outcome=refused
evidence_row: unit=U10c row=2 filename=checkpoint-u10c-fold-modeled.json sha256=b77cbd859bea4f18896ad4c20fbc48394f5d838d10dc11a9679bed0b167ce011 state=non_conforming destination=none outcome=refused
evidence_row: unit=U10c row=3 filename=checkpoint-u10c-already-abandoned.json sha256=82658bcfd1307af6e08c30d890aa2e184fa2fef0d5a1896b93be7c08aea2b9aa state=abandoned destination=none outcome=refused
evidence_scalar: unit=U10c abandoned_resolve_code=validation_failed
evidence_scalar: unit=U10c teardown_completed=true
```

## Live Corpus Integrity

Every file under `.backlogit/checkpoints/` (29 files, enumerated by directory listing, not
count-pinned) was SHA-256 hashed before the first scratch-workspace write and re-hashed after
every unit (U10, U10b, U10c) completed. The comparison shows **zero** differences at every
checkpoint: the live directory was never a mutation target.

## Teardown

The scratch workspace (`.copilot/scratch/checkpoint-verification/`, including the mirror and
the one-off MCP verification program) is removed by 147.041-T / U10c after all three rows above
passed. Nothing under `.copilot/` was ever tracked (`git ls-files .copilot` returns 0 entries
both before and after), so teardown leaves no trace in the tracked tree.
