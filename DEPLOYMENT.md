# YTB2Bili 开发和部署指南

参考项目：[golang-nextjs-portable](https://github.com/dstotijn/golang-nextjs-portable)

## 架构说明

本项目采用 **Go + Next.js** 混合架构：

- **开发模式**：前后端分离，Next.js 独立运行（端口 3000），通过代理访问 Go 后端（端口 5688）
- **生产模式**：Next.js 静态文件嵌入到 Go 二进制文件中，单一可执行文件部署

## 快速开始

### 开发模式

#### 1. 启动后端服务

```bash
go run main.go webembed.go
```

后端运行在 `http://localhost:8096`

#### 2. 启动前端服务（另一个终端）

```bash
cd web
npm install
npm run dev
```

前端运行在 `http://localhost:3000`，API 请求自动代理到后端。

### 生产构建

#### 使用 Makefile（推荐）

```bash
# 完整构建（包含前端静态文件）
make -f Makefile.web build-with-web

# 运行
./ytb2bili
```

#### 手动构建

```bash
# 1. 构建前端静态文件
cd web
npm install
cp next.config.export.js next.config.js
# 临时移除动态路由（App Router 限制）
mv src/app/dashboard/videos/[id] src/app/dashboard/videos/_id_backup
npm run build
# 恢复
mv src/app/dashboard/videos/_id_backup src/app/dashboard/videos/[id]
cp next.config.dev.backup.js next.config.js

# 2. 构建 Go 二进制
cd ..
go build -o ytb2bili main.go webembed.go

# 3. 运行
./ytb2bili
```

## Makefile 命令

```bash
make -f Makefile.web web-install      # 安装前端依赖
make -f Makefile.web web-dev           # 启动前端开发服务器
make -f Makefile.web web-export        # 导出前端静态文件
make -f Makefile.web build             # 仅构建 Go 二进制（不含前端）
make -f Makefile.web build-with-web    # 完整构建（含前端）
make -f Makefile.web clean             # 清理构建产物
```

## 配置文件说明

### Next.js 配置

- `web/next.config.js` - **开发模式配置**
  - 包含 API 代理（`/api` → `http://localhost:8096/api`）
  - 图片优化启用
  - 适用于 `npm run dev`

- `web/next.config.export.js` - **生产模式配置**
  - 启用静态导出 (`output: 'export'`)
  - 图片优化禁用（静态导出不支持）
  - 输出到 `out/` 目录
  - 适用于嵌入式部署

### Go 嵌入配置

`webembed.go` 文件包含嵌入指令：

```go
//go:embed all:web/out
var WebFS embed.FS
```

这会将 `web/out/` 目录的所有静态文件嵌入到编译后的二进制文件中。

## 技术细节

### 静态文件路由

生产模式下，Go 服务器同时提供：
- **API 路由**：`/api/*` - 后端 API
- **静态资源**：`/static/*` - 用户上传的视频、字幕等
- **Web 应用**：所有其他路由 - Next.js 静态页面

路由优先级：API > 静态资源 > Web 应用

### 动态路由处理

Next.js App Router 的动态路由（如 `/dashboard/videos/[id]`）在静态导出时有限制：

- **问题**：需要 `generateStaticParams()` 预渲染所有路由
- **当前方案**：构建时临时移除动态路由页面，运行时由客户端路由处理
- **未来改进**：考虑使用 Pages Router 或服务端渲染

### 依赖注入

项目使用 Uber Fx 进行依赖注入，`WebFS` 通过 `fx.Supply()` 提供：

```go
func NewApp(webFS embed.FS) *fx.App {
    return fx.New(
        fx.Supply(webFS), // 供全局使用
        // ...其他模块
    )
}
```

## 环境变量

### 后端

在 `configs/.env` 中配置：

```env
SERVER_HOST=0.0.0.0
SERVER_PORT=5688
DEBUG=true
```

### 前端

构建时可指定 API 地址：

```bash
NEXT_PUBLIC_API_URL=https://api.example.com npm run build
```

## 部署

### Docker 部署

```dockerfile
# 多阶段构建示例
FROM node:18-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN cp next.config.export.js next.config.js && \
    npm run build

FROM golang:1.21-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-builder /app/web/out ./web/out
RUN go build -o ytb2bili main.go webembed.go

FROM alpine:latest
WORKDIR /app
COPY --from=go-builder /app/ytb2bili ./
COPY configs/ ./configs/
EXPOSE 5688
CMD ["./ytb2bili"]
```

### 直接部署

只需将 `ytb2bili` 二进制文件和 `configs/` 目录复制到服务器：

```bash
scp ytb2bili configs/.env user@server:/opt/ytb2bili/
ssh user@server "cd /opt/ytb2bili && ./ytb2bili"
```

## 故障排除

### 构建错误：embed 路径无效

确保运行 `go build` 时包含 `webembed.go`：

```bash
go build -o ytb2bili main.go webembed.go
```

### 前端 404 错误

检查：
1. `web/out/` 目录是否存在且包含文件
2. 构建时是否使用了正确的配置文件
3. Go 服务器是否正确注册了 Web 路由

### API 请求失败

开发模式：检查 `next.config.js` 中的代理配置
生产模式：检查前端代码中的 API 基础 URL

## 最佳实践

1. **开发时**始终使用 `npm run dev`，不要用静态导出配置
2. **构建前**确保前端依赖已安装 (`npm install`)
3. **提交代码**前恢复 `next.config.js` 为开发配置
4. **生产环境**使用编译后的二进制文件，不要用 `go run`
5. **更新前端**后记得重新构建并嵌入
