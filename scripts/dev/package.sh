#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
version="${1:-dev}"
out_dir="${2:-$repo_root/dist}"

mkdir -p "$out_dir"
work="$(mktemp -d)"
trap 'rm -r -- "$work"' EXIT
: >"$out_dir/SHA256SUMS"

targets="${SPU_PACKAGE_TARGETS:-darwin/arm64 linux/amd64 windows/amd64}"
for target in $targets; do
  os="${target%/*}"
  arch="${target#*/}"
  package="spu-$version-$os-$arch"
  stage="$work/$package"

  mkdir -p "$stage/bin" "$stage/src/utils" "$stage/scripts" "$stage/libexec"
  cp "$repo_root/bin/spu" "$stage/bin/spu"
  cp "$repo_root/src/spu.sh" "$stage/src/"
  cp "$repo_root/src/utils/style.sh" "$stage/src/utils/"
  cp "$repo_root/scripts/update.sh" "$repo_root/scripts/install.sh" "$repo_root/scripts/uninstall.sh" "$stage/scripts/"

  suffix=''
  [[ "$os" == windows ]] && suffix='.exe'
  (cd "$repo_root" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -ldflags "-X main.version=$version" -o "$stage/libexec/spu$suffix" ./cmd/spu)
  # The MCP server ships beside spu rather than inside it: it needs no daemon, speaks
  # somebody else's protocol, and a fault in it should not take a person's shell tools
  # down with it.
  (cd "$repo_root" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -o "$stage/libexec/spu-clone-third-party$suffix" ./cmd/spu-clone-third-party)
  tar -czf "$out_dir/$package.tar.gz" -C "$stage" .

  if command -v sha256sum >/dev/null 2>&1; then
    digest="$(sha256sum "$out_dir/$package.tar.gz" | cut -d ' ' -f 1)"
  else
    digest="$(shasum -a 256 "$out_dir/$package.tar.gz" | cut -d ' ' -f 1)"
  fi
  printf '%s  %s\n' "$digest" "$package.tar.gz" >>"$out_dir/SHA256SUMS"
done
