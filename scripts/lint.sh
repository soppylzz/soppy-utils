#!/usr/bin/env zsh

# lint.sh — run zsh-lint over the project's zsh scripts.
#
# Requires zsh-lint on $PATH. Exits non-zero when it is missing or when any
# diagnostics are reported.

setopt LOCAL_OPTIONS NO_MONITOR

# Project root is one level above this script's directory.
__root="${(%):-%x}"
__root="${__root:A:h:h}"

source "$__root/src/utils/style.zsh"

if ! command -v zsh-lint >/dev/null 2>&1; then
  err "lint.sh: zsh-lint not found on PATH."
  err "Install it via: go install github.com/z-shell/zsh-lint/cmd/zsh-lint@latest"
  err "See https://github.com/z-shell/zsh-lint"
  exit 1
fi

# Lint targets: bin entries (no extension), src .sh/.zsh, and scripts.
__targets=(
  "$__root"/bin/*
  "$__root"/src/**/*.sh(N)
  "$__root"/src/**/*.zsh(N)
  "$__root"/scripts/*.sh(N)
)

# Expand to a flat list of regular files (drop directories).
__files=()
for t in "$__targets[@]"; do
  [[ -f "$t" ]] && __files+=("$t")
done

if (( ${#__files} == 0 )); then
  err "lint.sh: no scripts found to lint"
  exit 1
fi

__found=0
for f in "$__files[@]"; do
  if ! zsh-lint "$f"; then
    (( __found += 1 ))
  fi
done

if (( __found > 0 )); then
  err "lint.sh: $__found file(s) with diagnostics"
  exit 1
fi

style pass "lint.sh: no issues in ${#__files} file(s)" 1
