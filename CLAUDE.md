# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

An engineering-focused collection of zsh utilities for automating common development tasks. Each tool is a zsh script; tools are installed onto `PATH` via the install script so they can be invoked as commands.

## Language & format

All project text — code comments, git commit messages, and requirement documents — is written in English. Markdown follows GitHub Flavored Markdown (GFM).

## Directory structure

```
.
├── bin/            # Entry points (thin dispatchers)
├── src/            # Tool implementations and shared helpers
│   ├── usage/      # Help text, one file per tool + overview
│   ├── utils/      # Shared helpers
│   └── template/   # LICENSE templates (data, not logic)
├── scripts/        # Install/uninstall and development helpers
├── test/           # zsh test suite
│   └── general/    # Shared test primitives
└── plan/           # Requirement documents, one per demand
```

`bin/` holds only the thin `spu` launcher; all logic lives under `src/`, where each tool is one implementation plus a shared helper set. Help text is centralized in `src/usage/` and built via the `create_usager` factory, reusable helpers live in `src/utils/`, and LICENSE data lives in `src/template/`. `scripts/` contains the install/uninstall and development scripts, `test/` the zsh specs and shared primitives, and `plan/` the requirement documents (one per demand).


## Invocation convention

Tools in this collection are invoked using the `spu` (zsh Utility) launcher:

```sh
spu <util-name> [...args]
```

- `spu` is the single entry point installed onto `PATH`; it dispatches to the requested util by name.
- `<util-name>` is the name of the tool (matching its `bin/` wrapper and `src/` implementation).
- Remaining `[...args]` are forwarded to that tool as its positional arguments.

## Usage construction

Help/usage text is centralized under `src/usage/`, one file per tool plus an overview. Each usage file defines a `usage_<name>` function with two output modes selected by a `verbose` boolean argument:

- `verbose=true` — detailed help (invoked by `spu <util> --help`).
- `verbose=false` — brief one-line summary (invoked by `spu <util> -h`, and called by the overview).

All `usage_<name>` functions are created with the shared `create_usager <name> <brief> <detailed>` factory from `src/utils/usage.zsh`, which stores the text in global associative arrays (`__usage_brief` / `__usage_detailed`) and generates a lookup function. Keep the two text strings out of the generated function body so no shell-quoting is needed. The stored text may carry `{{token}}…{{/}}` styling markup (see "Terminal styling" below); the factory renders it through the shared `style_rich` helper, which drops the markup when color is disabled.

Layout:

- `src/usage/spu.zsh` — `usage_spu`, the overview. It composes each tool's brief output (`usage_<tool> false`) so descriptions have a single source of truth.
- `src/usage/<tool>.zsh` — `usage_<tool>`, created via `create_usager`.

Per-tool CLI entry functions (`<tool>_main` in `src/<tool>.sh`) source their own `src/usage/<tool>.zsh`, map flags, and call `usage_<tool> true`/`false` for help requests. Multi-semantic flag handling (flags that carry meaning beyond a simple toggle) lives in the per-tool flag parsing, not in the usage builder.

To add a new util: create `src/<tool>.sh` (a `<tool>_main` entry function), create `src/usage/<tool>.zsh` via `create_usager`, source it from `src/spu.zsh`'s dispatcher, and add the tool name to the dispatcher's routing `case`.

## Terminal styling

Colored terminal output is provided by `src/utils/style.zsh`, built on `zsh/zutil` + `zstyle`. Colors are configured via the `:spu:style:` namespace — a caller overrides a token with `zstyle ':spu:style:<token>' sgr <codes>`.

API:

- `style <token> <text> [fd]` — print `<text>` styled as `<token>`; defaults to stderr, pass `1` for stdout.
- `err` / `warn` / `info` — one-argument wrappers around `style` for the `error` / `warn` / `info` tokens on stderr.
- `style_rich <text> [fd]` — print `<text>` with inline `{{token}}…{{/}}` spans styled as `token`.

Semantic tokens: `error`, `warn`, `info`, `header`, `command`, `option`, `arg`, `hint`, `pass`, `fail`.

Color is emitted only when the target fd is a TTY, `TERM` is not `dumb`/empty, and `NO_COLOR` is unset; otherwise text is printed plain and `style_rich` strips its markup. `spu <util> --help`/`-h` and error messages route through this helper; the `license` util's rendered LICENSE body stays plain (no styling) so the emitted text remains valid and portable.

## Git conventions

### Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/) — `type(scope)?: description`. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

- Description is lowercase, imperative, and kept under ~72 characters.
- Breaking changes use a `!` after the type or a `BREAKING CHANGE:` footer.

## Lint & Testing

This is a **zsh project, not an npm/Node.js project**. Do not assume npm-based tooling or external test runners.

### Linting

The project uses **zsh-lint**, a standalone Go-based semantic analyzer.

- Repository: https://github.com/z-shell/zsh-lint
- `zsh-lint <file>` runs the default static-analysis rules and reports greppable diagnostics.
- `zsh-lint-survey` reports parser gaps only; it does not evaluate lint rules.
- Installation and setup for `zsh-lint` must live under `scripts/`, **never under `bin/`**.

### Testing

The project does **not** use zunit or any external test runner.

Instead, it provides a small, general-purpose unit-testing framework implemented directly in zsh. Tests are executed with the standard `zsh` interpreter.

#### Test Framework

Shared testing primitives live under:

    test/general/

This directory contains reusable helpers such as:

- `describe`
- `toBe`
- Other general test and assertion helpers

Each primitive is implemented as a zsh function and can be sourced by spec files.

#### Spec Files

Test specs:

- Must live under `test/` or one of its subdirectories.
- Must use the `*.spec.sh` naming convention.

Examples:

    test/my-tool.spec.sh
    test/utils/foo.spec.sh

A spec should source the shared test framework and then use its primitives:

    #!/usr/bin/env zsh

    source "${0:A:h}/general/helpers.zsh"

    describe "my util: basic behavior"
      toBe "returns expected value" my_util "expected"

Adjust the framework path as necessary when the spec is located in a subdirectory.

#### Running Tests

Run a spec directly with the standard zsh interpreter:

    zsh test/example.spec.sh

Do **not** introduce zunit or another external test runner.

A project-level runner may be added under `scripts/` in the future to discover and execute all `*.spec.sh` files.

### Rules for Changes

When adding or modifying linting or testing infrastructure:

1. Prefer existing zsh-native tooling and project conventions.
2. Do not introduce npm, Node.js, zunit, or other external test-runner dependencies.
3. Keep shared test primitives under `test/general/`.
4. Keep test specs under `test/` and follow the `*.spec.sh` naming convention.
5. Put installation, setup, and runner scripts under `scripts/`.
6. Keep `bin/` focused on project entry points.
7. Never put `zsh-lint` installation or setup logic under `bin/`.

## Vibe-coding workflow

The requirement workflow is implemented as skills, not inline here:

- `/v-demand <demand-name> [demand-description]` — create `plan/<demand-name>.md` and write the `## Demands` analysis.
- `/v-plan <demand-name>/<step-of-plan> [change description]` — edit a step of the demand's `## Plans` checklist.
- `/v-patch <demand-name>/<patch-description>` — append a change record to the demand's `## Patches`.

Each skill maintains the `## Plans` checkbox state in `plan/*.md` — flip the matching `- [ ]` to `- [x]` in real time as each step completes. See `.claude/skills/<name>/SKILL.md` for the exact file format and behavior.
