#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"

spu="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/bin/spu"
work="$(mktemp -d)"
trap 'rm -r -- "$work"' EXIT
home="$work/home"
dir="$home/.config/soppy-utils/welcome"
mkdir -p "$dir"

# A configuration from the contract before this one. welcome's is 2 now, so a 1 is a
# known older shape: readable, reported once, and not migrated on its own.
printf '1\n' >"$dir/VERSION"
printf '%s\n' '{"util":"welcome","schema_version":1,"data":{"name":"Alice","device":"Mac","message":"Ready","rc":"none"}}' >"$dir/config.json"
before="$(cat "$dir/config.json")"
warning="warning: welcome: config contract 1; required 2; run 'spu welcome migrate'"

# What spu update runs after the swap. It reports and exits zero: a package upgrade that
# found a configuration to migrate is a successful upgrade.
describe 'the read-only check after an upgrade'
HOME="$home" bash "$spu" config-status 2>"$work/err" >"$work/out"
toBe 'it reports the mismatch' "$(cat "$work/err")" "$warning"
toBe 'it exits zero' "$?" 0
toBe 'it writes nothing to stdout' "$(cat "$work/out")" ''
toBe 'and it rewrites nothing' "$(cat "$dir/config.json")" "$before"
toBe 'and it does not persist the migration' "$(cat "$dir/VERSION")" '1'

describe 'an ordinary command on an older contract'
HOME="$home" bash "$spu" welcome status 2>"$work/err" >"$work/out"
toBe 'it exits zero' "$?" 0
toBe 'it warns exactly once' "$(cat "$work/err")" "$warning"
toBe 'and it keeps stdout clean' "$(grep -c 'config contract' "$work/out")" '1'
toBe 'and still rewrites nothing' "$(cat "$dir/config.json")" "$before"

describe 'an explicit migration'
HOME="$home" bash "$spu" welcome migrate --name Bob --device Host --message Hi --rc none >/dev/null 2>&1
toBe 'it succeeds' "$?" 0
toBe 'and persists the new contract' "$(cat "$dir/VERSION")" '2'
HOME="$home" bash "$spu" config-status 2>"$work/err" >/dev/null
toBe 'and there is nothing left to report' "$(cat "$work/err")" ''

finish
