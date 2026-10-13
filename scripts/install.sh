#!/usr/bin/env bash
set -euo pipefail

repo_url='https://github.com/soppylzz/soppy-utils'
if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  SPU_HOME_NATIVE="$(cygpath -w "$HOME")"
  export SPU_HOME_NATIVE
fi

target="$HOME/.local/share/soppy-utils"
launcher="$HOME/.local/bin/spu"
upgrade=false
archive=''
version=''

while (($#)); do
  case "$1" in
  --upgrade)
    upgrade=true
    shift
    ;;
  --archive)
    [[ $# -ge 2 ]] || {
      echo 'install: --archive requires a path' >&2
      exit 1
    }
    archive="$2"
    shift 2
    ;;
  --version)
    [[ $# -ge 2 ]] || {
      echo 'install: --version requires a tag' >&2
      exit 1
    }
    version="$2"
    shift 2
    ;;
  *)
    echo "install: unknown option: $1" >&2
    exit 1
    ;;
  esac
done

if [[ -e "$target" && "$upgrade" != true ]]; then
  echo 'soppy-utils is installed; use spu update or install.sh --upgrade' >&2
  exit 1
fi
if [[ -e "$launcher" && ! -L "$launcher" ]] && ! grep -q '^# Managed by soppy-utils launcher$' "$launcher" 2>/dev/null; then
  echo "install: unmanaged launcher at $launcher" >&2
  exit 1
fi
if [[ -L "$launcher" ]] && [[ "$(readlink "$launcher")" != "$target/bin/spu" ]]; then
  echo "install: unmanaged launcher symlink at $launcher" >&2
  exit 1
fi

case "${OSTYPE:-}" in
darwin*) platform=darwin ;;
linux*) platform=linux ;;
msys* | cygwin*) platform=windows ;;
*)
  echo 'install: unsupported platform' >&2
  exit 1
  ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | ARM64 | aarch64) arch=arm64 ;;
*)
  echo 'install: unsupported architecture' >&2
  exit 1
  ;;
esac

case "$platform/$arch" in
darwin/arm64 | linux/amd64 | windows/amd64) ;;
*)
  printf 'install: no release package for %s/%s\n' "$platform" "$arch" >&2
  exit 1
  ;;
esac

tmp="$(mktemp -d)"
mkdir -p "$(dirname "$target")"
lock="$(dirname "$target")/.soppy-utils-install.lock"
if ! mkdir "$lock" 2>/dev/null; then
  echo 'install: another install or update is running' >&2
  exit 1
fi
trap 'rm -r -- "$tmp"; rmdir "$lock"' EXIT

if [[ -z "$archive" ]]; then
  command -v curl >/dev/null 2>&1 || {
    echo 'install: curl is required' >&2
    exit 1
  }
  if [[ -z "$version" ]]; then
    latest="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$repo_url/releases/latest")"
    version="${latest##*/}"
  fi
  [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
    echo 'install: invalid release version' >&2
    exit 1
  }
  filename="spu-$version-$platform-$arch.tar.gz"
  curl -fsSL "$repo_url/releases/download/$version/$filename" -o "$tmp/$filename"
  curl -fsSL "$repo_url/releases/download/$version/SHA256SUMS" -o "$tmp/SHA256SUMS"
else
  filename="${archive##*/}"
  cp "$archive" "$tmp/$filename"
  [[ "$filename" =~ ^spu-v[0-9]+\.[0-9]+\.[0-9]+-$platform-$arch\.tar\.gz$ ]] || {
    echo 'install: archive is for another platform or invalid' >&2
    exit 1
  }
  version="${filename#spu-}"
  version="${version%-"$platform"-"$arch".tar.gz}"
  checksum_dir="$(cd "$(dirname "$archive")" && pwd)"
  cp "$checksum_dir/SHA256SUMS" "$tmp/SHA256SUMS"
fi

expected="$(tr -d '\r' <"$tmp/SHA256SUMS" | awk -v file="$filename" '$2 == file {print $1}')"
[[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || {
  echo 'install: archive checksum missing or invalid' >&2
  exit 1
}

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$filename" | cut -d ' ' -f 1)"
else
  actual="$(shasum -a 256 "$tmp/$filename" | cut -d ' ' -f 1)"
fi
[[ "$actual" == "$expected" ]] || {
  echo 'install: archive checksum mismatch' >&2
  exit 1
}

mkdir "$tmp/stage"
tar -xzf "$tmp/$filename" -C "$tmp/stage"

helper="$tmp/stage/libexec/spu-helper"
[[ "$platform" == windows ]] && helper="$helper.exe"
[[ -f "$tmp/stage/bin/spu" && -f "$tmp/stage/src/spu.sh" && -f "$tmp/stage/src/utils/style.sh" && -f "$tmp/stage/scripts/update.sh" && -x "$helper" ]] || {
  echo 'install: incomplete package' >&2
  exit 1
}
[[ "$("$helper" --version)" == "$version" ]] || {
  echo 'install: helper version mismatch' >&2
  exit 1
}
[[ "$(bash "$tmp/stage/bin/spu" --version)" == "$version" ]] || {
  echo 'install: launcher smoke test failed' >&2
  exit 1
}

mkdir -p "$(dirname "$target")" "$(dirname "$launcher")"
backup="$tmp/previous"
if [[ -e "$target" ]]; then mv "$target" "$backup"; fi
if ! mv "$tmp/stage" "$target"; then
  if [[ -d "$backup" ]]; then mv "$backup" "$target"; fi
  echo 'install: package swap failed' >&2
  exit 1
fi

wrapper="$tmp/spu"
printf '#!/usr/bin/env bash\n# Managed by soppy-utils launcher\nexec bash %q "$@"\n' "$target/bin/spu" >"$wrapper"
chmod 755 "$wrapper"
if [[ -e "$launcher" || -L "$launcher" ]]; then cp -P "$launcher" "$tmp/previous-launcher"; fi
if ! mv "$wrapper" "$launcher"; then
  mv "$target" "$tmp/failed"
  if [[ -d "$backup" ]]; then mv "$backup" "$target"; fi
  echo 'install: launcher swap failed' >&2
  exit 1
fi

if ! bash "$launcher" --version >/dev/null; then
  mv "$target" "$tmp/failed"
  if [[ -d "$backup" ]]; then mv "$backup" "$target"; fi
  if [[ -e "$tmp/previous-launcher" || -L "$tmp/previous-launcher" ]]; then mv "$tmp/previous-launcher" "$launcher"; else rm -f -- "$launcher"; fi
  echo 'install: installed launcher failed smoke test' >&2
  exit 1
fi

"$target/libexec/spu-helper${helper##*spu-helper}" config-status
printf 'soppy-utils %s installed at %s\n' "$version" "$target"
