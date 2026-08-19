# scripts

## Demands

Implement the four scripts under `scripts/` — the installer/uninstaller and the two development helpers. Each script is a standalone zsh executable.

**install.sh**

Single install entry point with three execution modes, auto-detected at runtime.

- **Mode A — local clone**: the script is a real file inside a checked-out project (its `$0` path resolves under a directory containing `bin/spu`). Copy the whole project into the target directory, symlink `bin/spu` into `~/.local/bin/spu`, then remove the original clone directory. Guard: before removing, check whether the clone directory is already the install target (i.e. already at `~/.local/share/soppy-utils`); if so, skip the removal so the freshly installed copy is not deleted.
- **Mode B — curl + git present**: the script runs standalone (fetched via curl, no project files around it) and `command -v git` succeeds. Silently `git clone` the repo into the target directory, then symlink.
- **Mode C — curl, no git**: the script runs standalone and `git` is missing. Fetch the project via `curl` (tarball from GitHub) and extract it into the target directory, then symlink.

Detection rule: if `$0` is a real file whose ancestor contains `bin/spu` → Mode A; otherwise standalone → Mode B when `command -v git` succeeds, else Mode C.

Post-install, the script must:
- Ensure `~/.local/bin` exists.
- Verify `~/.local/bin` is on `$PATH`; if it is not, print a hint telling the user how to add it (e.g. a line for `~/.zshrc`).

**uninstall.sh**

Remove everything `install.sh` created: the `~/.local/bin/spu` symlink and the installed project directory. Should be a safe inverse of the installer and not touch unrelated files.

**test.sh**

Development runner: discover every `test/**/*.spec.sh` under the project and execute each with `zsh`. Report a pass/fail summary and exit non-zero if any spec fails.

**lint.sh**

Development runner: run `zsh-lint` over the project's zsh scripts to surface syntax/static-analysis issues. Exit non-zero when diagnostics are reported.

**Conventions**

- All scripts live under `scripts/` (the `script/` spelling in `CLAUDE.md`'s directory section is stale and should be corrected to `scripts/`).
- Text in English; comments match the existing terse style.
- Paths use `~`/`$HOME`, `~/.local/bin` for the bin symlink, and the project install target `~/.local/share/soppy-utils`.

**Decisions**

- **Repo URL** — `https://github.com/soppylzz/soppy-utils` (clone and tarball source).
- **Install target** — `~/.local/share/soppy-utils`; symlink `~/.local/bin/spu` → `<target>/bin/spu`.
- **Overwrite** — skip with a message when the target directory or `~/.local/bin/spu` already exists; do not overwrite.
- **lint.sh zsh-lint** — check `command -v zsh-lint`; if absent, print a message instructing the user to install it via `go install` and link `https://github.com/z-shell/zsh-lint`, then exit non-zero. No automatic installation.

## Plans

- [x] implementInstallScript — write `scripts/install.sh` with three modes (local clone / curl+git / curl-only), `~/.local/share/soppy-utils` target, `~/.local/bin/spu` symlink, skip-if-present, post-copy removal of the source clone (skipped when the clone is already the install target), and the post-install `$PATH` check/hint.
- [x] implementUninstallScript — write `scripts/uninstall.sh` removing the `~/.local/bin/spu` symlink and the `~/.local/share/soppy-utils` directory.
- [x] implementTestScript — write `scripts/test.sh` discovering `test/**/*.spec.sh`, running each with `zsh`, and summarizing with non-zero exit on any failure.
- [x] implementLintScript — write `scripts/lint.sh` checking `command -v zsh-lint`, printing the install hint and exiting non-zero when absent, else linting the project scripts and exiting non-zero on diagnostics.
- [x] fixClaudeMdScriptDir — correct the `script/` directory spelling to `scripts/` in `CLAUDE.md`.
- [x] verifyScripts — smoke-test install/uninstall in a sandboxed fake `$HOME`, and confirm `test.sh`/`lint.sh` exit codes.
