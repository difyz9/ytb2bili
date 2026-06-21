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

Language: [English](README.en.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

ytb2bili is a workflow platform for local video translation playback and YouTube-to-Bilibili publishing. It provides a Go backend, a Next.js web console, task orchestration, subtitle processing, AI metadata generation, subtitle voice synthesis, synchronized playback, and Bilibili upload automation.

## What ytb2bili does

- Imports local videos or online video links into a unified processing pipeline.
- Downloads media, extracts audio, generates subtitles, translates subtitles, and synthesizes voice tracks.
- Produces Bilibili-ready titles, descriptions, and tags with AI-assisted workflows.
- Uploads videos and subtitles to Bilibili after processing is complete.
- Exposes a web console for task monitoring, account linking, retries, manual uploads, and assistant-driven operations.

## Core Features

- Local video translation playback with synchronized subtitle and audio review.
- End-to-end YouTube to Bilibili processing pipeline.
- Configurable task chain: download, audio extraction, transcription, translation, metadata generation, subtitle voiceover, and upload.
- Bilibili account linking and publishing support.
- AI-assisted content generation for metadata and workflow operations.
- Web dashboard for settings, queue management, account management, and task history.

## Architecture

The repository is organized into three main layers:

- Processing engine: Go services handle download, transcription, translation, metadata generation, voice synthesis, Bilibili upload, and task orchestration.
- Web console: the Next.js frontend provides the management UI, assistant UI, task views, account management, and settings.
- Runtime and deployment assets: configuration files, Docker assets, and operational docs support local development and deployment.

## Repository Layout

- `internal/`: backend application code, routing, workflows, persistence, and bootstrap logic.
- `pkg/`: reusable packages, including LLM integrations, tools, Bilibili-related modules, and shared models.
- `web/`: frontend application.
- `configs/`: configuration examples and related notes.
- `docs/`: feature docs, deployment docs, architecture docs, and troubleshooting guides.
- `docker/`: container build and runtime files.

## Prerequisites

Recommended local environment:

- Go 1.20+
- Node.js 18+
- npm
- MySQL 8+
- ffmpeg
- yt-dlp

Depending on the features you enable, you may also need:

- API2Key-compatible backend services
- Microsoft Speech credentials for subtitle voice synthesis
- DeepSeek or other LLM credentials for translation and metadata generation

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. Prepare configuration

```bash
cp config.toml.example config.toml
```

Edit `config.toml` to match your environment. Start with:

- `server.*`
- `database.*`
- `workflow.*`
- `api2key.*`
- optional `deepseek.*`

See also:

- [config.toml.example](config.toml.example)
- [configs/README.md](configs/README.md)

### 3. Start the backend

```bash
go mod download
go run main.go
```

The backend listens on `http://localhost:8096` by default.

### 4. Start the frontend

```bash
cd web
npm install
npm run dev
```

The frontend dev server runs on `http://localhost:3000`.

### 5. Typical workflow

1. Open the web console.
2. Link your Bilibili account.
3. Upload a local video or submit a supported video link.
4. Review subtitles, translations, and synthesized voice if needed.
5. Track task progress in the dashboard.
6. Publish the processed result to Bilibili.

## Docker and Deployment

This repository contains multiple container-related workflows:

- [docker/README.md](docker/README.md): Docker-based test/deployment workflow.
- [README.docker.md](README.docker.md): Docker-based development workflow.

If you are publishing the project on GitHub, keep the main README concise and use those files for container-specific operational details.

## Community

If you want release updates, implementation discussion, or direct troubleshooting help, use the GitHub repository or the community QR codes below.

<p>
	<img src="img/220421_706.png" alt="QQ group QR code" width="280" />
	<img src="img/751763091471.jpg" alt="WeChat contact QR code" width="280" />
</p>

## Build and Validation

Common commands:

```bash
make dev
make web-dev
make build
make build-linux-amd64
make test
make vet
```

Validation examples:

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## Documentation

- [Documentation Index](docs/INDEX.md)
- [Project Guide](PROJECT_GUIDE.md)
- [Configuration Guide](docs/CONFIG_GUIDE.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)

## License

[MIT License](LICENSE)

## Contributing

Issues and pull requests are welcome.

## Contact

- GitHub: [@difyz9](https://github.com/difyz9)
- Repository: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
