#!/usr/bin/env zsh

# Spec for the license util.

source "${0:A:h}/general/helpers.zsh"

# Mock `git` and `date` before sourcing license.sh so the author and year are
# deterministic and independent of the real environment.
git() { print -r -- "Test User"; }
date() { print -r -- "2026"; }

source "${0:A:h}/../src/license.sh"

describe "license: MIT rendering"

  __out="$(license mit)"
  __first="${__out%%$'\n'*}"
  toBe "MIT starts with the copyright line" "$__first" "Copyright (c) 2026 Test User"
  [[ "$__out" == *"Permission is hereby granted"* ]]; __status=$?
  toBe "MIT contains the permission text" "$__status" "0"

describe "license: ISC rendering"

  __out="$(license isc)"
  __first="${__out%%$'\n'*}"
  toBe "ISC starts with the copyright line" "$__first" "Copyright (c) 2026 Test User"
  [[ "$__out" == *"Permission to use, copy, modify"* ]]; __status=$?
  toBe "ISC contains the permission text" "$__status" "0"

describe "license: unknown type"

  license bogus 2>/dev/null
  __status=$?
  toBe "unknown type returns non-zero" "$__status" "1"

describe "license: git missing falls back to USER"

  __out="$(
    unset __license_author
    unfunction git 2>/dev/null
    PATH=/nonexistent
    hash -r
    USER=fallbackuser
    source "${0:A:h}/../src/license.sh"
    license mit
  )"
  [[ "$__out" == *"fallbackuser"* ]]; __status=$?
  toBe "falls back to USER when git is missing" "$__status" "0"

describe "license: usage via spu"

  zsh "${0:A:h}/../bin/spu" license --help >/dev/null 2>&1
  __status=$?
  toBe "--help exits zero" "$__status" "0"

  zsh "${0:A:h}/../bin/spu" license -h >/dev/null 2>&1
  __status=$?
  toBe "-h exits zero" "$__status" "0"

  zsh "${0:A:h}/../bin/spu" license --bogus >/dev/null 2>&1
  __status=$?
  toBe "unknown option exits non-zero" "$__status" "1"

describe "license: brief vs detailed"

  __brief="$(zsh "${0:A:h}/../bin/spu" license -h 2>/dev/null)"
  __detailed="$(zsh "${0:A:h}/../bin/spu" license --help 2>/dev/null)"
  [[ "$__brief" == *"license — generate"* ]]; __status=$?
  toBe "brief is a one-line summary" "$__status" "0"
  [[ "$__detailed" == *"Options:"* ]]; __status=$?
  toBe "detailed lists options" "$__status" "0"

describe "license wrapper: output file"

  __tmp="$(mktemp -d)"
  zsh "${0:A:h}/../bin/spu" license --mit --output "$__tmp/LICENSE" >/dev/null 2>&1
  __status=$?
  toBe "--output exits zero" "$__status" "0"
  [[ -f "$__tmp/LICENSE" ]]; __status=$?
  toBe "--output writes the file" "$__status" "0"
  rm -rf "$__tmp"

describe "spu: overview and unknown util"

  __overview="$(zsh "${0:A:h}/../bin/spu" 2>/dev/null)"
  [[ "$__overview" == *"Usage: spu <util>"* ]]; __status=$?
  toBe "overview shows the spu usage line" "$__status" "0"
  [[ "$__overview" == *"vscode"* && "$__overview" == *"license"* ]]; __status=$?
  toBe "overview lists both utils" "$__status" "0"

  zsh "${0:A:h}/../bin/spu" bogus >/dev/null 2>&1
  __status=$?
  toBe "unknown util exits non-zero" "$__status" "1"

finish
