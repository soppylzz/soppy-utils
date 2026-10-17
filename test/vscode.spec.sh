#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE[0]}")/general/helpers.sh"

spu="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/bin/spu"
work="$(mktemp -d)"
trap 'rm -r -- "$work"' EXIT
mkdir -p "$work/bin" "$work/home"

# A stand-in for the code command. The client is the one that runs it now, so it has to
# be a real program on PATH: a shell function would not be visible to it.
cat >"$work/bin/code" <<STUB
#!/usr/bin/env bash
printf '%s\n' "\$1" >"$work/opened"
STUB
chmod 755 "$work/bin/code"
if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  # Go finds a bare name through PATHEXT on Windows, so a file called code is not enough.
  printf '@bash "%%~dp0code" %%*\r\n' >"$work/bin/code.cmd"
fi

case "${OSTYPE:-}" in
darwin*) dir="$work/home/Library/Application Support/Code/User" ;;
linux*) dir="$work/home/.config/Code/User" ;;
msys* | cygwin*)
  APPDATA='C:\Users\Test\AppData\Roaming'
  dir="$APPDATA/Code/User"
  if command -v cygpath >/dev/null 2>&1; then dir="$(cygpath -u "$dir")"; fi
  ;;
esac

open() {
  rm -f "$work/opened"
  HOME="$work/home" PATH="$work/bin:$PATH" APPDATA="${APPDATA:-}" bash "$spu" vscode "$@" >/dev/null 2>&1
  cat "$work/opened" 2>/dev/null
}

describe 'VS Code path mapping'
toBe 'settings path' "$(open --settings)" "$dir/settings.json"
toBe 'keybindings path' "$(open --keys)" "$dir/keybindings.json"
toBe 'snippets path' "$(open --snippets)" "$dir/snippets"

bash "$spu" vscode --bogus >/dev/null 2>&1
toBe 'unknown option fails' "$?" 1

finish
