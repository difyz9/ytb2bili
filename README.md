# Bilibili 视频上传后端系统

一个功能完整的 B站视频自动化处理和上传系统，支持视频下载、字幕生成/翻译、元数据生成和自动上传。

## ✨ 核心功能

### 1. 视频处理任务链
- ✅ **视频下载** - 从各大平台下载视频
- ✅ **字幕生成** - 使用 Whisper AI 自动生成字幕
- ✅ **字幕翻译** - 支持百度翻译和 DeepSeek 翻译
- ✅ **元数据生成** - AI 生成视频标题、描述、标签
- ✅ **上传到 B站** - 自动上传视频和字幕
- ✅ **任务步骤追踪** - 实时追踪每个步骤的状态

### 2. 视频详情和管理
- ✅ **视频列表** - 查看所有处理的视频
- ✅ **视频详情页** - 查看完整的视频信息和处理状态
- ✅ **任务步骤可视化** - 6步处理流程的状态展示
- ✅ **单步重试** - 支持重新运行失败的某个步骤
- ✅ **进度追踪** - 实时进度百分比和时长统计
- ✅ **文件管理** - 查看和下载生成的所有文件

### 3. B站账号认证
- ✅ **扫码登录** - 支持 B站 TV 扫码登录
- ✅ **QR 码展示** - 后端生成 PNG 格式二维码
- ✅ **自动轮询** - 前端自动检测登录状态
- ✅ **用户信息** - 获取并显示用户名和头像
- ✅ **登录持久化** - Token 和 Cookie 自动保存
- ✅ **状态检查** - 自动检查登录状态




---

## 🏗️ 技术架构

### 后端技术栈
- **语言**: Go 1.x
- **Web 框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL 8.0+ / PostgreSQL / SQLite (开发环境)
- **文件存储**: 腾讯云 COS
- **依赖注入**: Uber FX
- **配置管理**: TOML

### 前端技术栈
- **框架**: Next.js 15.5.4
- **语言**: TypeScript 5
- **UI 库**: React 18 + Tailwind CSS
- **图标**: Lucide React
- **HTTP 客户端**: Axios

### 外部服务集成
- **Whisper AI** - 语音识别和字幕生成
- **百度翻译 API** - 字幕翻译
- **DeepSeek API** - AI 翻译和元数据生成
- **Bilibili API** - 视频上传和用户信息
- **腾讯云 COS** - 文件存储

---

## 📁 项目结构

```
bili_up_backend/
├── cmd/                    # 命令行工具
│   └── main.go
├── internal/               # 内部包
│   ├── api/               # API 路由和控制器
│   │   ├── auth.go        # 认证接口
│   │   ├── category.go    # 分类接口
│   │   ├── routes.go      # 路由配置
│   │   └── upload.go      # 上传接口
│   ├── bilibili/          # B站 API 封装
│   │   ├── auth.go        # 认证和用户信息
│   │   └── upload.go      # 视频上传
│   ├── chain_task/        # 任务链处理
│   │   └── chain_task_handler.go
│   ├── config/            # 配置管理
│   │   └── config.go
│   ├── core/              # 核心业务逻辑
│   │   ├── services/      # 业务服务
│   │   │   ├── task_step_service.go    # 任务步骤服务
│   │   │   ├── video_service.go        # 视频服务
│   │   │   └── ...
│   │   └── types/         # 类型定义
│   │       └── app_config.go
│   └── handler/           # HTTP 处理器
│       ├── auth_handler.go      # 认证处理
│       ├── video_handler.go     # 视频处理
│       └── ...
├── pkg/                   # 可重用包
│   ├── appauth/          # 应用认证 ⭐ NEW
│   │   ├── client.go     # 认证客户端
│   │   └── manager.go    # 周期检查管理器
│   ├── bilibili/         # B站 API
│   │   └── auth.go       # 用户信息获取
│   └── store/            # 数据存储
│       └── model/
│           ├── task_step.go    # 任务步骤模型
│           ├── video.go        # 视频模型
│           └── ...
├── biliup-rs/            # Rust 下载和上传工具
│   └── crates/
│       ├── biliup/       # 核心库
│       └── biliup-cli/   # 命令行工具
├── docs/                 # 文档
│   ├── APP_AUTH_SETUP.md      # 应用认证设置指南
│   ├── APP_AUTH_TESTING.md    # 应用认证测试指南
│   └── APP_AUTH_SUMMARY.md    # 应用认证功能总结
├── config.toml           # 主配置文件
├── config.app_auth.example.toml  # 认证配置示例
├── test_app_auth.sh      # 认证 API 测试脚本
└── main.go               # 应用入口
```

---

## 🚀 快速开始

> 📚 **完整构建指南**: 请查看 [BUILD_GUIDE.md](./BUILD_GUIDE.md) 了解详细的构建和部署说明。

### 快速构建（一键打包前端+后端）

```bash
cd bili-up-api
make build
./bili-up-api-server
```

这将自动：
1. 构建 Next.js 前端并导出静态文件
2. 将前端嵌入到 Go 二进制中
3. 编译生成单个可执行文件

访问 `http://localhost:8096` 即可使用完整的前后端功能。

### 1. 环境要求

- Go 1.19+
- MySQL 8.0+ / PostgreSQL / SQLite
- Node.js 18+ (用于构建前端)
- Rust 1.70+ (biliup-rs，可选)

### 2. 配置数据库

```sql
CREATE DATABASE bili_up CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 配置文件

复制并修改配置文件：

```bash
cp config.toml.example config.toml
```

编辑 `config.toml`：

```toml
listen = ":8096"
environment = "development"
debug = true
FileUpDir = "/path/to/media"

[database]
  type = "mysql"
  host = "localhost"
  port = 3306
  username = "root"
  password = "your_password"
  database = "bili_up"

# 腾讯云 COS 配置
[TenCosConfig]
  Enabled = true
  CosBucketURL = "https://your-bucket.cos.region.myqcloud.com"
  CosSecretId = "your_secret_id"
  CosSecretKey = "your_secret_key"
  CosRegion = "ap-guangzhou"
  CosBucket = "your-bucket"

# 百度翻译配置
[BaiduTransConfig]
  enabled = true
  app_id = "your_app_id"
  secret_key = "your_secret_key"

# DeepSeek 配置
[DeepSeekTransConfig]
  enabled = true
  api_key = "your_api_key"
  model = "deepseek-chat"
  endpoint = "https://api.deepseek.com"

```

### 4. 编译运行

```bash
# 安装依赖
go mod download

# 编译
go build -o bili_up_backend main.go

# 运行
./bili_up_backend
```

---

## 📖 API 文档

### 视频管理

#### 获取视频列表
```http
GET /api/v1/videos
```

#### 获取视频详情（含任务步骤）
```http
GET /api/v1/videos/:id
```

响应示例：
```json
{
  "code": 200,
  "data": {
    "id": 1,
    "title": "视频标题",
    "description": "视频描述",
    "cover_url": "封面URL",
    "status": "completed",
    "task_steps": [
      {
        "step_name": "download_video",
        "step_order": 1,
        "status": "completed",
        "duration": 120,
        "can_retry": false
      },
      // ... 其他步骤
    ]
  }
}
```

#### 重试任务步骤
```http
POST /api/v1/videos/:id/steps/:stepName/retry
```

#### 获取视频文件列表
```http
GET /api/v1/videos/:id/files
```

### 认证相关

#### 获取登录二维码
```http
GET /api/v1/auth/qrcode
```

#### 获取二维码图片
```http
GET /api/v1/auth/qrcode/image/:authCode
```

#### 轮询登录状态
```http
POST /api/v1/auth/poll
Body: {"auth_code": "xxx"}
```

#### 检查登录状态
```http
GET /api/v1/auth/status
```

#### 获取用户信息
```http
GET /api/v1/auth/userinfo
```

#### 登出
```http
POST /api/v1/auth/logout
```

---



## 🎯 任务处理流程

### 6步处理链

1. **download_video** - 下载视频
   - 支持多平台（B站、抖音、YouTube 等）
   - 自动选择最佳清晰度
   - 保存到本地或云存储

2. **generate_subtitles** - 生成字幕
   - 使用 Whisper AI 语音识别
   - 自动断句和时间轴
   - 支持多语言识别

3. **translate_subtitles** - 翻译字幕
   - 百度翻译或 DeepSeek AI
   - 保留字幕格式和时间轴
   - 支持多语言翻译

4. **generate_metadata** - 生成元数据
   - AI 分析视频内容
   - 生成标题、描述、标签
   - 符合 B站 SEO 规范

5. **upload_to_bilibili** - 上传视频
   - 分片上传支持大文件
   - 自动重试机制
   - 实时进度反馈

6. **upload_subtitles** - 上传字幕
   - 支持双语字幕
   - 自动关联视频
   - SRT/ASS 格式支持

### 任务状态

- `pending` - 等待执行
- `running` - 执行中
- `completed` - 已完成
- `failed` - 失败
- `skipped` - 跳过

---

## 🧪 测试

### 运行单元测试
```bash
go test ./...
```

### API 测试

使用提供的测试脚本：

```bash
# 测试应用认证
./test_app_auth.sh

# 测试视频上传（需要先登录）
curl http://localhost:8096/api/v1/videos

# 测试认证状态
curl http://localhost:8096/api/v1/auth/status
```

---

## 📊 数据库表结构

### videos 表
```sql
CREATE TABLE videos (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  title VARCHAR(255),
  description TEXT,
  cover_url VARCHAR(512),
  file_path VARCHAR(512),
  status VARCHAR(50),
  bilibili_bvid VARCHAR(50),
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

### task_steps 表
```sql
CREATE TABLE task_steps (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  video_id BIGINT,
  step_name VARCHAR(100),
  step_order INT,
  status VARCHAR(50),
  start_time TIMESTAMP,
  end_time TIMESTAMP,
  duration INT,
  error_msg TEXT,
  result_data JSON,
  can_retry BOOLEAN,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (video_id) REFERENCES videos(id)
);
```

---

## 🛠️ 故障排查

### 问题1: 数据库连接失败

**错误**: `Error 1045: Access denied`

**解决**:
1. 检查 `config.toml` 中的数据库配置
2. 确认 MySQL 服务正在运行
3. 验证用户名和密码

### 问题2: B站登录失败

**错误**: 二维码过期或无法扫描

**解决**:
1. 刷新页面重新获取二维码
2. 检查网络连接
3. 确认 B站账号状态正常

### 问题3: 视频上传失败

**错误**: 上传超时或中断

**解决**:
1. 检查网络速度
2. 尝试减小视频文件大小
3. 查看 B站账号是否有上传权限
4. 检查 Token 是否过期

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

---

## 📄 许可证

MIT License

---
