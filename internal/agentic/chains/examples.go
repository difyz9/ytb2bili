package chains

import (
	"context"
	"fmt"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"github.com/difyz9/ytb2bili/internal/agentic/llm"
	"github.com/difyz9/ytb2bili/internal/agentic/tools"
	"github.com/difyz9/ytb2bili/internal/core"
	"go.uber.org/zap"
)

// ChainManager 任务链管理器 - 统一管理所有任务链
type ChainManager struct {
	app    *core.AppServer
	agent  *agentic.Agent
	logger *zap.Logger
	chains map[string]Chain
}

// NewChainManager 创建任务链管理器
func NewChainManager(app *core.AppServer, logger *zap.Logger) (*ChainManager, error) {
	// 初始化 LLM 客户端
	// 这里可以根据配置选择不同的 LLM
	llmClient, err := llm.NewOpenAICompatibleClient(
		app.Config.LLM.APIKey,
		app.Config.LLM.BaseURL,
		app.Config.LLM.Model,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}
	
	// 创建 Agent
	agent := &agentic.Agent{
		Name:          "ytb2bili_agent",
		Tools:         make(map[string]agentic.Tool),
		LLM:           llmClient,
		Logger:        logger,
		MaxIterations: 10,
	}
	
	// 注册工具（示例）
	// 实际使用时，这些工具应该根据具体任务动态注册
	videoID := "example_video_id"
	agent.Tools["download_video"] = tools.NewDownloadVideoTool(app, videoID)
	agent.Tools["download_thumbnail"] = tools.NewDownloadThumbnailTool(app, videoID)
	agent.Tools["extract_audio"] = tools.NewExtractAudioTool(app, videoID)
	agent.Tools["generate_subtitle"] = tools.NewGenerateSubtitleTool(app, videoID)
	agent.Tools["translate_subtitle"] = tools.NewTranslateSubtitleTool(app, videoID)
	agent.Tools["generate_metadata"] = tools.NewGenerateMetadataTool(app, videoID, llmClient)
	agent.Tools["upload_to_bilibili"] = tools.NewUploadToBiliTool(app, videoID)
	
	// 创建管理器
	manager := &ChainManager{
		app:    app,
		agent:  agent,
		logger: logger,
		chains: make(map[string]Chain),
	}
	
	// 注册预定义的任务链
	manager.chains["ytb_to_bili"] = NewYtbToBiliChain(app, agent, logger)
	manager.chains["agent"] = NewAgentChain(agent, logger, 10)
	
	logger.Info("Chain manager initialized",
		zap.Int("chain_count", len(manager.chains)))
	
	return manager, nil
}

// GetChain 获取指定的任务链
func (m *ChainManager) GetChain(name string) (Chain, error) {
	chain, exists := m.chains[name]
	if !exists {
		return nil, fmt.Errorf("chain not found: %s", name)
	}
	return chain, nil
}

// RegisterChain 注册新的任务链
func (m *ChainManager) RegisterChain(chain Chain) {
	m.chains[chain.Name()] = chain
	m.logger.Info("Chain registered", zap.String("name", chain.Name()))
}

// ListChains 列出所有可用的任务链
func (m *ChainManager) ListChains() []string {
	names := make([]string, 0, len(m.chains))
	for name := range m.chains {
		names = append(names, name)
	}
	return names
}

// Example1_SimpleSequentialChain 示例1：简单顺序链
// 适用场景：明确知道需要执行哪些步骤，按固定顺序执行
func Example1_SimpleSequentialChain(app *core.AppServer, videoID string) error {
	logger := app.Logger
	
	// 创建简单的任务链管理器
	manager, err := NewChainManager(app, logger)
	if err != nil {
		return err
	}
	
	// 获取 YouTube 到 Bilibili 的转换链
	chain, err := manager.GetChain("ytb_to_bili")
	if err != nil {
		return err
	}
	
	// 执行任务链
	ctx := context.Background()
	input := ChainInput{
		Query: fmt.Sprintf("将 YouTube 视频 %s 转换并上传到 Bilibili", videoID),
		Metadata: map[string]interface{}{
			"video_id": videoID,
		},
	}
	
	logger.Info("开始执行任务链", zap.String("video_id", videoID))
	
	output, err := chain.Execute(ctx, input)
	if err != nil {
		logger.Error("任务链执行失败", zap.Error(err))
		return err
	}
	
	logger.Info("任务链执行成功",
		zap.String("result", output.Result),
		zap.Int("steps", len(output.Steps)))
	
	// 打印每个步骤的结果
	for i, step := range output.Steps {
		logger.Info("步骤执行结果",
			zap.Int("step", i+1),
			zap.String("tool", step.ToolName),
			zap.String("output", step.Output))
	}
	
	return nil
}

// Example2_PartialChainExecution 示例2：部分执行
// 适用场景：只需要执行部分步骤，或者调试特定功能
func Example2_PartialChainExecution(app *core.AppServer, videoID string) error {
	logger := app.Logger
	manager, err := NewChainManager(app, logger)
	if err != nil {
		return err
	}
	
	chain, err := manager.GetChain("ytb_to_bili")
	if err != nil {
		return err
	}
	
	ytbChain, ok := chain.(*YtbToBiliChain)
	if !ok {
		return fmt.Errorf("chain is not YtbToBiliChain")
	}
	
	// 只执行下载和字幕生成步骤
	ctx := context.Background()
	input := ChainInput{
		Query: "下载视频并生成字幕",
		Metadata: map[string]interface{}{
			"video_id": videoID,
		},
	}
	
	steps := []string{
		"download_video",
		"extract_audio",
		"generate_subtitle",
	}
	
	logger.Info("执行部分步骤", zap.Strings("steps", steps))
	
	output, err := ytbChain.PartialExecute(ctx, input, steps)
	if err != nil {
		logger.Error("部分执行失败", zap.Error(err))
		return err
	}
	
	logger.Info("部分执行成功", zap.String("result", output.Result))
	return nil
}

// Example3_AgentDrivenChain 示例3：AI 驱动的智能链
// 适用场景：复杂、动态的任务，让 AI 自主决定执行策略
func Example3_AgentDrivenChain(app *core.AppServer, query string) error {
	logger := app.Logger
	manager, err := NewChainManager(app, logger)
	if err != nil {
		return err
	}
	
	chain, err := manager.GetChain("agent")
	if err != nil {
		return err
	}
	
	// 执行智能任务链，让 AI 自主决定如何完成任务
	ctx := context.Background()
	input := ChainInput{
		Query: query, // 例如："将视频 abc123 从 YouTube 搬运到 Bilibili，并翻译标题和简介"
		Metadata: map[string]interface{}{
			"allow_retry": true,
			"max_steps":   7,
		},
	}
	
	logger.Info("开始 AI 驱动任务", zap.String("query", query))
	
	output, err := chain.Execute(ctx, input)
	if err != nil {
		logger.Error("AI 任务执行失败", zap.Error(err))
		return err
	}
	
	logger.Info("AI 任务完成",
		zap.String("result", output.Result),
		zap.Int("steps", len(output.Steps)))
	
	return nil
}

// Example4_ChainWithCallback 示例4：带回调的执行
// 适用场景：需要实时更新 UI 或记录进度
func Example4_ChainWithCallback(app *core.AppServer, videoID string) error {
	logger := app.Logger
	manager, err := NewChainManager(app, logger)
	if err != nil {
		return err
	}
	
	chain, err := manager.GetChain("agent")
	if err != nil {
		return err
	}
	
	agentChain, ok := chain.(*AgentChain)
	if !ok {
		return fmt.Errorf("chain is not AgentChain")
	}
	
	ctx := context.Background()
	input := ChainInput{
		Query: fmt.Sprintf("处理视频 %s", videoID),
		Metadata: map[string]interface{}{
			"video_id": videoID,
		},
	}
	
	// 定义回调函数，每步执行后调用
	callback := func(step StepRecord, progress float64) {
		logger.Info("步骤完成",
			zap.String("tool", step.ToolName),
			zap.Float64("progress", progress),
			zap.Bool("success", step.Error == nil))
		
		// 这里可以发送 WebSocket 消息更新前端 UI
		// app.BroadcastProgress(videoID, progress, step)
	}
	
	logger.Info("开始带回调的执行")
	
	output, err := agentChain.ExecuteWithCallback(ctx, input, callback)
	if err != nil {
		logger.Error("执行失败", zap.Error(err))
		return err
	}
	
	logger.Info("执行完成", zap.String("result", output.Result))
	return nil
}

// Example5_ChainWithRetry 示例5：带重试的执行
// 适用场景：网络不稳定或需要更高可靠性
func Example5_ChainWithRetry(app *core.AppServer, videoID string) error {
	logger := app.Logger
	manager, err := NewChainManager(app, logger)
	if err != nil {
		return err
	}
	
	chain, err := manager.GetChain("ytb_to_bili")
	if err != nil {
		return err
	}
	
	ytbChain, ok := chain.(*YtbToBiliChain)
	if !ok {
		return fmt.Errorf("chain is not YtbToBiliChain")
	}
	
	ctx := context.Background()
	input := ChainInput{
		Query: fmt.Sprintf("转换视频 %s", videoID),
		Metadata: map[string]interface{}{
			"video_id": videoID,
		},
	}
	
	// 最多重试 3 次
	maxRetries := 3
	logger.Info("开始带重试的执行", zap.Int("max_retries", maxRetries))
	
	output, err := ytbChain.ExecuteWithRetry(ctx, input, maxRetries)
	if err != nil {
		logger.Error("执行失败（已重试）", zap.Error(err))
		return err
	}
	
	logger.Info("执行成功", zap.String("result", output.Result))
	return nil
}
