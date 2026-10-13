<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center"><img src="https://img.shields.io/badge/shell-Bash-89E051" alt="Bash runtime" /></p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

`spu` brings a few everyday development tools under one command. Its launcher runs on Bash 3.2; a Go helper handles licenses, welcome banners, and global Git settings. Release packages target Apple Silicon Macs, Ubuntu 24.04 LTS on x64, and Windows 10/11 on x64 with Git Bash. Release binaries do not require Go.

## Commands

| Command | What it does |
| --- | --- |
| `spu vscode` | Open VS Code settings, keybindings, or snippets |
| `spu license` | Create an MIT or ISC license with flags or a terminal form |
| `spu welcome` | Generate a shell banner and optionally load it from Bash or Zsh |
| `spu git` | Manage global Git defaults through one `include.path` |
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

`spu welcome install` writes `~/.config/soppy-utils/welcome/welcome.sh` and can add marked hooks to `~/.bashrc`, `~/.zshrc`, both, or neither (`--rc none`). The banner appears only when an interactive shell starts; a noninteractive run stays silent. `spu welcome edit` changes saved values, while another `install` refreshes the generated script after an upgrade. A previous managed `~/.config/soppy-utils/welcome.sh` can be adopted with `spu welcome migrate` when its checksum is valid. An older `$SOPPY/shell/welcome.sh` line in your rc file must be removed manually if it causes a duplicate banner.

FIGlet and fastfetch are optional. Without FIGlet or the custom `speed.flf` and `smslant.flf` fonts, the banner uses plain text; without fastfetch, it skips the system summary. `spu welcome status` reports what is available. `spu` does not install these tools.

`spu git install` registers `~/.config/soppy-utils/git-custom-ini.gitconfig` in **global** Git configuration. `spu git uninstall` removes only the matching include and an unchanged managed file; it does not edit repository-local settings. VS Code editor, diff, and merge commands require the `code` CLI when used.

Each Go util keeps a `VERSION` and `config.json` under `~/.config/soppy-utils/<util>/`. `VERSION` tracks that util's configuration format, independently of the package release. `spu update` reports mismatches without opening forms or rewriting preferences and generated files. Older recognized configs use in-memory defaults until you run `spu <util> migrate` or `edit`. For automation, use `spu <util> migrate --answers FILE` with a JSON object containing `util`, the current `schema_version`, and `data`.

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

Development requires [go-task](https://taskfile.dev/docs/installation) v3. `task build` writes the native helper to `dist/`; `task package` builds three archives and `SHA256SUMS`. Override the output directory with `OUT_DIR=...`; set `SPU_PACKAGE_TARGETS=darwin/arm64` to package one target. To install a local Apple Silicon build, run `bash scripts/install.sh --archive dist/spu-v0.0.0-darwin-arm64.tar.gz` after packaging. `task smoke` builds and installs the native archive in an isolated temporary home.

CI runs macOS arm64 smoke tests on macOS 15, Ubuntu x64 tests on 24.04 LTS, and Windows x64 tests on Windows Server 2025. GitHub does not provide hosted Windows 10/11 x64 runners, so desktop Windows compatibility needs separate manual verification. The formatter covers Bash and Go and requires shfmt; linting also requires ShellCheck. Windows commands run in Git Bash.

`gofmt` ships with the Go toolchain, while `shfmt` is installed separately (usually in `$(go env GOPATH)/bin`). Both only need to be on `PATH`.

The optional pre-commit hook checks the staged snapshot with the same format and lint gate as CI. It does not edit files or stage changes. Run `task format` and stage its changes before committing. `task hooks:uninstall` removes the repository-local hook setting; installation leaves existing Git hooks untouched. CI checks every push and pull request, and the release workflow repeats the quality gate before tagging. The underlying `scripts/dev/` commands remain available for the hook and direct use. Release archives contain only the installer, updater, and uninstaller from `scripts/`.

## License

[MIT](LICENSE).
