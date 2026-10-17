#!/usr/bin/env bash

spu_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)" || return 1
source "$spu_root/src/utils/style.sh"

# spu_go runs the Go executable. It is both the client and, when a client cannot reach
# one, the core it talks to: one binary, so the two can never be different builds.
spu_go() {
  local exe="$spu_root/libexec/spu"
  if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
    exe="$exe.exe"
    if command -v cygpath >/dev/null 2>&1; then
      SPU_HOME_NATIVE="$(cygpath -w "$HOME")"
      export SPU_HOME_NATIVE
      # The same conversion for the directory the client was run from, because a command
      # that works on a project needs a path the native daemon can open.
      SPU_WORKDIR_NATIVE="$(cygpath -w "$PWD")"
      export SPU_WORKDIR_NATIVE
    fi
  fi
  if [[ -x "$exe" ]]; then
    "$exe" "$@"
  elif command -v go >/dev/null 2>&1 && [[ -f "$spu_root/go.mod" ]]; then
    (cd "$spu_root" && go run ./cmd/spu "$@")
  else
    printf '%s\n' 'spu: the Go binary is missing; reinstall soppy-utils' >&2
    return 1
  fi
}

spu() {
  local util="${1:-}"
  case "$util" in
  # Help and version are the binary's: the list of utils lives in its registry, and a
  # copy of it here is a copy that can be wrong.
  '' | help | -h | --help | --version) spu_go "$@" ;;
  # Upgrading is the installer's job, not a util's: it replaces the installation this
  # launcher points at, and that is shell. It lives here rather than in the binary because
  # the client has nothing to do with it, and because a package manager can take the same
  # place later without the binary learning about any of them.
  update)
    shift
    bash "$spu_root/scripts/update.sh" "$@"
    ;;
  # Everything else is the Go binary's to route. A list of utils here would be a second
  # list to keep in step with the registry, and it was already one: __core was unknown to
  # this dispatcher, so the scripts could not ask a running core to stop.
  #
  # This case has to stay last. It was once written before the update case, which made
  # update unreachable — ShellCheck's SC2222, and an upgrade path that silently stopped
  # working.
  *) spu_go "$@" ;;
  esac
}
