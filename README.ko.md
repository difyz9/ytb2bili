# ytb2bili

언어: [English](README.en.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

ytb2bili는 로컬 비디오 번역 재생과 YouTube에서 Bilibili로의 게시를 지원하는 워크플로 플랫폼입니다. Go 백엔드, Next.js 기반 웹 콘솔, 작업 오케스트레이션, 자막 처리, AI 메타데이터 생성, 자막 음성 합성, 동기 재생, Bilibili 업로드 자동화 기능을 제공합니다.

## 이 프로젝트가 하는 일

- 로컬 비디오 또는 온라인 비디오 링크를 하나의 처리 파이프라인으로 가져옵니다.
- 다운로드, 오디오 추출, 자막 생성, 자막 번역, 음성 합성을 자동으로 수행합니다.
- Bilibili 업로드용 제목, 설명, 태그를 AI 지원으로 생성합니다.
- 처리 완료 후 비디오와 자막을 Bilibili에 업로드합니다.
- 웹 콘솔에서 작업 확인, 계정 연동, 재시도, 수동 업로드, AI 어시스턴트 작업을 수행할 수 있습니다.

## 핵심 기능

- 로컬 비디오 번역 재생과 자막/음성 싱크 검수.
- YouTube에서 Bilibili까지 이어지는 전체 처리 파이프라인.
- 다운로드, 오디오 추출, 전사, 번역, 메타데이터 생성, 자막 음성 합성, 업로드를 개별적으로 제어할 수 있는 작업 체인.
- Bilibili 계정 연동 및 게시 지원.
- AI 기반 메타데이터 생성과 워크플로 보조 기능.
- 작업 큐, 설정, 계정 관리, 이력 확인을 위한 웹 대시보드.

## 아키텍처 개요

저장소는 크게 세 부분으로 구성됩니다.

- 처리 엔진: Go 서비스가 다운로드, 전사, 번역, 메타데이터 생성, 음성 합성, Bilibili 업로드, 작업 제어를 담당합니다.
- 웹 콘솔: Next.js 프런트엔드가 관리 UI, AI 어시스턴트, 작업 화면, 계정 관리, 설정 화면을 제공합니다.
- 실행 및 운영 자산: 설정 파일, Docker 자원, 운영 문서가 개발과 배포를 지원합니다.

## 저장소 구조

- `internal/`: 백엔드 애플리케이션 코드, 라우팅, 워크플로, 영속성, 부트스트랩 로직.
- `pkg/`: LLM 연동, 도구, Bilibili 관련 모듈, 공용 모델 등 재사용 가능한 패키지.
- `web/`: 프런트엔드 애플리케이션.
- `configs/`: 설정 예제와 관련 설명.
- `docs/`: 기능, 배포, 아키텍처, 트러블슈팅 문서.
- `docker/`: 컨테이너 빌드 및 실행 파일.

## 사전 요구 사항

권장 로컬 환경:

- Go 1.20+
- Node.js 18+
- npm
- MySQL 8+
- ffmpeg
- yt-dlp

활성화하는 기능에 따라 다음도 필요할 수 있습니다.

- API2Key 호환 백엔드 서비스
- Microsoft Speech 자격 증명
- DeepSeek 또는 기타 LLM 자격 증명

## 빠른 시작

### 1. 저장소 클론

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. 설정 준비

```bash
cp config.toml.example config.toml
```

환경에 맞게 `config.toml` 을 수정하세요. 먼저 다음 항목을 확인하는 것이 좋습니다.

- `server.*`
- `database.*`
- `workflow.*`
- `api2key.*`
- 필요 시 `deepseek.*`

관련 문서:

- [config.toml.example](config.toml.example)
- [configs/README.md](configs/README.md)

### 3. 백엔드 실행

```bash
go mod download
go run main.go
```

기본 주소는 `http://localhost:8096` 입니다.

### 4. 프런트엔드 실행

```bash
cd web
npm install
npm run dev
```

프런트엔드 개발 서버는 일반적으로 `http://localhost:3000` 에서 실행됩니다.

### 5. 일반적인 사용 흐름

1. 웹 콘솔을 엽니다.
2. Bilibili 계정을 연동합니다.
3. 로컬 비디오를 업로드하거나 지원되는 비디오 링크를 제출합니다.
4. 필요하면 자막, 번역, 음성을 검토합니다.
5. 대시보드에서 작업 진행 상황을 확인합니다.
6. 처리된 결과를 Bilibili에 게시합니다.

## Docker 및 배포

컨테이너 관련 상세 내용은 아래 문서를 참고하세요.

- [docker/README.md](docker/README.md): Docker 기반 테스트/배포 절차.
- [README.docker.md](README.docker.md): Docker 기반 개발 절차.

GitHub 공개용으로는 메인 README를 간결하게 유지하고, 운영 세부 사항은 위 문서로 분리하는 구성이 적합합니다.

## 빌드 및 검증

자주 사용하는 명령:

```bash
make dev
make web-dev
make build
make build-linux-amd64
make test
make vet
```

검증 예시:

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## 문서

- [Documentation Index](docs/INDEX.md)
- [Project Guide](PROJECT_GUIDE.md)
- [Configuration Guide](docs/CONFIG_GUIDE.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)

## 라이선스

[MIT License](LICENSE)

## 기여

Issue 와 Pull Request 를 환영합니다.

## 연락처

- GitHub: [@difyz9](https://github.com/difyz9)
- Repository: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
