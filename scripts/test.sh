#!/usr/bin/env zsh

# test.sh — run every spec under test/ with zsh.
#
# Discovers test/**/*.spec.sh relative to this script, executes each in a fresh
# zsh, and summarizes. Exits non-zero if any spec fails.

setopt LOCAL_OPTIONS NO_MONITOR

# Project root is one level above this script's directory.
__root="${(%):-%x}"
__root="${__root:A:h:h}"

source "$__root/src/utils/style.zsh"

__specs=("$__root"/test/**/*.spec.sh(N))

if (( ${#__specs} == 0 )); then
  err "test.sh: no spec files found under $__root/test"
  exit 1
fi

__failed=0

for spec in "$__specs[@]"; do
  style header "=== ${spec:t} ===" 1
  if zsh "$spec"; then
    style pass "PASS ${spec:t}" 1
  else
    style fail "FAIL ${spec:t}" 1
    (( __failed += 1 ))
  fi
  print -r -- ""
done

style info "Specs: ${#__specs}, Passed: $(( ${#__specs} - __failed )), Failed: $__failed" 1

(( __failed == 0 ))
