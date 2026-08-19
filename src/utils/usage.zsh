#!/usr/bin/env zsh

# Usage-builder factory shared across tools.
#
# `create_usager` defines a `usage_<name>` function with two output modes,
# selected by a `verbose` boolean argument:
#   verbose=true  — detailed help text (invoked by `spu <util> --help`)
#   verbose=false — brief one-line summary (invoked by `spu <util> -h`, and
#                   called by the overview to compose its output)
#
# Brief and detailed text are stored in global associative arrays keyed by the
# function name; the generated function looks itself up via `$0`. Keeping the
# text out of the generated function body avoids any shell-quoting hazards.
#
# The stored text may carry `{{token}}…{{/}}` markup; the generated function
# renders it through the shared `style_rich` helper (see utils/style.zsh),
# which drops the markup when color is disabled.

__usage_utils_dir="${(%):-%N}"
__usage_utils_dir="${__usage_utils_dir:A:h}"

source "$__usage_utils_dir/style.zsh"

typeset -gA __usage_brief __usage_detailed

# create_usager <name> <brief> <detailed>
#
# Defines the function `<name>` such that `<name> false` prints `brief` and
# `<name> true` prints `detailed`, each styled via `style_rich`. The argument
# is optional; omitting it defaults to brief mode.
create_usager() {
  local name="$1"
  __usage_brief[$name]="$2"
  __usage_detailed[$name]="$3"

  functions[$name]='if [[ "${1:-}" == true ]]; then style_rich "${__usage_detailed[$0]}" 1; else style_rich "${__usage_brief[$0]}" 1; fi'
}
