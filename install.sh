#!/usr/bin/env sh
set -eu

REPO="johnkil/outlook-agent"
BIN_NAME="outlook-agent"
VERSION="${OUTLOOK_AGENT_VERSION:-latest}"
INSTALL_DIR="${OUTLOOK_AGENT_INSTALL_DIR:-}"
GITHUB_BASE_URL="https://github.com/${REPO}/releases"

usage() {
  cat <<EOF
Install ${BIN_NAME} from GitHub Releases.

Usage:
  sh install.sh [--version v0.2.0] [--dir /path/to/bin]

Options:
  --help             Show this help.
  --version VERSION  Install a release tag. Defaults to OUTLOOK_AGENT_VERSION or latest.
  --dir DIR          Install directory. Defaults to OUTLOOK_AGENT_INSTALL_DIR or the first writable PATH directory.

Examples:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
  OUTLOOK_AGENT_VERSION=v0.2.0 sh install.sh
EOF
}

die() {
  echo "install.sh: $*" >&2
  exit 1
}

need_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "$1 is required"
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --version)
      [ "$#" -ge 2 ] || die "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --version=*)
      VERSION="${1#--version=}"
      [ -n "$VERSION" ] || die "--version requires a value"
      shift
      ;;
    --dir)
      [ "$#" -ge 2 ] || die "--dir requires a value"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --dir=*)
      INSTALL_DIR="${1#--dir=}"
      [ -n "$INSTALL_DIR" ] || die "--dir requires a value"
      shift
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

detect_goos() {
  os="$(uname -s 2>/dev/null || true)"
  case "$os" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *) die "unsupported operating system: ${os:-unknown}; supported: darwin, linux" ;;
  esac
}

detect_goarch() {
  arch="$(uname -m 2>/dev/null || true)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) die "unsupported architecture: ${arch:-unknown}; supported: amd64, arm64" ;;
  esac
}

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$output"
  else
    die "curl or wget is required"
  fi
}

resolve_latest_version() {
  latest_url="${GITHUB_BASE_URL}/latest"
  if command -v curl >/dev/null 2>&1; then
    effective_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$latest_url")"
  elif command -v wget >/dev/null 2>&1; then
    effective_url="$(wget -qS --spider "$latest_url" 2>&1 | sed -n 's/^[[:space:]]*Location: //p' | tail -n 1 | tr -d '\r')"
  else
    die "curl or wget is required"
  fi
  tag="${effective_url##*/}"
  [ -n "$tag" ] && [ "$tag" != "latest" ] || die "could not resolve latest release tag"
  echo "$tag"
}

choose_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    echo "$INSTALL_DIR"
    return
  fi

  old_ifs="$IFS"
  IFS=:
  for dir in ${PATH:-}; do
    [ -n "$dir" ] || continue
    if [ -d "$dir" ] && [ -w "$dir" ]; then
      IFS="$old_ifs"
      echo "$dir"
      return
    fi
  done
  IFS="$old_ifs"

  if [ -n "${HOME:-}" ]; then
    echo "${HOME}/.local/bin"
    return
  fi

  die "no writable directory found on PATH and HOME is not set; pass --dir"
}

verify_checksum() {
  checksum_file="$1"
  archive_name="$2"
  checksum_line="$(grep "  ${archive_name}\$" "$checksum_file" || true)"
  [ -n "$checksum_line" ] || die "checksum entry not found for ${archive_name}"

  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "$checksum_line" | sha256sum -c -
  elif command -v shasum >/dev/null 2>&1; then
    printf '%s\n' "$checksum_line" | shasum -a 256 -c -
  else
    die "sha256sum or shasum is required"
  fi
}

validate_tar_members() {
  member_list="$1"
  expected_package_dir="$2"
  expected_binary_member="$3"
  found_binary=0

  while IFS= read -r member; do
    case "$member" in
      ""|/*|..|../*|*/..|*/../*)
        die "unsafe archive member: $member"
        ;;
    esac

    case "$member" in
      "$expected_package_dir"|"$expected_package_dir/"|"$expected_binary_member"|"$expected_package_dir/README.md"|"$expected_package_dir/RELEASE.md")
        ;;
      *)
        die "unexpected archive member: $member"
        ;;
    esac

    if [ "$member" = "$expected_binary_member" ]; then
      found_binary=1
    fi
  done < "$member_list"

  [ "$found_binary" -eq 1 ] || die "archive did not list ${expected_binary_member}"
}

validate_tar_binary_type() {
  verbose_member_list="$1"
  expected_binary_member="$2"
  found_binary=0

  while IFS= read -r line; do
    case "$line" in
      *" $expected_binary_member"|*" $expected_binary_member -> "*|*" $expected_binary_member link to "*)
        found_binary=1
        entry_type="$(printf '%s' "$line" | cut -c 1)"
        [ "$entry_type" = "-" ] || die "expected binary archive member is not a regular file: $expected_binary_member"
        ;;
    esac
  done < "$verbose_member_list"

  [ "$found_binary" -eq 1 ] || die "archive did not list ${expected_binary_member}"
}

goos="$(detect_goos)"
goarch="$(detect_goarch)"

if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_version)"
fi

case "$VERSION" in
  v*) ;;
  *) die "release version must be a tag like v0.2.0, got: $VERSION" ;;
esac

archive_name="${BIN_NAME}_${VERSION}_${goos}_${goarch}.tar.gz"
expected_package_dir="${BIN_NAME}_${VERSION}_${goos}_${goarch}"
expected_binary_member="${expected_package_dir}/${BIN_NAME}"
archive_url="${GITHUB_BASE_URL}/download/${VERSION}/${archive_name}"
checksum_url="${GITHUB_BASE_URL}/download/${VERSION}/SHA256SUMS.txt"
install_dir="$(choose_install_dir)"
target_path="${install_dir}/${BIN_NAME}"

need_command tar
need_command grep
need_command mktemp
need_command cut

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/outlook-agent-install.XXXXXX")"
install_tmp=""
cleanup() {
  rm -rf "$tmp_dir"
  if [ -n "$install_tmp" ]; then
    rm -f "$install_tmp"
  fi
}
trap cleanup EXIT HUP INT TERM

archive_path="${tmp_dir}/${archive_name}"
checksum_path="${tmp_dir}/SHA256SUMS.txt"
member_list_path="${tmp_dir}/archive-members.txt"
verbose_member_list_path="${tmp_dir}/archive-members-verbose.txt"

download "$archive_url" "$archive_path"
download "$checksum_url" "$checksum_path"

(
  cd "$tmp_dir"
  verify_checksum "$checksum_path" "$archive_name"
  tar -tzf "$archive_name" > "$member_list_path"
  tar -tvzf "$archive_name" > "$verbose_member_list_path"
  validate_tar_members "$member_list_path" "$expected_package_dir" "$expected_binary_member"
  validate_tar_binary_type "$verbose_member_list_path" "$expected_binary_member"
  tar -xzf "$archive_name" "$expected_binary_member"
)

package_dir="${tmp_dir}/${expected_package_dir}"
binary_path="${package_dir}/${BIN_NAME}"
[ -f "$binary_path" ] && [ ! -L "$binary_path" ] || die "archive did not contain a regular ${BIN_NAME} file"

mkdir -p "$install_dir"
install_tmp="$(mktemp "${install_dir}/.${BIN_NAME}.tmp.XXXXXX")"
cp "$binary_path" "$install_tmp"
chmod 0755 "$install_tmp"
if [ -L "$target_path" ]; then
  die "refusing to overwrite symlink: $target_path"
fi
if [ -e "$target_path" ] && [ ! -f "$target_path" ]; then
  die "refusing to overwrite non-file: $target_path"
fi
mv "$install_tmp" "$target_path"
install_tmp=""

cat <<EOF
Installed ${BIN_NAME} ${VERSION} to ${target_path}

Next steps:
  outlook-agent help
  outlook-agent doctor
EOF
