#!/usr/bin/env zsh

# Spec for the terminal styling helper (src/utils/style.zsh).

source "${0:A:h}/general/helpers.zsh"

__style_dir="${0:A:h}/../src/utils/style.zsh"
__spu="${0:A:h}/../bin/spu"

# run_pty <style-path> <zsh-code> — run code under a pseudo-tty so `-t 1` is
# true, and print its combined stdout+stderr. Used to exercise the
# color-enabled path. The style path is passed as `$1` to the inner zsh.
run_pty() {
  local style_path="$1" code="$2"
  # macOS `script` writes a prologue (`^D^H^H`) and CRLF line endings; the
  # assertions below only check for the presence of ESC sequences, so neither
  # artifact matters. TERM is forced to a color-capable value so the CI default
  # (TERM=dumb) does not suppress color and mask the tty path.
  TERM=xterm-256color script -q /dev/null zsh -c "$code" -- "$style_path" 2>&1
}

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

describe "style: colored (tty) output"

  __out="$(run_pty "$__style_dir" 'source "$1"; style error "boom" 1')"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "style emits ANSI when on a tty" "$__status" "0"

  __out="$(run_pty "$__style_dir" 'source "$1"; style_rich "{{header}}Options:{{/}}" 1')"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "style_rich emits ANSI when on a tty" "$__status" "0"

describe "style: NO_COLOR forces plain even on a tty"

  __out="$(run_pty "$__style_dir" 'source "$1"; NO_COLOR=1 style error "boom" 1')"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "NO_COLOR suppresses ANSI on a tty" "$__status" "1"

describe "spu: usage output stays plain through a pipe"

  __out="$(zsh "$__spu" license --help 2>&1)"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "license --help has no ANSI through a pipe" "$__status" "1"

  __out="$(zsh "$__spu" bogus 2>&1)"
  [[ "$__out" == *$'\e['* ]]; __status=$?
  toBe "error output has no ANSI through a pipe" "$__status" "1"

finish
