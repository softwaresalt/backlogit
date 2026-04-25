#!/usr/bin/env bash
# Release an advisory file lock.
#
# Removes the .{filename}.lock file for the specified file.
#
# Usage: scripts/release_lock.sh <filepath>

set -euo pipefail

FILEPATH="${1:?Usage: release_lock.sh <filepath>}"

DIR="$(dirname "$FILEPATH")"
FILENAME="$(basename "$FILEPATH")"
LOCKFILE="$DIR/.$FILENAME.lock"

if [ ! -f "$LOCKFILE" ]; then
    echo "Warning: No lock file found for: $FILEPATH" >&2
    exit 0
fi

rm -f "$LOCKFILE"
echo "Lock released: $FILEPATH"
exit 0
