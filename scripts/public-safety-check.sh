#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

failed=0

while IFS= read -r artifact; do
  echo "forbidden generated artifact: ${artifact}" >&2
  failed=1
done < <(
  find . \
    \( -path "./.git" -o -path "./.cache" -o -path "./.worktrees" -o -path "./dist" \) -prune -o \
    -type f \( \
      -name "*.har" -o \
      -name "*.webm" -o \
      -name "*.png" -o \
      -name "*.jpg" -o \
      -name "*.jpeg" -o \
      -name "*.html" -o \
      -name "*.playwright-cli" \
    \) -print | sort
)

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

pattern="${OUTLOOK_AGENT_PUBLIC_SAFETY_PATTERN:-}"
if [[ -n "$pattern" ]]; then
  if command -v rg >/dev/null 2>&1; then
    if rg --hidden -n "$pattern" . -g "!/.git/**" -g "!/.cache/**" -g "!/.worktrees/**" -g "!dist/**"; then
      echo "OUTLOOK_AGENT_PUBLIC_SAFETY_PATTERN matched repository content" >&2
      exit 1
    fi
  elif grep -ERIn --exclude-dir=.git --exclude-dir=.cache --exclude-dir=.worktrees --exclude-dir=dist "$pattern" .; then
    echo "OUTLOOK_AGENT_PUBLIC_SAFETY_PATTERN matched repository content" >&2
    exit 1
  fi
fi

echo "public safety check passed"
