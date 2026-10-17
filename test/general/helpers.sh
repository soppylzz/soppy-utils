#!/usr/bin/env bash

# A spec that runs a util starts a core, and each one lives in the spec's temporary home.
# The timeout is short so they leave on their own rather than accumulating for a quarter of
# an hour. It is set here rather than in the runner so that a spec run on its own gets it
# too.
export SPU_CORE_IDLE_TIMEOUT="${SPU_CORE_IDLE_TIMEOUT:-2s}"

# Go's defaults are derived from HOME, which the next lines replace. A spec that runs spu
# out of a source checkout goes through `go run`, and with nowhere else to look that builds
# the whole module graph into the spec's own temporary home: every spec pays for a fresh
# cache, and the read-only files it leaves are what makes removing that home fail.
#
# Read before HOME moves, so these are the cache the machine already has.
GOPATH="$(go env GOPATH)"
GOCACHE="$(go env GOCACHE)"
GOMODCACHE="$(go env GOMODCACHE)"
export GOPATH GOCACHE GOMODCACHE

# Every spec runs in a home of its own, set here for the same reason: a spec that forgot
# to set one reached the real one. That is how license.spec.sh came to start a core in the
# configuration directory of whoever ran the suite, and leave it running there.
_spu_spec_home="$(mktemp -d)"
export HOME="$_spu_spec_home"

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
  # Removed here rather than in a trap, because a spec that sets one of its own would
  # replace it and leave this behind.
  [[ -n "${_spu_spec_home:-}" ]] && rm -r -- "$_spu_spec_home"
  printf 'Tests: %s, Passed: %s, Failed: %s\n' "$_tests_run" "$((_tests_run - _tests_failed))" "$_tests_failed"
  ((_tests_failed == 0))
}
