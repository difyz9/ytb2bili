# YTB2BILI - YouTube 到 Bilibili 全自动转载系统

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?style=flat&logo=next.js)](https://nextjs.org/)
[![Docker Pulls](https://img.shields.io/docker/pulls/difyz9/ytb2bili)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-blue)](https://hub.docker.com/r/difyz9/ytb2bili)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat)](LICENSE)

**一个全自动化的视频搬运系统，从下载、字幕生成、AI翻译到定时发布的完整解决方案**

[功能特性](#-核心功能) • [一键部署](#-一键部署docker-推荐) • [配置说明](#️-配置说明) • [使用指南](#-使用指南) • [技术架构](#-技术架构)

</div>

---

## 🎯 项目简介

**YTB2BILI** 是一个专为内容创作者打造的智能视频搬运工具。通过整合 **yt-dlp**、**Whisper AI**、**DeepSeek/Gemini** 等先进技术，实现从 YouTube/TikTok 等平台到 Bilibili 的**零人工介入**全流程自动化。

### 🌟 核心亮点

- ✅ **完全自动化** - 从下载到发布，仅需提供视频链接
- 🧠 **AI 驱动** - 智能字幕生成、多语言翻译、元数据优化
- ⚡ **定时发布** - 智能调度避免频控，支持每小时自动上传
- 🔄 **失败重试** - 任务级精细化控制，支持单步骤重试
- 📊 **可视化管理** - 现代化 Web 管理界面，实时监控任务状态
- 🐳 **Docker 一键部署** - 预构建镜像，开箱即用，支持 amd64 / arm64

---

## ✨ 核心功能

### 🎬 智能任务链处理引擎

系统采用**责任链模式**，将视频处理拆解为独立任务步骤：

```
📥 下载视频  →  🎤 提取音频  →  📝 生成字幕  →  🌐 翻译字幕
     →  📷 下载封面  →  🤖 生成元数据  →  📤 上传B站  →  📝 上传字幕
```

每个步骤支持独立执行、失败重试，状态实时可查。

#### 📥 视频下载 (`yt-dlp`)
- 支持 **YouTube、TikTok、Twitter** 等 1000+ 平台
- 自动选择最高清晰度（支持 4K）
- 智能元数据提取（标题、描述、标签、播放量等）

#### 🎤 字幕生成 (`Whisper AI`)
- **本地离线生成**，无需依赖第三方 API
- 支持 90+ 种语言自动识别
- 生成带时间轴的 SRT 格式字幕

#### 🌐 智能翻译（多引擎）
- **DeepSeek** - 高质量 AI 翻译，上下文理解强，价格低廉
- **OpenAI** - GPT 系列模型
- **兼容 OpenAI 接口** - OpenRouter / 本地 Ollama 等均可接入

#### 🤖 元数据生成（AI）
- **标题优化** - 符合 B站 SEO，提升推荐率
- **简介生成** - 自动总结视频内容，添加关键词
- **标签提取** - 分析视频内容，生成相关话题标签

#### 📤 Bilibili 上传
- 大文件分片上传，支持 GB 级视频稳定上传
- 可选腾讯云 COS 加速
- 自动投稿，配置版权、分区、封面等
- 视频发布后自动追加多语言 CC 字幕

---

## 🚀 一键部署（Docker 推荐）

> **镜像已预装** yt-dlp、FFmpeg、Whisper，无需额外安装任何依赖，5 分钟即可完成部署。

### 第一步：准备配置文件

新建工作目录并获取配置文件（二选一）：

**方式 A：直接下载（无需克隆整个仓库）**

```bash
mkdir ytb2bili && cd ytb2bili

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/config.toml \
     -o config.toml

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/docker-compose.yml \
     -o docker-compose.yml
```

**方式 B：克隆本仓库后使用 `docker/` 目录下的文件**

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
```

---

### 第二步：修改 `config.toml`

> 数据库部分**无需改动**，默认已与 `docker-compose.yml` 保持一致。

打开 `config.toml`，**只需配置 LLM 翻译服务**（三选一）：

首先开启翻译开关：

```toml
[workflow]
llm_translation_enabled     = true
llm_translation_source_lang = "en"        # 原始字幕语言
llm_translation_target_lang = "zh-Hans"   # 目标语言（简体中文）
llm_translation_batch_size  = 25          # 每批翻译字幕条数
llm_translation_max_workers = 3           # 并发翻译协程数
```

然后按需选择 LLM 服务：

**方案 A — DeepSeek（推荐，中文效果好，价格低）**

> 申请 Key：https://platform.deepseek.com

```toml
[agent.llm]
provider = "deepseek"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "deepseek-chat"
```

**方案 B — OpenAI**

> 申请 Key：https://platform.openai.com

```toml
[agent.llm]
provider = "openai"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "gpt-4o-mini"   # 速度快、价格低；换 gpt-4o 效果更好
```

**方案 C — 兼容 OpenAI 接口（OpenRouter / 本地 Ollama 等）**

```toml
# OpenRouter 示例
[agent.llm]
provider = "openai"
api_key  = "sk-or-v1-xxxxxxxxxxxxxxxx"
base_url = "https://openrouter.ai/api/v1"
model    = "anthropic/claude-3.5-sonnet"
```

```toml
# 本地 Ollama 示例
[agent.llm]
provider = "openai"
api_key  = "ollama"
base_url = "http://host.docker.internal:11434/v1"
model    = "qwen2.5:14b"
```

---

### 第三步：启动服务

```bash
docker compose up -d
```

首次启动会自动拉取镜像并等待 MySQL 就绪（约 30 秒），可用以下命令查看进度：

```bash
docker compose logs -f
```

启动成功后，访问以下地址：

| 服务 | 地址 |
|------|------|
| Web 管理后台 | http://localhost:8096 |
| MySQL（调试用） | localhost:3309 |

---

### 第四步：登录 Bilibili 账号

1. 打开 `http://localhost:8096`
2. 进入 **设置 → B站账号**
3. 用 **Bilibili App** 扫码完成授权
4. Cookie 自动保存，后续无需重复登录

> **提示**：Cookie 有效期约 30 天，过期后需重新扫码登录。

---

### 第五步：添加搬运任务

1. 进入 **任务 → 新建任务**
2. 粘贴 YouTube / TikTok 等平台的视频链接
3. 点击 **创建**，系统自动依次执行：
   - 下载视频（yt-dlp）
   - 提取音频，Whisper 生成字幕
   - LLM 翻译字幕
   - AI 生成标题 / 简介 / 标签
   - 上传到 B站并追加字幕

---

### 常用命令

```bash
# 查看实时日志
docker compose logs -f ytb2bili

# 升级到最新版本
docker compose pull && docker compose up -d

# 停止服务（保留数据）
docker compose down

# 彻底清除（含数据库数据）
docker compose down -v
```

---

## ⚙️ 配置说明

### `docker-compose.yml` 结构

```yaml
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: your_password
      MYSQL_DATABASE: bili_up
      MYSQL_USER: ytb2bili
      MYSQL_PASSWORD: ytb2bili@123
    volumes:
      - ./mysql_data:/var/lib/mysql   # 数据库文件持久化到本地
    ports:
      - "3309:3306"                   # 宿主机 3309 → 容器 3306

  ytb2bili:
    image: difyz9/ytb2bili:latest     # 从 Docker Hub 拉取预构建镜像
    depends_on:
      mysql:
        condition: service_healthy    # 等待 MySQL 健康检查通过
    ports:
      - "8096:8096"
    volumes:
      - ./config.toml:/app/config.toml   # 挂载配置文件
      - ./logs:/app/do                   # 日志目录
      - ./downloads:/app/downloads       # 视频下载目录
    environment:
      - TZ=Asia/Shanghai
```

> 镜像支持 `linux/amd64` 和 `linux/arm64`（Apple Silicon Mac、树莓派 4/5）。

---

### 完整 `config.toml` 参考

<details>
<summary><b>数据库配置</b></summary>

```toml
[database]
type     = "mysql"
host     = "mysql"           # Docker Compose 服务名，无需改动
port     = 3306
user     = "ytb2bili"
password = "ytb2bili@123"
dbname   = "bili_up"
timezone = "Asia/Shanghai"
auto_migrate = true
```

</details>

<details>
<summary><b>视频处理工作流</b></summary>

```toml
[workflow]
download_dir              = "./downloads"            # 视频下载目录
ytdlp_path                = "/usr/local/bin/yt-dlp"  # 镜像内已预装
ffmpeg_path               = "/usr/bin/ffmpeg"         # 镜像内已预装

# 代理（可选，用于访问受限地区视频）
# proxy_url = "http://127.0.0.1:7890"

# Cookies 文件路径（可选，用于下载需要登录才能看的视频）
# cookies_file = "/path/to/cookies.txt"

# LLM 翻译
llm_translation_enabled      = true
llm_translation_batch_size   = 25
llm_translation_max_workers  = 3
llm_translation_context_size = 2
llm_translation_source_lang  = "en"
llm_translation_target_lang  = "zh-Hans"
```

</details>

<details>
<summary><b>LLM 翻译配置（三选一）</b></summary>

```toml
# DeepSeek（推荐）
[agent.llm]
provider = "deepseek"
api_key  = "sk-..."
model    = "deepseek-chat"

# OpenAI
# [agent.llm]
# provider = "openai"
# api_key  = "sk-..."
# model    = "gpt-4o-mini"

# 自定义兼容接口
# [agent.llm]
# provider = "openai"
# api_key  = "..."
# base_url = "https://your-endpoint/v1"
# model    = "your-model"
```

</details>

<details>
<summary><b>腾讯云 COS 配置（可选，大文件加速）</b></summary>

```toml
[tencos]
enabled        = true
cos_bucket_url = "https://your-bucket.cos.ap-guangzhou.myqcloud.com"
cos_secret_id  = "AKIDxxxxxxxx"
cos_secret_key = "xxxxxxxx"
cos_region     = "ap-guangzhou"
cos_bucket     = "your-bucket-name"
sub_app_id     = "125xxxxxx"
```

启用 COS 后，上传速度可提升 3-5 倍，支持超大文件（>4GB）分片续传。

</details>

<details>
<summary><b>自动更新配置</b></summary>

```toml
[updater]
enabled        = true
auto_update    = false   # 建议设为 false，手动执行 docker compose pull 更新
check_interval = 24      # 检查更新间隔（小时）
```

</details>

---

## 📖 使用指南

### 任务处理流程

系统创建任务后自动执行以下步骤：

```
1. 📥 下载视频           [约 1-5 分钟]
2. 🎤 提取音频           [约 10-30 秒]
3. 📝 Whisper 生成字幕   [约 1-10 分钟，取决于视频长度]
4. 🌐 LLM 翻译字幕       [约 30 秒-2 分钟]
5. 📷 下载封面           [约 5-10 秒]
6. 🤖 AI 生成元数据      [约 20-60 秒]
7. 📤 上传到 Bilibili    [约 5-30 分钟，取决于视频大小]
8. 📝 追加 CC 字幕       [约 10-30 秒]
```

### 失败重试

如果某个步骤失败，在"任务详情"页面找到红色标记的步骤，点击"重试"即可单步重试，无需从头开始。

**常见失败原因**：
- **下载失败** - 视频已删除或地区限制 → 在配置中添加 `proxy_url`
- **翻译失败** - API Key 无效或额度耗尽 → 检查 Key 或更换引擎
- **上传失败** - Cookie 过期 → 重新扫码登录；大文件 → 启用 COS 加速

### 支持的视频平台

通过 yt-dlp 支持 1000+ 平台，常用平台包括：

- YouTube (`youtube.com`, `youtu.be`)
- TikTok (`tiktok.com`)
- Twitter / X (`twitter.com`, `x.com`)
- Instagram (`instagram.com`)
- [完整平台列表](https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md)

---

## 🏗️ 技术架构

### 后端（Golang）

| 组件 | 技术选型 | 用途 |
|------|---------|------|
| Web 框架 | Gin | HTTP 路由和中间件 |
| 依赖注入 | Uber FX | 模块化依赖管理 |
| ORM | GORM v2 | 支持 MySQL / PostgreSQL |
| 定时任务 | Robfig Cron v3 | 秒级精度调度器 |
| 日志系统 | Zap + Lumberjack | 结构化日志与自动轮转 |
| 认证鉴权 | JWT + Cookie | 双重认证机制 |

**核心模块**：
- `internal/chain_task` - 任务链处理引擎（责任链模式）
- `internal/handler` - HTTP API 路由控制器
- `pkg/translator` - 多翻译引擎（工厂模式）
- `pkg/subtitle` - 字幕生成和格式转换
- `pkg/cos` - 腾讯云 COS 客户端封装

### 前端（Next.js 15）

| 组件 | 技术选型 |
|------|---------|
| 框架 | Next.js 15 (App Router) |
| 语言 | TypeScript 5 |
| UI | TailwindCSS 3 |
| 状态管理 | Zustand |
| HTTP 客户端 | Axios |

### 外部工具（镜像内预装）

- **yt-dlp** - 视频下载
- **FFmpeg** - 音视频处理
- **Whisper** - 语音识别（本地运行，无额外 API 费用）

---

## 本地编译部署

如需从源码构建，需要以下依赖：

- Go 1.24+
- Node.js 18+
- yt-dlp
- FFmpeg
- Whisper（`pip3 install openai-whisper`）
- MySQL 8.0+ 或 PostgreSQL 14+

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili

# 编译前后端
make build

# 复制并配置文件
cp config.toml.example config.toml
# 编辑 config.toml，配置数据库和 LLM

# 启动服务
./ytb2bili
```

---

## 🐛 常见问题

**Q: 首次启动很慢？**

首次 `docker compose up -d` 会拉取镜像（含 Whisper 模型），根据网速需要几分钟。之后启动只需几秒。

**Q: 视频下载失败 / 提示地区限制？**

在 `config.toml` 中配置代理：

```toml
[workflow]
proxy_url = "http://127.0.0.1:7890"   # 替换为实际代理地址
```

**Q: MySQL 端口是 3309 还是 3306？**

容器内部使用 3306，宿主机映射到 3309（避免与本地已运行的 MySQL 冲突）。`config.toml` 中连接的是容器内部端口，无需修改。

**Q: 如何查看 Whisper 字幕生成进度？**

```bash
docker compose logs -f ytb2bili
```

**Q: 如何升级到新版本？**

```bash
docker compose pull && docker compose up -d
```

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

提交规范参考 [Conventional Commits](https://www.conventionalcommits.org/)：
- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档更新
- `refactor:` 代码重构

---

## 📄 许可证

本项目采用 [MIT License](LICENSE) 开源协议。

---

## 🙏 致谢

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - 视频下载核心
- [OpenAI Whisper](https://github.com/openai/whisper) - 语音识别
- [bilibili-go-sdk](https://github.com/difyz9/bilibili-go-sdk) - B站 API 封装
- [Gin](https://github.com/gin-gonic/gin) - Web 框架
- [Next.js](https://nextjs.org/) - 前端框架
- [GORM](https://gorm.io/) - ORM 框架

---

<div align="center">

**如果这个项目对你有帮助，请点一个 ⭐Star！**

**💬 QQ 交流群：773066052**

<img src="img/220421_706.png" alt="QQ群二维码" width="180"/>
<img src="img/751763091471.jpg" alt="微信二维码" width="180"/>

Made with ❤️ by [difyz9](https://github.com/difyz9)

</div>
