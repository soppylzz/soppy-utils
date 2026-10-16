# AGENTS.md

This file is the shared guidance for agents working in this repository. Harness-specific configuration belongs in `CLAUDE.md` or `.claude/`.

Read the current code and tests before changing behavior. Files in `.soppy/plans/` record requirements and earlier designs: each opens with a status note, `.soppy/README.md` indexes them, and designs for the retired Zsh implementation sit under `plans/archive/`. That directory is excluded from Git by the global exclude file, so plans exist only on the machine that wrote them. Preserve unrelated work already present in the tree, and commit only when the user requests it.

## Overview

`soppy-utils` exposes one `spu <util> [...args]` command. The public launcher, installer, updater, and VS Code integration use Bash 3.2. The `license`, `welcome`, and `git` utils use a precompiled Go helper; end users do not need Go. Release packages target Apple Silicon Macs, Ubuntu 24.04 LTS on amd64, and Windows 10/11 Git Bash on amd64.

## Language and format

Write code comments, commit messages, requirement documents, and the English README in English. Keep Markdown in GitHub Flavored Markdown. The existing Chinese README is the localization of user documentation.

## Layout

- `bin/spu`: thin Bash launcher.
- `src/spu.sh`, `src/vscode.sh`, `src/utils/style.sh`: Bash runtime.
- `cmd/spu-helper`: Go entry point for Go-owned utils and read-only config checks.
- `internal/clioutput`: shared Go CLI diagnostics on stderr.
- `internal/configstore`: per-util `VERSION`, JSON config, schema validation, and in-memory defaults.
- `internal/{license,welcome,gitconfig}`: util-specific CLI, forms, templates, rendering, and ownership rules.
- `internal/ui`: terminal checks and Huh form setup.
- `Taskfile.yml`: go-task entry point for development, build, and CI commands.
- `.githooks/pre-commit`: optional staged-snapshot quality gate.
- `scripts/`: installation, update, and uninstall scripts shipped in release archives.
- `scripts/dev/`: development, hook setup, test, version, and release packaging scripts excluded from release archives.
- `test/`: Bash `*.spec.sh` files and shared assertions in `test/general/`.
- `.soppy/plans/`: requirement and design documents. Not tracked; `**/.soppy/` is in the global Git exclude file, so this is absent from a clone.

The public Git command is `spu git`; `spu git-custom-ini` is not an alias. The managed Git include file keeps its previous path, `~/.config/soppy-utils/git-custom-ini.gitconfig`.

## Configuration contracts

Each Go util owns `~/.config/soppy-utils/<util>/{VERSION,config.json}`. `VERSION` is an integer configuration contract version, independent of the package release version. `config.json` has the same `schema_version`, a `util` name, and typed `data`. Schemas are embedded in their util package. Generated artifacts use `state.json` for hashes and provenance.

Ordinary commands warn once on a known older contract and use an in-memory default polyfill. They do not persist migration. `spu <util> migrate` or `edit` is explicit; `spu update` checks for mismatches without rewriting config or opening forms. Treat future versions, malformed files, and `VERSION`/JSON disagreements as read-only conflicts. Never overwrite a modified managed artifact. Build generated shell scripts from trusted embedded templates and quote user data as inert Bash words.

Welcome hooks run only from an interactive parent shell. Powerlevel10k instant prompt may temporarily redirect stdout while `.zshrc` loads, so a generated banner must recognize a controlling terminal even when stdout is not a TTY. Keep direct noninteractive and terminal-free execution silent. Increment the welcome template revision when generated script behavior changes; refresh an installed script through `spu welcome install`, not by editing managed output.

## Shell and Go rules

Use `#!/usr/bin/env bash` for all project-maintained shell scripts. Bash 3.2 is the floor; avoid Bash 4 associative arrays, namerefs, `mapfile`, and `globstar`. Use `${BASH_SOURCE[0]}` for script paths. In Windows Git Bash, pass `SPU_HOME_NATIVE` from `cygpath -w "$HOME"` before starting a native Go executable. Use Huh v2 for multi-field terminal interaction and keep one-shot CLI output clean on stdout.

For Go changes, keep file operations and migrations in the owning package or shared `internal/` helpers. Validate JSON with the pinned Draft 2020-12 validator and validate domain rules in Go. JSON Schema `default` annotations do not fill values; polyfills are explicit code. Use `os/exec` argument arrays for Git, not shell command strings.

## Lint and tests

Use go-task v3 as the development and CI entry point: run `task format`, `task lint`, `task test`, and `task smoke`. `task check` runs lint and tests; `task build` writes a native helper to `dist/`. The underlying `scripts/dev/` commands remain available for direct use and the staged-snapshot hook. Lint requires ShellCheck, shfmt, Go, and gofmt. The optional hook is installed with `task hooks:install` and checks the staged snapshot without modifying the index. Bash specs use `test/general/helpers.sh`; Go packages use `go test ./...`. Do not introduce Node.js, npm, zunit, or another shell test runner. Keep tooling and runner scripts under `scripts/dev/`, never `bin/`. Test generated scripts in noninteractive shells and keep real user home and global Git config untouched by tests.

CI runs native tests and archive smoke tests on three targets through Task: macOS 15 arm64, Ubuntu 24.04 amd64, and Windows Server 2025 amd64. Windows Server is only a CI proxy for the Windows 10/11 desktop release target. `task package VERSION=<version> OUT_DIR=<output-dir>` invokes `scripts/dev/package.sh` to build three release archives and `SHA256SUMS`; CD validates them before tagging. `scripts/dev/version.sh` uses Conventional Commits: breaking changes bump major, `feat` bumps minor, other conventional commits bump patch.

Keep the English and Chinese README examples aligned with actual CLI behavior. Verify public release assets before documenting an installer command as usable; a source checkout or cross-build is not proof that a binary release has been published.

## Git conventions

Use Conventional Commits: `type(scope)?: description`. Keep descriptions lowercase, imperative, and under about 72 characters. Supported types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, and `revert`. Mark breaking changes with `!` or a `BREAKING CHANGE:` footer.

## Requirement and design documents

Requirement and design documents live in `.soppy/plans/`, one file per topic. Each keeps a `## Demands` section stating the requirement and a `## Plans` checklist that is updated as steps finish.

The directory is outside version control, so a document is local to the machine that wrote it: it has no history, does not appear in review, and is missing from a clone. Anything that must outlive this machine or reach another person belongs in `AGENTS.md`, the README files, or the code itself.

There is no skills-driven requirement workflow. Guidance shared by all agents belongs here; keep `CLAUDE.md` as a thin shim and `.claude/settings.json` for harness settings only.
