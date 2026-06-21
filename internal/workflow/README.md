# YouTube 视频处理工作流

## 概述

这是一个基于任务链模式的 YouTube 视频处理工作流，提供了简洁优雅的 API 来处理视频下载、音频提取和转录等任务。

## 架构设计

### 核心组件

```
YouTubeChain (高层API)
    ↓
Chain (任务链引擎)
    ↓
Steps (独立的处理步骤)
    ↓
Tools (底层工具)
```

### 设计优势

1. **声明式配置**：通过 fx 依赖注入，自动组装所有组件
2. **步骤化处理**：每个步骤职责单一，易于测试和维护
3. **优雅降级**：非必需步骤失败不会中断整个流程
4. **上下文传递**：使用 VideoContext 在步骤间传递状态
5. **统一配置**：WorkflowConfig 集中管理所有工具配置

## 快速开始

### 基本使用

```go
// 1. 注入 YouTubeChain (fx 自动完成)
type Handler struct {
    chain *workflow.YouTubeChain
}

// 2. 处理视频（一行代码）
result, err := h.chain.Process(ctx, "https://www.youtube.com/watch?v=VIDEO_ID")
if err != nil {
    log.Fatal(err)
}

// 3. 获取结果
fmt.Println("视频路径:", result.VideoPath)
fmt.Println("音频路径:", result.AudioPath)
fmt.Println("转录文本:", result.Transcript.FullText)
```

### 自定义配置

通过环境变量配置工作流参数：

```bash
# 设置下载目录
export WORKFLOW_DOWNLOAD_DIR=/data/videos

# 指定 yt-dlp 路径
export WORKFLOW_YTDLP_PATH=/usr/local/bin/yt-dlp

# 指定 ffmpeg 路径
export WORKFLOW_FFMPEG_PATH=/usr/local/bin/ffmpeg

# 使用 cookies（用于下载受限视频）
export WORKFLOW_COOKIES_FILE=/path/to/cookies.txt

# 使用代理加速下载
export WORKFLOW_PROXY_URL=http://127.0.0.1:7890
```

或者在 `.env` 文件中配置：

```env
WORKFLOW_DOWNLOAD_DIR=/data/videos
WORKFLOW_YTDLP_PATH=/usr/local/bin/yt-dlp
WORKFLOW_FFMPEG_PATH=/usr/local/bin/ffmpeg
WORKFLOW_COOKIES_FILE=/path/to/cookies.txt
WORKFLOW_PROXY_URL=http://127.0.0.1:7890
```

配置会自动从 `AppConfig` 中加载，无需修改代码。

## 工作流步骤

### 1. InitStep (必需)
- 初始化 VideoContext
- 验证输入 URL
- 提取视频 ID

### 2. DownloadVideoStep (必需)
- 使用 yt-dlp 下载视频
- 自动选择最佳画质
- 支持代理和 cookies

### 3. DownloadThumbnailStep (可选)
- 下载视频封面
- 多质量级别自动降级
- 失败不影响后续步骤

### 4. ExtractAudioStep (必需)
- 使用 FFmpeg 提取音频
- 输出 MP3 格式 (192kbps)
- 自动检测已存在文件

### 5. TranscribeStep (可选)
- 使用 BCut API 转录音频
- 支持多语言
- 生成时间轴字幕

### 6. SaveDatabaseStep (必需)
- 保存处理结果到数据库
- 生成 SRT 字幕文件
- 更新视频状态

## 进阶使用

### 获取详细执行信息

```go
// Process 内部调用 chain.Run，可以直接使用 chain 获取详细信息
result := yc.chain.Run(ctx, videoURL)

fmt.Printf("执行步骤数: %d\n", result.ExecutedSteps)
fmt.Printf("跳过步骤数: %d\n", result.SkippedSteps)
fmt.Printf("失败步骤数: %d\n", result.FailedSteps)
fmt.Printf("总耗时: %v\n", result.Duration)

// 查看每个步骤的详细信息
for name, detail := range result.StepDetails {
    fmt.Printf("步骤 %s: 状态=%s, 耗时=%v\n", 
        name, detail.Status, detail.Duration)
}
```

### 自定义步骤

如果需要添加新的处理步骤，按以下模式创建：

```go
// 1. 定义步骤结构
type MyCustomStep struct {
    BaseStep
    myTool  *MyTool
    logger  *zap.Logger
}

// 2. 定义依赖参数
type MyCustomStepParams struct {
    fx.In
    Tool   *MyTool
    Logger *zap.Logger
}

// 3. 实现构造函数
func NewMyCustomStep(params MyCustomStepParams) *MyCustomStep {
    return &MyCustomStep{
        BaseStep: NewBaseStep("MyCustom", true), // true=必需
        myTool:   params.Tool,
        logger:   params.Logger,
    }
}

// 4. 实现 Execute 方法
func (s *MyCustomStep) Execute(ctx context.Context, input any) (any, error) {
    vctx := input.(*VideoContext)
    
    // 执行自定义逻辑
    result, err := s.myTool.Process(vctx.VideoPath)
    if err != nil {
        return nil, err
    }
    
    // 更新上下文
    vctx.CustomField = result
    return vctx, nil
}

// 5. 注册到模块
var YouTubeWorkflowModule = fx.Module("youtube_workflow",
    // ... 其他提供者
    fx.Provide(AsStep(NewMyCustomStep)), // 添加这一行
)
```

## 错误处理

工作流提供了完善的错误处理机制：

```go
result, err := chain.Process(ctx, videoURL)
if err != nil {
    // 检查是哪个步骤失败
    if strings.Contains(err.Error(), "download video failed") {
        // 处理下载失败
    } else if strings.Contains(err.Error(), "extract audio failed") {
        // 处理音频提取失败
    }
    
    return err
}

// 即使某些可选步骤失败，也能继续
if result.ThumbnailPath == "" {
    log.Warn("缩略图下载失败，继续处理")
}
```

## 性能优化

### 并发处理

如果需要批量处理多个视频：

```go
var wg sync.WaitGroup
semaphore := make(chan struct{}, 3) // 限制并发数为 3

for _, videoURL := range videoURLs {
    wg.Add(1)
    go func(url string) {
        defer wg.Done()
        
        semaphore <- struct{}{}        // 获取令牌
        defer func() { <-semaphore }() // 释放令牌
        
        result, err := chain.Process(ctx, url)
        if err != nil {
            log.Error("处理失败", zap.String("url", url), zap.Error(err))
            return
        }
        
        log.Info("处理成功", zap.String("video_id", result.VideoID))
    }(videoURL)
}

wg.Wait()
```

### 资源清理

工作流不会自动清理临时文件，建议：

```go
result, err := chain.Process(ctx, videoURL)
if err != nil {
    return err
}

// 上传到云存储后清理本地文件
defer func() {
    os.Remove(result.VideoPath)
    os.Remove(result.AudioPath)
    os.Remove(result.ThumbnailPath)
}()

// 执行上传逻辑
uploadToCloud(result)
```

## 测试

### 单元测试步骤

```go
func TestDownloadVideoStep(t *testing.T) {
    logger := zap.NewNop()
    tool, _ := tools.NewDownloadVideoTool(/* ... */, logger)
    
    step := NewDownloadVideoStep(DownloadVideoStepParams{
        Tool:   tool,
        Logger: logger,
    })
    
    ctx := context.Background()
    input := &VideoContext{VideoURL: "test_url"}
    
    output, err := step.Execute(ctx, input)
    assert.NoError(t, err)
    
    vctx := output.(*VideoContext)
    assert.NotEmpty(t, vctx.VideoPath)
}
```

### 集成测试

参考 `examples/workflow/main.go` 的完整示例。

## 最佳实践

1. **使用上下文超时**：避免长时间运行的任务阻塞
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
   defer cancel()
   
   result, err := chain.Process(ctx, videoURL)
   ```

2. **日志记录**：每个步骤都有详细的日志
   ```go
   logger.Info("开始处理视频", zap.String("url", videoURL))
   result, err := chain.Process(ctx, videoURL)
   logger.Info("处理完成", zap.Duration("耗时", time.Since(start)))
   ```

3. **监控指标**：收集处理时间、成功率等指标
   ```go
   metrics.Histogram("video_processing_duration", result.Duration)
   metrics.Counter("video_processing_success").Inc()
   ```

4. **优雅降级**：可选步骤失败时继续处理
   ```go
   // TranscribeStep 和 DownloadThumbnailStep 是可选的
   // 它们失败不会导致整个流程失败
   ```

## 常见问题

### Q: 如何跳过某个步骤？

A: 修改步骤的 `Required` 字段为 `false`，然后在步骤中返回特定错误即可跳过。

### Q: 如何自定义下载目录？

A: 修改 `DefaultWorkflowConfig()` 中的 `DownloadDir` 字段。

### Q: 转录支持哪些语言？

A: BCut API 支持中文、英文等多种语言，自动检测。

### Q: 如何处理超大视频文件？

A: 增加 context 超时时间，或者分块处理。

## 总结

这个工作流设计遵循以下原则：

- ✅ **简洁性**：一行代码完成复杂任务
- ✅ **可扩展性**：轻松添加新步骤
- ✅ **可测试性**：每个组件都可独立测试
- ✅ **可观测性**：完整的日志和指标
- ✅ **健壮性**：完善的错误处理和降级策略
