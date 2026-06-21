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

ytb2bili is a video workflow system for local video translation playback and YouTube-to-Bilibili publishing. It combines a Go backend, a Next.js web console, subtitle processing, AI copy generation, subtitle voice synthesis, synchronized audio/video playback, and Bilibili upload automation.

## Overview

- Local video translation and review with subtitles, voiceover, and synchronized playback.
- End-to-end YouTube to Bilibili workflow: download, transcription, translation, metadata generation, upload, and subtitle upload.
- Task-based pipeline with configurable steps for downloading, audio extraction, transcription, translation, and publishing.
- Web console for task tracking, account linking, settings, retries, manual uploads, and assistant-driven operations.

## Read The Full README

- [English README](README.en.md)
- [简体中文 README](README.zh-CN.md)
- [日本語 README](README.ja.md)
- [한국어 README](README.ko.md)

## Quick Links

- [Documentation Index](docs/INDEX.md)
- [Configuration Example](config.toml.example)
- [Docker Test/Deployment Notes](docker/README.md)
- [Docker Development Guide](README.docker.md)
- [Project Guide](PROJECT_GUIDE.md)

## Community

If you want updates, troubleshooting help, or product discussion, you can reach out through the GitHub repository or scan the community QR codes below.

<p>
	<img src="img/220421_706.png" alt="QQ group QR code" width="280" />
	<img src="img/751763091471.jpg" alt="WeChat contact QR code" width="280" />
</p>

## Repository Layout

- `internal/`: backend application code, handlers, workflows, storage, and bootstrap logic.
- `pkg/`: reusable packages such as LLM integrations, tools, and shared models.
- `web/`: Next.js frontend for the management console.
- `configs/`: configuration examples and related notes.
- `docs/`: deployment, feature, architecture, and troubleshooting documentation.
- `docker/`: container build and runtime files.

## Local Development

```bash
cp config.toml.example config.toml
go run main.go

cd web
npm install
npm run dev
```

Default local URLs:

- Backend: `http://localhost:8096`
- Frontend: `http://localhost:3000`

## Build

```bash
make build
make build-linux-amd64
make test
```

## License

[MIT License](LICENSE)

## Contributing

Issues and pull requests are welcome.

## Contact

- GitHub: [@difyz9](https://github.com/difyz9)
- Repository: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
