#!/usr/bin/env bash

spu_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" || return 1
source "$spu_root/src/vscode.sh"
source "$spu_root/src/utils/style.sh"

spu_helper() {
  local helper="$spu_root/libexec/spu-helper"
  if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
    helper="$helper.exe"
    if command -v cygpath >/dev/null 2>&1; then
      SPU_HOME_NATIVE="$(cygpath -w "$HOME")"
      export SPU_HOME_NATIVE
    fi
  fi
  if [[ -x "$helper" ]]; then
    "$helper" "$@"
  elif command -v go >/dev/null 2>&1 && [[ -f "$spu_root/go.mod" ]]; then
    (cd "$spu_root" && go run ./cmd/spu-helper "$@")
  else
    printf '%s\n' 'spu: Go helper binary is missing; reinstall soppy-utils' >&2
    return 1
  fi
}

spu() {
  local util="${1:-}"
  case "$util" in
  '' | help | -h)
    printf '%s\n' 'Usage: spu <util> [args]' 'Utils: vscode, license, welcome, git, update'
    ;;
  --help)
    printf '%s\n' 'Usage: spu <util> [args]' '  vscode   Open VS Code configuration' '  license  Generate MIT or ISC license' '  welcome  Generate a shell banner' '  git      Manage global Git defaults' '  update   Upgrade soppy-utils'
    ;;
  --version) spu_helper --version ;;
  vscode)
    shift
    vscode_main "$@"
    ;;
  license | welcome | git)
    shift
    spu_helper "$util" "$@"
    ;;
  update)
    shift
    bash "$spu_root/scripts/update.sh" "$@"
    ;;
  *)
    err "spu: unknown util: $util"
    return 1
    ;;
  esac
}
