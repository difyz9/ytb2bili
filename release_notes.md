# ytb2bili 使用说明

> 通过 GitHub Releases 下载预构建安装包，无需本地编译即可运行。

---

## 目录

1. [前置依赖](#1-前置依赖)
2. [下载安装包](#2-下载安装包)
3. [创建配置文件](#3-创建配置文件)
4. [首次运行](#4-首次运行)
5. [配置说明](#5-配置说明)
6. [升级到新版本](#6-升级到新版本)
7. [常见问题](#7-常见问题)

---

## 1. 前置依赖

| 工具 | 用途 | 是否必须 |
|------|------|----------|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | YouTube 视频下载 | ✅ 必须 |
| [ffmpeg](https://ffmpeg.org/download.html) | 音视频处理 | ✅ 必须 |
| `api2key.base_url` 或服务端兜底 LLM Key | AI 功能 | 可选 |

**安装 yt-dlp：**
```bash
# macOS
brew install yt-dlp

# Linux
pip install yt-dlp
# 或下载二进制：https://github.com/yt-dlp/yt-dlp/releases/latest

# Windows
winget install yt-dlp
```

**安装 ffmpeg：**
```bash
# macOS
brew install ffmpeg

# Ubuntu / Debian
sudo apt install ffmpeg

# Windows
winget install ffmpeg
```

---

## 2. 下载安装包

前往 [GitHub Releases](https://github.com/zolagz/ytb2bili/releases/latest) 下载对应平台的二进制文件：

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Linux | x86_64 (AMD64) | `ytb2bili-linux-amd64` |
| Linux | ARM64 (树莓派 / 服务器) | `ytb2bili-linux-arm64` |
| macOS | Intel | `ytb2bili-darwin-amd64` |
| macOS | Apple Silicon (M1/M2/M3) | `ytb2bili-darwin-arm64` |
| Windows | x86_64 | `ytb2bili-windows-amd64.exe` |

**命令行下载示例（Linux / macOS）：**
```bash
# 以 Linux AMD64 为例，替换版本号和文件名
VERSION=v0.0.18
curl -L -o ytb2bili \
  "https://github.com/zolagz/ytb2bili/releases/download/${VERSION}/ytb2bili-linux-amd64"

# 授予执行权限
chmod +x ytb2bili
```

---

## 3. 创建配置文件

程序启动时会从**当前工作目录**读取 `config.toml`。

```bash
# 下载配置模板（用于参考）
curl -L -o config.toml \
  "https://raw.githubusercontent.com/zolagz/ytb2bili/main/config.toml.example"
```

或手动创建 `config.toml`，以下是**最小必要配置**：

```toml
# ── 服务 ──────────────────────────────────────────────
[server]
host = "0.0.0.0"
port = 8096
static_dir  = "./downloads"
static_path = "/static"

# ── 数据库（SQLite，开箱即用）─────────────────────────
[database]
type         = "sqlite"
path         = "ytb2bili.db"
auto_migrate = true

# ── 视频处理工作流 ─────────────────────────────────────
[workflow]
download_dir = "./downloads"
# ytdlp_path  = "/usr/local/bin/yt-dlp"   # 若不在 PATH 中，取消注释并填写绝对路径
# ffmpeg_path = "/usr/local/bin/ffmpeg"
# proxy_url   = "http://127.0.0.1:7890"   # 代理（可选）

# ── AI Agent（可选兜底，不影响 ytb2bili-api 网关模式）──────────────
[agent]
name = "ytb2bili_agent"

[agent.llm]
# api_key  = "sk-..."         # 可选：仅本地兜底时填写
model    = "gpt-4o-mini"
# base_url = "https://open.ytb2bili.com/ai/v1"

# ── 自动更新 ──────────────────────────────────────────
[updater]
enabled        = true
auto_update    = false
check_interval = 24
```

> **说明**：Web AI 助手默认走 ytb2bili-api /ai/v1 网关，`agent.llm` 只作为旧流程或离线环境的兜底配置。
> ```toml
> [agent.llm]
> api_key  = "sk-..."
> model    = "gpt-4o-mini"
> base_url = "https://open.ytb2bili.com/ai/v1"
> ```

---

## 4. 首次运行

```bash
# 确保 config.toml 与可执行文件在同一目录
ls
# ytb2bili   config.toml

# Linux / macOS
./ytb2bili

# Windows
.\ytb2bili-windows-amd64.exe
```

启动成功后，终端输出类似：

```
🚀 YTB2BILI 启动中... 版本: v0.0.18, 构建时间: 2026-03-04T...
✅ 应用启动成功，按 Ctrl+C 停止...
```

**访问地址：**

| 入口 | 地址 |
|------|------|
| Web 界面 | `http://localhost:8096` |
| API 健康检查 | `http://localhost:8096/health` |
| API 文档（Swagger） | `http://localhost:8096/swagger/index.html` |

---

## 5. 配置说明

### 5.1 指定配置文件路径

通过环境变量 `CONFIG_FILE` 指定任意路径的配置文件：

```bash
CONFIG_FILE=/etc/ytb2bili/config.toml ./ytb2bili
```

### 5.2 cookies 文件（下载受限视频）

若需要下载需要登录的 YouTube 视频，在 `config.toml` 中配置：

```toml
[workflow]
cookies_file = "/path/to/cookies.txt"
```

> 可通过浏览器插件（如 [Get cookies.txt](https://chrome.google.com/webstore/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)）导出 YouTube cookies。

### 5.3 开启 LLM 字幕翻译

```toml
[workflow]
llm_translation_enabled    = true
llm_translation_batch_size = 25
llm_translation_target_lang = "zh-Hans"
```

### 5.4 静态文件目录

`downloads/` 目录将自动创建，用于存储下载的视频/字幕/缩略图。  
可通过 `http://localhost:8096/static/<文件名>` 直接访问。

---

## 6. 升级到新版本

### 方式一：手动替换二进制

1. 从 [Releases 页面](https://github.com/zolagz/ytb2bili/releases/latest) 下载新版本
2. 停止旧进程（`Ctrl+C` 或 `kill`）
3. 替换可执行文件，重新启动

### 方式二：API 检查并更新（experimental）

启动后通过 API 检查更新：

```bash
# 检查是否有新版本
curl -X POST http://localhost:8096/api/v1/updater/check

# 查看当前版本
curl http://localhost:8096/api/v1/updater/version
```

> `auto_update = true` 时程序会在后台自动下载并重启（实验性功能，建议生产环境手动升级）。

---

## 7. 常见问题

**Q: 启动报错 `config.toml: no such file or directory`**  
A: 确保 `config.toml` 与可执行文件在同一目录，或通过 `CONFIG_FILE` 环境变量指定路径。

**Q: 下载视频失败，提示 `yt-dlp not found`**  
A: 将 `yt-dlp` 安装后确保在 `PATH` 中，或在 `config.toml` 中填写绝对路径：
```toml
[workflow]
ytdlp_path = "/usr/local/bin/yt-dlp"
```

**Q: macOS 提示"无法验证开发者"**  
A: 在终端执行以下命令移除隔离标记：
```bash
xattr -d com.apple.quarantine ytb2bili-darwin-arm64
```

**Q: 端口 8096 被占用**  
A: 修改 `config.toml` 中的 `server.port` 为其他端口。

**Q: AI 功能不可用**  
A: 先检查是否已登录、`api2key.base_url` 是否指向可用的 `api2key` 网关、以及会员/积分状态是否正常。`[agent.llm]` 仅影响本地兜底模式。
