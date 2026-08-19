<p align="center">
  <img src="assets/icon.png" alt="soppy-utils" width="150"/>
</p>

<h1 align="center">soppy-utils</h1>
<center>

![zsh-compatible](https://img.shields.io/badge/shell-zsh-brightgreen)
![license](https://img.shields.io/badge/license-MIT-green)

</center>

一个面向工程的 zsh 工具集合，用于自动化常见的开发任务。每个工具都是一个 zsh 脚本；工具通过安装脚本安装到 `PATH` 上，之后即可当作命令调用。

所有工具都通过单一的 `spu` 入口统一调用：

```sh
spu <util> [...args]
```

内置工具：

| 工具 | 说明 |
| --- | --- |
| `vscode` | 快速打开 VS Code 的配置文件/目录 |
| `license` | 从模板生成 LICENSE 文本 |

## 项目说明

soppy-utils 是 soppylzz 的个人 zsh 开发工具包，用于处理一些零散的开发杂项任务（misc tasks）。

目录结构：

```
.
├── bin/            # 程序入口（薄分发器）
├── src/            # 工具实现与共享函数
│   ├── usage/      # 帮助文本，每个工具一个文件 + 总览
│   ├── utils/      # 共享辅助函数
│   └── template/   # LICENSE 模板（纯数据，非逻辑）
├── scripts/        # 安装/卸载及开发辅助脚本
├── test/           # zsh 测试套件
│   └── general/    # 共享测试原语
└── plan/           # 需求文档，每个需求一份
```

`bin/` 只保留轻量的 `spu` 启动器；全部逻辑位于 `src/` 下，每个工具对应一份实现，外加共享的辅助函数。帮助文本集中在 `src/usage/`，并通过 `create_usager` 工厂生成；可复用辅助函数放在 `src/utils/`；LICENSE 数据放在 `src/template/`。`scripts/` 包含安装/卸载及开发脚本，`test/` 存放 zsh 测试规格与共享原语，`plan/` 存放需求文档（每个需求一份）。

## 安装

安装会把项目复制到 `~/.local/share/soppy-utils`，并把 `bin/spu` 软链接到 `~/.local/bin/spu`。

### 方式一：git clone 后安装

```sh
git clone https://github.com/soppylzz/soppy-utils.git
cd soppy-utils
./scripts/install.sh
```

脚本会复制项目到安装目录、建立软链接，然后删除原来的 clone 目录。

### 方式二：curl 一键安装

```sh
curl -fsSL https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh | zsh
```

- 本地有 `git`：脚本会静默 `git clone` 到安装目录。
- 本地没有 `git`：脚本会直接通过 `curl` 下载 GitHub 上的压缩包并解压。

安装完成后，如果 `~/.local/bin` 不在你的 `PATH` 中，脚本会输出提示。你可以把下面这行加到 `~/.zshrc`：

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## 卸载

运行安装目录里的卸载脚本即可：

```sh
~/.local/share/soppy-utils/scripts/uninstall.sh
```

它会删除 `~/.local/bin/spu` 软链接和 `~/.local/share/soppy-utils` 目录。

## usage

查看总览：

```sh
spu
# 或
spu --help
```

输出：

```
Usage: spu <util> [...args]

Utils:
  vscode — open VS Code configuration files quickly
  license — generate a LICENSE text from a template
```

### spu vscode

```sh
spu vscode [OPTION]
```

| 选项 | 说明 |
| --- | --- |
| `-S`, `--snippets` | 打开 VS Code 的 snippets 目录 |
| `-s`, `--settings` | 打开 VS Code 的 `settings.json` |
| `-k`, `--keys` | 打开 VS Code 的 `keybindings.json` |
| `-h` | 显示简短帮助 |
| `--help` | 显示详细帮助 |

```sh
spu vscode --settings
spu vscode -k
```

### spu license

```sh
spu license [OPTION]
```

| 选项 | 说明 |
| --- | --- |
| `-M`, `--mit` | 生成 MIT license |
| `-I`, `--isc` | 生成 ISC license |
| `-o`, `--output <file>` | 将 license 写入 `<file>`（默认输出到 stdout） |
| `-h` | 显示简短帮助 |
| `--help` | 显示详细帮助 |

license 的作者名来自 `git config user.name`（缺失时回退到 `$USER`），年份来自 `date`。

```sh
spu license --mit
spu license --isc --output LICENSE
```
