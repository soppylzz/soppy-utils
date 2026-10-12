#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../src/vscode.sh"

code() { _code_arg="$1"; }
HOME='/tmp/soppy-utils-test-home'
case "$OSTYPE" in darwin*) _dir="$HOME/Library/Application Support/Code/User" ;; linux*) _dir="$HOME/.config/Code/User" ;; msys* | cygwin*)
  APPDATA='C:\Users\Test\AppData\Roaming'
  _dir="$APPDATA/Code/User"
  if command -v cygpath >/dev/null 2>&1; then _dir="$(cygpath -u "$_dir")"; fi
  ;;
esac

describe 'VS Code path mapping'
vscode_main --settings
toBe 'settings path' "$_code_arg" "$_dir/settings.json"

vscode_main --keys
toBe 'keybindings path' "$_code_arg" "$_dir/keybindings.json"

vscode_main --snippets
toBe 'snippets path' "$_code_arg" "$_dir/snippets"

bash "$(dirname "${BASH_SOURCE[0]}")/../bin/spu" vscode --bogus >/dev/null 2>&1
toBe 'unknown option fails' "$?" 1

finish
