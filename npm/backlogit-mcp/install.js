#!/usr/bin/env node
'use strict';

/**
 * Postinstall: pre-resolves the backlogit binary via npm optional deps (tier 2)
 * or GitHub Releases download (tier 3). Tier 1 (PATH) is runtime-only.
 * Fails silently so `npm install` succeeds even on unsupported platforms.
 */
const { tier2, tier3, platformKey, PLATFORM_MAP } = require('./lib/resolve');
const { version } = require('./package.json');

async function preinstall() {
  const key = platformKey();

  if (!PLATFORM_MAP[key]) {
    console.log(`[backlogit-mcp] Platform ${key} not in support matrix; binary will resolve at runtime.`);
    return;
  }

  // Tier 2: fast path via npm optional dep
  const fromNpm = tier2(key);
  if (fromNpm) {
    console.log(`[backlogit-mcp] Using platform binary from npm: ${fromNpm}`);
    return;
  }

  // Tier 3: download from GitHub Releases
  console.log(`[backlogit-mcp] Downloading backlogit v${version} for ${key}...`);
  console.log('[backlogit-mcp] Tip: run `npm install -g @backlogit/backlogit-mcp` to pre-install and avoid this on first use.');
  try {
    const bin = await tier3(version, key);
    console.log(`[backlogit-mcp] Cached at: ${bin}`);
  } catch (err) {
    console.warn(`[backlogit-mcp] Preinstall download failed (non-fatal): ${err.message}`);
    console.warn('[backlogit-mcp] backlogit will be downloaded on first invocation.');
  }
}

preinstall().catch((err) => {
  console.warn(`[backlogit-mcp] postinstall warning: ${err.message}`);
});
