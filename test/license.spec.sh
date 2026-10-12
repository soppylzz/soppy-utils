#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"

spu="$(dirname "${BASH_SOURCE[0]}")/../bin/spu"

describe 'one-shot license output'
output="$(bash "$spu" license --mit --name Alice --year 2026)"
toBe 'MIT header' "${output%%$'\n'*}" 'Copyright (c) 2026 Alice'

output="$(bash "$spu" license --isc --name Bob --year 2025)"
toBe 'ISC header' "${output%%$'\n'*}" 'Copyright (c) 2025 Bob'

bash "$spu" license --mit --isc >/dev/null 2>&1
toBe 'conflicting license flags fail' "$?" 1

finish
