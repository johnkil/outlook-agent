#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

build_check="${OUTLOOK_AGENT_BUILD_CHECK:-/private/tmp/outlook-agent-build-check}"

cleanup() {
  rm -f "$build_check"
}
trap cleanup EXIT

gofmt_input="$(
  find . \
    \( -path "./.git" -o -path "./.cache" -o -path "./.worktrees" -o -path "./dist" \) -prune -o \
    -type f -name "*.go" -print | sort
)"
if [[ -n "$gofmt_input" ]]; then
  test -z "$(gofmt -l $gofmt_input)"
fi
go test -count=1 ./...
go build -o "$build_check" ./cmd/outlook-agent
git diff --check
scripts/public-safety-check.sh
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
