# ytb2bili

<p align="center">
	<img src="web/public/logo.png" alt="ytb2bili logo" width="640" />
</p>

<p align="center">
	<a href="https://github.com/difyz9/ytb2bili/releases"><img alt="GitHub release" src="https://img.shields.io/github/v/release/difyz9/ytb2bili?display_name=tag" /></a>
	<a href="https://github.com/difyz9/ytb2bili/stargazers"><img alt="GitHub stars" src="https://img.shields.io/github/stars/difyz9/ytb2bili?style=social" /></a>
	<a href="https://github.com/difyz9/ytb2bili/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/difyz9/ytb2bili" /></a>
	<a href="https://github.com/difyz9/ytb2bili/commits/main"><img alt="Last commit" src="https://img.shields.io/github/last-commit/difyz9/ytb2bili" /></a>
</p>

语言: [English](README.en.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

ytb2bili 是一个面向本地视频翻译播放与 YouTube 到 Bilibili 发布的工作流平台，提供 Go 后端、Next.js Web 管理后台、任务链编排、字幕处理、AI 文案生成、字幕配音、音视频同步播放和 B 站上传自动化能力。

## 项目能做什么

- 将本地视频或在线视频链接导入统一处理流水线。
- 自动完成下载、提取音频、生成字幕、翻译字幕和合成配音。
- 通过 AI 生成适合 B 站投稿的标题、简介和标签。
- 在处理完成后上传视频和字幕到 Bilibili。
- 通过 Web 后台完成任务查看、账号绑定、重试、手动上传和 AI 助手操作。

## 核心特性

- 本地视频翻译播放与字幕、配音同步校对。
- YouTube 到 Bilibili 的端到端处理流程。
- 可配置任务链：下载、提取音频、转录、翻译、生成元数据、字幕配音、上传。
- 支持 Bilibili 账号绑定与发布。
- 支持 AI 辅助元数据生成和工作流操作。
- 提供任务队列、设置、账号管理和任务历史等后台页面。

## 架构概览

仓库主要由三部分组成：

- 处理引擎：Go 服务负责下载、转录、翻译、元数据生成、语音合成、B 站上传和任务编排。
- Web 管理后台：Next.js 前端提供管理界面、AI 助手、任务视图、账号管理和设置页面。
- 运行与部署支撑：配置文件、Docker 资源和运维文档支撑本地开发与部署。

## 仓库结构

- `internal/`: 后端应用代码、路由、工作流、持久化和启动装配。
- `pkg/`: 可复用包，包括 LLM 集成、工具、B 站相关模块和共享模型。
- `web/`: 前端应用。
- `configs/`: 配置示例与说明。
- `docs/`: 功能、部署、架构和排障文档。
- `docker/`: 容器构建与运行文件。

## 环境要求

推荐本地环境：

- Go 1.20+
- Node.js 18+
- npm
- MySQL 8+
- ffmpeg
- yt-dlp

如果你启用了相关能力，通常还需要：

- API2Key 兼容后端服务
- 微软语音服务凭证
- DeepSeek 或其他 LLM 的访问凭证

## 快速开始

### 1. 克隆仓库

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. 准备配置

```bash
cp config.toml.example config.toml
```

根据你的环境修改 `config.toml`。建议优先配置：

- `server.*`
- `database.*`
- `workflow.*`
- `api2key.*`
- 可选的 `deepseek.*`

相关入口：

- [config.toml.example](config.toml.example)
- [configs/README.md](configs/README.md)

### 3. 启动后端

```bash
go mod download
go run main.go
```

默认地址为 `http://localhost:8096`。

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

前端开发服务器默认地址为 `http://localhost:3000`。

### 5. 常见使用流程

1. 打开 Web 管理后台。
2. 绑定 Bilibili 账号。
3. 上传本地视频或提交支持的视频链接。
4. 按需检查字幕、翻译和配音效果。
5. 在后台查看任务进度。
6. 将处理结果发布到 Bilibili。

## Docker 与部署

仓库中已经包含多种容器相关说明：

- [docker/README.md](docker/README.md): Docker 测试/部署流程。
- [README.docker.md](README.docker.md): Docker 开发环境流程。

如果你是发布到 GitHub，建议主 README 保持精简，容器细节放在上述文档中维护。

## 社区交流

如果你想获取版本更新、排障支持或交流使用方式，可以直接通过 GitHub 仓库联系，也可以扫描下面的社群二维码。

<p>
	<img src="img/220421_706.png" alt="QQ群二维码" width="280" />
	<img src="img/751763091471.jpg" alt="微信联系二维码" width="280" />
</p>

## 构建与验证

常用命令：

```bash
make dev
make web-dev
make build
make build-linux-amd64
make test
make vet
```

验证示例：

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## 文档索引

- [文档索引](docs/INDEX.md)
- [项目指南](PROJECT_GUIDE.md)
- [配置指南](docs/CONFIG_GUIDE.md)
- [部署指南](DEPLOYMENT_GUIDE.md)

## 许可证

[MIT License](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request。

## 联系方式

- GitHub: [@difyz9](https://github.com/difyz9)
- 仓库地址: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
