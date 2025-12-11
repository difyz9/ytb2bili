package chains

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"github.com/difyz9/ytb2bili/internal/agentic/tools"
	"github.com/difyz9/ytb2bili/internal/core"
	"go.uber.org/zap"
)

// YtbToBiliChain YouTube 到 Bilibili 的完整转换链
// 这是一个典型的任务链示例，展示如何优雅地组合多个工具
type YtbToBiliChain struct {
	*BaseChain
	app *core.AppServer
}

// NewYtbToBiliChain 创建 YouTube 到 Bilibili 转换链
func NewYtbToBiliChain(app *core.AppServer, agent *agentic.Agent, logger *zap.Logger) *YtbToBiliChain {
	return &YtbToBiliChain{
		BaseChain: NewBaseChain("ytb_to_bili", agent, logger),
		app:       app,
	}
}

// Execute 执行完整的转换流程
// 这个方法展示了如何将多个工具组合成一个完整的工作流
func (c *YtbToBiliChain) Execute(ctx context.Context, input ChainInput) (*ChainOutput, error) {
	// 1. 解析输入参数
	videoID, ok := input.Metadata["video_id"].(string)
	if !ok || videoID == "" {
		return nil, fmt.Errorf("video_id is required in metadata")
	}
	
	c.logger.Info("Starting YtbToBili chain",
		zap.String("video_id", videoID))
	
	// 2. 构建工具链 - 这是关键部分，定义了完整的执行流程
	toolChain := []struct {
		tool  agentic.Tool
		input string
	}{
		{
			tool:  tools.NewDownloadVideoTool(c.app, videoID),
			input: videoID,
		},
		{
			tool:  tools.NewDownloadThumbnailTool(c.app, videoID),
			input: videoID,
		},
		{
			tool:  tools.NewExtractAudioTool(c.app, videoID),
			input: videoID,
		},
		{
			tool:  tools.NewGenerateSubtitleTool(c.app, videoID),
			input: videoID,
		},
		{
			tool:  tools.NewTranslateSubtitleTool(c.app, videoID),
			input: fmt.Sprintf(`{"video_id": "%s", "target_lang": "zh-CN"}`, videoID),
		},
		{
			tool:  tools.NewGenerateMetadataTool(c.app, videoID, c.agent.LLM),
			input: videoID,
		},
		{
			tool:  tools.NewUploadToBiliTool(c.app, videoID),
			input: videoID,
		},
	}
	
	// 3. 执行工具链
	output := &ChainOutput{
		Steps:    make([]StepRecord, 0, len(toolChain)),
		Metadata: make(map[string]interface{}),
	}
	
	for i, tc := range toolChain {
		c.logger.Info("Executing tool",
			zap.Int("step", i+1),
			zap.Int("total", len(toolChain)),
			zap.String("tool", tc.tool.Name()))
		
		result, err := tc.tool.Call(ctx, tc.input)
		
		step := StepRecord{
			ToolName: tc.tool.Name(),
			Input:    tc.input,
			Output:   result,
			Error:    err,
		}
		output.Steps = append(output.Steps, step)
		
		if err != nil {
			c.logger.Error("Tool execution failed",
				zap.String("tool", tc.tool.Name()),
				zap.Error(err))
			return output, fmt.Errorf("chain failed at step %d (%s): %w", i+1, tc.tool.Name(), err)
		}
		
		// 记录中间结果到元数据
		output.Metadata[tc.tool.Name()+"_result"] = result
	}
	
	// 4. 设置最终结果（最后一个工具的输出，即 Bilibili BV 号）
	if len(output.Steps) > 0 {
		output.Result = output.Steps[len(output.Steps)-1].Output
	}
	
	c.logger.Info("YtbToBili chain completed successfully",
		zap.String("video_id", videoID),
		zap.String("result", output.Result))
	
	return output, nil
}

// ExecuteWithRetry 带重试的执行 - 提供更强的容错能力
func (c *YtbToBiliChain) ExecuteWithRetry(ctx context.Context, input ChainInput, maxRetries int) (*ChainOutput, error) {
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		c.logger.Info("Attempting chain execution",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries))
		
		output, err := c.Execute(ctx, input)
		if err == nil {
			return output, nil
		}
		
		lastErr = err
		c.logger.Warn("Chain execution failed, will retry",
			zap.Int("attempt", attempt),
			zap.Error(err))
	}
	
	return nil, fmt.Errorf("chain failed after %d attempts: %w", maxRetries, lastErr)
}

// PartialExecute 部分执行 - 只执行指定的步骤
// 这对于调试或只需要部分功能的场景很有用
func (c *YtbToBiliChain) PartialExecute(ctx context.Context, input ChainInput, steps []string) (*ChainOutput, error) {
	videoID, ok := input.Metadata["video_id"].(string)
	if !ok || videoID == "" {
		return nil, fmt.Errorf("video_id is required in metadata")
	}
	
	// 构建工具映射
	toolMap := map[string]agentic.Tool{
		"download_video":      tools.NewDownloadVideoTool(c.app, videoID),
		"download_thumbnail":  tools.NewDownloadThumbnailTool(c.app, videoID),
		"extract_audio":       tools.NewExtractAudioTool(c.app, videoID),
		"generate_subtitle":   tools.NewGenerateSubtitleTool(c.app, videoID),
		"translate_subtitle":  tools.NewTranslateSubtitleTool(c.app, videoID),
		"generate_metadata":   tools.NewGenerateMetadataTool(c.app, videoID, c.agent.LLM),
		"upload_to_bilibili":  tools.NewUploadToBiliTool(c.app, videoID),
	}
	
	output := &ChainOutput{
		Steps:    make([]StepRecord, 0, len(steps)),
		Metadata: make(map[string]interface{}),
	}
	
	for i, stepName := range steps {
		tool, exists := toolMap[stepName]
		if !exists {
			return output, fmt.Errorf("unknown step: %s", stepName)
		}
		
		c.logger.Info("Executing partial step",
			zap.Int("step", i+1),
			zap.String("tool", stepName))
		
		result, err := tool.Call(ctx, videoID)
		
		step := StepRecord{
			ToolName: stepName,
			Input:    videoID,
			Output:   result,
			Error:    err,
		}
		output.Steps = append(output.Steps, step)
		
		if err != nil {
			return output, fmt.Errorf("partial execution failed at %s: %w", stepName, err)
		}
		
		output.Metadata[stepName+"_result"] = result
	}
	
	if len(output.Steps) > 0 {
		output.Result = output.Steps[len(output.Steps)-1].Output
	}
	
	return output, nil
}

// GetProgress 获取执行进度（基于已完成的步骤）
func (c *YtbToBiliChain) GetProgress(output *ChainOutput) float64 {
	totalSteps := 7 // 总共 7 个步骤
	if output == nil {
		return 0.0
	}
	
	completedSteps := len(output.Steps)
	return float64(completedSteps) / float64(totalSteps) * 100
}

// ToJSON 将输出转换为 JSON
func (output *ChainOutput) ToJSON() (string, error) {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
