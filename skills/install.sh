#!/usr/bin/env bash
# Symlink the bundled skill(s) into the Claude Code and agents skill directories.
# Idempotent: re-running refreshes the links. Override targets with
# CLAUDE_SKILLS_DIR / AGENTS_SKILLS_DIR.
set -euo pipefail

# Directory holding this script (…/confluence-cli/skills).
SKILLS_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CLAUDE_SKILLS_DIR="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
AGENTS_SKILLS_DIR="${AGENTS_SKILLS_DIR:-$HOME/.agents/skills}"

# Every immediate subdirectory of skills/ that contains a SKILL.md is a skill.
link_skill() {
  local skill_dir="$1" name target
  name="$(basename "$skill_dir")"
  for dest in "$CLAUDE_SKILLS_DIR" "$AGENTS_SKILLS_DIR"; do
    mkdir -p "$dest"
    target="$dest/$name"
    ln -sfn "$skill_dir" "$target"
    echo "linked $target -> $skill_dir"
  done
}

found=0
for d in "$SKILLS_SRC"/*/; do
  d="${d%/}"
  if [[ -f "$d/SKILL.md" ]]; then
    link_skill "$d"
    found=1
  fi
done

if [[ "$found" -eq 0 ]]; then
  echo "no skills (no */SKILL.md) found under $SKILLS_SRC" >&2
  exit 1
fi

echo "done."
