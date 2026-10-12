<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center"><img src="https://img.shields.io/badge/shell-Bash-89E051" alt="Bash runtime" /></p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

`spu` brings a few everyday development tools under one command. Its launcher runs on Bash 3.2; a Go helper handles licenses, welcome banners, and global Git settings. The release workflow targets macOS and Linux on amd64/arm64, plus Windows Git Bash on amd64/arm64. Release binaries will not require Go.

## Commands

| Command | What it does |
| --- | --- |
| `spu vscode` | Open VS Code settings, keybindings, or snippets |
| `spu license` | Create an MIT or ISC license with flags or a terminal form |
| `spu welcome` | Generate a shell banner and optionally load it from Bash or Zsh |
| `spu git` | Manage global Git defaults through one `include.path` |
| `spu update` | Upgrade an installed release and report config version differences |

## Try the current source

The [latest published release](https://github.com/soppylzz/soppy-utils/releases) is the earlier Zsh version and does not yet contain the Bash + Go archives. To use the commands in this README now, build from this checkout with Go 1.24.2 or newer:

```sh
git clone https://github.com/soppylzz/soppy-utils.git
cd soppy-utils
bash bin/spu license --mit --name 'Ada Lovelace' --year 2026
```

To install a local build, run `bash scripts/package.sh v0.0.0 dist`, then choose the archive for your OS and architecture:

```sh
bash scripts/install.sh --archive dist/spu-v0.0.0-darwin-arm64.tar.gz
```

The example archive is for Apple Silicon. Replace `darwin-arm64` with `darwin-amd64`, `linux-amd64`, `linux-arm64`, `windows-amd64`, or `windows-arm64` as appropriate. The installer verifies the archive against `dist/SHA256SUMS` and puts `spu` in `~/.local/bin`; add that directory to `PATH` if needed. It leaves the source checkout alone.

After the first Bash + Go release publishes the archives and `SHA256SUMS`, new users can download and run [the installer](scripts/install.sh) without Go:

```sh
curl -fsSLo install-spu.sh https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh
bash install-spu.sh
```

Installed release users can run `spu update`. An older Zsh installation without that command can run the new installer with `--upgrade` once the new release is available.

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

```sh
bash scripts/test.sh
bash scripts/lint.sh
```

Development requires Go 1.24.2 or newer. Lint also requires ShellCheck and shfmt. Windows commands run in Git Bash.

## License

[MIT](LICENSE).
