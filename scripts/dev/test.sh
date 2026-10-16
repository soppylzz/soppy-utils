#!/usr/bin/env bash
set -u

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# A spec that runs a util starts a core, and each one lives in the spec's temporary home.
# A short idle timeout means they leave on their own rather than accumulating for a
# quarter of an hour after the suite has finished.
export SPU_CORE_IDLE_TIMEOUT=2s
failed=0

for spec in "$root"/test/*.spec.sh; do
  printf '=== %s ===\n' "${spec##*/}"
  bash "$spec" || failed=$((failed + 1))
done

(cd "$root" && go test ./...) || failed=$((failed + 1))

if ((failed)); then
  printf 'test: %d suite(s) failed\n' "$failed" >&2
  exit 1
fi

printf '%s\n' 'test: all suites passed'
