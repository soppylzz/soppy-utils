#!/usr/bin/env bash
set -euo pipefail

if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  SPU_HOME_NATIVE="$(cygpath -w "$HOME")"
  export SPU_HOME_NATIVE
fi

target="$HOME/.local/share/soppy-utils"
launcher="$HOME/.local/bin/spu"
purge=false

if [[ "${1:-}" == --purge && $# -eq 1 ]]; then
  purge=true
elif (($#)); then
  echo 'Usage: uninstall.sh [--purge]' >&2
  exit 1
fi

[[ -f "$target/src/spu.sh" && -f "$target/bin/spu" ]] || {
  echo 'uninstall: no managed installation found' >&2
  exit 1
}

helper="$target/libexec/spu-helper"
[[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]] && helper="$helper.exe"
if [[ "$purge" == true ]]; then
  [[ -x "$helper" ]] || {
    echo 'uninstall: helper missing; cannot purge safely' >&2
    exit 1
  }
  "$helper" welcome uninstall --purge
  "$helper" git uninstall --purge
  "$helper" license purge
fi

if [[ -L "$launcher" ]]; then
  [[ "$(readlink "$launcher")" == "$target/bin/spu" ]] || {
    echo 'uninstall: launcher points elsewhere' >&2
    exit 1
  }
  rm -f -- "$launcher"
elif [[ -f "$launcher" ]] && grep -q '^# Managed by soppy-utils launcher$' "$launcher"; then
  rm -f -- "$launcher"
else
  echo 'uninstall: launcher is unmanaged or missing; leaving it' >&2
fi

rm -r -- "$target"

if [[ "$purge" != true ]]; then
  echo 'soppy-utils removed; welcome hooks, Git include, and user config remain active.' >&2
else
  echo 'soppy-utils and managed optional configuration removed.' >&2
fi
