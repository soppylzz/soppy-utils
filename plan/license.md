# license

## Demands

Provide a `license` util — invoked as `spu license` — that generates a LICENSE file from a template, filling in dynamic author/date content.

**Options**

- `-M`, `--mit` — generate an MIT license.
- `-I`, `--isc` — generate an ISC license.
- `-o`, `--output <file>` — write the rendered license to `<file>` instead of stdout (default is stdout).
- `-h`, `--help` — show usage.

**Dynamic content**

- Author name from `git config user.name`; year from the `date` tool.
- Collect these into a readonly associative array declared with `typeset -rA` (e.g. keys `name` and `year`) so the template filler can look up values by placeholder name.
- Handle a missing `git` binary (or a git config with no `user.name`): fall back to `$USER` rather than failing. `date` is a POSIX core utility and is assumed always present.

**Templates**

- Templates live under `src/template/`: `mit.template` and `isc.template`.
- Each template contains placeholder tokens (e.g. `{{name}}`, `{{year}}`) that the filler substitutes with the dynamic values.

**Template-filling helper**

- Abstract the substitution logic into a reusable function under `src/utils` (shared helper), separate from the `license` tool itself, so future template-based tools can reuse it.

**Interface**

- Expose a single function — mirroring the `vscode` util — that the `bin/` wrapper dispatches to.
- The function maps a **license type name** (`mit` | `isc`) to its template and renders it; the CLI flag mapping (`-M`/`--mit` → `mit`, `-I`/`--isc` → `isc`) and output routing (`-o`/`--output` vs stdout) happen at the dispatch layer, not inside the function.

**Architecture**

- One function in `src/`; a `bin/license` wrapper maps flags to the type name, collects the optional output path, and calls the function.
- A shared filler in `src/utils` handles placeholder substitution for all template-based tools.
- Templates in `src/template/` are data, not logic.

**Output**

- Default: render to stdout.
- `-o`/`--output <file>`: write the rendered license to `<file>`.

## Plans

- [x] addTemplates — create `src/template/mit.template` and `src/template/isc.template` with `{{name}}`/`{{year}}` placeholder tokens.
- [x] implementFillerUtil — add a shared `fill_template` helper under `src/utils` that substitutes `{{key}}` tokens from a readonly associative array.
- [x] implementLicenseFunction — add a single `license` function to `src/license.sh` that takes a type name (`mit` | `isc`), builds the `typeset -rA` value map (author from `git config user.name` with `$USER` fallback, year from `date`), and renders the template via the filler.
- [x] createBinWrapper — add `bin/license`, a thin executable that maps `-M`/`--mit` → `mit` and `-I`/`--isc` → `isc`, supports `-o`/`--output <file>` (default stdout) and `-h`/`--help`, sources `src/license.sh`, and calls the function.
- [x] testLicenseUtil — write `test/license.spec.sh` that sources `test/general/helpers.zsh` and covers MIT/ISC rendering, the git-missing `$USER` fallback, and the unknown-option/usage path.
