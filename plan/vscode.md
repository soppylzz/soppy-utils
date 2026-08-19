# vscode

## Demands

Provide a `vscode` util — invoked as `spu vscode` — that opens VS Code configuration files and directories with one command.

**Options**

- `-S`, `--snippets` — open the VS Code snippets directory.
- `-s`, `--settings` — open VS Code `settings.json`.
- `-k`, `--keys` — open VS Code `keybindings.json`.

**Behavior**

- Resolve the VS Code user directory per OS. macOS: `$HOME/Library/Application Support/Code/User`; other OSes unsupported for now.
- Map each target name to its path and open it with the `code` command.
- Unknown target prints usage and exits non-zero.

**Interface**

- The function takes a single positional **target name** — `snippets`, `settings`, or `keys` — not the CLI flags.
- Flag-to-target mapping (`-S`/`--snippets` → `snippets`, etc.) happens at the dispatch layer when the tool is invoked, not inside the function.

**Architecture**

- Exposed as a single function in `src/`; a `bin/` wrapper maps CLI flags to the target name and calls the function.
- Install script is out of scope for this demand.

## Plans

- [x] implementVscodeFunction — add a `vscode` function to `src/vscode.sh` that takes a single target name (`snippets` | `settings` | `keys`), resolves the macOS VS Code user dir, and opens the mapped path via `code`.
- [x] createBinWrapper — add `bin/vscode`, a thin executable that maps `-S`/`--snippets` → `snippets`, `-s`/`--settings` → `settings`, `-k`/`--keys` → `keys`, sources `src/vscode.sh`, and calls the function.
- [x] testVscodeUtil — write a spec `test/vscode.spec.sh` that sources `test/general/helpers.zsh` and covers each target mapping and the unknown-target/usage path.
