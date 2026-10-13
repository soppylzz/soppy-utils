#!/usr/bin/env bash

bump_version() {
  local version="${1#v}" bump="$2" major=0 minor=0 patch=0
  IFS=. read -r major minor patch <<<"$version"
  major="${major:-0}"
  minor="${minor:-0}"
  patch="${patch:-0}"

  case "$bump" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch) patch=$((patch + 1)) ;;
  esac

  printf '%s.%s.%s\n' "$major" "$minor" "$patch"
}

commit_bump() {
  local message="$1" line subject breaking_re='^[A-Za-z]+(\([^)]*\))?!:' feat_re='^feat(\([^)]*\))?:' patch_re='^(fix|chore|docs|refactor|perf|style|test|build|ci|revert)(\([^)]*\))?:'
  while IFS= read -r line; do
    [[ "$line" =~ ^BREAKING[\ -]CHANGE: ]] && {
      echo major
      return
    }
  done <<<"$message"

  subject="${message%%$'\n'*}"
  [[ "$subject" =~ $breaking_re ]] && {
    echo major
    return
  }
  [[ "$subject" =~ $feat_re ]] && {
    echo minor
    return
  }
  [[ "$subject" =~ $patch_re ]] && {
    echo patch
    return
  }

  echo none
}

latest_tag() { git describe --tags --abbrev=0 2>/dev/null || true; }

suggest_version() {
  local tag base hashes hash bump=none this
  tag="$(latest_tag)"
  base="${tag#v}"

  if [[ -n "$tag" ]]; then
    hashes="$(git rev-list "$tag..HEAD" 2>/dev/null || true)"
  else hashes="$(git rev-list HEAD 2>/dev/null || true)"; fi

  while IFS= read -r hash; do
    [[ -n "$hash" ]] || continue
    this="$(commit_bump "$(git log -1 --format=%B "$hash" 2>/dev/null)")"
    case "$this" in
    major)
      bump=major
      break
      ;;
    minor) [[ "$bump" != major ]] && bump=minor ;;
    patch) [[ "$bump" == none ]] && bump='patch' ;;
    esac
  done <<<"$hashes"

  printf 'bump=%s\nnext=v%s\n' "$bump" "$(bump_version "$base" "$bump")"
}

version_main() {
  local mode="${1:-}" bump="${2:-auto}" suggested tag next
  case "$mode" in
  suggest) suggest_version ;;
  resolve)
    if [[ "$bump" == auto ]]; then
      suggested="$(suggest_version)"
      bump="${suggested#bump=}"
      bump="${bump%%$'\n'*}"
    fi

    case "$bump" in major | minor | patch | none) ;; *)
      echo 'version: invalid bump' >&2
      return 1
      ;;
    esac
    if [[ "$bump" == none ]]; then
      echo 'bump=none'
      [[ -z "${GITHUB_OUTPUT:-}" ]] || echo 'bump=none' >>"$GITHUB_OUTPUT"
      return 0
    fi

    tag="$(latest_tag)"
    next="v$(bump_version "${tag#v}" "$bump")"
    printf 'bump=%s\nnext=%s\n' "$bump" "$next"
    if [[ -n "${GITHUB_OUTPUT:-}" ]]; then printf 'bump=%s\nnext=%s\n' "$bump" "$next" >>"$GITHUB_OUTPUT"; fi
    ;;
  *)
    echo 'Usage: version.sh suggest|resolve [auto|major|minor|patch]' >&2
    return 1
    ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then version_main "$@"; fi
