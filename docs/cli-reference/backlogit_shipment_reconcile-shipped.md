---
chunk_strategy: h1-h2-h3
description: Reconcile a legacy archived shipment to shipped with governed evidence binding
doc_type: reference
ingested_at: "2026-09-13T04:01:57Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_reconcile-shipped.md
title: backlogit shipment reconcile-shipped
---

## backlogit shipment reconcile-shipped

Reconcile a legacy archived shipment from `archived_status: active` to
`archived_status: shipped`.

### Synopsis

`backlogit shipment reconcile-shipped` is a CLI-only repair path for a shipment
that was already delivered but was archived with the wrong archived status.
It is explicit-confirmation-gated and fully audited.

Use it only when you can prove the shipment's delivery merge from a trusted ref
and a shipment-scoped closure note.

```text
backlogit shipment reconcile-shipped <shipment-id> [flags]
```

[!WARNING]
This command is confirmation-gated and audited. It is not a
machine-authenticated approval boundary. A caller that can run backlogit can
also supply `--confirm` or allocate a TTY. The `--actor` and
`--second-approver` values are self-asserted audit attributes. Authenticated
operator authorization is a deferred residual tracked as `866FDC8C`; see
[`docs/decisions/2026-09-06-866fdc8c-trust-boundary-split-crypto-authz.md`](../decisions/2026-09-06-866fdc8c-trust-boundary-split-crypto-authz.md).

### Preconditions

The command is intentionally narrow. It only accepts this v1 repair case:

* the target artifact is a shipment
* the shipment resolves under `.backlogit/archive/`
* `status` is `archived`
* `archived_status` is `active`
* the manifest is non-empty
* every manifest member is already terminal under the v1 allowlist:
  `done` or `accepted`
* archived members are re-evaluated by their archived status using the same
  allowlist
* descoped or rejected members are not supported in v1 and cause a refusal

### Evidence binding

The repair is allowed only when both evidence anchors succeed:

* `--merge-sha` must name a full git object ID for a true merge commit with two
  or more parents
* that merge commit must be reachable from a trusted ref
  (`reconcile.trusted_refs`; when unset, backlogit uses the repository default
  branch)
* `--closure-evidence` must resolve to a non-empty regular file inside the
  workspace
* the closure note must mention the shipment ID and bind the delivery merge with
  a shipment-scoped `(feature)` role marker

The verifier accepts the real closure shape below:

```text
---
shipments:
  - 047-S
  - 048-S
---

### 048-S
Merged as PR #101 (feature), commit ac8847..., and PR #102 (post-merge closure), commit 0cf49a...
```

Backlogit looks for `PR #<n> (<role>), commit <sha>` tokens. When a heading such
as `### 048-S` is present, parsing is scoped to that shipment section. Inside
that scope, exactly one `(feature)`-annotated commit must exist and it must
match `--merge-sha`. A `(post-merge closure)` or `(closure)` commit does not
satisfy the delivery-merge check.

[!IMPORTANT]
The closure path participates in replay identity, but closure file content does
not. Closure bytes are hashed separately into the evidence digest.

### Idempotency and replay

The command stores a scalar request-identity digest built from:

* `idempotency_key`
* `merge_sha`
* `reason`
* `actor`
* `second_approver`
* ordered `evidence_refs`
* the closure path

The digest does not hash closure content or evidence file bytes.

Replay behavior is strict:

* same idempotency key plus the same request identity returns `no_op`
* same idempotency key plus a different request identity returns `conflict`
* if a crash left prepared resume markers in frontmatter but the durable event
  was not appended yet, a same-request replay resumes from the persisted
  prepared event instead of recomputing evidence

### Dry run

`--dry-run` performs the full precondition and evidence evaluation without doing
any reconciliation write.

The shipment archive file, SQLite index row, and item log remain unchanged.
Lock sidecars or lock directories may still appear because the same lock helpers
are used for a truthful preview. Those lock artifacts are not reconciliation
writes.

Dry-run does not require `--confirm`.

### Rollback and indeterminate states

Backlogit distinguishes between writes that definitely did not apply and writes
whose outcome is ambiguous:

* `ErrWriteNotApplied` means the failed write did not reach the canonical state.
  Backlogit restores the pre-mutation snapshot of the shipment archive file and
  its SQLite row while the write locks are still held
* `ErrWriteIndeterminate` means the write may already have committed. Backlogit
  does not restore the snapshot, because restoring could overwrite a repair that
  already landed

An `indeterminate` result means you must inspect the shipment file and its item
log before retrying. Follow up with `backlogit doctor` or equivalent manual
inspection rather than assuming the write failed cleanly.

### Locking model

At an operator level, the transaction uses two phases:

* the shipment membership lock is held for the full transaction
* slow evidence verification runs while only the membership lock is held
* the shipment item-log lock and the artifact-mutation lock batch are acquired
  together only for the atomic decision-and-write phases
* those write-phase locks are taken in item-log-first order, then
  artifact-mutation order
* no reconciliation writes occur during the slow evidence-verification phase

This design prevents manifest drift during the repair while avoiding long-held
write locks during git and closure verification.

### Confirmation

For a real mutation, `--confirm` must exactly equal:

```text
reconcile-shipped <shipment-id>
```

Example:

```text
reconcile-shipped 048-S
```

If stdin is an interactive TTY, backlogit may prompt for that exact phrase.
Without `--dry-run`, any other token is refused.

### Examples

```text
  backlogit shipment reconcile-shipped 048-S --reason "legacy shipped repair" --actor "release-ops" --idempotency-key "repair-048-001" --merge-sha ac8847c9c0ffee1234567890abcdef1234567890 --closure-evidence docs/closure/2026-09-01-047-s-048-s-closure-summary.md --confirm "reconcile-shipped 048-S"
  backlogit shipment reconcile-shipped 048-S --reason "preview only" --actor "release-ops" --idempotency-key "repair-048-preview" --merge-sha ac8847c9c0ffee1234567890abcdef1234567890 --closure-evidence docs/closure/2026-09-01-047-s-048-s-closure-summary.md --dry-run
```

### Options

```text
      --actor string             audit actor recorded on the reconciliation event (required)
      --closure-evidence string  shipment-scoped closure note inside the workspace (required)
      --confirm string           exact confirmation phrase for non-dry-run mutations: reconcile-shipped <shipment-id>
      --dry-run                  evaluate the repair without changing the archive file, index row, or item log
      --evidence-ref strings     additional evidence paths to record and bind, in order
  -h, --help                     help for reconcile-shipped
      --idempotency-key string   idempotency key for replay detection (required)
      --merge-sha string         trusted delivery merge commit SHA (required)
      --reason string            audit reason recorded verbatim on the reconciliation event (required)
      --second-approver string   optional second audit approver, distinct from --actor
```

### Outcomes and exit behavior

The result surface uses four outcomes:

| Outcome | Meaning | Exit behavior |
|---|---|---|
| `reconciled` | the shipment was updated to `archived_status: shipped` and the durable reconciliation event was appended | `0` |
| `no_op` | the same request was already durably recorded, or a persisted same-request resume completed without changing the requested identity | `0` |
| `conflict` | the same idempotency key was reused for a different request identity, or previously recorded state conflicts with this repair | non-zero refusal |
| `indeterminate` | backlogit could not prove the current state, or a write may already have committed and now needs inspection | non-zero refusal |

Precondition failures, evidence failures, and confirmation failures are also
non-zero refusals.

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md) - Manage shipment work groups
* [backlogit reconcile](backlogit_reconcile.md) - Reconcile archived items by correcting their lifecycle status