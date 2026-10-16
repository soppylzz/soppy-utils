<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center"><img src="https://img.shields.io/badge/shell-Bash-89E051" alt="Bash runtime" /></p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

`spu` brings a few everyday development tools under one command. Its launcher runs on Bash 3.2; the tools run in a Go core that the command starts on demand and talks to, and that exits on its own once nothing has used it for a while. Release packages target Apple Silicon Macs, Ubuntu 24.04 LTS on x64, and Windows 10/11 on x64 with Git Bash. Release binaries do not require Go.

## Commands

| Command | What it does |
| --- | --- |
| `spu vscode` | Open VS Code settings, keybindings, or snippets |
| `spu license` | Create an MIT or ISC license with flags or a terminal form |
| `spu welcome` | Generate a shell banner and optionally load it from Bash or Zsh |
| `spu git` | Manage global Git defaults through one `include.path` |
| `spu agent` | Generate a project's agent conventions and skills |
| `spu update` | Upgrade an installed release and report config version differences |

## Install

The [latest release](https://github.com/soppylzz/soppy-utils/releases/latest) provides precompiled binaries. Download and run [the installer](scripts/install.sh) on a supported platform (Git Bash on Windows):

```sh
curl -fsSLo install-spu.sh https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh
bash install-spu.sh
```

The installer selects the matching archive, verifies it against `SHA256SUMS`, and puts `spu` in `~/.local/bin`; add that directory to `PATH` if needed. To upgrade an existing v0.1.0 Zsh installation, run `bash install-spu.sh --upgrade`. After installation, use `spu update` for later releases.

## Use

```sh
spu vscode --settings
spu license --isc --output LICENSE
spu license                    # terminal form
spu welcome install --rc zsh
spu welcome status
spu git install
spu git edit
spu update --status
```

Run `spu <command> --help` for options. `spu license --mit` and `--isc` print only license text to stdout unless `--output` is given. In a terminal, `spu license` opens a form and can save your preferred type, author, and output path. Neither updates nor preference edits rewrite an existing LICENSE file.

`spu welcome install` writes `~/.config/soppy-utils/welcome/welcome.sh` and can add marked hooks to `~/.bashrc`, `~/.zshrc`, both, or neither (`--rc none`). The banner appears only when an interactive shell starts; a noninteractive run stays silent. When the standard Powerlevel10k instant prompt preamble is present, the Zsh hook is placed before it to avoid a startup warning. `spu welcome edit` changes saved values, while another `install` refreshes the generated script after an upgrade. A previous managed `~/.config/soppy-utils/welcome.sh` can be adopted with `spu welcome migrate` when its checksum is valid. An older `$SOPPY/shell/welcome.sh` line in your rc file must be removed manually if it causes a duplicate banner.

FIGlet and fastfetch are optional. Without FIGlet or the custom `speed.flf` and `smslant.flf` fonts, the banner uses plain text; without fastfetch, it skips the system summary. `spu welcome install` offers the package manager command that would install them and asks before running it, and `spu welcome status` reports what is available. The FIGlet fonts are not offered, because there is no source `spu` installs them from.

`spu git install` registers `~/.config/soppy-utils/git-custom-ini.gitconfig` in **global** Git configuration. `spu git uninstall` removes only the matching include and an unchanged managed file; it does not edit repository-local settings. VS Code editor, diff, and merge commands require the `code` CLI when used.

Each Go util keeps a `VERSION` and `config.json` under `~/.config/soppy-utils/<util>/`. `VERSION` tracks that util's configuration format, independently of the package release. `spu update` reports mismatches without opening forms or rewriting preferences and generated files. Older recognized configs use in-memory defaults until you run `spu <util> migrate` or `edit`. For automation, use `spu <util> migrate --answers FILE` with a JSON object containing `util`, the current `schema_version`, and `data`.

`spu agent` generates the text a coding agent reads into one canonical `.agents/` directory: the conventions documents and skills you choose, from item definitions embedded in `spu`. It asks which items to generate, then each item's questions, and records what it wrote and with which answers in `.agents/spu.json`.

Three harnesses are understood. Pi and Codex read `.agents/skills` themselves, so the canonical directory reaches them directly; Claude Code does not, and gets a single `.claude/skills` link to it. A project with an existing file at one of these paths is never overwritten: a path counts as spu's only when the record names it, and `spu agent status` reports what has changed since it was generated.

```sh
spu agent                  # choose and generate
spu agent status           # what is generated, missing, or changed
spu agent edit             # answer one item's questions again
```

`spu agent` also writes `.agents/SPU.md`, which explains what is generated and lists every generated document as an `@` reference. Reference it once from the project's instruction file:

```markdown
@.agents/SPU.md
```

Nothing generates into `AGENTS.md` or `CLAUDE.md`: those are the project's own files. Claude Code expands the `@`; Codex and Pi have no import syntax and read the line as an instruction to open that file. Nothing generated is copied into a harness directory except the single `.claude/skills` link.

Do not edit a generated file. Add your own document beside it and say so in `AGENTS.md`; a document you own is never touched. Changing what a generated document says means changing the answer it came from.

Choosing a harness also registers `spu-clone-third-party` with it, in the file that harness reads: `.mcp.json` at the project root for Claude Code, `.pi/mcp.json` for Pi, and `[mcp_servers.clone-third-party]` in `.codex/config.toml` for Codex. Codex's file is a project's general configuration rather than one that exists to hold servers, so the table is appended to it rather than the file being rewritten — a project's own comments and ordering survive. Deselecting a harness removes spu's entry and nothing else. `.agents/mcp/spu.json` records what was registered and with which harnesses.

spu writes no permission rules for any harness, so nothing stops an agent writing into that cache. Claude Code can be told to, by adding `{"permissions":{"deny":["Edit(~/code/reference/**)"]}}` to `.claude/settings.json`. The other two cannot be told at all: Codex's sandbox is an allowlist whose glob rules deny reads rather than writes, and Pi has no built-in sandbox and asks for approval of nothing. If write access matters, isolate those two. `.agents/SPU.md` says the same thing to whoever reads it.

`spu-clone-third-party` is installed alongside `spu`, on `PATH`, so a registration committed to a repository works on any machine that installed it. It is an MCP server that hands an agent a local checkout of a third-party package to read, so an agent reads a directory instead of making repeated web requests. A version may be a major (`8`), a full version (`8.1.0`), or omitted for the newest release; a major means the newest release of that major, and the tool reports the version it actually checked out rather than the one it was asked for. A checkout whose major matches is reused without any network access. An exact version is never fetched on its own, because pulling a pinned version without being told to is how a cache becomes a supply chain.

## Uninstall

```sh
bash ~/.local/share/soppy-utils/scripts/uninstall.sh
bash ~/.local/share/soppy-utils/scripts/uninstall.sh --purge
```

The default uninstall removes the package and launcher but preserves preferences, generated files, welcome hooks, and the global Git include. `--purge` also removes saved defaults, managed welcome hooks and files, and the managed Git include. It stops if a managed artifact was modified, leaving the package installed; any earlier cleanup steps may already have completed.

## Develop

Clone the repository to run from source with Go 1.24.2 or newer:

```sh
git clone https://github.com/soppylzz/soppy-utils.git
cd soppy-utils
bash bin/spu license --mit --name 'Ada Lovelace' --year 2026
```

```sh
task format
task format:check
task check
task build
task package VERSION=v0.0.0
task smoke
task hooks:install
```

Development requires [go-task](https://taskfile.dev/docs/installation) v3. `task build` writes the native binary to `dist/`; `task package` builds three archives and `SHA256SUMS`. Override the output directory with `OUT_DIR=...`; set `SPU_PACKAGE_TARGETS=darwin/arm64` to package one target. To install a local Apple Silicon build, run `bash scripts/install.sh --archive dist/spu-v0.0.0-darwin-arm64.tar.gz` after packaging. `task smoke` builds and installs the native archive in an isolated temporary home.

CI runs macOS arm64 smoke tests on macOS 15, Ubuntu x64 tests on 24.04 LTS, and Windows x64 tests on Windows Server 2025. GitHub does not provide hosted Windows 10/11 x64 runners, so desktop Windows compatibility needs separate manual verification. The formatter covers Bash and Go and requires shfmt; linting also requires ShellCheck. Windows commands run in Git Bash.

`gofmt` ships with the Go toolchain, while `shfmt` is installed separately (usually in `$(go env GOPATH)/bin`). Both only need to be on `PATH`.

The optional pre-commit hook checks the staged snapshot with the same format and lint gate as CI. It does not edit files or stage changes. Run `task format` and stage its changes before committing. `task hooks:uninstall` removes the repository-local hook setting; installation leaves existing Git hooks untouched. CI checks every push and pull request. A release is dispatched by hand on `main` with a choice of automatic or forced bump; the workflow repeats the quality gate, then builds and validates every archive before it tags. The underlying `scripts/dev/` commands remain available for the hook and direct use. Release archives contain only the installer, updater, and uninstaller from `scripts/`.

## License

[MIT](LICENSE).
