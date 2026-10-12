#!/usr/bin/env bash

vscode_main() {
  local target="${1:-}" user_dir path
  if [[ $# -gt 1 ]]; then
    printf '%s\n' 'vscode: too many arguments' >&2
    return 1
  fi
  case "$target" in
  -h | '')
    printf '%s\n' 'vscode: open VS Code configuration files'
    return 0
    ;;
  --help)
    printf '%s\n' 'Usage: spu vscode -S|--snippets|-s|--settings|-k|--keys'
    return 0
    ;;
  -S | --snippets | -s | --settings | -k | --keys) ;;
  *)
    printf 'vscode: unknown option: %s\n' "$target" >&2
    return 1
    ;;
  esac
  case "${OSTYPE:-}" in
  darwin*) user_dir="$HOME/Library/Application Support/Code/User" ;;
  linux*) user_dir="$HOME/.config/Code/User" ;;
  msys* | cygwin*)
    [[ -n "${APPDATA:-}" ]] || {
      printf '%s\n' 'vscode: APPDATA is required' >&2
      return 1
    }
    user_dir="$APPDATA/Code/User"
    if command -v cygpath >/dev/null 2>&1; then user_dir="$(cygpath -u "$user_dir")"; fi
    ;;
  *)
    printf 'vscode: unsupported os: %s\n' "${OSTYPE:-unknown}" >&2
    return 1
    ;;
  esac
  case "$target" in
  -S | --snippets) path="$user_dir/snippets" ;;
  -s | --settings) path="$user_dir/settings.json" ;;
  -k | --keys) path="$user_dir/keybindings.json" ;;
  esac
  command -v code >/dev/null 2>&1 || {
    printf '%s\n' 'vscode: code command is required' >&2
    return 1
  }
  code "$path"
}
