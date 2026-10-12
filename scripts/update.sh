#!/usr/bin/env bash
set -euo pipefail

repo_url='https://github.com/soppylzz/soppy-utils'
if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  SPU_HOME_NATIVE="$(cygpath -w "$HOME")"
  export SPU_HOME_NATIVE
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
installed="$HOME/.local/share/soppy-utils"
[[ "$root" == "$installed" ]] || {
  echo 'update: run the installed spu command' >&2
  exit 1
}

helper="$root/libexec/spu-helper"
[[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]] && helper="$helper.exe"
status=false
version=''
archive=''

while (($#)); do
  case "$1" in
  --status)
    status=true
    shift
    ;;
  --version)
    [[ $# -ge 2 ]] || {
      echo 'update: --version needs a tag' >&2
      exit 1
    }
    version="$2"
    shift 2
    ;;
  --archive)
    [[ $# -ge 2 ]] || {
      echo 'update: --archive needs a path' >&2
      exit 1
    }
    archive="$2"
    shift 2
    ;;
  *)
    echo "update: unknown option: $1" >&2
    exit 1
    ;;
  esac
done

if [[ -n "$archive" ]]; then
  [[ -z "$version" ]] || {
    echo 'update: choose --archive or --version' >&2
    exit 1
  }
  filename="${archive##*/}"
  [[ "$filename" =~ ^spu-(v[0-9]+\.[0-9]+\.[0-9]+)- ]] || {
    echo 'update: invalid archive name' >&2
    exit 1
  }
  version="${BASH_REMATCH[1]}"
fi

old="$(bash "$root/bin/spu" --version)"
[[ "$old" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo 'update: installed package has no release version' >&2
  exit 1
}

if [[ -z "$version" ]]; then
  latest="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$repo_url/releases/latest")"
  version="${latest##*/}"
fi
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo 'update: invalid target version' >&2
  exit 1
}

if [[ "$status" == true ]]; then
  printf 'installed: %s\navailable: %s\n' "$old" "$version"
  "$helper" config-status
  exit 0
fi

if [[ "$old" == "$version" ]]; then
  echo "soppy-utils is already $version"
  "$helper" config-status
  exit 0
fi

# Version comparison is numeric; an older release is never installed by update.
IFS=. read -r old_major old_minor old_patch <<<"${old#v}"
IFS=. read -r new_major new_minor new_patch <<<"${version#v}"
old_major=$((10#$old_major))
old_minor=$((10#$old_minor))
old_patch=$((10#$old_patch))
new_major=$((10#$new_major))
new_minor=$((10#$new_minor))
new_patch=$((10#$new_patch))
if ((new_major < old_major || new_major == old_major && new_minor < old_minor || new_major == old_major && new_minor == old_minor && new_patch < old_patch)); then
  echo 'update: downgrade refused' >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -r -- "$tmp"' EXIT
cp "$root/scripts/install.sh" "$tmp/install.sh"

# The copy runs outside the installed root, so Windows can replace spu-helper.exe.
if [[ -n "$archive" ]]; then
  bash "$tmp/install.sh" --upgrade --archive "$archive"
else
  bash "$tmp/install.sh" --upgrade --version "$version"
fi
