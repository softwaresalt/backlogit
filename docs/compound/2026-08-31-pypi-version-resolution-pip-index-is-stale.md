---
chunk_strategy: h1-h2-h3
description: "Why `pip index versions` returns stale maxima and why the PyPI JSON API is the only authoritative source when pinning a tool version in CI."
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-31-pypi-version-resolution-pip-index-is-stale.md
title: "Dependency pinning: `pip index versions` is stale — use the PyPI JSON API"
---

## Insight

Never pin a CI dependency to a version discovered via `pip index versions`.
That command reads a cached/mirrored index that can lag the real registry by
multiple releases. The authoritative source is the PyPI JSON API.

```powershell
(Invoke-RestMethod 'https://pypi.org/pypi/<package>/json').info.version
```

## Context

While hardening the `pipeline-topology` CI job, `pip index versions autoharness`
reported a maximum of `1.4.11`. That value was used to pin the job:

```yaml
pip install autoharness==1.4.11
```

The job immediately began failing with `Unknown gate subcommand:
pipeline-topology`. The subcommand does not exist before 1.5.0.

The PyPI JSON API showed the real published maximum was `1.5.0` — the version
that had actually been installed locally all along. The pin was corrected and
the job went green.

## Why It Bites

The failure is especially deceptive here for two compounding reasons:

1. **The stale reading looks authoritative.** `pip index versions` is the
   obvious command for the question, and it returns a confident, well-formed
   answer with no staleness indicator.

2. **The CI job masks its own failure.** The topology job runs with
   `continue-on-error: ${{ vars.PIPELINE_TOPOLOGY_GATE_REQUIRED != 'true' }}`,
   so the *overall workflow run* reports `success` while the job itself shows
   `fail`. A green checkmark on the PR does not mean the gate passed.

To see real per-job conclusions:

```powershell
gh run view <id> --json conclusion,jobs --jq '.conclusion, (.jobs[] | "\(.name) -> \(.conclusion)")'
```

Note also that `gh run list` includes the Copilot reviewer run (job
`copilot-pull-request-reviewer`), which is *not* the CI run.

## Rule

* Resolve published versions through the registry's JSON API, not through
  `pip index versions`.
* When a pin change lands, verify the *per-job* conclusion, not the workflow
  run conclusion, whenever any job sets `continue-on-error`.
* An automated reviewer repeating a stale-version claim is not corroboration —
  it may be reading the same stale surface. Verify against the registry.

## Related

* `autoharness version` is a **subcommand**, not a flag. `autoharness
  --version` exits non-zero with `Unknown command: --version`.
