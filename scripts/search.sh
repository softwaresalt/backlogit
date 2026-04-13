#!/usr/bin/env bash
# Search installed skills by keyword.
#
# Scans all SKILL.md files under .github/skills/ and returns matches
# where the keyword appears in the skill name or its YAML frontmatter
# description field.
#
# Usage: scripts/search.sh <keyword>

set -euo pipefail

KEYWORD="${1:?Usage: search.sh <keyword>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILLS_ROOT="$SCRIPT_DIR/../.github/skills"

if [ ! -d "$SKILLS_ROOT" ]; then
    echo "Skills directory not found at .github/skills/" >&2
    exit 1
fi

printf "%-28s %-70s %s\n" "SKILL" "DESCRIPTION" "PATH"
printf "%-28s %-70s %s\n" "-----" "-----------" "----"

found=0

for skill_dir in "$SKILLS_ROOT"/*/; do
    [ -d "$skill_dir" ] || continue
    skill_name="$(basename "$skill_dir")"
    skill_file="$skill_dir/SKILL.md"
    [ -f "$skill_file" ] || continue

    # Extract description from YAML frontmatter
    description=""
    if head -20 "$skill_file" | grep -q '^---'; then
        description=$(sed -n '/^---$/,/^---$/p' "$skill_file" | grep -i 'description:' | head -1 | sed 's/^[^"]*"//;s/"[^"]*$//' | sed "s/^[^']*'//;s/'[^']*$//")
    fi

    # Match keyword (case-insensitive) against skill name or description
    if echo "$skill_name" | grep -qiF -- "$KEYWORD" || echo "$description" | grep -qiF -- "$KEYWORD"; then
        # Truncate description for display
        if [ ${#description} -gt 70 ]; then
            description="${description:0:67}..."
        fi
        printf "%-28s %-70s %s\n" "$skill_name" "$description" ".github/skills/$skill_name/SKILL.md"
        found=1
    fi
done

if [ "$found" -eq 0 ]; then
    echo "No skills matching '$KEYWORD' found."
fi
