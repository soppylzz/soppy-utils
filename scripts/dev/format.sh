#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mode="${1:---write}"
if (($# > 1)) || [[ "$mode" != --write && "$mode" != --check ]]; then
  echo 'Usage: format.sh [--write|--check]' >&2
  exit 1
fi

command -v shfmt >/dev/null 2>&1 || {
  echo 'format: shfmt is required' >&2
  exit 1
}
command -v gofmt >/dev/null 2>&1 || {
  echo 'format: gofmt is required' >&2
  exit 1
}

shell_files=("$root/.githooks/pre-commit" "$root/bin/spu" "$root/src/spu.sh" "$root"/src/utils/*.sh "$root"/scripts/*.sh "$root"/scripts/dev/*.sh "$root"/test/*.spec.sh "$root"/test/general/*.sh)
if [[ "$mode" == --write ]]; then
  shfmt -w -i 2 "${shell_files[@]}"
  (cd "$root" && gofmt -w cmd internal)
  echo 'format: files updated'
  exit 0
fi

failed=0
shfmt -d -i 2 "${shell_files[@]}" || failed=1
unformatted="$(cd "$root" && gofmt -l cmd internal)"
if [[ -n "$unformatted" ]]; then
  printf 'format: gofmt needed:\n%s\n' "$unformatted" >&2
  failed=1
fi
if ((failed)); then
  echo 'format: run task format and stage the changes' >&2
  exit 1
fi

echo 'format: passed'
