<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center">
  <a href="https://github.com/soppylzz/soppy-utils/releases"><img src="https://img.shields.io/github/v/release/soppylzz/soppy-utils" alt="Latest GitHub release" /></a>
  <img src="https://img.shields.io/badge/shell-zsh-89E051" alt="zsh" />
</p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

一组用于日常开发任务的 zsh 命令，通过统一的 `spu` 入口调用。

## 特性

| 命令 | 用途 |
| --- | --- |
| `spu vscode` | 打开 VS Code 设置、快捷键配置或代码片段目录 |
| `spu license` | 根据模板生成 MIT 或 ISC 许可证 |

## 安装

安装脚本会将项目复制到 `~/.local/share/soppy-utils`，并将 `spu` 链接到 `~/.local/bin`。

```sh
curl -fsSL https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh | zsh
```

也可以克隆仓库后运行 `./scripts/install.sh`。安装脚本在复制完成后可能删除没有未提交改动的源码目录；如果需要保留工作副本，请先确认安装方式。

若安装脚本提示 `~/.local/bin` 不在 `PATH` 中，可添加：

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## 使用

```sh
spu --help
spu vscode --settings
spu vscode --keys
spu vscode --snippets
spu license --mit
spu license --isc --output LICENSE
```

完整选项见 `spu <command> --help`。`spu license` 使用 `git config user.name`（缺失时使用 `$USER`）作为作者，年份取当前年份。

## 卸载

```sh
~/.local/share/soppy-utils/scripts/uninstall.sh
```

## 许可证

[MIT](./LICENSE)。
