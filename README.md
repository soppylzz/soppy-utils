<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center">
  <a href="https://github.com/soppylzz/soppy-utils/releases"><img src="https://img.shields.io/github/v/release/soppylzz/soppy-utils" alt="Latest GitHub release" /></a>
  <img src="https://img.shields.io/badge/shell-zsh-89E051" alt="zsh" />
</p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

A collection of zsh commands for everyday development tasks, available through one `spu` command.

## Features

| Command | Purpose |
| --- | --- |
| `spu vscode` | Open VS Code settings, keyboard shortcuts, or snippets |
| `spu license` | Generate an MIT or ISC license from a template |

## Installation

The installer copies the project to `~/.local/share/soppy-utils` and links `spu` into `~/.local/bin`.

```sh
curl -fsSL https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh | zsh
```

Alternatively, clone the repository and run `./scripts/install.sh`. The installer may remove a clean source clone after copying it; keep a working checkout with uncommitted changes if you want it retained.

Add `~/.local/bin` to your `PATH` if the installer prompts you:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## Usage

```sh
spu --help
spu vscode --settings
spu vscode --keys
spu vscode --snippets
spu license --mit
spu license --isc --output LICENSE
```

Run `spu <command> --help` for all options. `spu license` uses `git config user.name` or `$USER` for the author and the current year for the date.

## Uninstall

```sh
~/.local/share/soppy-utils/scripts/uninstall.sh
```

## License

[MIT](./LICENSE).
