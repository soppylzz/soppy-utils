#!/usr/bin/env zsh

# Template helpers shared across tools.
#
# `fill_template` substitutes `{{key}}` placeholder tokens in a template string
# with values supplied as key/value pairs. It is generic: any tool that renders
# a template can pass the same kind of value list.

# fill_template <template> <key> <value> [<key> <value> ...]
#
# Prints the template with every `{{<key>}}` token replaced by the
# corresponding value. Pairs are consumed in order; a value may be empty.
# Tokens without a supplied value are left untouched.
fill_template() {
  local rendered="$1"
  shift

  while (( $# >= 2 )); do
    local key="$1" value="$2"
    shift 2
    rendered="${rendered//\{\{$key\}\}/$value}"
  done

  print -r -- "$rendered"
}
