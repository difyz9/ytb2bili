# difyz9/ytb2bili

> YouTube / TikTok → Bilibili 全自动搬运系统  
> 支持 AI 字幕生成、LLM 翻译、定时发布，开箱即用。

[![Docker Pulls](https://img.shields.io/docker/pulls/difyz9/ytb2bili)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Image Size](https://img.shields.io/docker/image-size/difyz9/ytb2bili/latest)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-blue)](https://hub.docker.com/r/difyz9/ytb2bili)

---

## 快速开始

### 1. 创建工作目录并下载配置文件

```bash
mkdir ytb2bili && cd ytb2bili

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/config.toml \
     -o config.toml

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/docker-compose.yml \
     -o docker-compose.yml
```

### 2. 启动服务

```bash
docker compose up -d
```

服务就绪后访问 **http://localhost:8096**，用 B 站 App 扫码登录即可开始搬运。

---

## 配置 LLM 翻译（可选）

打开 `config.toml`，找到 `[workflow]` 和 `[agent.llm]` 两个节进行配置。

### 开启翻译开关

```toml
[workflow]
llm_translation_enabled    = true   # 必须设为 true 才会调用 LLM
llm_translation_source_lang = "en"
llm_translation_target_lang = "zh-Hans"
llm_translation_batch_size  = 25    # 每批翻译字幕条数
llm_translation_max_workers = 3     # 并发翻译协程数
```

---

### 使用 DeepSeek

> 官网申请 Key：https://platform.deepseek.com

```toml
[agent.llm]
provider = "deepseek"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "deepseek-chat"
# model  = "deepseek-reasoner"   # 推理模型，效果更好但较慢
```

---

### 使用 OpenAI

> 官网申请 Key：https://platform.openai.com

```toml
[agent.llm]
provider = "openai"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "gpt-4o-mini"   # 推荐：速度快、价格低
# model  = "gpt-4o"        # 质量更高
```

---

### 使用 OpenAI 兼容接口（OpenRouter / 本地 Ollama 等）

任何兼容 OpenAI Chat Completions API 的服务均可接入，通过 `base_url` 指定端点：

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

## 完整 config.toml 参考

```toml
# ── 服务 ──────────────────────────────────────────
[server]
host = "0.0.0.0"
port = 8096

# ── 数据库（与 docker-compose.yml 默认值保持一致）──
[database]
type     = "mysql"
host     = "mysql"
port     = 3306
user     = "ytb2bili"
password = "ytb2bili@123"
dbname   = "bili_up"
timezone = "Asia/Shanghai"
auto_migrate = true

# ── 工作流 ────────────────────────────────────────
[workflow]
download_dir             = "./media"
ytdlp_path               = "/usr/local/bin/yt-dlp"
ffmpeg_path              = "/usr/bin/ffmpeg"
llm_translation_enabled  = true          # false = 不调用 LLM，直接用原字幕
llm_translation_batch_size   = 25
llm_translation_max_workers  = 3
llm_translation_source_lang  = "en"
llm_translation_target_lang  = "zh-Hans"

# ── LLM（三选一）────────────────────────────────
# DeepSeek
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

---

## 常用命令

```bash
# 查看实时日志
docker compose logs -f ytb2bili

# 升级到最新版本
docker compose pull && docker compose up -d

# 停止（保留数据）
docker compose down

# 完全清除（含数据库）
docker compose down -v
```

---

## 支持架构

| 架构 | 适用设备 |
|------|---------|
| `linux/amd64` | 普通 x86 服务器、VPS、PC |
| `linux/arm64` | Apple Silicon Mac（Rosetta-free）、树莓派 4/5、ARM 服务器 |

---

## 相关链接

- 源码仓库：[difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
- Docker 配置仓库：[difyz9/ytb2bili-docker](https://github.com/difyz9/ytb2bili-docker)
- 问题反馈：[GitHub Issues](https://github.com/difyz9/ytb2bili/issues)
