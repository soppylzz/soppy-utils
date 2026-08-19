#!/usr/bin/env zsh

# Spec for the terminal styling helper (src/utils/style.zsh).

source "${0:A:h}/general/helpers.zsh"

__style_dir="${0:A:h}/../src/utils/style.zsh"
__spu="${0:A:h}/../bin/spu"

describe "style: plain (non-tty) output"

  __out="$(zsh -c 'source "$1"; style error "boom" 1' -- "$__style_dir")"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "style emits no ANSI when not a tty" "$__status" "1"

  __out="$(zsh -c 'source "$1"; style_rich "{{header}}Options:{{/}}" 1' -- "$__style_dir")"
  [[ "$__out" == "Options:" ]]; __status=$?
  toBe "style_rich strips markup when not a tty" "$__status" "0"

  __out="$(zsh -c 'source "$1"; style_rich "  {{option}}-M, --mit{{/}} desc" 1' -- "$__style_dir")"
  [[ "$__out" == "  -M, --mit desc" ]]; __status=$?
  toBe "style_rich strips inline markup when not a tty" "$__status" "0"

describe "spu: usage output stays plain through a pipe"

  __out="$(zsh "$__spu" license --help 2>&1)"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "license --help has no ANSI through a pipe" "$__status" "1"

  __out="$(zsh "$__spu" bogus 2>&1)"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "error output has no ANSI through a pipe" "$__status" "1"

finish
