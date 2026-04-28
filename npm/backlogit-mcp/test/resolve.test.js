'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { resolve, parseSums, PLATFORM_MAP } = require('../lib/resolve');

// ---------------------------------------------------------------------------
// parseSums
// ---------------------------------------------------------------------------
describe('parseSums', () => {
  it('parses a standard SHA256SUMS entry', () => {
    const text = [
      'abc123def456  backlogit-linux-amd64',
      'fedcba987654  backlogit-linux-arm64',
    ].join('\n');
    assert.equal(parseSums(text, 'backlogit-linux-amd64'), 'abc123def456');
  });

  it('parses a ./prefixed entry', () => {
    const text = 'abc123  ./backlogit-linux-amd64\n';
    assert.equal(parseSums(text, 'backlogit-linux-amd64'), 'abc123');
  });

  it('returns null when the filename is not present', () => {
    assert.equal(parseSums('abc123  other-binary\n', 'backlogit-linux-amd64'), null);
  });

  it('returns null on empty input', () => {
    assert.equal(parseSums('', 'backlogit-linux-amd64'), null);
  });
});

// ---------------------------------------------------------------------------
// PLATFORM_MAP completeness
// ---------------------------------------------------------------------------
describe('PLATFORM_MAP', () => {
  const expected = ['linux-x64', 'linux-arm64', 'darwin-x64', 'darwin-arm64', 'win32-x64'];
  for (const key of expected) {
    it(`contains entry for ${key}`, () => {
      assert.ok(PLATFORM_MAP[key], `missing platform: ${key}`);
      assert.ok(PLATFORM_MAP[key].pkg, `missing pkg for ${key}`);
      assert.ok(PLATFORM_MAP[key].bin, `missing bin for ${key}`);
    });
  }
});

// ---------------------------------------------------------------------------
// resolve — tier 1: binary found in PATH
// ---------------------------------------------------------------------------
describe('resolve (tier 1 — PATH)', () => {
  it('returns tier 1 when PATH binary is found', async () => {
    const result = await resolve({
      platformKey: 'linux-x64',
      version: '1.1.0',
      tier1: () => '/usr/bin/backlogit',
      tier2: () => { throw new Error('tier2 must not be called'); },
      tier3: async () => { throw new Error('tier3 must not be called'); },
    });
    assert.equal(result.tier, 1);
    assert.equal(result.binary, '/usr/bin/backlogit');
  });
});

// ---------------------------------------------------------------------------
// resolve — tier 2: npm optional dependency
// ---------------------------------------------------------------------------
describe('resolve (tier 2 — npm optional dep)', () => {
  it('falls through to tier 2 when tier 1 returns null', async () => {
    const result = await resolve({
      platformKey: 'linux-x64',
      version: '1.1.0',
      tier1: () => null,
      tier2: (key) => `/node_modules/@backlogit/${key}/bin/backlogit`,
      tier3: async () => { throw new Error('tier3 must not be called'); },
    });
    assert.equal(result.tier, 2);
    assert.match(result.binary, /@backlogit\/linux-x64/);
  });
});

// ---------------------------------------------------------------------------
// resolve — tier 3: GitHub Releases download
// ---------------------------------------------------------------------------
describe('resolve (tier 3 — GitHub Releases)', () => {
  it('falls through to tier 3 when tiers 1 and 2 both miss', async () => {
    const result = await resolve({
      platformKey: 'linux-x64',
      version: '1.1.0',
      tier1: () => null,
      tier2: () => null,
      tier3: async (ver) => `/home/user/.cache/backlogit/bin/backlogit-${ver}`,
    });
    assert.equal(result.tier, 3);
    assert.match(result.binary, /backlogit-1\.1\.0/);
  });

  it('propagates tier3 SHA256 mismatch error', async () => {
    await assert.rejects(
      () => resolve({
        platformKey: 'linux-x64',
        version: '1.1.0',
        tier1: () => null,
        tier2: () => null,
        tier3: async () => { throw new Error('SHA256 mismatch for backlogit-linux-amd64'); },
      }),
      /SHA256 mismatch/,
    );
  });

  it('propagates unsupported platform error from tier3', async () => {
    await assert.rejects(
      () => resolve({
        platformKey: 'sunos-x64',
        version: '1.1.0',
        tier1: () => null,
        tier2: () => null,
        tier3: async (_, key) => { throw new Error(`Unsupported platform: ${key}`); },
      }),
      /Unsupported platform/,
    );
  });
});
