# AGENTS.md

This file is the shared guidance for agents working in this repository. Harness-specific configuration belongs in `CLAUDE.md` or `.claude/`.

Read the current code and tests before changing behavior. Files in `.soppy/plans/` record requirements and earlier designs: each opens with a status note and `.soppy/README.md` indexes them. That directory is excluded from Git by the global exclude file, so plans exist only on the machine that wrote them; it holds the current design and not the retired ones, which are in Git history under `plan/`. Preserve unrelated work already present in the tree, and commit only when the user requests it.

## Overview

`soppy-utils` exposes one `spu <util> [...args]` command. The public launcher, installer, and updater use Bash 3.2. Every util runs in a Go core that the command starts on demand and talks to over a socket; end users do not need Go. Release packages target Apple Silicon Macs, Ubuntu 24.04 LTS on amd64, and Windows 10/11 Git Bash on amd64.

## Language and format

Write code comments, commit messages, requirement documents, and the English README in English. Keep Markdown in GitHub Flavored Markdown. The existing Chinese README is the localization of user documentation.

## Layout

- `bin/spu`: thin Bash launcher.
- `src/spu.sh`, `src/utils/style.sh`: the launcher's dispatcher and its presentation helpers.
- `cmd/spu`: the one executable. It is a client, and the core daemon it starts when it cannot reach one.
- `cmd/spu-clone-third-party`: a second binary, an MCP server over stdio. It is deliberately not a mode of `spu`: it needs no daemon, speaks somebody else's protocol, and a fault in it must not take a person's shell tools down with it.
- `internal/clioutput`: shared Go CLI diagnostics on stderr.
- `internal/configstore`: per-util `VERSION`, JSON config, schema validation, and in-memory defaults.
- `internal/{welcome,license,gitconfig,vscode}`: util-specific behaviour, form and action descriptions, templates, and ownership rules.
- `internal/agent`: the `agent` util, split so each boundary is testable alone. `internal/agent/item` parses and validates an item definition, `render` turns answers into an output's bytes, `ledger` reads and writes `spu.json`, `decide` is the fact-and-policy table with no filesystem in it, `harness` knows where each coding agent reads text, and `content` is what is embedded.
- `internal/thirdparty`: the cache behind `spu-clone-third-party`.
- `internal/{protocol,transport,registry,form,action,core,cored,client,cli}`: the core and the clients that talk to it.
- `Taskfile.yml`: go-task entry point for development, build, and CI commands.
- `.githooks/pre-commit`: optional staged-snapshot quality gate.
- `scripts/`: installation, update, and uninstall scripts shipped in release archives.
- `scripts/dev/`: development, hook setup, test, version, and release packaging scripts excluded from release archives.
- `test/`: Bash `*.spec.sh` files and shared assertions in `test/general/`.
- `.soppy/plans/`: requirement and design documents. Not tracked; `**/.soppy/` is in the global Git exclude file, so this is absent from a clone.

The public Git command is `spu git`; `spu git-custom-ini` is not an alias. The managed Git include file keeps its previous path, `~/.config/soppy-utils/git-custom-ini.gitconfig`.

## Configuration contracts

Each Go util owns `~/.config/soppy-utils/<util>/{VERSION,config.json}`. `VERSION` is an integer configuration contract version, independent of the package release version. `config.json` has the same `schema_version`, a `util` name, and typed `data`. Schemas are embedded in their util package. Generated artifacts use `state.json` for hashes and provenance.

`spu agent` keeps its own record in the project it generates into, at `.agents/spu.json`, not under the configuration directory: what was generated belongs to the project, and a second machine cloning it should start from nothing rather than from somebody else's answers. A path is spu's only if that record names it; existence alone never implies it. The recorded SHA-256 decides nothing by itself — it is a fact the item's `on_modified` policy reads.

Ordinary commands warn once on a known older contract and use an in-memory default polyfill. They do not persist migration. `spu <util> migrate` or `edit` is explicit; `spu update` checks for mismatches without rewriting config or opening forms. Treat future versions, malformed files, and `VERSION`/JSON disagreements as read-only conflicts. Never overwrite a modified managed artifact. Build generated shell scripts from trusted embedded templates and quote user data as inert Bash words.

Welcome hooks run only from an interactive parent shell. Powerlevel10k instant prompt may temporarily redirect stdout while `.zshrc` loads, so a generated banner must recognize a controlling terminal even when stdout is not a TTY. Keep direct noninteractive and terminal-free execution silent. Increment the welcome template revision when generated script behavior changes; refresh an installed script through `spu welcome install`, not by editing managed output.

## Shell and Go rules

Use `#!/usr/bin/env bash` for all project-maintained shell scripts. Bash 3.2 is the floor; avoid Bash 4 associative arrays, namerefs, `mapfile`, and `globstar`. Use `${BASH_SOURCE[0]}` for script paths. In Windows Git Bash, pass `SPU_HOME_NATIVE` from `cygpath -w "$HOME"` before starting a native Go executable. Use Huh v2 for multi-field terminal interaction, but only in the client: a util package must not import it, and must not import `internal/client`, `internal/cored`, `internal/transport`, or any terminal library. Keep one-shot CLI output clean on stdout.

For Go changes, keep file operations and migrations in the owning package or shared `internal/` helpers. Validate JSON with the pinned Draft 2020-12 validator and validate domain rules in Go. JSON Schema `default` annotations do not fill values; polyfills are explicit code. Use `os/exec` argument arrays for Git, not shell command strings.

## Core and clients

One executable is both a client and the core daemon it starts when it cannot reach one. These are the rules that keep the arrangement worth having, and a change that breaks one has broken the reason for it:

- A util package owns behaviour only. It writes what it wants shown to an `io.Writer` and asks through a `core.Responder`; it never decides what a terminal is, and never prints to this process's output.
- Capabilities are judged against the session's environment, never against the daemon's own. The daemon outlives the client that started it, so its `PATH` and home are the wrong answer for everyone who arrives later.
- The registry is the only list of what exists, what each util accepts, and how it is judged. Do not add a second one for help, for dispatch, or in a shell script: this repository has had four of those, and each was wrong at least once.
- A capability is a hint, never an authorization. Every command revalidates its own preconditions and reports a typed failure, because the machine changes between the snapshot and the call.
- Work that changes the machine is never carried out without an answer.
- Every boundary here is testable on its own. Go packages use `go test ./...`; a util's tests must not need a daemon, and a daemon's tests must not need a util.

## Lint and tests

Use go-task v3 as the development and CI entry point: run `task format`, `task lint`, `task test`, and `task smoke`. `task check` runs lint and tests; `task build` writes the native binary to `dist/`. The underlying `scripts/dev/` commands remain available for direct use and the staged-snapshot hook. Lint requires ShellCheck, shfmt, Go, and gofmt. The optional hook is installed with `task hooks:install` and checks the staged snapshot without modifying the index. Bash specs use `test/general/helpers.sh`; Go packages use `go test ./...`. Do not introduce Node.js, npm, zunit, or another shell test runner. Keep tooling and runner scripts under `scripts/dev/`, never `bin/`. Test generated scripts in noninteractive shells and keep real user home and global Git config untouched by tests.

CI runs native tests and archive smoke tests on three targets through Task: macOS 15 arm64, Ubuntu 24.04 amd64, and Windows Server 2025 amd64. Windows Server is only a CI proxy for the Windows 10/11 desktop release target. `task package VERSION=<version> OUT_DIR=<output-dir>` invokes `scripts/dev/package.sh` to build three release archives and `SHA256SUMS`; CD validates them before tagging. `scripts/dev/version.sh` uses Conventional Commits: breaking changes bump major, `feat` bumps minor, other conventional commits bump patch.

Keep the English and Chinese README examples aligned with actual CLI behavior. Verify public release assets before documenting an installer command as usable; a source checkout or cross-build is not proof that a binary release has been published.

## Git conventions

Use Conventional Commits: `type(scope)?: description`. Keep descriptions lowercase, imperative, and under about 72 characters. Supported types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, and `revert`. Mark breaking changes with `!` or a `BREAKING CHANGE:` footer.

## Requirement and design documents

Requirement and design documents live in `.soppy/plans/`, one file per topic. Each keeps a `## Demands` section stating the requirement and a `## Plans` checklist that is updated as steps finish.

The directory is outside version control, so a document is local to the machine that wrote it: it has no history, does not appear in review, and is missing from a clone. Anything that must outlive this machine or reach another person belongs in `AGENTS.md`, the README files, or the code itself.

There is no skills-driven requirement workflow. Guidance shared by all agents belongs here; keep `CLAUDE.md` as a thin shim and `.claude/settings.json` for harness settings only.
