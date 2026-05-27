#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "$repo_root"

version="${1:-snapshot}"
dist_dir="${OUTLOOK_AGENT_DIST_DIR:-${repo_root}/dist}"
binary_name="outlook-agent"
checksum_file="${dist_dir}/SHA256SUMS.txt"

targets=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

checksum() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

rm_generated_outputs() {
  mkdir -p "$dist_dir"
  rm -f "${dist_dir}"/*.tar.gz "${dist_dir}"/*.zip "${checksum_file}" "${checksum_file}.asc"
  local generated
  for generated in "${dist_dir}/${binary_name}_"*; do
    if [[ -e "$generated" ]]; then
      rm -rf "$generated"
    fi
  done
}

rm_generated_outputs

for target in "${targets[@]}"; do
  read -r GOOS GOARCH <<<"$target"
  package_name="${binary_name}_${version}_${GOOS}_${GOARCH}"
  package_dir="${dist_dir}/${package_name}"
  output_name="$binary_name"
  if [[ "$GOOS" == "windows" ]]; then
    output_name="${binary_name}.exe"
  fi

  mkdir -p "$package_dir"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags "-s -w" -o "${package_dir}/${output_name}" ./cmd/outlook-agent
  cp README.md "${package_dir}/"
  cp docs/RELEASE.md "${package_dir}/"

  if [[ "$GOOS" == "windows" ]]; then
    (cd "$dist_dir" && zip -qr "${package_name}.zip" "$package_name")
  else
    tar -czf "${dist_dir}/${package_name}.tar.gz" -C "$dist_dir" "$package_name"
  fi

  rm -rf "$package_dir"
done

: > "$checksum_file"
while IFS= read -r archive; do
  printf "%s  %s\n" "$(checksum "$archive")" "$(basename "$archive")" >> "$checksum_file"
done < <(find "$dist_dir" -maxdepth 1 -type f \( -name "*.tar.gz" -o -name "*.zip" \) | sort)

if [[ -n "${OUTLOOK_AGENT_SIGN_RELEASE:-}" ]]; then
  if ! command -v gpg >/dev/null 2>&1; then
    echo "OUTLOOK_AGENT_SIGN_RELEASE is set, but gpg is not available" >&2
    exit 1
  fi
  gpg --armor --detach-sign --output "${checksum_file}.asc" "$checksum_file"
fi

echo "Release artifacts written to ${dist_dir}"
echo "Checksums written to ${checksum_file}"
