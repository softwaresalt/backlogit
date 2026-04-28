#!/usr/bin/env node
'use strict';

const { execFileSync } = require('child_process');
const { resolve } = require('../lib/resolve');

async function main() {
  let resolved;
  try {
    resolved = await resolve();
  } catch (err) {
    process.stderr.write(`backlogit-mcp: failed to resolve backlogit binary: ${err.message}\n`);
    process.stderr.write('\nTo fix, choose one of:\n');
    process.stderr.write('  npm install -g @backlogit/backlogit-mcp   (recommended — faster starts)\n');
    process.stderr.write('  go install github.com/softwaresalt/backlogit/cmd/backlogit@latest\n');
    process.exit(1);
  }

  try {
    execFileSync(resolved.binary, process.argv.slice(2), { stdio: 'inherit' });
  } catch (err) {
    process.exit(err.status != null ? err.status : 1);
  }
}

main().catch((err) => {
  process.stderr.write(`backlogit-mcp: ${err.message}\n`);
  process.exit(1);
});
