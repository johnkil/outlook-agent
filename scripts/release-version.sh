#!/usr/bin/env bash

validate_release_version() {
  local version="$1"

  if [[ -z "$version" ]]; then
    echo "invalid release version: must be non-empty" >&2
    echo "expected format: v<major>.<minor>.<patch>[-suffix], e.g. v0.1.0 or v0.0.0-smoke" >&2
    exit 2
  fi

  if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z._-]+)?$ ]]; then
    echo "invalid release version: ${version}" >&2
    echo "expected format: v<major>.<minor>.<patch>[-suffix], e.g. v0.1.0 or v0.0.0-smoke" >&2
    exit 2
  fi
}
