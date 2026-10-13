#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
action="${1:-status}"
if (($# > 1)) || [[ "$action" != install && "$action" != uninstall && "$action" != status ]]; then
  echo 'Usage: hooks.sh [install|uninstall|status]' >&2
  exit 1
fi

repo="$(git -C "$root" rev-parse --show-toplevel 2>/dev/null)" || {
  echo 'hooks: run from a Git checkout' >&2
  exit 1
}
repo="$(cd "$repo" && pwd -P)"
[[ "$repo" == "$root" ]] || {
  echo 'hooks: script must be inside the repository root' >&2
  exit 1
}

effective=''
effective_set=false
if effective="$(git -C "$root" config --get core.hooksPath)"; then
  effective_set=true
fi
local_path=''
if local_path="$(git -C "$root" config --local --get core.hooksPath)"; then
  :
fi

case "$action" in
install)
  [[ -x "$root/.githooks/pre-commit" ]] || {
    echo 'hooks: tracked pre-commit hook is missing or not executable' >&2
    exit 1
  }
  if [[ "$effective" == .githooks ]]; then
    echo 'hooks: already enabled'
    exit 0
  fi
  if [[ "$effective_set" == true ]]; then
    printf 'hooks: existing core.hooksPath is %q; leaving it unchanged\n' "$effective" >&2
    exit 1
  fi
  default_hooks="$(git -C "$root" rev-parse --git-path hooks)"
  case "$default_hooks" in
  /* | [A-Za-z]:/*) ;;
  *) default_hooks="$root/$default_hooks" ;;
  esac
  for hook in "$default_hooks"/*; do
    [[ -e "$hook" || -L "$hook" ]] || continue
    [[ "$hook" == *.sample ]] && continue
    echo 'hooks: existing default hooks would be bypassed; leaving them unchanged' >&2
    exit 1
  done
  git -C "$root" config --local core.hooksPath .githooks
  echo 'hooks: enabled for this checkout'
  ;;
uninstall)
  if [[ "$local_path" != .githooks ]]; then
    echo 'hooks: no locally managed hook setting to remove' >&2
    exit 1
  fi
  git -C "$root" config --local --unset core.hooksPath
  echo 'hooks: disabled for this checkout'
  ;;
status)
  if [[ "$effective" == .githooks ]]; then
    echo 'hooks: enabled'
  elif [[ "$effective_set" == true ]]; then
    printf 'hooks: using %q\n' "$effective"
  else
    echo 'hooks: disabled'
  fi
  ;;
esac
