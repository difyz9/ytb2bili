# ytb2bili

言語: [English](README.en.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

ytb2bili は、ローカル動画の翻訳再生と YouTube から Bilibili への公開を支援するワークフロープラットフォームです。Go バックエンド、Next.js ベースの Web 管理画面、タスクオーケストレーション、字幕処理、AI メタデータ生成、字幕音声合成、同期再生、Bilibili への自動投稿機能を備えています。

## できること

- ローカル動画やオンライン動画リンクを統一された処理パイプラインに投入できます。
- ダウンロード、音声抽出、字幕生成、字幕翻訳、音声合成を自動で実行できます。
- Bilibili 向けのタイトル、概要、タグを AI 支援で生成できます。
- 処理完了後に動画と字幕を Bilibili へアップロードできます。
- Web 管理画面からタスク確認、アカウント連携、再実行、手動アップロード、AI アシスタント操作を行えます。

## 主な機能

- ローカル動画の翻訳再生と字幕・音声の同期確認。
- YouTube から Bilibili までの一連の処理フロー。
- ダウンロード、音声抽出、文字起こし、翻訳、メタデータ生成、字幕音声合成、アップロードを個別に制御可能なタスクチェーン。
- Bilibili アカウント連携と投稿対応。
- AI によるメタデータ生成とワークフロー支援。
- タスクキュー、設定、アカウント管理、履歴確認のための Web ダッシュボード。

## アーキテクチャ概要

リポジトリは主に次の 3 層で構成されています。

- 処理エンジン: Go サービスがダウンロード、文字起こし、翻訳、メタデータ生成、音声合成、Bilibili 投稿、タスク制御を担当します。
- Web 管理画面: Next.js フロントエンドが管理 UI、AI アシスタント、タスク画面、アカウント管理、設定画面を提供します。
- 実行・運用資産: 設定ファイル、Docker 関連ファイル、運用ドキュメントが開発とデプロイを支えます。

## リポジトリ構成

- `internal/`: バックエンドのアプリケーションコード、ルーティング、ワークフロー、永続化、起動処理。
- `pkg/`: LLM 連携、ツール、Bilibili 関連モジュール、共有モデルなどの再利用可能パッケージ。
- `web/`: フロントエンドアプリケーション。
- `configs/`: 設定例と補足資料。
- `docs/`: 機能、デプロイ、アーキテクチャ、トラブルシューティングに関するドキュメント。
- `docker/`: コンテナのビルドと実行用ファイル。

## 前提条件

推奨ローカル環境:

- Go 1.20+
- Node.js 18+
- npm
- MySQL 8+
- ffmpeg
- yt-dlp

使用する機能によっては、以下も必要です。

- API2Key 互換バックエンドサービス
- Microsoft Speech の認証情報
- DeepSeek などの LLM 認証情報

## クイックスタート

### 1. リポジトリを取得

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. 設定ファイルを準備

```bash
cp config.toml.example config.toml
```

`config.toml` を環境に合わせて編集してください。まずは次を確認するのがおすすめです。

- `server.*`
- `database.*`
- `workflow.*`
- `api2key.*`
- 必要に応じて `deepseek.*`

関連資料:

- [config.toml.example](config.toml.example)
- [configs/README.md](configs/README.md)

### 3. バックエンドを起動

```bash
go mod download
go run main.go
```

デフォルトでは `http://localhost:8096` で待ち受けます。

### 4. フロントエンドを起動

```bash
cd web
npm install
npm run dev
```

フロントエンドの開発サーバーは通常 `http://localhost:3000` です。

### 5. 基本的な利用手順

1. Web 管理画面を開きます。
2. Bilibili アカウントを連携します。
3. ローカル動画をアップロードするか、対応する動画リンクを送信します。
4. 必要に応じて字幕、翻訳、音声を確認します。
5. ダッシュボードで進行状況を確認します。
6. 処理済みの結果を Bilibili に公開します。

## Docker とデプロイ

コンテナ関連の詳細は次のドキュメントを参照してください。

- [docker/README.md](docker/README.md): Docker ベースのテスト/デプロイ手順。
- [README.docker.md](README.docker.md): Docker ベースの開発手順。

GitHub 向けにはメイン README を簡潔に保ち、運用の詳細は上記ドキュメントに分離する構成が適しています。

## ビルドと検証

よく使うコマンド:

```bash
make dev
make web-dev
make build
make build-linux-amd64
make test
make vet
```

検証例:

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## ドキュメント

- [Documentation Index](docs/INDEX.md)
- [Project Guide](PROJECT_GUIDE.md)
- [Configuration Guide](docs/CONFIG_GUIDE.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)

## ライセンス

[MIT License](LICENSE)

## コントリビューション

Issue と Pull Request を歓迎します。

## 連絡先

- GitHub: [@difyz9](https://github.com/difyz9)
- Repository: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
