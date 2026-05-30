#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

build_check="${OUTLOOK_AGENT_BUILD_CHECK:-/private/tmp/outlook-agent-build-check}"
staticcheck_home="${OUTLOOK_AGENT_STATICCHECK_HOME:-${repo_root}/.cache/home}"

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
go mod tidy
git diff --exit-code go.mod go.sum
go test -count=1 ./...
go test -race ./...
go vet ./...
mkdir -p "$staticcheck_home"
HOME="$staticcheck_home" go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go build -o "$build_check" ./cmd/outlook-agent
git diff --check
sh -n install.sh
sh install.sh --help >/dev/null
scripts/public-safety-check.sh
scripts/action-coverage-smoke.sh
go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...
