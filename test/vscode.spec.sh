#!/usr/bin/env zsh

# Spec for the vscode util.

source "${0:A:h}/general/helpers.zsh"
source "${0:A:h}/../src/vscode.sh"

# Mock `code` so tests capture the path it would open instead of launching VS Code.
__code_arg=""
code() { __code_arg="$1"; }

# Deterministic HOME so expected paths are stable; the expected user dir must
# match the OS branch in vscode.sh.
HOME="/tmp/soppy-utils-test-home"
case "$OSTYPE" in
  darwin*) __user_dir="$HOME/Library/Application Support/Code/User" ;;
  linux*)  __user_dir="$HOME/.config/Code/User" ;;
  *)       __user_dir="$HOME/Code/User" ;;
esac

describe "vscode: target mapping"

  vscode snippets
  toBe "snippets opens the snippets dir" "$__code_arg" "$__user_dir/snippets"

  vscode settings
  toBe "settings opens settings.json" "$__code_arg" "$__user_dir/settings.json"

  vscode keys
  toBe "keys opens keybindings.json" "$__code_arg" "$__user_dir/keybindings.json"

describe "vscode: unknown target"

  vscode bogus 2>/dev/null
  __status=$?
  toBe "unknown target returns non-zero" "$__status" "1"

describe "vscode: usage via spu"

  zsh "${0:A:h}/../bin/spu" vscode --help >/dev/null 2>&1
  __status=$?
  toBe "--help exits zero" "$__status" "0"

  zsh "${0:A:h}/../bin/spu" vscode -h >/dev/null 2>&1
  __status=$?
  toBe "-h exits zero" "$__status" "0"

  zsh "${0:A:h}/../bin/spu" vscode --bogus >/dev/null 2>&1
  __status=$?
  toBe "unknown option exits non-zero" "$__status" "1"

describe "vscode: brief vs detailed"

  __brief="$(zsh "${0:A:h}/../bin/spu" vscode -h 2>/dev/null)"
  __detailed="$(zsh "${0:A:h}/../bin/spu" vscode --help 2>/dev/null)"
  [[ "$__brief" == *"vscode — open"* ]]; __status=$?
  toBe "brief is a one-line summary" "$__status" "0"
  [[ "$__detailed" == *"Options:"* ]]; __status=$?
  toBe "detailed lists options" "$__status" "0"

finish
