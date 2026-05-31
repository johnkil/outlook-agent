#!/usr/bin/env bash
set -euo pipefail

# This writes a dependency manifest for release evidence. It is not a formal
# SPDX/CycloneDX SBOM or signed software-supply-chain attestation.

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

source "${script_dir}/release-version.sh"

version="${1:-v0.0.0-snapshot}"
dist_dir="${2:-${OUTLOOK_AGENT_DIST_DIR:-${repo_root}/dist}}"
binary_name="outlook-agent"
manifest_file="${dist_dir}/${binary_name}_${version}_dependency-manifest.json"

validate_release_version "$version"

mkdir -p "$dist_dir"

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

module_template='{"path":{{printf "%q" .Path}},"version":{{printf "%q" .Version}},"main":{{if .Main}}true{{else}}false{{end}}{{with .Replace}},"replace":{"path":{{printf "%q" .Path}},"version":{{printf "%q" .Version}}}{{end}}}'
module_file="$(mktemp "${TMPDIR:-/tmp}/outlook-agent-modules.XXXXXX")"
trap 'rm -f "$module_file"' EXIT

go list -m -f "$module_template" all > "$module_file"

commit="unknown"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  commit="$(git rev-parse HEAD)"
fi
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
go_version="$(go version)"

{
  echo "{"
  printf '  "schema": "outlook-agent-dependency-manifest-v1",\n'
  printf '  "version": %s,\n' "$(json_string "$version")"
  printf '  "generated_at": %s,\n' "$(json_string "$generated_at")"
  printf '  "commit": %s,\n' "$(json_string "$commit")"
  printf '  "go_version": %s,\n' "$(json_string "$go_version")"
  printf '  "source": "go list -m -f",\n'
  printf '  "modules": [\n'
  first=1
  while IFS= read -r module; do
    if [[ -z "$module" ]]; then
      continue
    fi
    if [[ "$first" -eq 1 ]]; then
      first=0
    else
      printf ',\n'
    fi
    printf '    %s' "$module"
  done < "$module_file"
  printf '\n  ]\n'
  echo "}"
} > "$manifest_file"

echo "Dependency manifest written to ${manifest_file}"
