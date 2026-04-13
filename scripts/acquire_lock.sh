#!/usr/bin/env bash
# Acquire an advisory file lock.
#
# Creates a .{filename}.lock file in the same directory as the target file.
# Fails with exit code 1 if the lock already exists.
#
# Usage: scripts/acquire_lock.sh <filepath>

set -euo pipefail

FILEPATH="${1:?Usage: acquire_lock.sh <filepath>}"

if [ ! -f "$FILEPATH" ]; then
    echo "File not found: $FILEPATH" >&2
    exit 1
fi

DIR="$(dirname "$FILEPATH")"
FILENAME="$(basename "$FILEPATH")"
LOCKFILE="$DIR/.$FILENAME.lock"

if [ -f "$LOCKFILE" ]; then
    echo "File is already locked: $FILEPATH" >&2
    echo "Lock info:" >&2
    cat "$LOCKFILE" >&2
    exit 1
fi

AGENT_NAME="${AGENT_NAME:-unknown}"
TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

cat > "$LOCKFILE" <<EOF
agent: $AGENT_NAME
timestamp: $TIMESTAMP
pid: $$
EOF

echo "Lock acquired: $FILEPATH"
exit 0
