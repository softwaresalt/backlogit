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

AGENT_NAME="${AGENT_NAME:-unknown}"
TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Use noclobber (O_EXCL) for atomic lock creation to eliminate TOCTOU race
if ! (set -C; printf "agent: %s\ntimestamp: %s\npid: %d\n" "$AGENT_NAME" "$TIMESTAMP" "$$" > "$LOCKFILE") 2>/dev/null; then
    echo "File is already locked: $FILEPATH" >&2
    if [ -f "$LOCKFILE" ]; then
        echo "Lock info:" >&2
        cat "$LOCKFILE" >&2
    fi
    exit 1
fi

echo "Lock acquired: $FILEPATH"
exit 0
