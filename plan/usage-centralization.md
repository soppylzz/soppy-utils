# usageCentralization

## Demands

Consolidate the `vscode` and `license` tools behind a single `spu` entry point, and move all usage/help text into a centralized `src/usage/` directory with a standardized, reusable usage-builder pattern.

**Current state**

- `bin/` holds three files: `spu` (an empty stub) plus `bin/vscode` and `bin/license`, each a self-contained wrapper with inline `usage()` heredocs and flag parsing.
- `src/vscode.sh` exposes `vscode <target>`; `src/license.sh` exposes `license <type>` (sourcing `src/utils/template.zsh`).
- `test/vscode.spec.sh` and `test/license.spec.sh` invoke `bin/vscode` and `bin/license` directly.

**Target state**

- `bin/` contains only `spu`, the single program entry point: `spu <util> [...args]`.
- Usage text lives under `src/usage/`, one file per tool plus an overview. Each usage file defines **two output modes** via a single `usage` function that accepts a `verbose` boolean parameter:
  - `verbose=true` — detailed help text (invoked by `spu <util> --help`)
  - `verbose=false` — brief summary text (invoked by `spu <util> -h`, and also called by the overview)
- A `create_usager` factory function is defined to **return** the `usage` function, ensuring a consistent, DRY pattern for all future usage definitions.

**Usage layout**

- `src/usage/spu.zsh` — `usage_spu`: top-level overview listing the available utils (`vscode`, `license`) and the `Usage: spu <util> [...args]` line. The overview calls each tool's brief-mode usage (via `verbose=false`) to compose its output.
- `src/usage/vscode.zsh` — `usage_vscode`: created via `create_usager`, emitting:
  - *brief*: one-line summary of the vscode util
  - *detailed*: full options list (`-S`/`--snippets`, `-s`/`--settings`, `-k`/`--keys`, `-h`/`--help`)
- `src/usage/license.zsh` — `usage_license`: created via `create_usager`, emitting:
  - *brief*: one-line summary of the license util
  - *detailed*: full options list (`-M`/`--mit`, `-I`/`--isc`, `-o`/`--output <file>`, `-h`/`--help`)

**Dispatch**

- `bin/spu` stays thin: source `src/spu.zsh` and call its dispatcher with `"$@"`.
- `src/spu.zsh` defines the dispatcher that routes the first argument:
  - empty, `help`, `-h`, `--help` → `usage_spu` (with appropriate verbose mode per caller context)
  - `vscode` → forward remaining args to the vscode entry function
  - `license` → forward remaining args to the license entry function
  - unknown util → error message + `usage_spu` (brief), exit non-zero
- Per-tool flag parsing moves out of the removed wrappers into per-tool entry functions in `src/` (`vscode_main` / `license_main`), keeping `bin/spu` thin and all logic under `src/`. Each tool's `src/<tool>.sh` sources its own `src/usage/<tool>.zsh`.
- **Multi-semantic flag handling** (e.g. `-S|--snippets` which carries additional meaning beyond a simple toggle) remains implemented in per-tool flag parsing, **not** in the usage builder.

**Behavior preserved**

- `spu vscode -S|-s|-k` and `spu license -M|-I|-o <file>` keep their current flag semantics.
- `spu <util> --help` → detailed usage (exit zero).
- `spu <util> -h` → brief usage (exit zero).
- Unknown option to a tool → that tool's detailed usage printed to stderr, exit non-zero.

**Tests**

- `test/vscode.spec.sh` and `test/license.spec.sh` are updated to invoke `bin/spu vscode …` / `bin/spu license …` instead of the removed wrappers.
- Add coverage for:
  - Overview usage (`spu`, `spu --help`) — including brief-mode output from each tool
  - The unknown-util path
  - `-h` (brief) vs `--help` (detailed) per tool

**Open points**

- `spu vscode` with no args prints usage and exits zero today; `spu license` with no args prints usage to stderr and exits one. Propose normalizing both to print the tool's **brief** usage and exit zero, matching `spu`'s no-args overview.
- Exact entry-function names (`vscode_main` / `license_main`) are an implementation detail; the requirement is one CLI entry function per tool in `src/`.

**Documentation**

- Document the usage construction flow — including the `create_usager` pattern, `verbose` parameter convention, and file layout — in `CLAUDE.md` so that all future tool additions follow the same consistent process.

**Out of scope**

- Install/uninstall scripts and their PATH wiring.

## Plans

- [x] implementUsagerFactory — add a shared `create_usager` factory under `src/utils/usage.zsh` that returns a `usage` function accepting a `verbose` boolean and emitting brief vs detailed text.
- [x] addUsageOverview — create `src/usage/spu.zsh` defining `usage_spu`, the overview that lists available utils and composes each tool's brief-mode output via `verbose=false`.
- [x] addUsageVscode — create `src/usage/vscode.zsh` defining `usage_vscode` via `create_usager` (brief one-liner + detailed options list for `-S`/`--snippets`, `-s`/`--settings`, `-k`/`--keys`, `-h`/`--help`).
- [x] addUsageLicense — create `src/usage/license.zsh` defining `usage_license` via `create_usager` (brief one-liner + detailed options list for `-M`/`--mit`, `-I`/`--isc`, `-o`/`--output <file>`, `-h`/`--help`).
- [x] implementSpuDispatcher — add `src/spu.zsh` with the dispatcher: empty/`help`/`-h`/`--help` → `usage_spu` (verbose per context), `vscode`/`license` → forward remaining args to their entry functions, unknown util → error + brief `usage_spu`, exit non-zero.
- [x] moveVscodeFlagParsing — move vscode flag parsing out of the removed wrapper into a `vscode_main` entry function in `src/vscode.sh`, sourcing `src/usage/vscode.zsh`; preserve `-S|-s|-k`, `--help` (detailed) and `-h` (brief).
- [x] moveLicenseFlagParsing — move license flag parsing into a `license_main` entry function in `src/license.sh`, sourcing `src/usage/license.zsh`; preserve `-M|-I|-o <file>`, `--help` (detailed) and `-h` (brief).
- [x] collapseBinToSpu — rewrite `bin/spu` as the sole thin entry point (source `src/spu.zsh`, dispatch `"$@"`) and remove `bin/vscode` and `bin/license`.
- [x] updateSpecs — update `test/vscode.spec.sh` and `test/license.spec.sh` to invoke `bin/spu …`; add coverage for the overview (`spu`, `spu --help`), the unknown-util path, and `-h` (brief) vs `--help` (detailed) per tool.
- [x] documentUsageFlow — document the usage construction flow (`create_usager` pattern, `verbose` convention, `src/usage/` layout) in `CLAUDE.md`.