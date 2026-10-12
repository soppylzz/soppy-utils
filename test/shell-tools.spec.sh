#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"

spu="$(dirname "${BASH_SOURCE[0]}")/../bin/spu"
# Go's defaults are derived from HOME, which the test replaces below.
GOPATH="$(go env GOPATH)"
GOCACHE="$(go env GOCACHE)"
GOMODCACHE="$(go env GOMODCACHE)"
export GOPATH GOCACHE GOMODCACHE
_tmp="$(mktemp -d)"
trap 'rm -r -- "$_tmp"' EXIT

describe 'generated banner and shell hooks'
HOME="$_tmp" bash "$spu" welcome install --name Alice --device Mac --message Ready --rc both >/dev/null 2>&1
status=$?
toBe 'install succeeds' "$status" 0

output="$(HOME="$_tmp" bash "$_tmp/.config/soppy-utils/welcome/welcome.sh")"
toBe 'direct noninteractive invocation is quiet' "$output" ''

output="$(HOME="$_tmp" bash -c 'source "$HOME/.bashrc"')"
toBe 'noninteractive Bash startup is quiet' "$output" ''

if command -v zsh >/dev/null 2>&1; then
  output="$(HOME="$_tmp" zsh -c 'source "$HOME/.zshrc"')"
  toBe 'noninteractive Zsh startup is quiet' "$output" ''
fi

HOME="$_tmp" bash "$spu" welcome uninstall >/dev/null 2>&1
status=$?
toBe 'uninstall succeeds' "$status" 0

finish
