---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Policy engine review-fix classes from S11"
description: "Recurring review and Copilot finding classes on the S11 workflow-policy enforcement engine (145-S): unsigned evidence, registry sourcing, derived-fact timestamps, RawMessage digests, git replace refs, cancellation vs exit code, trust root, and EOL fixtures."
doc_type: learning
source: docs/compound/2026-10-17-policy-engine-review-fix-classes.md
docline:
    date: 2026-10-17T00:00:00Z
    severity: medium
    tags:
        - review
        - policyengine
        - 145-S
---

# Policy engine review-fix classes

## Problem

The S11 engine (`internal/policyengine`, 145-S) drew findings in eight classes that apply to any Go code that enforces policy from evidence and git state.

## Learnings

* **Unsigned evidence is not proof.** Do not let a gate pass on evidence that carries no verifiable signature or binding; require the seal before trusting a fact.
* **Never load the registry from the workspace under test.** A policy registry read from the checked-out workspace can be edited by the change being judged. Source it from a trusted location.
* **Propagate timestamps into derived facts.** A fact derived from timestamped evidence must carry that timestamp, or expiry and ordering checks silently use the wrong time.
* **Normalize `json.RawMessage` before digesting.** `encoding/json` HTML-escapes (`<`, `>`, `&`) when re-marshalling, so a digest over re-encoded bytes differs from one over the original. Normalize consistently on both sides.
* **Disable git replace refs.** Run git with `--no-replace-objects` (and ignore `refs/replace`) so history inspection cannot be redirected by replacement objects.
* **Cancellation versus exit code.** On Windows a cancelled context can surface as a process exit code rather than `ctx.Err()`. Check `ctx.Err()` first and classify cancellation separately from tool failure.
* **Pass the trust root explicitly via argv.** Do not infer the trust root from environment or cwd; take it as an explicit argument.
* **Keep fixtures EOL-stable.** CRLF fixtures fail `TestU175_001_BaselineEol`; write test fixtures with LF.
* **Audit by class.** Before pushing a fix, grep for sibling paths that share the flaw.
