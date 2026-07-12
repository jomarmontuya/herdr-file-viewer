#!/usr/bin/env sh
# Install-time build step for the Herdr plugin.
#
# This fork has behavior that is not present in the upstream release binaries,
# so it must build its own source. Downloading the upstream v0.1.6 binary here
# would silently remove mouse-driven Herdr tabs.
#
# Run from the plugin root as: sh scripts/fetch-or-build.sh

DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="$(cat "$DIR/VERSION" 2>/dev/null)"
mkdir -p "$DIR/bin"

build_from_source() {
  command -v go >/dev/null 2>&1 || return 1
  echo "Building from source with Go…"
  ( cd "$DIR" && go build -ldflags "-X main.version=$VERSION" -o bin/file-viewer ./cmd/file-viewer )
}

if build_from_source; then
  echo "Built file-viewer from source."
else
  echo "ERROR: Go is required to build this local fork." >&2
  echo "Install Go 1.25+ from https://go.dev/dl and retry." >&2
  exit 1
fi
