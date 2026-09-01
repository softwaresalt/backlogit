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

### Isolating the comparison: stop enumerating, use `--isolated`

`--index-url` alone is **not** sufficient. It replaces the *primary* index, but
pip still consults `extra-index-url`, `PIP_EXTRA_INDEX_URL`, `PIP_FIND_LINKS`,
`PIP_NO_INDEX`, and more.

The tempting fix is to `unset` the offending variables. **Do not take that
path.** Pip exposes essentially every long option as a `PIP_<LONG_OPTION>`
environment variable, and `pip index versions` alone honors `--pre`,
`--all-releases`, `--only-final`, `--python-version`, `--platform`,
`--implementation`, `--abi`, `--ignore-requires-python`, `--no-binary`,
`--only-binary`, `--prefer-binary`, `--uploaded-prior-to`, and others. Any
hand-written list is a snapshot of an open-ended surface: each variable you add
still leaves the next one unhandled, and the list silently rots as pip gains
options.

Clear the *namespace*, not the members. Pip ships a first-class flag for
exactly this:

```bash
pip --isolated index versions <package> \
    --index-url https://pypi.org/simple --no-cache-dir
```

`--isolated` runs pip "ignoring environment variables and user configuration",
which neutralizes the entire `PIP_*` surface in one move — no enumeration, no
shell-specific `unset` loop, and identical syntax on POSIX and PowerShell.

### `--isolated` alone is still not enough

Verified empirically, and this is the part that is easy to get wrong:

| Invocation | Reported max |
|---|---|
| `pip index versions autoharness` | 1.4.11 |
| `pip --isolated index versions autoharness` | **1.4.11** |
| `pip --isolated index versions autoharness --index-url https://pypi.org/simple` | **1.5.0** |
| `https://pypi.org/pypi/autoharness/json` → `.info.version` | 1.5.0 |

`--isolated` ignores environment variables and **user** configuration — but not
**global** or **site** configuration. On this machine `pip config debug` located
the proxy in the global scope:

```text
global:
    global.index-url: https://packagefeedproxy.microsoft.io/pypi/simple/
user:
  C:\Users\<user>\pip\pip.ini, exists: False
```

So `--isolated` stripped the environment and still returned the stale 1.4.11.
Only adding an explicit `--index-url` — which outranks the surviving global
config — produced 1.5.0, matching the canonical registry.

**Both flags are required, and they do different jobs:** `--isolated` removes
the open-ended environment and user-config surface; explicit `--index-url`
overrides the global/site config that `--isolated` leaves standing. Use
`pip config debug` to see which scope actually holds a setting rather than
assuming `--isolated` covered it.

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
  gates CI. Isolate with **`pip --isolated ... --index-url https://pypi.org/simple`** —
  both parts are required. `--isolated` clears the open-ended `PIP_*` and
  user-config surface; explicit `--index-url` overrides the global/site config
  that `--isolated` leaves in place.
* Do not isolate a config surface by enumerating its members. Pip maps nearly
  every long option to a `PIP_*` variable, so any `unset` list is a snapshot
  that is already incomplete and rots as the tool gains options. Prefer a
  namespace-clearing switch; where none exists, say plainly which subset was
  neutralized rather than implying the whole surface was.
* When any job sets `continue-on-error`, verify the *per-job* conclusion rather
  than the workflow run conclusion.
* An automated reviewer repeating the same version claim is not corroboration —
  it may be reading the same lagging surface. Verify against the registry.

## Related

`autoharness version` is a **subcommand**, not a flag. `autoharness --version`
exits non-zero with `Unknown command: --version`.
