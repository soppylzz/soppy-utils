#!/usr/bin/env bash
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
