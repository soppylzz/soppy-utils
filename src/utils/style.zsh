#!/usr/bin/env zsh

# Terminal styling helpers shared across tools and scripts.
#
# Colors are configured with zstyle under the `:spu:style:` namespace; a caller
# overrides a token by setting `zstyle ':spu:style:<token>' sgr <codes>`. ANSI
# escapes are only emitted when the target fd is a TTY, TERM is not "dumb", and
# NO_COLOR is unset — otherwise the text is printed unchanged. All output goes
# to stderr by default so styled text never contaminates stdout streams (such
# as the license util's rendered LICENSE text).

zmodload zsh/zutil

# Source guard: this file is sourced from usage.zsh, which the dispatcher and
# per-tool files both pull in. Re-declaring the readonly reset would error.
[[ -n "${__style_loaded:-}" ]] && return 0
typeset -g __style_loaded=1

# Terminal reset; not user-overridable.
typeset -r __style_reset=$'\e[0m'

# Default styles, applied first so user zstyle overrides win.
zstyle ':spu:style:error'   sgr $'\e[31m'     # red
zstyle ':spu:style:warn'    sgr $'\e[33m'     # yellow
zstyle ':spu:style:info'    sgr $'\e[34m'     # blue
zstyle ':spu:style:header'  sgr $'\e[1m'      # bold
zstyle ':spu:style:command' sgr $'\e[1;34m'   # bold blue
zstyle ':spu:style:option'  sgr $'\e[36m'     # cyan
zstyle ':spu:style:arg'     sgr $'\e[36m'     # cyan (placeholder args)
zstyle ':spu:style:hint'    sgr $'\e[2m'      # dim
zstyle ':spu:style:pass'    sgr $'\e[32m'     # green
zstyle ':spu:style:fail'    sgr $'\e[31m'     # red

# __style_enabled [fd]
#
# True (exit 0) when colors should be emitted to fd, false otherwise.
__style_enabled() {
  local fd="${1:-1}"
  [[ -t $fd ]] || return 1
  [[ "${NO_COLOR:-}" == "" ]] || return 1
  case "${TERM:-}" in
    dumb|"") return 1 ;;
  esac
  return 0
}

# __style_sgr <token>
#
# Prints the ANSI SGR codes for <token>, or nothing when unset.
__style_sgr() {
  local c=""
  zstyle -s ":spu:style:$1" sgr c
  print -rn -- "$c"
}

# style <token> <text> [fd]
#
# Prints <text> styled as <token>. Defaults to stderr (fd 2); pass 1 to write
# to stdout.
style() {
  local token="$1" text="$2" fd="${3:-2}"
  if __style_enabled "$fd"; then
    print -r -- "$(__style_sgr "$token")${text}${__style_reset}" >&$fd
  else
    print -r -- "$text" >&$fd
  fi
}

# err <text>   — styled error message on stderr
# warn <text>  — styled warning message on stderr
# info <text>  — styled info message on stderr
err()  { style error "$1" 2; }
warn() { style warn  "$1" 2; }
info() { style info  "$1" 2; }

# style_rich <text> [fd]
#
# Prints <text> with inline `{{token}}…{{/}}` spans styled as `token`. With
# color disabled the markup is stripped and the text prints plain. Used for
# multi-part output such as usage lines.
style_rich() {
  local text="$1" fd="${2:-2}"
  local enabled=0
  __style_enabled "$fd" && enabled=1

  local out="" rest="$text" tok="" opened=0
  while [[ "$rest" =~ '\{\{([a-zA-Z0-9_-]+)\}\}' ]]; do
    tok="${match[1]}"
    out+="${rest%%\{\{$tok\}\}*}"
    rest="${rest#*\{\{$tok\}\}}"
    if (( enabled )); then
      out+="$(__style_sgr "$tok")"
      opened=1
    fi
    if [[ "$rest" == *'{{/}}'* ]]; then
      out+="${rest%%\{\{/\}\}*}"
      (( enabled )) && out+="${__style_reset}"
      rest="${rest#*\{\{/\}\}}"
      opened=0
    else
      out+="$rest"
      rest=""
    fi
  done
  out+="$rest"
  (( enabled && opened )) && out+="${__style_reset}"

  print -r -- "$out" >&$fd
}
