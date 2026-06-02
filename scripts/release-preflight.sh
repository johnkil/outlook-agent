#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "release preflight failed: $*" >&2
  exit 1
}

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

source "${script_dir}/release-version.sh"

version="${1:-${GITHUB_REF_NAME:-}}"
validate_release_version "$version"
plugin_version="${version#v}"

if [[ "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  if [[ "${GITHUB_REF_NAME:-}" != "$version" ]]; then
    fail "GITHUB_REF_NAME must match requested version ${version}"
  fi
  if ! git tag --points-at HEAD | grep -Fxq "$version"; then
    fail "git tag --points-at HEAD does not contain ${version}"
  fi
fi

if ! git rev-parse --verify --quiet origin/main >/dev/null; then
  if git remote get-url origin >/dev/null 2>&1; then
    git fetch origin main:refs/remotes/origin/main >/dev/null 2>&1 || true
  fi
fi

if ! git rev-parse --verify --quiet origin/main >/dev/null; then
  fail "origin/main is unavailable; cannot prove release commit is on main"
fi

if ! git merge-base --is-ancestor HEAD origin/main; then
  fail "release commit HEAD is not on origin/main"
fi

if ! codex_plugin_version="$(
  python3 - <<'PY'
import pathlib
import re
import sys

text = pathlib.Path("internal/setup/plugin.go").read_text()
match = re.search(r'const\s+codexPluginVersion\s*=\s*"([^"]+)"', text)
if not match:
    print("missing codexPluginVersion", file=sys.stderr)
    sys.exit(1)
print(match.group(1))
PY
)"; then
  fail "could not read codexPluginVersion from internal/setup/plugin.go"
fi

if [[ "$codex_plugin_version" != "$plugin_version" ]]; then
  fail "codexPluginVersion ${codex_plugin_version} does not match release version ${plugin_version}"
fi

if ! manifest_plugin_version="$(
  python3 - <<'PY'
import json
import pathlib
import sys

path = pathlib.Path("plugins/outlook-agent/.codex-plugin/plugin.json")
payload = json.loads(path.read_text())
version = payload.get("version")
if not isinstance(version, str) or not version:
    print("missing .codex-plugin/plugin.json version", file=sys.stderr)
    sys.exit(1)
print(version)
PY
)"; then
  fail "could not read plugins/outlook-agent/.codex-plugin/plugin.json version"
fi

if [[ "$manifest_plugin_version" != "$plugin_version" ]]; then
  fail ".codex-plugin/plugin.json version ${manifest_plugin_version} does not match release version ${plugin_version}"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
exported_dir="${tmp_dir}/outlook-agent"

if ! go run ./cmd/outlook-agent setup plugin export --client codex --output "$exported_dir" --force >/dev/null; then
  fail "setup plugin export failed"
fi

if ! diff -qr "$exported_dir" plugins/outlook-agent; then
  fail "committed Codex plugin package differs from setup plugin export"
fi

if ! go test ./internal/setup -run 'TestCodexMarketplacePackage(UsesPluginRootLayout|CommittedPackageMatchesExporter)' -count=1; then
  fail "setup parity test failed"
fi

echo "release preflight passed: ${version}"
