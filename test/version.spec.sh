#!/usr/bin/env zsh

# Spec for scripts/version.sh.

source "${0:A:h}/general/helpers.zsh"

# Mock git so versioning is deterministic and independent of the real repo.
__log=()           # lines of `hash|message` — commits since the latest tag
__pre_tag=()       # lines of `hash|message` — commits before the latest tag
__tags=()          # tags reachable from HEAD, newest first
git() {
  case "$1" in
    describe)
      (( ${#__tags} )) && print -r -- "${__tags[1]}" || return 1
      ;;
    rev-list)
      # `git rev-list HEAD` (no range) returns full history; `git rev-list
      # <tag>..HEAD` returns only the commits since <tag>. Respect the range so
      # a regression in __suggest that walks the wrong refs is caught.
      if [[ "${2:-}" == *..* ]]; then
        for entry in "$__log[@]"; do
          print -r -- "${entry%%|*}"
        done
      else
        for entry in "$__pre_tag[@]" "$__log[@]"; do
          print -r -- "${entry%%|*}"
        done
      fi
      ;;
    log)
      # `git log -1 --format=%B <hash>` — emit the message for that hash.
      local h="${@[-1]}"
      for entry in "$__pre_tag[@]" "$__log[@]"; do
        [[ "${entry%%|*}" == "$h" ]] && print -r -- "${entry#*|}"
      done
      ;;
  esac
}

source "${0:A:h}/../scripts/version.sh"

describe "version: __bump_version"

  toBe "major resets minor/patch" "$(__bump_version 1.2.3 major)" "2.0.0"
  toBe "minor resets patch" "$(__bump_version 1.2.3 minor)" "1.3.0"
  toBe "patch increments patch" "$(__bump_version 1.2.3 patch)" "1.2.4"
  toBe "strips leading v" "$(__bump_version v1.2.3 patch)" "1.2.4"
  toBe "defaults missing segments" "$(__bump_version 0 major)" "1.0.0"

describe "version: __commit_bump"

  toBe "breaking footer is major" "$(__commit_bump $'fix: x\n\nBREAKING CHANGE: y')" "major"
  toBe "hyphen footer is major" "$(__commit_bump $'fix: x\n\nBREAKING-CHANGE: y')" "major"
  toBe "body prose mentioning token is not major" "$(__commit_bump $'chore: docs say breaking (! or BREAKING CHANGE:) -> major')" "patch"
  toBe "type bang is major" "$(__commit_bump $'feat(api)!: drop old flag')" "major"
  toBe "feat is minor" "$(__commit_bump $'feat: add util')" "minor"
  toBe "fix is patch" "$(__commit_bump $'fix(core): bug')" "patch"
  toBe "chore is patch" "$(__commit_bump $'chore: tidy')" "patch"
  toBe "non-conventional is none" "$(__commit_bump $'random text')" "none"

describe "version: __suggest"

  __tags=(v1.2.3)
  __log=(
    "a|fix: one"
    "b|feat: two"
    "c|chore: three"
  )
  toBe "feat dominates -> minor" "$(__suggest | grep '^bump=')" "bump=minor"
  toBe "next is v1.3.0" "$(__suggest | grep '^next=')" "next=v1.3.0"

  __log=(
    "a|fix: one"
    "b|docs: two"
  )
  toBe "only patch-level -> patch" "$(__suggest | grep '^bump=')" "bump=patch"

  __log=(
    "a|feat: one"
    "b|feat(api)!: break"
  )
  toBe "breaking dominates -> major" "$(__suggest | grep '^bump=')" "bump=major"

  __log=()
  toBe "no commits -> none" "$(__suggest | grep '^bump=')" "bump=none"

  # A breaking commit before the tag must not influence the suggestion: the
  # range walk must exclude pre-tag history.
  __log=("a|fix: one")
  __pre_tag=("z|feat(api)!: old breaking")
  toBe "pre-tag breaking is excluded -> patch" "$(__suggest | grep '^bump=')" "bump=patch"

describe "version: first release from 0.0.0"

  __tags=()
  __pre_tag=()
  __log=("a|feat: first")
  toBe "no tag suggests minor" "$(__suggest | grep '^bump=')" "bump=minor"
  toBe "no tag next is v0.1.0" "$(__suggest | grep '^next=')" "next=v0.1.0"

  toBe "resolve major from empty -> v1.0.0" "$(__resolve major | grep '^next=')" "next=v1.0.0"

describe "version: __resolve"

  __tags=(v1.2.3)
  __pre_tag=()
  __log=("a|feat: x")
  toBe "resolve minor -> v1.3.0" "$(__resolve minor | grep '^next=')" "next=v1.3.0"
  toBe "resolve auto -> v1.3.0" "$(__resolve auto | grep '^next=')" "next=v1.3.0"

  __log=()
  toBe "resolve auto with no commits -> none" "$(__resolve auto | grep '^bump=')" "bump=none"

finish
