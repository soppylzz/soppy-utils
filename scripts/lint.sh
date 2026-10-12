#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
files=("$root/bin/spu" "$root/src/spu.sh" "$root/src/vscode.sh" "$root"/src/utils/*.sh "$root"/scripts/*.sh "$root"/test/*.spec.sh "$root"/test/general/*.sh)
for file in "${files[@]}"; do bash -n "$file"; done

command -v shellcheck >/dev/null 2>&1 || {
  echo 'lint: shellcheck is required' >&2
  exit 1
}
command -v shfmt >/dev/null 2>&1 || {
  echo 'lint: shfmt is required' >&2
  exit 1
}

shellcheck -S warning "${files[@]}"
shfmt -d -i 2 "${files[@]}"

(cd "$root" && test -z "$(gofmt -l cmd internal)" && go vet ./...)
echo 'lint: passed'
