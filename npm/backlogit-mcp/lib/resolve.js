'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync } = require('child_process');
const https = require('https');
const crypto = require('crypto');

/**
 * Maps npm platform keys to npm package names and Go binary filenames.
 * npm platform key = `${process.platform}-${os.arch()}`
 */
const PLATFORM_MAP = {
  'linux-x64':    { pkg: '@backlogit/linux-x64',    bin: 'backlogit-linux-amd64' },
  'linux-arm64':  { pkg: '@backlogit/linux-arm64',  bin: 'backlogit-linux-arm64' },
  'darwin-x64':   { pkg: '@backlogit/darwin-x64',   bin: 'backlogit-darwin-amd64' },
  'darwin-arm64': { pkg: '@backlogit/darwin-arm64', bin: 'backlogit-darwin-arm64' },
  'win32-x64':    { pkg: '@backlogit/win32-x64',    bin: 'backlogit-windows-amd64.exe' },
};

function platformKey() {
  return `${process.platform}-${os.arch()}`;
}

/** Tier 1: native `backlogit` binary already in PATH. */
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

/** Tier 2: platform binary shipped as an npm optional dependency. */
function tier2(key) {
  const entry = PLATFORM_MAP[key];
  if (!entry) return null;
  const ext = process.platform === 'win32' ? '.exe' : '';
  try {
    return require.resolve(`${entry.pkg}/bin/backlogit${ext}`);
  } catch { /**/ }
  return null;
}

/** Tier 3: download from GitHub Releases and cache locally with SHA256 verification. */
async function tier3(version, key) {
  const entry = PLATFORM_MAP[key];
  if (!entry) throw new Error(`Unsupported platform: ${key}`);

  const ext = process.platform === 'win32' ? '.exe' : '';
  const cacheDir = path.join(os.homedir(), '.cache', 'backlogit', 'bin');
  const cachedBin = path.join(cacheDir, `backlogit-${version}${ext}`);
  if (fs.existsSync(cachedBin)) return cachedBin;

  fs.mkdirSync(cacheDir, { recursive: true });

  const base = `https://github.com/softwaresalt/backlogit/releases/download/v${version}`;
  const [sums, binBuf] = await Promise.all([
    fetchBuf(`${base}/SHA256SUMS`),
    fetchBuf(`${base}/${entry.bin}`),
  ]);

  const expected = parseSums(sums.toString('utf8'), entry.bin);
  if (!expected) throw new Error(`No SHA256 entry for ${entry.bin} in SHA256SUMS`);

  const actual = crypto.createHash('sha256').update(binBuf).digest('hex');
  if (actual !== expected) {
    throw new Error(`SHA256 mismatch for ${entry.bin}: expected ${expected}, got ${actual}`);
  }

  fs.writeFileSync(cachedBin, binBuf, { mode: 0o755 });
  return cachedBin;
}

/** Parse a SHA256SUMS file for a specific binary filename. */
function parseSums(text, name) {
  for (const line of text.split('\n')) {
    const parts = line.trim().split(/\s+/);
    if (parts.length >= 2) {
      const [hash, file] = parts;
      if (file === name || file === `./${name}`) return hash;
    }
  }
  return null;
}

function fetchBuf(url) {
  return new Promise((resolve, reject) => {
    const get = (u) =>
      https.get(u, (res) => {
        const { statusCode, headers } = res;
        if (statusCode === 301 || statusCode === 302) return get(headers.location);
        if (statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${statusCode}: ${u}`));
        }
        const chunks = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => resolve(Buffer.concat(chunks)));
        res.on('error', reject);
      }).on('error', reject);
    get(url);
  });
}

/**
 * Resolve the backlogit binary using three-tier fallback.
 *
 * Accepts an optional `deps` object for dependency injection in tests:
 *   { platformKey, version, tier1, tier2, tier3 }
 *
 * Returns { binary: string, tier: 1|2|3 }
 */
async function resolve(deps = {}) {
  const key = deps.platformKey != null ? deps.platformKey : platformKey();
  const ver = deps.version || require('../package.json').version;

  const b1 = (deps.tier1 !== undefined ? deps.tier1 : tier1)();
  if (b1) return { binary: b1, tier: 1 };

  const b2 = (deps.tier2 !== undefined ? deps.tier2 : tier2)(key);
  if (b2) return { binary: b2, tier: 2 };

  const b3 = await (deps.tier3 !== undefined ? deps.tier3 : tier3)(ver, key);
  return { binary: b3, tier: 3 };
}

module.exports = { resolve, tier1, tier2, tier3, platformKey, parseSums, PLATFORM_MAP };
