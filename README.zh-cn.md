<h1 align="center"><img width="60%" src="./assets/wordmark.png" alt="soppy-utils" /></h1>

<p align="center"><img src="https://img.shields.io/badge/shell-Bash-89E051" alt="Bash 运行环境" /></p>

<div align="center">

[English](README.md) | [简体中文](README.zh-cn.md)

</div>

`spu` 用一条命令收纳常用的开发工具。启动器基于 Bash 3.2；工具逻辑运行在一个 Go core 中，命令按需启动它并与之通信，核心在一段时间无人使用后会自行退出。发布包面向 Apple 芯片 Mac、x64 的 Ubuntu 24.04 LTS，以及使用 Git Bash 的 x64 Windows 10/11。发布版二进制无需安装 Go。

## 命令

| 命令 | 用途 |
| --- | --- |
| `spu vscode` | 打开 VS Code 设置、快捷键配置或代码片段目录 |
| `spu license` | 用命令行参数或终端表单生成 MIT、ISC 许可证 |
| `spu welcome` | 生成欢迎横幅，可选接入 Bash 或 Zsh |
| `spu git` | 通过单个 `include.path` 管理 Git 全局默认配置 |
| `spu update` | 更新已安装的发布版并报告配置版本差异 |

## 安装

[最新发布版](https://github.com/soppylzz/soppy-utils/releases/latest)已提供预编译程序。在支持的平台下载并运行[安装器](scripts/install.sh)；Windows 请使用 Git Bash：

```sh
curl -fsSLo install-spu.sh https://raw.githubusercontent.com/soppylzz/soppy-utils/main/scripts/install.sh
bash install-spu.sh
```

安装器会选择对应平台的压缩包，用 `SHA256SUMS` 校验后将 `spu` 放到 `~/.local/bin`；如有需要，请将该目录加入 `PATH`。从旧版 v0.1.0 Zsh 安装升级时，运行 `bash install-spu.sh --upgrade`。安装后可使用 `spu update` 获取后续版本。

## 使用

```sh
spu vscode --settings
spu license --isc --output LICENSE
spu license                    # 终端表单
spu welcome install --rc zsh
spu welcome status
spu git install
spu git edit
spu update --status
```

运行 `spu <命令> --help` 可查看选项。`spu license --mit` 和 `--isc` 默认只向标准输出写入许可证文本；指定 `--output` 才写入文件。在终端中直接运行 `spu license` 会打开表单，并可保存常用的类型、作者和输出路径。更新程序或编辑偏好不会重写已有的 LICENSE 文件。

`spu welcome install` 生成 `~/.config/soppy-utils/welcome/welcome.sh`，还可以在 `~/.bashrc`、`~/.zshrc`、两者或都不接入（`--rc none`）写入受管理的启动区块。横幅只在交互式 Shell 启动时显示；非交互运行保持静默。存在 Powerlevel10k 标准 instant prompt 预加载代码时，Zsh 钩子会放在它之前，避免启动警告。`spu welcome edit` 修改保存的内容，更新程序后再次运行 `install` 可刷新生成脚本。以前的 `~/.config/soppy-utils/welcome.sh` 在校验通过后可用 `spu welcome migrate` 接管。如果旧的 `$SOPPY/shell/welcome.sh` 引用造成重复横幅，需要手动从 rc 文件中移除。

FIGlet 和 fastfetch 都是可选工具。缺少 FIGlet 或自定义字体 `speed.flf`、`smslant.flf` 时，横幅会显示普通文字；缺少 fastfetch 时跳过系统信息。`spu welcome install` 会给出可安装它们的包管理器命令，并在执行前征求同意；`spu welcome status` 可查看当前状态。FIGlet 字体不在提议之列，因为 `spu` 没有约定的下载来源。

`spu git install` 在 Git **全局**配置中注册 `~/.config/soppy-utils/git-custom-ini.gitconfig`。`spu git uninstall` 仅移除匹配的引用和未被改动的受管理文件，不会修改仓库本地设置。使用 VS Code 作为编辑、差异或合并工具时需要 `code` 命令。

每个 Go 工具在 `~/.config/soppy-utils/<util>/` 下保存 `VERSION` 与 `config.json`。`VERSION` 是该工具的配置格式版本，与程序发布版本无关。`spu update` 遇到差异只提示，不打开表单，也不重写偏好或生成文件。对于已知的旧配置，普通命令在内存中补默认值；运行 `spu <util> migrate` 或 `edit` 后才会保存更新。自动化场景可运行 `spu <util> migrate --answers FILE`，其中 JSON 对象须包含 `util`、当前 `schema_version` 和 `data`。

## 卸载

```sh
bash ~/.local/share/soppy-utils/scripts/uninstall.sh
bash ~/.local/share/soppy-utils/scripts/uninstall.sh --purge
```

默认卸载删除程序和启动器，保留偏好、生成文件、欢迎横幅启动区块及 Git 全局引用。`--purge` 还会删除保存的默认选项、受管理的欢迎横幅区块与文件，以及 Git 全局引用。若受管理的生成文件被修改，清理会停止，程序仍保留；此前已完成的清理步骤不会自动撤销。

## 开发

使用 Go 1.24.2 或更新版本从源码运行：

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

开发还需要 [go-task](https://taskfile.dev/docs/installation) v3。`task build` 将当前平台的 Go 程序写入 `dist/`；`task package` 生成三个平台的压缩包及 `SHA256SUMS`。可用 `OUT_DIR=...` 指定输出目录，也可设置 `SPU_PACKAGE_TARGETS=darwin/arm64` 只打包一个目标。打包后可运行 `bash scripts/install.sh --archive dist/spu-v0.0.0-darwin-arm64.tar.gz` 安装 Apple 芯片 Mac 的本地构建。`task smoke` 会在隔离的临时 HOME 下构建、安装并运行当前平台的压缩包。

CI 在 macOS 15 上测试 macOS arm64，在 Ubuntu 24.04 LTS 上测试 Linux x64，在 Windows Server 2025 上测试 Windows x64。GitHub 没有托管的 Windows 10/11 x64 runner，因此桌面版 Windows 的兼容性还需要单独手动验证。格式化命令处理 Bash 和 Go 文件，需要 shfmt；lint 还需要 ShellCheck。Windows 命令在 Git Bash 中运行。

`gofmt` 随 Go 工具链提供，`shfmt` 需要单独安装（通常位于 `$(go env GOPATH)/bin`）。两者只需都在 `PATH` 中即可。

可选的 pre-commit hook 会对暂存区快照执行与 CI 相同的格式和 lint 检查，不会修改文件或自动暂存。提交前运行 `task format` 并暂存结果。`task hooks:uninstall` 会移除本仓库的 hook 设置；安装命令不会覆盖已有的 Git hooks。CI 检查每次 push 和 pull request，发布流程也会在打标签前重复质量检查。底层 `scripts/dev/` 命令仍供 hook 和直接调用使用。发布压缩包只包含 `scripts/` 下的安装、更新和卸载脚本。

## 许可证

[MIT](LICENSE)。
