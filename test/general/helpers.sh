#!/usr/bin/env bash

# A spec that runs a util starts a core, and each one lives in the spec's temporary home.
# The timeout is short so they leave on their own rather than accumulating for a quarter of
# an hour. It is set here rather than in the runner so that a spec run on its own gets it
# too.
export SPU_CORE_IDLE_TIMEOUT="${SPU_CORE_IDLE_TIMEOUT:-2s}"

_tests_run=0
_tests_failed=0

describe() { printf 'describe: %s\n' "$1"; }

toBe() {
  local description="$1" actual="$2" expected="$3"
  _tests_run=$((_tests_run + 1))
  if [[ "$actual" == "$expected" ]]; then
    printf '  ok - %s\n' "$description"
  else
    printf '  not ok - %s\n    expected: %s\n    actual: %s\n' "$description" "$expected" "$actual"
    _tests_failed=$((_tests_failed + 1))
  fi
}

finish() {
  printf 'Tests: %s, Passed: %s, Failed: %s\n' "$_tests_run" "$((_tests_run - _tests_failed))" "$_tests_failed"
  ((_tests_failed == 0))
}
