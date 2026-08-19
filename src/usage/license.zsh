#!/usr/bin/env zsh

# Usage for the license util.

__usage_dir="${(%):-%N}"
__usage_dir="${__usage_dir:A:h}"

source "$__usage_dir/../utils/usage.zsh"

create_usager usage_license \
  "{{command}}license{{/}} — generate a LICENSE text from a template" \
  "Usage: spu license [OPTION]
Generate a LICENSE text from a template.

{{header}}Options:{{/}}
  {{option}}-M, --mit{{/}}           Generate an MIT license
  {{option}}-I, --isc{{/}}           Generate an ISC license
  {{option}}-o, --output <file>{{/}} Write the license to <file> instead of stdout
  {{option}}-h, --help{{/}}          Show brief help (--help for details)

{{header}}Examples:{{/}}
  spu license --mit
  spu license --isc --output LICENSE"
