---
chunk_strategy: h1-h2-h3
description: "A version pin is only as trustworthy as the index that produced it: identify which index answered, and cross-check against the canonical registry before pinning."
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-31-verify-which-package-index-answered-before-pinning.md
title: "Dependency pinning: verify which package index answered before you pin"
---

## Insight

Before pinning a dependency to a discovered maximum version, establish **which
index answered the query**. `pip index versions` reports what the *configured*
index knows — which may be a corporate mirror, a proxy, or a stale local cache,
not the canonical registry. The command is not inherently wrong; the answer is
scoped to a resolver configuration that is easy to forget you have.

Cross-check the discovered maximum against the canonical registry before the
pin lands.

## Context

While hardening the `pipeline-topology` CI job, `pip index versions autoharness`
reported a maximum of `1.4.11`. That value was used to pin the job:

```yaml
pip install autoharness==1.4.11
```

The job immediately began failing with `Unknown gate subcommand:
pipeline-topology`. That subcommand does not exist before 1.5.0.

The canonical registry showed the real published maximum was `1.5.0` — the
version already installed locally. The configured index was a corporate proxy
feed (`packagefeedproxy.microsoft.io`) that had not yet surfaced the release.
The pin was corrected and the job went green.

## Verifying the Answer

Show which index is actually in play, then cross-check:

```powershell
# 1. What index will pip query?
pip config list                       # look for global.index-url / extra-index-url
python -m pip config debug            # includes env vars and config file precedence
```

In the environment where this failure occurred, that first command was the
whole story:

```text
global.index-url='https://packagefeedproxy.microsoft.io/pypi/simple/'
```

The configured index was a corporate proxy feed, not canonical PyPI. Nothing
about the `pip index versions` output disclosed that.

```powershell
# 2. What does the canonical registry say?
(Invoke-RestMethod 'https://pypi.org/pypi/<package>/json').info.version
```

### Forcing a genuinely isolated comparison

`--index-url` alone is **not** sufficient. It replaces the *primary* index, but
pip still consults any configured `extra-index-url` or `PIP_EXTRA_INDEX_URL` —
which is the very class of setting most likely to have produced the misleading
answer. Neutralize the config file and the environment overrides as well:

```powershell
# PowerShell. Use $env:PIP_CONFIG_FILE = '/dev/null' on POSIX shells;
# the portable value is python -c "import os; print(os.devnull)".
$env:PIP_CONFIG_FILE = 'nul'          # make pip ignore all config files
Remove-Item Env:PIP_INDEX_URL       -ErrorAction SilentlyContinue
Remove-Item Env:PIP_EXTRA_INDEX_URL -ErrorAction SilentlyContinue

pip index versions <package> --index-url https://pypi.org/simple --no-cache-dir
```

Setting `PIP_CONFIG_FILE` to the null device makes pip skip global, user, and
site config files; clearing the two environment variables removes the
higher-precedence overrides. Only then does `--index-url` describe the complete
resolver view.

Run against the same package that produced the bad pin, this returned
`LATEST: 1.5.0` — the value the proxy feed had been withholding.

## Why It Bites

The failure is deceptive for two compounding reasons:

1. **The lagging reading looks authoritative.** `pip index versions` is the
   obvious command for the question, and it returns a confident, well-formed
   answer with no indication that it reflects a mirror rather than the
   canonical registry.

2. **The CI job masks its own failure.** The topology job runs with
   `continue-on-error: ${{ vars.PIPELINE_TOPOLOGY_GATE_REQUIRED != 'true' }}`,
   so the *overall workflow run* reports `success` while the job itself shows
   `fail`. A green checkmark on the pull request does not mean the gate passed.

To see real per-job conclusions:

```powershell
gh run view <id> --json conclusion,jobs --jq '.conclusion, (.jobs[] | "\(.name) -> \(.conclusion)")'
```

Note also that `gh run list` includes the Copilot reviewer run (job
`copilot-pull-request-reviewer`), which is *not* the CI run.

## Rule

* Treat a discovered maximum version as **index-scoped**, not absolute. Know
  which index answered before you pin.
* Cross-check the maximum against the canonical registry endpoint when the pin
  gates CI. `--index-url` alone does not isolate the comparison — clear
  `PIP_CONFIG_FILE`, `PIP_INDEX_URL`, and `PIP_EXTRA_INDEX_URL` too.
* When any job sets `continue-on-error`, verify the *per-job* conclusion rather
  than the workflow run conclusion.
* An automated reviewer repeating the same version claim is not corroboration —
  it may be reading the same lagging surface. Verify against the registry.

## Related

`autoharness version` is a **subcommand**, not a flag. `autoharness --version`
exits non-zero with `Unknown command: --version`.
