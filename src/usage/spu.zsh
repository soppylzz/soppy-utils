#!/usr/bin/env zsh

# Usage for the spu launcher overview.

__usage_dir="${(%):-%N}"
__usage_dir="${__usage_dir:A:h}"

source "$__usage_dir/vscode.zsh"
source "$__usage_dir/license.zsh"

# usage_spu [verbose]
#
# Prints the top-level overview, composing each tool's brief one-liner so the
# descriptions have a single source of truth (the raw text in `__usage_brief`).
# Detailed mode adds a hint footer.
usage_spu() {
  local verbose="${1:-false}"

  style_rich "Usage: spu {{arg}}<util>{{/}} [{{arg}}...args{{/}}]" 1
  style_rich "{{header}}Utils:{{/}}" 1
  style_rich "  ${__usage_brief[usage_vscode]}" 1
  style_rich "  ${__usage_brief[usage_license]}" 1

  if [[ "$verbose" == true ]]; then
    print -r -- ""
    style_rich "{{hint}}Run 'spu <util> --help' for a util's detailed help.{{/}}" 1
  fi
}
