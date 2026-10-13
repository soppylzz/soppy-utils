#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
temp="$(mktemp -d)"
trap 'chmod -R u+w "$temp"; rm -r -- "$temp"' EXIT
repo="$temp/repo"
mkdir -p "$repo/scripts/dev" "$repo/.githooks"
cp "$root/scripts/dev/hooks.sh" "$repo/scripts/dev/hooks.sh"
cp "$root/.githooks/pre-commit" "$repo/.githooks/pre-commit"
cat >"$repo/scripts/dev/lint.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test "$(cat "$root/check.txt")" = good
EOF

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
git -C "$repo" init -q
git -C "$repo" config user.name 'Hook Test'
git -C "$repo" config user.email 'hook@example.invalid'
git -C "$repo" config commit.gpgsign false

describe 'local Git hook configuration'
git -C "$repo" config --local core.hooksPath "$temp/other-hooks"
original_hooks_path="$(git -C "$repo" config --local --get core.hooksPath)"
bash "$repo/scripts/dev/hooks.sh" install >/dev/null 2>&1
toBe 'existing hook path is preserved' "$?" 1
toBe 'existing path is unchanged' "$(git -C "$repo" config --local --get core.hooksPath)" "$original_hooks_path"
git -C "$repo" config --local --unset core.hooksPath

git -C "$repo" config --local core.hooksPath ''
bash "$repo/scripts/dev/hooks.sh" install >/dev/null 2>&1
toBe 'explicit empty hook path is preserved' "$?" 1
git -C "$repo" config --local --unset core.hooksPath

default_hook="$(git -C "$repo" rev-parse --git-path hooks/pre-commit)"
case "$default_hook" in
/* | [A-Za-z]:/*) ;;
*) default_hook="$repo/$default_hook" ;;
esac
printf '#!/usr/bin/env bash\n' >"$default_hook"
bash "$repo/scripts/dev/hooks.sh" install >/dev/null 2>&1
toBe 'existing pre-commit hook is preserved' "$?" 1
rm -- "$default_hook"

other_hook="${default_hook%/pre-commit}/commit-msg"
printf '#!/usr/bin/env bash\n' >"$other_hook"
bash "$repo/scripts/dev/hooks.sh" install >/dev/null 2>&1
toBe 'other default hooks are preserved' "$?" 1
rm -- "$other_hook"

bash "$repo/scripts/dev/hooks.sh" install >/dev/null 2>&1
toBe 'project hook installs' "$?" 0
toBe 'local hook path is configured' "$(git -C "$repo" config --local --get core.hooksPath)" .githooks

describe 'pre-commit checks the staged snapshot'
printf 'bad\n' >"$repo/check.txt"
git -C "$repo" add .
printf 'good\n' >"$repo/check.txt"
git -C "$repo" commit -qm 'test: staged snapshot' >/dev/null 2>&1
toBe 'unstaged fix does not hide staged failure' "$?" 1

git -C "$repo" add check.txt
git -C "$repo" commit -qm 'test: staged snapshot' >/dev/null 2>&1
toBe 'valid staged files commit' "$?" 0

bash "$repo/scripts/dev/hooks.sh" uninstall >/dev/null 2>&1
toBe 'project hook uninstalls' "$?" 0
toBe 'local hook path is removed' "$(git -C "$repo" config --local --get core.hooksPath || true)" ''

finish
