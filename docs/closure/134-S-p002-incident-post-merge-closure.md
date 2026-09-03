---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter. The autoharness pipeline-topology gate reads them via
# fm.get("closure_status") and fm.get("compaction_status"). Do NOT run
# `backlogit docs migrate --apply` against this file; doing so would fold these
# fields under docline: and silently re-break the 135-S predecessor-closure gate.
closure_status: READY
compaction_status: degraded
description: "Topology gate registration for 134-S/152-F post-merge closure — machine-readable predecessor-closure record. Authoritative narrative and P-002 incident disposition are in docs/closure/2026-08-30-152f-134s-p002-incident-closure.md."
doc_type: closure
docline:
  backlogit:
    gate_registration: true
    incident: INC-P002-152F-134S
    schema_version: "1.0"
ingested_at: "2026-09-03T02:23:00Z"
schema_version: "1.0"
source: docs/closure/134-S-p002-incident-post-merge-closure.md
title: "134-S / 152-F Post-Merge Closure Gate Registration"
---

# 134-S / 152-F Post-Merge Closure Gate Registration

**Shipment:** 134-S — Ship 152-F: governed lifecycle reconciliation and stash provenance correction  
**Feature:** 152-F — Governed lifecycle reconciliation and stash provenance correction  
**Merge commit:** `b415f40354e48dd238d4348f1be5eaca0ea2f1ad` (PR #396)  
**Closure date:** 2026-08-30  
**Authoritative closure narrative:** `docs/closure/2026-08-30-152f-134s-p002-incident-closure.md`  
**Gate registration created:** 2026-09-03

## Purpose

This file provides the machine-readable topology gate registration for shipment 134-S.
The `autoharness gate pipeline-topology` predecessor-closure check looks for a file
matching `{shipment_id}-*-post-merge-closure.md` in `docs/closure/` and reads the
top-level `closure_status` and `compaction_status` frontmatter fields from it.

The original closure narrative (`2026-08-30-152f-134s-p002-incident-closure.md`) does
not satisfy the gate's glob on two counts: its filename does not carry the
`post-merge-closure` suffix required by the pattern, and it uses the token `134s`
rather than `134-S` as the shipment identifier. This gate-registration file satisfies
that requirement without altering, superseding, or weakening the authoritative
closure narrative in any way.

**⚠ Do not run `backlogit docs migrate --apply` against this file.** The fields
`closure_status` and `compaction_status` are placed at the top level of the
frontmatter because the topology gate reads them at that location. Running
`docs migrate --apply` would fold them under `docline:`, which would silently
re-break the 135-S predecessor-closure gate. This constraint is documented in the
frontmatter comments above.

**This file is the machine-readable gate record only.** All P-002 incident details,
breach disposition table, runtime verification evidence, terminal resolution of the
11FFF601/150-F/133-S closure block, and provenance preservation statements are
recorded exclusively in the authoritative narrative referenced above. The P-002 breach
(INC-P002-152F-134S) is a permanent historical process incident acknowledged by the
operator on 2026-08-30 via PR #398; it is not waived, normalized, weakened, or
relabeled by this gate-registration file.

## Closure Evidence Summary

| Field | Value |
|---|---|
| Shipment archive | `.backlogit/archive/134-S.md` — `archived_status: shipped` ✅ |
| Feature archive | `.backlogit/archive/152-F.md` — `status: done` ✅ |
| Merge commit (PR #396) | `b415f40354e48dd238d4348f1be5eaca0ea2f1ad` ✅ |
| All task archives | 152.001-T through 152.011-T — `done → archived` at `b415f403` ✅ |
| 150.001-T reconciled | `archived_status: done`, `reconciled_at: 2026-08-30T05:00:02Z` ✅ |
| 150.002-T reconciled | `archived_status: done`, `reconciled_at: 2026-08-30T05:00:53Z` ✅ |
| 11FFF601 provenance | Corrected — canonical: 150-F, historical: 151-F preserved ✅ |
| 133-S archive | `archived_status: shipped` ✅ |
| P-002 breach | `acknowledged_historical_incident` — INC-P002-152F-134S |
| P-002 compliance claim | NONE — breach is not claimed compliant |
| Operator acknowledgement | PR #398 — 2026-08-30 ✅ |
| FC-5 forward control | Active — harvested to queued feature `163-F` (145-S, S11 plan); not a closure precondition |
| Compaction | Degraded — compact-context invocation not verifiably recorded for this historical closure (original PR #396, 2026-08-30) |

## Closure Status

**READY.** All shipment closure criteria were met or formally acknowledged as of
2026-08-30. The full closure rationale and per-criterion evidence table are in the
authoritative narrative at
`docs/closure/2026-08-30-152f-134s-p002-incident-closure.md`.

The P-002 breach (INC-P002-152F-134S) is a permanent historical process incident.
P-002 (TDD Gate / Harness-Satisfied Precondition) and Constitution Principle II
remain fully in force; this closure is a one-time, terminal, named disposition
for a specific historical incident only. It is not a policy change, not a waiver,
and not a precedent.

FC-5 (forward control: deterministic harness-wide workflow-policy enforcement
engine) has been deliberated via `064-DL` and harvested into queued feature
`163-F` (shipment 145-S, S11 plan at
`docs/exec-plans/2026-09-03-s11-workflow-policy-enforcement-engine-plan.md`).
It is not a closure precondition for 134-S.

## Gate Registration Rationale

This file was created on 2026-09-03 to resolve a
`PREDECESSOR_CLOSURE_INCOMPLETE` block on shipment 135-S. The topology gate found
`closure_complete: null` for predecessor 134-S because no file matching
`134-S-*-post-merge-closure.md` existed in `docs/closure/`. The underlying
closure of 134-S was complete on 2026-08-30; this file adds the machine-readable
gate recognition without altering any historical evidence, breach disposition, or
provenance record.

The correction is the minimum change required: a new file that satisfies the gate's
naming glob and provides the two required frontmatter fields. No existing files are
modified, renamed, or deleted.
