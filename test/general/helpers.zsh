#!/usr/bin/env zsh

# Shared test primitives for the soppy-utils test suite.
#
# Minimal for now: `describe` groups assertions and `toBe` compares a value
# against an expected value. `finish` prints a summary and returns non-zero if
# any assertion failed.

__tests_run=0
__tests_failed=0

describe() {
  print -- "describe: $1"
}

toBe() {
  local desc="$1" actual="$2" expected="$3"
  (( __tests_run += 1 ))
  if [[ "$actual" == "$expected" ]]; then
    print -- "  ok - $desc"
  else
    print -- "  not ok - $desc"
    print -- "    expected: $expected"
    print -- "    actual:   $actual"
    (( __tests_failed += 1 ))
  fi
}

finish() {
  print -- ""
  print -- "Tests: $__tests_run, Passed: $(( __tests_run - __tests_failed )), Failed: $__tests_failed"
  (( __tests_failed == 0 ))
}
