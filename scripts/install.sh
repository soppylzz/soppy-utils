#!/usr/bin/env zsh

# install.sh — install the soppy-utils collection into $HOME/.local.
#
# Three execution modes, auto-detected:
#   A. Local clone  — this script is a file inside a checked-out project.
#                     Copy the project into the install target, symlink
#                     bin/spu into ~/.local/bin, then remove the source clone.
#   B. curl + git   — this script is standalone and git is available.
#                     Silently git-clone the repo into the install target.
#   C. curl only    — this script is standalone and git is missing.
#                     Download a tarball from GitHub and extract it.

setopt LOCAL_OPTIONS NO_MONITOR

# Install layout.
__install_bin="$HOME/.local/bin"
__install_target="$HOME/.local/share/soppy-utils"
__install_link="$__install_bin/spu"

# Remote source (used by modes B and C).
__repo_url="https://github.com/soppylzz/soppy-utils"
__repo_branch="main"

# Resolve this script's own location.
__script_path="${(%):-%x}"
__script_path="${__script_path:A}"
__script_dir="${__script_path:h}"

# A local clone has bin/spu one level above this script's directory.
__clone_root="${__script_dir:h}"
__is_local_clone=false
[[ -f "$__clone_root/bin/spu" ]] && __is_local_clone=true

# Skip when already installed.
if [[ -e "$__install_target" ]]; then
  print -r -- "soppy-utils: already installed at $__install_target" >&2
  exit 0
fi
if [[ -e "$__install_link" || -L "$__install_link" ]]; then
  print -r -- "soppy-utils: $__install_link already exists" >&2
  exit 0
fi

mkdir -p "$__install_bin"

if [[ $__is_local_clone == true ]]; then
  # Mode A — copy the checked-out project.
  print -r -- "soppy-utils: installing from local clone $__clone_root"
  mkdir -p "$__install_target"
  cp -R "$__clone_root"/. "$__install_target"/ || {
    print -r -- "soppy-utils: copy failed" >&2
    exit 1
  }
elif command -v git >/dev/null 2>&1; then
  # Mode B — silent clone.
  print -r -- "soppy-utils: cloning $__repo_url"
  git clone --quiet "$__repo_url" "$__install_target" || {
    print -r -- "soppy-utils: git clone failed" >&2
    exit 1
  }
else
  # Mode C — tarball download.
  print -r -- "soppy-utils: downloading $__repo_url (no git)"
  __tmp_dir="$(mktemp -d)"
  __tar="$__tmp_dir/soppy-utils.tar.gz"
  __tarball_url="$__repo_url/archive/refs/heads/$__repo_branch.tar.gz"
  curl -fsSL "$__tarball_url" -o "$__tar" || {
    print -r -- "soppy-utils: download failed" >&2
    rm -rf "$__tmp_dir"
    exit 1
  }
  tar -xzf "$__tar" -C "$__tmp_dir" || {
    print -r -- "soppy-utils: extraction failed" >&2
    rm -rf "$__tmp_dir"
    exit 1
  }
  mv "$__tmp_dir/soppy-utils-$__repo_branch" "$__install_target"
  rm -rf "$__tmp_dir"
fi

# Symlink the launcher.
ln -s "$__install_target/bin/spu" "$__install_link" || {
  print -r -- "soppy-utils: failed to create $__install_link" >&2
  exit 1
}

# Verify ~/.local/bin is on $PATH; hint otherwise.
if [[ ":$PATH:" != *":$__install_bin:"* ]]; then
  print -r -- "soppy-utils: installed, but $__install_bin is not on your PATH."
  print -r -- "Add this line to your ~/.zshrc:"
  print -r -- "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Mode A — remove the source clone, unless it is already the install target.
if [[ $__is_local_clone == true && "$__clone_root" != "$__install_target" ]]; then
  print -r -- "soppy-utils: removing source clone $__clone_root"
  rm -rf "$__clone_root"
fi
