#!/usr/bin/env zsh

# vscode — open a VS Code configuration target with the `code` command.

__vscode_dir="${(%):-%N}"
__vscode_dir="${__vscode_dir:A:h}"

source "$__vscode_dir/usage/vscode.zsh"

# vscode <target>
#
# The low-level function takes a single target name and opens the mapped VS Code
# user path with `code`. The user directory is resolved per OS:
#   snippets — the snippets directory
#   settings — settings.json
#   keys     — keybindings.json
vscode() {
  local target="$1"
  local vscode_user_dir

  case "$OSTYPE" in
    darwin*)
      vscode_user_dir="$HOME/Library/Application Support/Code/User"
      ;;
    linux*)
      vscode_user_dir="$HOME/.config/Code/User"
      ;;
    *)
      err "unsupported os: $OSTYPE"
      return 1
      ;;
  esac

  case "$target" in
    snippets) code "$vscode_user_dir/snippets" ;;
    settings) code "$vscode_user_dir/settings.json" ;;
    keys)     code "$vscode_user_dir/keybindings.json" ;;
    *)
      err "unknown target: $target"
      return 1
      ;;
  esac
}

# vscode_main [...args]
#
# CLI entry point: maps flags to target names and dispatches to `vscode`.
vscode_main() {
  case "${1:-}" in
    --snippets|-S) vscode snippets ;;
    --settings|-s) vscode settings ;;
    --keys|-k)     vscode keys     ;;
    --help)        usage_vscode true ;;
    -h|"")         usage_vscode false ;;
    *)
      err "vscode: unknown option: $1"
      usage_vscode true >&2
      return 1
      ;;
  esac
}
