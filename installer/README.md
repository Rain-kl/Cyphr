# Cyphr Installer & Agent Manager (Bubble Tea TUI)

基于 [Charmbracelet Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建的现代化终端 TUI 安装与 Agent 进程生命周期管理工具。

## 功能特性

- **一键独立交付与 Agent 安装**：支持从 GitHub Release 自动拉取 `cyphr-agent.zip` 运行时包，解压部署并自动配置 Python 虚拟环境与默认配置，支持国内下载镜像加速（`ghproxy.net`）。
- **一键在线更新（Agent & Installer 自更新）**：集成 GitHub Release 检测与自动拉取，支持在线升级 `cyphr-agent.zip` 业务包与 `cyphr-installer` 本身二进制无缝原地热替换（Self-Update）。
- **交互式 TUI 控制面板**：基于 Bubble Tea 与 Lipgloss 构建，包含状态指示徽章、Spinner 动态加载与卡片布局。
- **完整的守护进程（Daemon）隔离**：通过底层 POSIX 会话解耦（`Setsid: true`）与独立输入输出重定向，确保退出管理面板或关闭终端后，后台运行的 Agent 服务与模型下载任务持续稳定运行。
- **ASR 模型库下载**：内置 Qwen3-ASR (0.6B / 1.7B) 与 Whisper 全系列预设模型，支持一键切换国内镜像加速（`https://hf-mirror.com`）与断点续传。
- **实时日志与进度监控**：支持后台下载进度查看与 `agent.log` 实时日志查看。
- **双模运行（TUI / Headless CLI）**：无参数直接启动沉浸式交互 TUI；带参数直接以 CLI 模式执行，完美适配自动化运维。

## 安装与编译

```bash
cd /Users/ryan/Code/Go/Cyphr/installer
go build -o cyphr-installer main.go
```

## 使用方法

### 1. 交互式 TUI 模式

```bash
./cyphr-installer
```

快捷键指南：
- `↑ / ↓` 或 `j / k`：上下移动光标
- `i`：进入【从 GitHub 下载安装 Agent】页面
- `u`：进入【检查与在线更新】页面，支持自更新 Installer 或升级 Agent 运行时
- `1 - 9`：数字快速选择服务生命周期与模型管理功能
- `Enter`：确认进入所选项
- `m`（下载/安装/更新页面）：快速切换官方源 / 国内加速镜像
- `v`（安装页面）：切换是否自动初始化 Python 虚拟环境
- `Esc` 或 `q`：返回上级菜单 / 退出面板（不会终止后台服务与下载）

### 2. 无界面 CLI 命令行模式

```bash
./cyphr-installer install [DIR]              # 一键从 GitHub 下载并安装/部署 Agent 运行时环境
./cyphr-installer update [agent|installer]  # 在线更新 Agent 运行时或更新 Installer 自身程序
./cyphr-installer start                      # 后台启动 Agent 服务
./cyphr-installer stop                       # 优雅停止 Agent 服务
./cyphr-installer restart                    # 重启 Agent 服务
./cyphr-installer status                     # 查看服务与下载综合状态
./cyphr-installer download <ID> [DIR]        # 下载指定 ASR 模型
./cyphr-installer progress                   # 查看当前下载任务进度与实时日志
./cyphr-installer stop-download              # 停止当前后台下载任务
./cyphr-installer models                     # 列出本地已下载模型库
./cyphr-installer help                       # 查看帮助说明
```
