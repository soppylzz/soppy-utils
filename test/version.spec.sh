#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../scripts/version.sh"

describe 'version arithmetic and commit messages'
toBe 'major resets' "$(bump_version 1.2.3 major)" '2.0.0'

toBe 'minor resets' "$(bump_version 1.2.3 minor)" '1.3.0'

toBe 'patch increments' "$(bump_version v1.2.3 patch)" '1.2.4'

toBe 'breaking footer' "$(commit_bump $'fix: x\nBREAKING CHANGE: y')" major

toBe 'breaking scope' "$(commit_bump 'feat(api)!: break')" major

toBe 'feature' "$(commit_bump 'feat: add')" minor

toBe 'fix' "$(commit_bump 'fix: x')" patch

toBe 'random' "$(commit_bump 'random text')" none

_tags=(v1.2.3)
_log=('a|fix: one' 'b|feat: two')
_pre_tag=()
git() {
  local -a entries
  local entry hash
  case "$1" in
  describe) ((${#_tags[@]})) && printf '%s\n' "${_tags[0]}" ;;
  rev-list)
    if [[ "${2:-}" == *..* ]]; then entries=("${_log[@]}"); else entries=("${_pre_tag[@]}" "${_log[@]}"); fi
    for entry in "${entries[@]}"; do printf '%s\n' "${entry%%|*}"; done
    ;;
  log)
    hash="${4:-}"
    for entry in "${_pre_tag[@]}" "${_log[@]}"; do [[ "${entry%%|*}" == "$hash" ]] && printf '%s\n' "${entry#*|}"; done
    ;;
  esac
}

describe 'release suggestion and tag boundaries'
toBe 'feature dominates' "$(suggest_version | head -1)" 'bump=minor'

toBe 'next minor release' "$(suggest_version | tail -1)" 'next=v1.3.0'

_log=('a|fix: one')
_pre_tag=('z|feat(api)!: old breaking')
toBe 'old breaking commit is excluded' "$(suggest_version | head -1)" 'bump=patch'

_log=()
toBe 'no new commit means no release' "$(version_main resolve auto)" 'bump=none'

_tags=()
_pre_tag=()
_log=('a|feat: first')
toBe 'first release starts from zero' "$(suggest_version | tail -1)" 'next=v0.1.0'

finish
