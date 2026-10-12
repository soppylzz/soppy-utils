#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../src/utils/style.sh"

spu="$(dirname "${BASH_SOURCE[0]}")/../bin/spu"

describe 'plain output through pipes'
output="$(style_rich '{{header}}Options:{{/}}' 1)"
toBe 'rich markup is stripped' "$output" 'Options:'

output="$(bash "$spu" license --help 2>&1)"
case "$output" in *$'\033['*) status=1 ;; *) status=0 ;; esac
toBe 'license help has no ANSI' "$status" 0

output="$(bash "$spu" bogus 2>&1)"
case "$output" in *$'\033['*) status=1 ;; *) status=0 ;; esac
toBe 'errors have no ANSI' "$status" 0

finish
