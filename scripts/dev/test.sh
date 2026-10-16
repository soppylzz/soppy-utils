#!/usr/bin/env bash
set -u

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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
