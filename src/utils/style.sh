#!/usr/bin/env bash

_spu_color_enabled() {
  local fd="${1:-2}"
  [[ -z "${NO_COLOR:-}" && -n "${TERM:-}" && "$TERM" != dumb && -t "$fd" ]]
}

_spu_color_code() {
  case "$1" in
  error | fail) echo 31 ;;
  warn | hint) echo 33 ;;
  info | command | option) echo 36 ;;
  header | arg) echo 35 ;;
  pass) echo 32 ;;
  *) echo 0 ;;
  esac
}

style() {
  local token="$1" message="$2" fd="${3:-2}" code
  if _spu_color_enabled "$fd"; then
    code="$(_spu_color_code "$token")"
    printf '\033[%sm%s\033[0m\n' "$code" "$message" >&"$fd"
  else
    printf '%s\n' "$message" >&"$fd"
  fi
}
err() { style error "$1"; }
warn() { style warn "$1"; }
info() { style info "$1"; }

style_rich() {
  local remaining="$1" fd="${2:-2}" prefix inner token code
  while [[ "$remaining" == *'{{'* && "$remaining" == *'{{/}}'* ]]; do
    prefix="${remaining%%'{{'*}"
    remaining="${remaining#*'{{'}"
    token="${remaining%%'}}'*}"
    remaining="${remaining#*'}}'}"
    inner="${remaining%%'{{/}}'*}"
    remaining="${remaining#*'{{/}}'}"
    printf '%s' "$prefix" >&"$fd"
    if _spu_color_enabled "$fd"; then
      code="$(_spu_color_code "$token")"
      printf '\033[%sm%s\033[0m' "$code" "$inner" >&"$fd"
    else
      printf '%s' "$inner" >&"$fd"
    fi
  done
  printf '%s\n' "$remaining" >&"$fd"
}
