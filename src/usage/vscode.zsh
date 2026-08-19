#!/usr/bin/env zsh

# Usage for the vscode util.

__usage_dir="${(%):-%N}"
__usage_dir="${__usage_dir:A:h}"

source "$__usage_dir/../utils/usage.zsh"

create_usager usage_vscode \
  "{{command}}vscode{{/}} — open VS Code configuration files quickly" \
  "Usage: spu vscode [OPTION]
Open VS Code configuration files quickly.

{{header}}Options:{{/}}
  {{option}}-S, --snippets{{/}}     Open VS Code snippets directory
  {{option}}-s, --settings{{/}}     Open VS Code settings.json
  {{option}}-k, --keys{{/}}         Open VS Code keybindings.json
  {{option}}-h, --help{{/}}         Show brief help (--help for details)

{{header}}Examples:{{/}}
  spu vscode --settings
  spu vscode -k"
