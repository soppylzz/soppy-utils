#!/usr/bin/env zsh

# license — render a LICENSE text from a template.

# Resolve this file's directory once at source time so the template path stays
# correct whether the file is sourced by the dispatcher or by a test spec.
__license_dir="${(%):-%N}"
__license_dir="${__license_dir:A:h}"

source "$__license_dir/utils/template.zsh"
source "$__license_dir/usage/license.zsh"

# license <type>
#
# Builds the readonly value map (author + year), reads the template for the
# given type, and prints the rendered result to stdout.
license() {
  local type="$1"

  typeset -rA __license_values=(
    name "$__license_author"
    year "$(date +%Y)"
  )

  local template="$__license_dir/template/$type.template"
  [[ -f "$template" ]] || {
    err "license: unknown type: $type"
    return 1
  }

  local rendered
  rendered="$(fill_template "$(<"$template")" "${(@kv)__license_values}")"
  print -r -- "$rendered"
}

# license_main [...args]
#
# CLI entry point: maps flags to a license type and routes output to stdout or
# the file given via --output.
license_main() {
  local type="" output=""

  while (( $# )); do
    case "$1" in
      --mit|-M) type="mit" ;;
      --isc|-I) type="isc" ;;
      --output|-o)
        [[ -n "${2:-}" ]] || { err "license: --output requires a file"; usage_license true >&2; return 1; }
        output="$2"
        shift
        ;;
      --help) usage_license true; return 0 ;;
      -h)     usage_license false; return 0 ;;
      *)
        err "license: unknown option: $1"
        usage_license true >&2
        return 1
        ;;
    esac
    shift
  done

  if [[ -z "$type" ]]; then
    usage_license false
    return 0
  fi

  if [[ -n "$output" ]]; then
    license "$type" > "$output"
  else
    license "$type"
  fi
}

# Resolve the author name once: prefer `git config user.name`, falling back to
# `$USER` when git is missing or user.name is unset.
if command -v git >/dev/null 2>&1; then
  __license_author="$(git config user.name 2>/dev/null)"
fi
[[ -n "$__license_author" ]] || __license_author="${USER:-}"
