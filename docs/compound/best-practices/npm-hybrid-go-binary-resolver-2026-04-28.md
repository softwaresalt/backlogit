---
title: "Hybrid npm wrapper for Go binary distribution (three-tier resolver)"
description: "Distribute a native Go binary via npm without requiring Go on the user's system using a three-tier PATH/optionalDep/GitHub-Releases resolver with SHA256 verification."
problem_type: best_practice
category: best_practice
component: cli
root_cause: missing_import
resolution_type: feature_gate
severity: high
message: "Distribute a native Go binary via npm without requiring Go on the user's system using a three-tier PATH/optionalDep/GitHub-Releases resolver with SHA256 verification."
file_path: "npm/backlogit-mcp/lib/resolve.js"
resolved: true
tags: [npm, go, binary-distribution, copilot-plugin, nodejs, sha256, optional-dependencies, postinstall]
date: 2026-04-28
---

## Problem

A Go CLI tool needs to be invokable via `npx @scope/package` without requiring
Go to be installed on the end user's system. npm has no native mechanism for
distributing pre-built binaries — it is designed for Node.js code. Platform
packages exist but require careful wiring to work reliably across package managers
and CI environments.

## Symptoms

- `npx @backlogit/backlogit-mcp mcp` fails with "binary not found"
- npm install succeeds but the binary is not executable on the target platform
- `go run` workaround works but adds 5–10 s latency per invocation

## What Did Not Work

| Approach | Why Rejected |
|---|---|
| `go run ./cmd/backlogit` from Node entrypoint | 5–10 s startup latency per invocation — unacceptable for a CLI tool |
| CGO-dependent builds | Static compilation fails with linked system libraries |
| npm gyp (native modules) | Requires build toolchain at install time; fails in locked-down CI |
| Embedded WASM | Go WASM support is experimental; out of scope |

## Solution

A **three-tier resolver** that falls through from fastest to most reliable:

```
Tier 1 → PATH   (system install, zero latency)
Tier 2 → npm optional dep   (platform package, zero latency)
Tier 3 → GitHub Releases + SHA256 → local cache   (network, one-time)
```

### Tier 1 — PATH lookup

```js
function tier1() {
  try {
    const cmd = process.platform === 'win32' ? 'where' : 'which';
    const result = execFileSync(cmd, ['backlogit'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim().split(/\r?\n/)[0];
    if (result && fs.existsSync(result)) return result;
  } catch { /**/ }
  return null;
}
```

### Tier 2 — optional npm platform package

```js
const PLATFORM_MAP = {
  'linux-x64':    { pkg: '@backlogit/linux-x64',    bin: 'backlogit-linux-amd64' },
  'linux-arm64':  { pkg: '@backlogit/linux-arm64',  bin: 'backlogit-linux-arm64' },
  'darwin-x64':   { pkg: '@backlogit/darwin-x64',   bin: 'backlogit-darwin-amd64' },
  'darwin-arm64': { pkg: '@backlogit/darwin-arm64', bin: 'backlogit-darwin-arm64' },
  'win32-x64':    { pkg: '@backlogit/win32-x64',    bin: 'backlogit-windows-amd64.exe' },
};

function tier2(key) {
  const entry = PLATFORM_MAP[key];
  if (!entry) return null;
  const ext = process.platform === 'win32' ? '.exe' : '';
  try {
    return require.resolve(`${entry.pkg}/bin/backlogit${ext}`);
  } catch { /**/ }
  return null;
}
```

`package.json` wiring:
```json
"optionalDependencies": {
  "@backlogit/linux-x64": "0.0.0",
  "@backlogit/linux-arm64": "0.0.0",
  "@backlogit/darwin-x64": "0.0.0",
  "@backlogit/darwin-arm64": "0.0.0",
  "@backlogit/win32-x64": "0.0.0"
}
```

Each platform package has `"preferUnplugged": true` so `require.resolve` finds
the binary file rather than a Yarn PnP virtual path.

### Tier 3 — GitHub Releases download + SHA256 + local cache

```js
async function tier3(version, key) {
  const entry = PLATFORM_MAP[key];
  if (!entry) throw new Error(`Unsupported platform: ${key}`);

  const ext = process.platform === 'win32' ? '.exe' : '';
  const cacheDir = path.join(os.homedir(), '.cache', 'backlogit', 'bin');
  const cachedBin = path.join(cacheDir, `backlogit-${version}${ext}`);
  if (fs.existsSync(cachedBin)) return cachedBin;

  fs.mkdirSync(cacheDir, { recursive: true });

  const base = `https://github.com/owner/repo/releases/download/v${version}`;
  const [sums, binBuf] = await Promise.all([
    fetchBuf(`${base}/SHA256SUMS`),
    fetchBuf(`${base}/${entry.bin}`),
  ]);

  const expected = parseSums(sums.toString('utf8'), entry.bin);
  if (!expected) throw new Error(`No SHA256 entry for ${entry.bin} in SHA256SUMS`);

  const actual = crypto.createHash('sha256').update(binBuf).digest('hex');
  if (actual !== expected) throw new Error(`SHA256 mismatch for ${entry.bin}`);

  fs.writeFileSync(cachedBin, binBuf, { mode: 0o755 });
  return cachedBin;
}
```

### Unified resolver with dependency injection for testing

```js
async function resolve(deps = {}) {
  const key = deps.platformKey != null ? deps.platformKey : platformKey();
  const ver  = deps.version || require('../package.json').version;

  const b1 = (deps.tier1 !== undefined ? deps.tier1 : tier1)();
  if (b1) return { binary: b1, tier: 1 };

  const b2 = (deps.tier2 !== undefined ? deps.tier2 : tier2)(key);
  if (b2) return { binary: b2, tier: 2 };

  const b3 = await (deps.tier3 !== undefined ? deps.tier3 : tier3)(ver, key);
  return { binary: b3, tier: 3 };
}
```

Return value: `{ binary: string, tier: 1|2|3 }` — callers know which tier resolved.

### Postinstall pre-resolver (install.js)

Run at `npm install` time to populate tier 2/3 before first use:

```js
async function preinstall() {
  const key = platformKey();
  if (!PLATFORM_MAP[key]) {
    console.log(`Platform ${key} not in support matrix; binary will resolve at runtime.`);
    return;
  }
  const fromNpm = tier2(key);
  if (fromNpm) { console.log(`Using platform binary from npm: ${fromNpm}`); return; }
  try {
    const bin = await tier3(version, key);
    console.log(`Cached at: ${bin}`);
  } catch (err) {
    console.warn(`Preinstall download failed (non-fatal): ${err.message}`);
  }
}
```

Non-fatal: `npm install` always succeeds; tier 3 fires at first invocation if preinstall failed.

### CLI entrypoint pattern

```js
#!/usr/bin/env node
const { resolve } = require('../lib/resolve');
const { execFileSync } = require('child_process');

resolve().then(({ binary }) => {
  execFileSync(binary, process.argv.slice(2), { stdio: 'inherit' });
}).catch((err) => {
  console.error(`backlogit-mcp: failed to resolve binary\n${err.message}`);
  process.exit(1);
});
```

`execFileSync` with `stdio: 'inherit'` is correct for MCP stdio servers — it
passes stdin/stdout/stderr through without buffering.

## Why This Works

- Tier 1 costs zero network and zero disk: works for developers who `go install`
- Tier 2 costs zero network: npm installs only the matching platform package via
  `optionalDependencies`; `preferUnplugged: true` ensures `require.resolve` works
  even in Yarn PnP environments
- Tier 3 is a fallback that caches permanently: after the first `npx`, the
  binary is in `~/.cache` and subsequent invocations hit tier 1 (via PATH) or
  tier 3 cache check (< 1 ms)
- Dependency injection makes all three tiers independently mockable in unit tests
  without network or filesystem side effects

## Prevention

- Add `preferUnplugged: true` to every platform package's `package.json` or
  Yarn PnP will intercept `require.resolve` and return a virtual path
- Add `.gitignore` negation exceptions for the wrapper's `bin/` directory:
  `!npm/backlogit-mcp/bin/` (broad `bin/` patterns in `.gitignore` will catch it)
- Pin version to `0.0.0` in `optionalDependencies` during development; CI stamps
  the real version before publishing
- Add a `version` guard in tier 3 to reject `0.0.0` with an actionable message
  rather than a confusing 404
- `npm publish` should use `continue-on-error: true` on CI so a missing npm org
  scope or absent `NPM_TOKEN` does not block the GitHub Release

## Related Solutions

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — SHA pinning for
  the CI jobs that publish these platform packages
