# ytb2bili — YouTube 自动搬运到 Bilibili

[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Docker Pulls](https://img.shields.io/docker/pulls/difyz9/ytb2bili)](https://hub.docker.com/r/difyz9/ytb2bili)

全自动视频搬运工具：下载 → AI 字幕 → 翻译 → 上传 B 站，一条链路完成。

---

## 快速开始

### 第一步：准备文件

新建工作目录，下载配置文件和 compose 文件：

```bash
mkdir ytb2bili && cd ytb2bili

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/config.toml -o config.toml
curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/docker-compose.yml -o docker-compose.yml
```

也可以直接克隆本仓库：

```bash
git clone https://github.com/difyz9/ytb2bili-docker.git
cd ytb2bili-docker
cp config.toml.example config.toml
```

---

### 第二步：修改 `config.toml`

用任意编辑器打开 `config.toml`，**数据库部分无需改动**，默认已与 `docker-compose.yml` 保持一致。

#### 配置 LLM 翻译（三选一）

首先确保翻译开关已打开：

```toml
[workflow]
llm_translation_enabled     = true
llm_translation_source_lang = "en"       # 原始字幕语言
llm_translation_target_lang = "zh-Hans"  # 目标语言（简体中文）
```

然后按需选择 LLM 服务：

**方案 A — DeepSeek（推荐，中文效果好，价格低）**

> Key 申请：https://platform.deepseek.com

```toml
[agent.llm]
provider = "deepseek"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "deepseek-chat"
```

**方案 B — OpenAI**

> Key 申请：https://platform.openai.com

```toml
[agent.llm]
provider = "openai"
api_key  = "sk-xxxxxxxxxxxxxxxxxxxxxxxx"
model    = "gpt-4o-mini"
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

首次启动会自动拉取镜像并等待 MySQL 就绪（约 30 秒），用以下命令查看进度：

```bash
docker compose logs -f
```

启动成功后：

| 服务 | 地址 |
|------|------|
| Web 管理后台 | http://localhost:8096 |
| MySQL（调试用） | localhost:3309 |

---

### 第四步：B 站账号登录

1. 打开 http://localhost:8096
2. 进入 **设置 → B站账号**
3. 用 B 站 App 扫码完成授权
4. Cookie 自动保存，后续无需重复登录

---

### 第五步：添加搬运任务

1. 进入 **任务 → 新建任务**
2. 粘贴 YouTube / TikTok 等平台视频链接
3. 点击 **创建**，系统自动依次执行：
   - 下载视频（yt-dlp）
   - 提取音频，生成字幕（Whisper）
   - 翻译字幕（LLM）
   - 生成标题 / 简介 / 标签（AI）
   - 上传到 B 站并追加字幕

---

## 常用命令

```bash
# 查看实时日志
docker compose logs -f ytb2bili

# 停止服务（保留数据）
docker compose down

# 升级到最新镜像
docker compose pull && docker compose up -d

# 彻底清除（含数据库数据）
docker compose down -v
```

---

## docker-compose.yml 说明

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
      - ./mysql_data:/var/lib/mysql
    ports:
      - "3309:3306"

  ytb2bili:
    image: difyz9/ytb2bili:latest   # 从 Docker Hub 拉取预构建镜像
    depends_on:
      mysql:
        condition: service_healthy
    ports:
      - "8096:8096"
    volumes:
      - ./config.toml:/app/config.toml   # 配置文件挂载
      - ./logs:/app/do                   # 日志目录
      - ./downloads:/app/downloads       # 视频下载目录
    environment:
      - TZ=Asia/Shanghai
```

镜像支持 `linux/amd64` 和 `linux/arm64`（Apple Silicon / 树莓派）。

---

## 代理配置（可选）

如果服务器访问 YouTube 受限，在 `config.toml` 中添加：

```toml
[workflow]
proxy_url = "http://127.0.0.1:7890"   # 替换为实际代理地址
```

---

## 目录结构

```
ytb2bili/
├── config.toml          # 主配置文件（需手动创建/修改）
├── docker-compose.yml   # Docker 编排文件
├── downloads/           # 视频、字幕下载目录（自动创建）
├── logs/                # 运行日志（自动创建）
└── mysql_data/          # 数据库持久化（自动创建）
```

---

## 相关链接

- Docker Hub 镜像：https://hub.docker.com/r/difyz9/ytb2bili
- 主项目源码：https://github.com/difyz9/ytb2bili
- 本仓库（Docker 配置）：https://github.com/difyz9/ytb2bili-docker
