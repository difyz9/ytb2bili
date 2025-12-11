# 基于 LangChainGo 的任务链使用指南

本指南展示如何在 ytb2bili 项目中优雅地使用 LangChainGo 进行自然语言任务链处理。

## 核心概念

### 1. Tool（工具）
- **定义**：独立的功能单元，如下载视频、生成字幕等
- **接口**：`Name()`, `Description()`, `Call()`
- **职责**：执行单一、明确的任务

### 2. Chain（任务链）
- **定义**：多个工具的有序组合，完成复杂任务
- **类型**：
  - **顺序链**：按固定顺序执行工具
  - **智能链**：由 LLM 动态决定执行策略

### 3. Agent（代理）
- **定义**：基于 LLM 的智能决策者
- **职责**：分析任务，选择和编排工具

## 使用场景

### 场景 1：固定流程任务（推荐用于生产环境）

```go
import (
    "context"
    "github.com/difyz9/ytb2bili/internal/agentic/chains"
)

func ProcessVideo(app *core.AppServer, videoID string) error {
    // 创建任务链管理器
    manager, err := chains.NewChainManager(app, app.Logger)
    if err != nil {
        return err
    }
    
    // 获取预定义的 YouTube 到 Bilibili 转换链
    chain, err := manager.GetChain("ytb_to_bili")
    if err != nil {
        return err
    }
    
    // 执行任务链
    output, err := chain.Execute(context.Background(), chains.ChainInput{
        Query: "转换视频",
        Metadata: map[string]interface{}{
            "video_id": videoID,
        },
    })
    
    if err != nil {
        return err
    }
    
    // 处理结果
    app.Logger.Info("任务完成", zap.String("bv_id", output.Result))
    return nil
}
```

**优点**：
- 快速、可预测
- 便于调试和监控
- 成本低（不需要 LLM 决策）

**适用于**：
- 明确的业务流程
- 对稳定性要求高的场景
- 批量处理任务

### 场景 2：灵活的智能任务（适合复杂场景）

```go
func SmartProcess(app *core.AppServer, query string) error {
    manager, err := chains.NewChainManager(app, app.Logger)
    if err != nil {
        return err
    }
    
    // 使用 AI 驱动的智能链
    chain, err := manager.GetChain("agent")
    if err != nil {
        return err
    }
    
    // 自然语言描述任务，让 AI 自主决定如何执行
    output, err := chain.Execute(context.Background(), chains.ChainInput{
        Query: query, // 例如："下载视频 abc123，生成中英字幕，优化标题后上传"
    })
    
    if err != nil {
        return err
    }
    
    app.Logger.Info("智能任务完成", zap.String("result", output.Result))
    return nil
}
```

**优点**：
- 极致灵活，适应各种复杂需求
- 可处理模糊或动态的任务描述
- 自动错误处理和重试

**适用于**：
- 需求不明确的探索性任务
- 用户自定义的复杂工作流
- 需要智能决策的场景

### 场景 3：部分执行（调试/测试专用）

```go
func TestSubtitleGeneration(app *core.AppServer, videoID string) error {
    manager, err := chains.NewChainManager(app, app.Logger)
    if err != nil {
        return err
    }
    
    chain, _ := manager.GetChain("ytb_to_bili")
    ytbChain := chain.(*chains.YtbToBiliChain)
    
    // 只执行字幕相关步骤
    output, err := ytbChain.PartialExecute(
        context.Background(),
        chains.ChainInput{
            Metadata: map[string]interface{}{"video_id": videoID},
        },
        []string{
            "extract_audio",
            "generate_subtitle",
            "translate_subtitle",
        },
    )
    
    return err
}
```

**适用于**：
- 开发调试
- 功能测试
- 单独验证某个步骤

### 场景 4：实时进度反馈（UI 集成）

```go
func ProcessWithProgress(app *core.AppServer, videoID string) error {
    manager, _ := chains.NewChainManager(app, app.Logger)
    chain, _ := manager.GetChain("agent")
    agentChain := chain.(*chains.AgentChain)
    
    // 定义进度回调
    progressCallback := func(step chains.StepRecord, progress float64) {
        // 通过 WebSocket 更新前端进度
        app.BroadcastMessage(map[string]interface{}{
            "type":     "progress",
            "video_id": videoID,
            "step":     step.ToolName,
            "progress": progress,
            "status":   "processing",
        })
    }
    
    output, err := agentChain.ExecuteWithCallback(
        context.Background(),
        chains.ChainInput{
            Query: fmt.Sprintf("处理视频 %s", videoID),
            Metadata: map[string]interface{}{"video_id": videoID},
        },
        progressCallback,
    )
    
    // 完成后通知
    if err == nil {
        app.BroadcastMessage(map[string]interface{}{
            "type":     "complete",
            "video_id": videoID,
            "result":   output.Result,
        })
    }
    
    return err
}
```

## 最佳实践

### 1. 选择合适的链类型

| 场景 | 推荐方案 | 理由 |
|------|---------|------|
| 固定业务流程 | `YtbToBiliChain` | 快速、稳定、成本低 |
| 动态需求 | `AgentChain` | 灵活、智能 |
| 调试测试 | `PartialExecute` | 快速验证单个功能 |
| UI 集成 | `ExecuteWithCallback` | 实时反馈 |

### 2. 错误处理策略

```go
// 方案 A：自动重试（推荐用于网络操作）
output, err := ytbChain.ExecuteWithRetry(ctx, input, 3)

// 方案 B：手动处理（适合需要精细控制的场景）
output, err := chain.Execute(ctx, input)
if err != nil {
    // 分析错误类型
    if isNetworkError(err) {
        // 重试
        return chain.Execute(ctx, input)
    } else if isAuthError(err) {
        // 刷新凭证
        refreshAuth()
        return chain.Execute(ctx, input)
    }
    return err
}
```

### 3. 性能优化

```go
// 并行执行独立任务
// 注意：这需要自定义链实现
type ParallelChain struct {
    *chains.BaseChain
}

func (c *ParallelChain) Execute(ctx context.Context, input chains.ChainInput) (*chains.ChainOutput, error) {
    // 下载视频和缩略图可以并行
    var wg sync.WaitGroup
    var videoPath, thumbPath string
    var videoErr, thumbErr error
    
    wg.Add(2)
    
    go func() {
        defer wg.Done()
        videoPath, videoErr = downloadVideo(ctx, input)
    }()
    
    go func() {
        defer wg.Done()
        thumbPath, thumbErr = downloadThumbnail(ctx, input)
    }()
    
    wg.Wait()
    
    // 继续后续串行步骤...
    return output, nil
}
```

### 4. 日志和监控

```go
// 在关键步骤添加详细日志
func (t *DownloadVideoTool) Call(ctx context.Context, input string) (string, error) {
    t.logger.Info("开始下载视频",
        zap.String("video_id", input),
        zap.Time("start_time", time.Now()))
    
    defer func(start time.Time) {
        t.logger.Info("视频下载完成",
            zap.Duration("duration", time.Since(start)))
    }(time.Now())
    
    // 实际下载逻辑...
    return path, nil
}
```

### 5. 配置管理

```go
// 在 config.toml 中配置 LLM
// [llm]
// provider = "openai"  # 或 "deepseek", "local"
// api_key = "sk-xxx"
// base_url = "https://api.openai.com/v1"
// model = "gpt-4o"

// 代码中使用配置
llmClient, err := llm.NewOpenAICompatibleClient(
    app.Config.LLM.APIKey,
    app.Config.LLM.BaseURL,
    app.Config.LLM.Model,
    logger,
)
```

## 扩展指南

### 创建自定义工具

```go
package tools

type MyCustomTool struct {
    app *core.AppServer
}

func (t *MyCustomTool) Name() string {
    return "my_custom_tool"
}

func (t *MyCustomTool) Description() string {
    return "工具的详细描述，用于 LLM 理解其功能"
}

func (t *MyCustomTool) Call(ctx context.Context, input string) (string, error) {
    // 1. 解析输入
    // 2. 执行逻辑
    // 3. 返回结果
    return "result", nil
}
```

### 创建自定义任务链

```go
package chains

type MyCustomChain struct {
    *BaseChain
}

func NewMyCustomChain(app *core.AppServer, agent *agentic.Agent) *MyCustomChain {
    return &MyCustomChain{
        BaseChain: NewBaseChain("my_chain", agent, app.Logger),
    }
}

func (c *MyCustomChain) Execute(ctx context.Context, input ChainInput) (*ChainOutput, error) {
    // 定义你的任务流程
    // 可以使用 c.ExecuteToolSequence() 辅助函数
    return output, nil
}
```

## 常见问题

### Q: 何时使用顺序链 vs 智能链？
**A:** 
- 生产环境、明确流程 → 顺序链
- 探索、复杂动态需求 → 智能链
- 成本敏感 → 顺序链（不调用 LLM）

### Q: 如何处理长时间运行的任务？
**A:** 使用 `ExecuteWithCallback` + 后台任务队列
```go
go func() {
    output, err := chain.ExecuteWithCallback(ctx, input, progressCallback)
    // 完成后保存到数据库
    saveResult(output, err)
}()
```

### Q: 如何支持多种 LLM？
**A:** 通过配置文件切换
```go
switch app.Config.LLM.Provider {
case "openai":
    llmClient, _ = llm.NewOpenAIClient(apiKey, model, logger)
case "deepseek":
    llmClient, _ = llm.NewOpenAICompatibleClient(apiKey, "https://api.deepseek.com", model, logger)
case "local":
    llmClient, _ = llm.NewOpenAICompatibleClient("", "http://localhost:1234", model, logger)
}
```

### Q: 如何实现任务暂停和恢复？
**A:** 保存任务状态到数据库
```go
// 执行前保存检查点
for i, tool := range tools {
    if err := checkResumePoint(taskID, i); err == nil {
        // 从这里恢复
        continue
    }
    
    result, err := tool.Call(ctx, input)
    saveCheckpoint(taskID, i, result, err)
}
```

## 总结

### 简洁优雅的关键
1. **明确抽象**：Tool → Chain → Agent 三层架构
2. **组合优于继承**：通过组合工具构建链
3. **声明式设计**：用数据描述流程，而非代码
4. **统一接口**：所有工具和链使用相同的接口
5. **可测试性**：每个工具可独立测试

### 推荐实践
- ✅ 生产环境优先使用预定义的顺序链
- ✅ 复杂场景使用智能链
- ✅ 为每个工具编写清晰的 `Description`
- ✅ 使用回调函数提供实时反馈
- ✅ 在关键节点添加日志和监控

### 避免陷阱
- ❌ 不要在智能链中使用过多工具（建议 < 10 个）
- ❌ 不要在循环中频繁调用 LLM（成本高）
- ❌ 不要忽略错误处理和重试机制
- ❌ 不要在生产环境直接使用未测试的智能链
