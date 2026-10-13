#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
target="${SPU_PACKAGE_TARGETS:-$(go env GOOS)/$(go env GOARCH)}"
case "$target" in
darwin/arm64 | linux/amd64 | windows/amd64) ;;
*)
  printf 'smoke: unsupported target: %s\n' "$target" >&2
  exit 1
  ;;
esac

work="$(mktemp -d)"
trap 'rm -r -- "$work"' EXIT
mkdir -p "$work/home"

SPU_PACKAGE_TARGETS="$target" bash "$root/scripts/dev/package.sh" v0.0.0 "$work/artifacts"
archive="$work/artifacts/spu-v0.0.0-${target//\//-}.tar.gz"
manifest="$work/artifacts/SHA256SUMS"
if LC_ALL=C grep -q $'\r' "$manifest"; then
  echo 'smoke: package checksum manifest must use LF' >&2
  exit 1
fi
while IFS= read -r line; do printf '%s\r\n' "$line"; done <"$manifest" >"$work/SHA256SUMS.crlf"
cp "$work/SHA256SUMS.crlf" "$manifest"

HOME="$work/home" bash "$root/scripts/install.sh" --archive "$archive"
HOME="$work/home" bash "$work/home/.local/bin/spu" --version
HOME="$work/home" bash "$work/home/.local/bin/spu" license --mit --name Smoke --year 2026 >/dev/null

echo 'smoke: passed'
