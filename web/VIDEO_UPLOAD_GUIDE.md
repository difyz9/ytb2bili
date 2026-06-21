# 本地视频上传功能说明

## 功能特性

✅ **拖拽上传**：支持直接拖拽视频文件到上传区域  
✅ **批量上传**：可同时选择或拖拽多个视频文件  
✅ **实时进度**：显示每个文件的上传和处理进度  
✅ **格式支持**：MP4, AVI, MOV, MKV, FLV, WMV, WEBM  
✅ **自动处理**：上传完成后自动提取音频并转录文本

## 使用方法

### 方式一：点击上传

1. 切换到"上传本地视频"标签页
2. 点击上传区域
3. 在文件选择器中选择一个或多个视频文件
4. 等待上传和处理完成

### 方式二：拖拽上传

1. 切换到"上传本地视频"标签页
2. 从文件管理器中选择视频文件
3. 直接拖拽到上传区域
4. 释放鼠标，文件将自动上传

## 处理流程

```
1. 选择/拖拽视频文件
   ↓
2. 上传到服务器 (显示上传中...)
   ↓
3. 保存文件到 uploads/videos/
   ↓
4. 提取音频 (显示处理中...)
   ↓
5. 转录文本 (使用 BCut API)
   ↓
6. 保存到数据库
   ↓
7. 完成 ✓
```

## 文件状态说明

| 状态 | 说明 |
|------|------|
| 上传中... | 文件正在上传到服务器 |
| 处理中... | 正在提取音频和转录文本 |
| ✓ 完成 | 处理成功 |
| ✗ 失败 | 处理失败，查看错误信息 |

## 进度条说明

- **蓝色进度条**：上传中或处理中
- **绿色进度条**：处理完成
- **0-50%**：文件上传阶段
- **50-100%**：音频提取和转录阶段

## 支持的文件格式

- MP4 (推荐)
- AVI
- MOV
- MKV
- FLV
- WMV
- WEBM

## 注意事项

1. **文件大小**：建议单个文件不超过 2GB
2. **处理时间**：取决于视频长度和大小，通常需要几分钟
3. **批量上传**：建议一次不超过 10 个文件
4. **存储空间**：确保服务器有足够的磁盘空间
5. **网络连接**：需要稳定的网络连接（转录使用在线 API）

## API 端点

### 1. 上传视频文件

**端点：** `POST /api/v1/video-process/upload`

**请求：** multipart/form-data
```
file: [视频文件]
```

**响应：**
```json
{
  "success": true,
  "message": "文件上传成功",
  "video_path": "/absolute/path/to/video.mp4",
  "file_name": "video.mp4"
}
```

### 2. 处理视频文件

**端点：** `POST /api/v1/video-process/submit-video`

**请求：**
```json
{
  "video_path": "/absolute/path/to/video.mp4",
  "user_id": "user123",
  "title": "视频标题"
}
```

**响应：**
```json
{
  "success": true,
  "message": "视频处理成功",
  "data": {
    "video_id": "1234567890_video",
    "video_path": "/absolute/path/to/video.mp4",
    "audio_path": "/absolute/path/to/audio.mp3",
    "transcript": {
      "language": "zh",
      "full_text": "完整的转录文本...",
      "segments": [...],
      "srt_path": "/absolute/path/to/subtitle.srt"
    },
    "status": "processed",
    "processed_at": "2026-01-23T10:30:00Z"
  }
}
```

## 前端组件

### 主要功能

```typescript
// 处理文件上传
const handleFileUpload = async (files: FileList | null) => {
  // 1. 验证文件类型
  // 2. 上传文件到服务器
  // 3. 提交处理请求
  // 4. 更新进度和状态
}

// 拖拽处理
const handleDragOver = (e: React.DragEvent) => { ... }
const handleDragLeave = (e: React.DragEvent) => { ... }
const handleDrop = (e: React.DragEvent) => { ... }

// 文件选择
const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => { ... }
```

### 状态管理

```typescript
interface UploadingFile {
  file: File;
  progress: number;
  status: 'uploading' | 'processing' | 'success' | 'error';
  message?: string;
  videoPath?: string;
}

const [uploadingFiles, setUploadingFiles] = useState<UploadingFile[]>([]);
const [isDragging, setIsDragging] = useState(false);
```

## 文件存储

上传的文件保存在：
```
backend/uploads/videos/
├── 1706001234_video1.mp4
├── 1706001235_video2.mp4
└── 1706001236_video3.mp4
```

文件命名规则：`{timestamp}_{original_filename}`

## 错误处理

### 常见错误

1. **不支持的文件格式**
   - 错误信息：`不支持的文件格式: .xxx`
   - 解决方案：使用支持的视频格式

2. **文件上传失败**
   - 错误信息：`保存文件失败`
   - 解决方案：检查服务器磁盘空间和权限

3. **音频提取失败**
   - 错误信息：`音频提取失败: ffmpeg error`
   - 解决方案：确认 ffmpeg 已正确安装

4. **转录失败**
   - 错误信息：`音频转录失败`
   - 解决方案：检查网络连接和 BCut API 状态

## 性能优化建议

1. **文件压缩**：上传前可以压缩视频以减少传输时间
2. **分片上传**：对于大文件，可以实现分片上传
3. **队列处理**：限制同时处理的文件数量
4. **进度反馈**：实时显示上传进度百分比
5. **断点续传**：实现上传中断后的续传功能

## 未来增强

- [ ] 显示详细的上传进度百分比
- [ ] 支持暂停/恢复上传
- [ ] 支持取消上传
- [ ] 上传前预览视频
- [ ] 分片上传大文件
- [ ] 压缩视频选项
- [ ] 批量导出转录结果
- [ ] 自定义转录语言

## 测试示例

### 测试单个文件上传

1. 准备一个小视频文件（< 100MB）
2. 拖拽到上传区域
3. 观察进度条变化
4. 等待处理完成
5. 检查是否显示"✓ 完成"

### 测试批量上传

1. 准备 3-5 个视频文件
2. 同时选择所有文件
3. 观察每个文件的进度
4. 确认所有文件都能正确处理

### 测试错误处理

1. 尝试上传非视频文件
2. 确认显示错误提示
3. 上传超大文件
4. 确认错误提示清晰

## 技术栈

**后端：**
- Go + Gin (Web 框架)
- FFmpeg (音频提取)
- BCut API (语音转录)

**前端：**
- React + Next.js
- TypeScript
- Tailwind CSS
- React Hot Toast (消息提示)

## 相关文档

- [视频处理 API 文档](../backend/docs/VIDEO_PROCESS_API.md)
- [前端集成指南](./INTEGRATION_GUIDE.md)
