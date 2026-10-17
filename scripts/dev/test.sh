#!/usr/bin/env bash
set -u

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
failed=0

# Build the binary the launcher will use, once.
#
# Without it bin/spu falls back to `go run`, which compiles on every invocation: the first
# spec's first command pays for the whole build, and every spec leaves a `go run` wrapper
# between it and the client. Those wrappers outlive the spec, hold its output stream open,
# and are what made a CI job sit on one line for three hours despite the watchdog below.
#
# Removed again only when this run is what created it, so a developer's build is left alone.
built_here=0
if [[ ! -x "$root/libexec/spu" ]]; then
  mkdir -p "$root/libexec"
  (cd "$root" && go build -o "$root/libexec/spu" ./cmd/spu) || {
    printf 'test: could not build the binary the launcher needs\n' >&2
    exit 1
  }
  built_here=1
fi
cleanup_build() {
  if ((built_here)); then
    rm -f -- "$root/libexec/spu"
    rmdir "$root/libexec" 2>/dev/null
  fi
}
trap cleanup_build EXIT

# A spec that blocks must fail rather than hold the job. One did: the suite sat on the
# same line for three hours on CI, and a job that never finishes says nothing about what
# is wrong. `timeout` is not everywhere — macOS ships no such command — so this is the
# same idea in the shell it has to run in.
spec_timeout="${SPU_SPEC_TIMEOUT:-120}"

run_spec() {
  local spec="$1" pid watchdog
  bash "$spec" &
  pid=$!
  (
    # The watchdog sleeps in the background and kills the spec if it outlives its budget.
    # SIGTERM first, so a core the spec started can leave; SIGKILL after, in case it cannot.
    sleep "$spec_timeout"
    kill -TERM "$pid" 2>/dev/null || exit 0
    sleep 5
    kill -KILL "$pid" 2>/dev/null || true
  ) &
  watchdog=$!
  wait "$pid"
  local status=$?
  kill "$watchdog" 2>/dev/null || true
  wait "$watchdog" 2>/dev/null || true
  return $status
}

for spec in "$root"/test/*.spec.sh; do
  printf '=== %s ===\n' "${spec##*/}"
  if ! run_spec "$spec"; then
    if [[ $? -eq 143 || $? -eq 137 ]]; then
      printf 'test: %s did not finish within %ss\n' "${spec##*/}" "$spec_timeout" >&2
    fi
    failed=$((failed + 1))
  fi
done

(cd "$root" && go test ./...) || failed=$((failed + 1))

if ((failed)); then
  printf 'test: %d suite(s) failed\n' "$failed" >&2
  exit 1
fi

printf '%s\n' 'test: all suites passed'
