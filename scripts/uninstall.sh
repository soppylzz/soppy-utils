#!/usr/bin/env zsh

# uninstall.sh — remove the soppy-utils installation.
#
# Removes the ~/.local/bin/spu symlink and the installed project directory.
# Only touches the paths install.sh created; unrelated files are left alone.

setopt LOCAL_OPTIONS NO_MONITOR

__install_bin="$HOME/.local/bin"
__install_target="$HOME/.local/share/soppy-utils"
__install_link="$__install_bin/spu"

# Remove the symlink only if it points at the install target.
if [[ -L "$__install_link" ]]; then
  if [[ "$(readlink "$__install_link")" == "$__install_target/bin/spu" ]]; then
    print -r -- "soppy-utils: removing $__install_link"
    rm -f "$__install_link"
  else
    print -r -- "soppy-utils: $__install_link points elsewhere; leaving it" >&2
  fi
fi

# Remove the installed project directory.
if [[ -d "$__install_target" ]]; then
  print -r -- "soppy-utils: removing $__install_target"
  rm -rf "$__install_target"
else
  print -r -- "soppy-utils: nothing to remove at $__install_target" >&2
fi
