#!/usr/bin/env zsh

# version.sh — semver determination for the CI/CD release flow.
#
# Two modes:
#   suggest              — print the bump (major/minor/patch/none) and next
#                          version derived from Conventional Commits since the
#                          latest tag. Used by CI as a non-blocking hint.
#   resolve <bump>       — print the bump and next version for CD. <bump> is
#                          `auto` (derive from commits) or an explicit
#                          major/minor/patch override. Writes `bump`/`next` to
#                          $GITHUB_OUTPUT when available so the release workflow
#                          can consume them as step outputs.
#
# Versioning rules: breaking change (! or a `BREAKING CHANGE:` footer) → major;
# a `feat` commit → minor; any other conventional commit → patch; no
# conventional commits → none. With no tag yet, the base is 0.0.0.

setopt LOCAL_OPTIONS NO_MONITOR
zmodload zsh/regex

# __bump_version <v> <bump>
#
# Prints <v> with the given bump applied (major/minor/patch). <v> may omit the
# leading `v`; missing minor/patch segments default to 0.
__bump_version() {
  local v="${1#v}" bump="$2"
  local major minor patch
  local IFS=.
  local -a parts=(${=v})
  major="${parts[1]:-0}"
  minor="${parts[2]:-0}"
  patch="${parts[3]:-0}"

  case "$bump" in
    major) major=$(( major + 1 )); minor=0; patch=0 ;;
    minor) minor=$(( minor + 1 )); patch=0 ;;
    patch) patch=$(( patch + 1 )) ;;
  esac

  print -r -- "$major.$minor.$patch"
}

# __commit_bump <full-commit-message>
#
# Prints the bump this commit implies: major / minor / patch / none.
__commit_bump() {
  local msg="$1"

  # Breaking footer (matches `BREAKING CHANGE:` at any position).
  [[ "$msg" == *'BREAKING CHANGE:'* ]] && { print -r -- "major"; return 0; }

  local subject="${msg%%$'\n'*}"

  # Breaking `!` in the type/scope prefix (e.g. `feat!:` / `fix(scope)!:`).
  [[ "$subject" =~ '^[A-Za-z]+(\([^)]*\))?!:' ]] && { print -r -- "major"; return 0; }

  [[ "$subject" =~ '^feat(\([^)]*\))?:' ]] && { print -r -- "minor"; return 0; }

  [[ "$subject" =~ '^(fix|chore|docs|refactor|perf|style|test|build|ci|revert)(\([^)]*\))?:' ]] \
    && { print -r -- "patch"; return 0; }

  print -r -- "none"
}

# __latest_tag
#
# Prints the most recent reachable tag, or nothing when none exists.
__latest_tag() {
  git describe --tags --abbrev=0 2>/dev/null
}

# __suggest
#
# Prints `bump=<major|minor|patch|none>` and `next=v<X.Y.Z>` for the commits
# since the latest tag (or the whole history when no tag exists yet).
__suggest() {
  local tag
  tag="$(__latest_tag)"
  local base="${tag#v}"

  local -a hashes
  if [[ -n "$tag" ]]; then
    hashes=("${(@f)$(git rev-list "$tag..HEAD" 2>/dev/null)}")
  else
    hashes=("${(@f)$(git rev-list HEAD 2>/dev/null)}")
  fi

  local bump="none"
  local h this
  for h in "$hashes[@]"; do
    this="$(__commit_bump "$(git log -1 --format=%B "$h" 2>/dev/null)")"
    case "$this" in
      major) bump="major"; break ;;
      minor) [[ "$bump" == none || "$bump" == patch ]] && bump="minor" ;;
      patch) [[ "$bump" == none ]] && bump="patch" ;;
    esac
  done

  print -r -- "bump=$bump"
  print -r -- "next=v$(__bump_version "$base" "$bump")"
}

# __resolve <bump>
#
# Prints `bump=` and `next=` for CD. `auto` derives the bump from commits (and
# emits `bump=none` with no `next` when there is nothing to release); an
# explicit major/minor/patch forces that bump. Writes both to $GITHUB_OUTPUT.
__resolve() {
  local bump="${1:-auto}"

  if [[ "$bump" == auto ]]; then
    local suggested line
    suggested="$(__suggest)"
    while IFS= read -r line; do
      [[ "$line" == bump=* ]] && bump="${line#bump=}"
    done <<< "$suggested"
    if [[ "$bump" == none ]]; then
      print -r -- "bump=none"
      if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
        print -r -- "bump=none" >> "$GITHUB_OUTPUT"
      fi
      return 0
    fi
  fi

  local tag base next
  tag="$(__latest_tag)"
  base="${tag#v}"
  next="$(__bump_version "$base" "$bump")"

  print -r -- "bump=$bump"
  print -r -- "next=v$next"

  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    print -r -- "bump=$bump" >> "$GITHUB_OUTPUT"
    print -r -- "next=v$next" >> "$GITHUB_OUTPUT"
  fi
}

# version_main <mode> [bump]
version_main() {
  case "${1:-}" in
    suggest) __suggest ;;
    resolve) __resolve "${2:-auto}" ;;
    *)
      print -r -- "usage: version.sh <suggest|resolve [major|minor|patch|auto]>" >&2
      return 1
      ;;
  esac
}

# Only dispatch when run as a script, not when sourced by a spec.
# $ZSH_EVAL_CONTEXT is `toplevel` for a directly-run script and ends in `:file`
# when the file is sourced (e.g. `toplevel:file`, `cmdarg:file`).
if [[ "${ZSH_EVAL_CONTEXT}" != *:file ]]; then
  version_main "$@"
fi
