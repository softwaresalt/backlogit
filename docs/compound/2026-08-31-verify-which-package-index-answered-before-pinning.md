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
pip config debug                      # includes env vars and config file precedence
```

Keep the executable consistent across every step. `pip` and `python -m pip` can
resolve to *different* installations — the first is whatever `pip` is on `PATH`,
the second is whichever pip belongs to the `python` on `PATH`. Diagnosing one
and then querying the other reports a precedence that is not the one in play.
Pick one form and use it for `config list`, `config debug`, and `index versions`
alike.

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
pip --isolated index versions <package> --no-cache-dir
```

`--isolated` runs pip "ignoring environment variables and user configuration",
which neutralizes the `PIP_*` surface in one move — no enumeration and no
shell-specific `unset` loop. The one deliberate exception is `PIP_CONFIG_FILE`,
which pip still honors in isolated mode; that exception is what the next
section relies on.

### `--isolated` alone is not enough

Verified empirically, and this is the part that is easy to get wrong:

| Invocation | Reported max |
|---|---|
| `pip index versions autoharness` | 1.4.11 |
| `pip --isolated index versions autoharness` | **1.4.11** |
| `PIP_CONFIG_FILE=<null> pip --isolated index versions autoharness` | **1.5.0** |
| `https://pypi.org/pypi/autoharness/json` → `.info.version` | 1.5.0 |

`--isolated` ignores environment variables and **user** configuration — but not
**global** or **site** configuration files. On this machine `pip config debug`
located the proxy in the global scope, where `--isolated` does not reach:

```text
global:
    global.index-url: https://packagefeedproxy.microsoft.io/pypi/simple/
user:
  C:\Users\<user>\pip\pip.ini, exists: False
```

`--index-url` does not close that gap either: it replaces only the *primary*
index, so a global or site `extra-index-url`, `find-links`, or `no-index` still
participates in resolution.

### The one variable `--isolated` does not strip

`PIP_CONFIG_FILE` is the config-file *selector*, and pip still honors it in
isolated mode. Pointing it at the platform null device suppresses **every**
config-file scope — global, site, and user — which is precisely the gap
`--isolated` leaves open. That is why row 3 of the table above returns 1.5.0
without any `--index-url` at all.

Combine all three for a comparison that is actually canonical-only:

```bash
# POSIX shells (bash, zsh)
PIP_CONFIG_FILE=/dev/null pip --isolated index versions <package> \
    --index-url https://pypi.org/simple --no-cache-dir \
    --ignore-requires-python
```

```powershell
# PowerShell (the null device is 'nul' on Windows)
$had  = Test-Path Env:PIP_CONFIG_FILE
$prev = if ($had) { $env:PIP_CONFIG_FILE } else { $null }
try {
    $env:PIP_CONFIG_FILE = 'nul'
    pip --isolated index versions <package> `
        --index-url https://pypi.org/simple --no-cache-dir `
        --ignore-requires-python
}
finally {
    if ($had) { $env:PIP_CONFIG_FILE = $prev }
    else      { Remove-Item Env:PIP_CONFIG_FILE -ErrorAction SilentlyContinue }
}
```

The two shells differ in an important way here. The POSIX form is a *prefix
assignment*, which scopes the variable to that single command and leaves the
caller's environment untouched automatically. PowerShell has no equivalent, so
the value must be saved and restored explicitly — and restoring means putting
back a prior value if one existed, not blindly deleting the variable. A bare
`Remove-Item Env:PIP_CONFIG_FILE` at the end would silently discard a
`PIP_CONFIG_FILE` the caller had deliberately set, changing pip's behavior for
the rest of the session.

Each part does a distinct job, and none is redundant:

| Part | Neutralizes |
|---|---|
| `PIP_CONFIG_FILE=<null device>` | all config files: global, site, and user |
| `--isolated` | the `PIP_*` environment surface (except `PIP_CONFIG_FILE`) and user config |
| `--index-url https://pypi.org/simple` | states the intended index explicitly rather than relying on the default |
| `--ignore-requires-python` | the running interpreter's compatibility filter — see below |

### Source isolation is not scope isolation

Clearing the index sources still leaves a second, independent filter in play.
`pip index versions` only reports releases whose `Requires-Python` matches the
**running interpreter**, while the JSON `.info.version` cross-check reports the
registry's latest release regardless of interpreter. Compare those two directly
and an ordinary compatibility filter is easily misread as a stale index.

Verified on Python 3.14 against `autoharness` (`requires_python: >=3.10`), with
source isolation already applied in every row:

| Invocation | Result |
|---|---|
| `--python-version 3.9 --only-binary=:all:` | `ERROR: No matching distribution found` |
| same, plus `--ignore-requires-python` | `1.5.0` |
| `https://pypi.org/pypi/autoharness/json` → `.info.version` | `1.5.0` |

Row 1 is the trap: fully isolated, pointed at canonical PyPI, and still
disagreeing with the registry — for a reason that has nothing to do with which
index answered.

Choose the scope deliberately, and say which one you chose:

* comparing **absolute maxima** against the registry endpoint — pass
  `--ignore-requires-python`, as the recipe above does;
* asking **what this environment can actually install** — omit it, and describe
  the result as compatibility-scoped rather than as the registry maximum.

The null device is `/dev/null` on POSIX and `nul` on Windows; for a
shell-agnostic value use `python -c "import os; print(os.devnull)"`.

Use `pip config debug` to see which scope actually holds a setting rather than
assuming a flag covered it — that command is what located the global-scope
proxy here.

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

* Treat a discovered maximum version as **index-scoped and
  interpreter-scoped**, not absolute. Know which index answered *and* which
  Python was asked before you pin.
* Cross-check the maximum against the canonical registry endpoint when the pin
  gates CI. Isolate with all three of
  **`PIP_CONFIG_FILE=<null device>`**, **`pip --isolated`**, and an explicit
  **`--index-url https://pypi.org/simple`**. None is redundant: the selector
  kills every config-file scope, `--isolated` kills the rest of the `PIP_*`
  environment surface — it deliberately still honors `PIP_CONFIG_FILE`, which
  is what makes the selector work — and `--index-url` states the intended
  index. `--isolated` alone
  leaves global and site config active, and `--index-url` alone replaces only
  the primary index — a global `extra-index-url` or `find-links` survives both.
* Source isolation is not scope isolation. `pip index versions` filters by the
  running interpreter's `Requires-Python`; the registry JSON endpoint does not.
  Add `--ignore-requires-python` when comparing absolute maxima, or state that
  the result is compatibility-scoped. Otherwise a routine version floor reads
  as a stale index.
* Keep the pip executable consistent across diagnosis and query. `pip` and
  `python -m pip` may be different installations with different config.
* Do not isolate a config surface by enumerating its members. Pip maps nearly
  every long option to a `PIP_*` variable, so any `unset` list is a snapshot
  that is already incomplete and rots as the tool gains options. Prefer a
  namespace-clearing switch; where none exists, say plainly which subset was
  neutralized rather than implying the whole surface was.
* Use `pip config debug` to see which scope holds a setting before assuming a
  flag neutralized it.
* When any job sets `continue-on-error`, verify the *per-job* conclusion rather
  than the workflow run conclusion.
* An automated reviewer repeating the same version claim is not corroboration —
  it may be reading the same lagging surface. Verify against the registry.

## Related

`autoharness version` is a **subcommand**, not a flag. `autoharness --version`
exits non-zero with `Unknown command: --version`.
