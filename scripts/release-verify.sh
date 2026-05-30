#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

dist_dir="${1:-${OUTLOOK_AGENT_DIST_DIR:-${repo_root}/dist}}"
checksum_file="${dist_dir}/SHA256SUMS.txt"
signature_file="${dist_dir}/SHA256SUMS.txt.asc"

if [[ ! -d "$dist_dir" ]]; then
  echo "release dist directory does not exist: ${dist_dir}" >&2
  exit 1
fi
if [[ ! -f "$checksum_file" ]]; then
  echo "missing SHA256SUMS.txt in ${dist_dir}" >&2
  exit 1
fi

checksum() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

archive_count=0
non_archive_count=0
first_non_archive=""
verified_archives=""
dependency_manifest_count=0
verified_dependency_manifests=""
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -z "$line" ]] && continue
  read -r expected archive_name extra <<<"$line"
  if [[ -z "${expected:-}" || -z "${archive_name:-}" || -n "${extra:-}" ]]; then
    echo "invalid checksum line: ${line}" >&2
    exit 1
  fi
  if [[ "$archive_name" == */* || "$archive_name" == .* ]]; then
    echo "unsafe checksum archive name: ${archive_name}" >&2
    exit 1
  fi
  archive_path="${dist_dir}/${archive_name}"
  if [[ ! -f "$archive_path" ]]; then
    echo "checksum references missing archive: ${archive_name}" >&2
    exit 1
  fi
  actual="$(checksum "$archive_path")"
  if [[ "$actual" != "$expected" ]]; then
    echo "checksum mismatch for ${archive_name}" >&2
    echo "expected ${expected}" >&2
    echo "actual   ${actual}" >&2
    exit 1
  fi
  case "$archive_name" in
    *.tar.gz|*.zip)
      archive_count=$((archive_count + 1))
      verified_archives="${verified_archives}${archive_name}"$'\n'
      ;;
    *_dependency-manifest.json)
      dependency_manifest_count=$((dependency_manifest_count + 1))
      verified_dependency_manifests="${verified_dependency_manifests}${archive_name}"$'\n'
      ;;
    *)
      non_archive_count=$((non_archive_count + 1))
      if [[ -z "$first_non_archive" ]]; then
        first_non_archive="$archive_name"
      fi
      ;;
  esac
done < "$checksum_file"

if [[ "$archive_count" -eq 0 ]]; then
  if [[ "$non_archive_count" -gt 0 ]]; then
    echo "checksum entry is not a release archive: ${first_non_archive}" >&2
    exit 1
  fi
  echo "SHA256SUMS.txt did not list any archives" >&2
  exit 1
fi

while IFS= read -r archive; do
  archive_name="$(basename "$archive")"
  if ! grep -Fxq "$archive_name" <<< "$verified_archives"; then
    echo "archive ${archive_name} is missing from SHA256SUMS.txt" >&2
    exit 1
  fi
done < <(find "$dist_dir" -maxdepth 1 -type f \( -name "*.tar.gz" -o -name "*.zip" \) | sort)

actual_dependency_manifest_count="$(
  find "$dist_dir" -maxdepth 1 -type f -name "*_dependency-manifest.json" \
    | wc -l \
    | tr -d " "
)"
if [[ "$actual_dependency_manifest_count" -ne 1 ]]; then
  echo "expected exactly one dependency manifest in ${dist_dir}, got ${actual_dependency_manifest_count}" >&2
  exit 1
fi
if [[ "$dependency_manifest_count" -ne "$actual_dependency_manifest_count" ]]; then
  echo "dependency manifest is missing from SHA256SUMS.txt" >&2
  exit 1
fi
while IFS= read -r manifest; do
  manifest_name="$(basename "$manifest")"
  if ! grep -Fxq "$manifest_name" <<< "$verified_dependency_manifests"; then
    echo "dependency manifest ${manifest_name} is missing from SHA256SUMS.txt" >&2
    exit 1
  fi
done < <(find "$dist_dir" -maxdepth 1 -type f -name "*_dependency-manifest.json" | sort)

signature_status="absent"
if [[ -f "$signature_file" ]]; then
  if command -v gpg >/dev/null 2>&1; then
    gpg --verify "$signature_file" "$checksum_file" >/dev/null
    signature_status="verified"
  else
    signature_status="skipped: gpg unavailable"
  fi
fi

echo "release verify passed: ${archive_count} archives in ${dist_dir}; dependency_manifest=verified; signature=${signature_status}"
