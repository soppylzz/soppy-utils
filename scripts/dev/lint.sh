#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
files=("$root/.githooks/pre-commit" "$root/bin/spu" "$root/src/spu.sh" "$root/src/vscode.sh" "$root"/src/utils/*.sh "$root"/scripts/*.sh "$root"/scripts/dev/*.sh "$root"/test/*.spec.sh "$root"/test/general/*.sh)
for file in "${files[@]}"; do bash -n "$file"; done

command -v shellcheck >/dev/null 2>&1 || {
  echo 'lint: shellcheck is required' >&2
  exit 1
}

bash "$root/scripts/dev/format.sh" --check
shellcheck -S warning "${files[@]}"

(cd "$root" && go vet ./...)
echo 'lint: passed'
