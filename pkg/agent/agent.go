package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/zap"
)

// videoProcessingInstruction is the built-in fallback system prompt.
// The content is kept in Go code so packaged binaries do not depend on external prompt files.
const videoProcessingInstruction = `# ytb2bili AI 助手提示词

你是 ytb2bili 的 AI 助手，专注于将 YouTube 视频处理并发布到 B 站。除非用户使用其他语言，否则请始终用中文回复。

## 可用工具

- **download_video**：下载 YouTube 视频。必填：video_url
- **fetch_one_video_by_share_url**：解析抖音分享链接，获取抖音视频详细信息。必填：share_url
- **download_douyin_video**：下载抖音视频到本地。必填：share_url 或 video_info
- **extract_audio**：从本地视频文件提取音频。必填：video_path
- **transcode_video**：对本地视频执行转码、压缩、改分辨率、改格式、改帧率。必填：video_path
- **download_thumbnail**：下载视频封面图。必填：video_url
- **submit_pipeline**：一键提交 YouTube URL 或抖音分享链接到完整处理流程（下载 → 转写 → 翻译 → 生成元数据）。必填：url
- **query_videos**：查询用户本地视频库。可选过滤：status、platform、limit、has_bili_upload
- **rewrite_metadata**：使用 AI 为视频重新生成 B 站标题/描述/标签。必填：video_id；可选：hint
- **subtitle_action**：对视频字幕执行 AI 操作（summarize 摘要 / translate 翻译）。必填：video_id、action；可选：target_lang
- **manage_subscription**：管理频道订阅（list/add/remove）。必填：action；可选：channel_id、platform

## 工作原则

- 调用工具前先逐步思考。
- 完整处理 YouTube 或抖音视频任务请优先使用 submit_pipeline（单次调用）。
- 仅需下载时：download_video → extract_audio → download_thumbnail。
- 抖音分享链接仅需下载时：fetch_one_video_by_share_url → download_douyin_video。
- 对本地格式转换、压缩体积、改分辨率、改编码、改帧率时，使用 transcode_video。
- 用户要求对 YouTube 链接直接进行转码时，先调用 download_video，再把返回的本地文件路径传给 transcode_video。
- 不要臆造本地文件路径；如果既没有现成路径也没有可下载 URL，就先向用户索取路径。
- 处理自然语言转码需求时，按下面规则映射参数：
	- “转成 mp4/mkv/mov/webm” -> output_format
	- “转成 720p/1080p/2K/4K” -> resolution；其中 2K 近似映射到 1440p，4K 映射到 2160p
	- “压缩体积/减小文件/发微信更容易” -> 优先使用 output_format=mp4、video_codec=h264、crf=26
	- “尽量清晰/高质量” -> 使用较低 crf，通常 18-22
	- “转成 HEVC/H.265” -> video_codec=h265
	- “转成 VP9/webm” -> output_format=webm 且 video_codec=vp9
	- “保持原编码/不重新编码/封装转换” -> video_codec=copy，且不要同时设置 resolution 或 fps
	- “30帧/60帧” -> fps
- 未给出详细参数时，优先选择兼容性更好的默认值，不要激进压缩。
- 将 download_video 返回的确切文件路径传给 extract_audio。
- 工具失败时，清晰说明错误原因并建议解决方案。
- 完成后用中文简要总结操作结果。`

// loadInstruction returns the system prompt.
// Priority: cfg.InstructionFile > built-in default.
func loadInstruction(cfg *Config, logger *zap.Logger) string {
	if cfg != nil && cfg.InstructionFile != "" {
		data, err := os.ReadFile(cfg.InstructionFile)
		if err == nil {
			logger.Info("agent instruction loaded from file", zap.String("file", cfg.InstructionFile))
			return string(data)
		}
		logger.Warn("failed to load instruction file, using built-in default",
			zap.String("file", cfg.InstructionFile),
			zap.Error(err))
	}

	return videoProcessingInstruction
}

// StepSummary captures one tool call and its result.
type StepSummary struct {
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RunResult is returned by NanoAgent.Run.
type RunResult struct {
	FinalAnswer string        `json:"final_answer"`
	Success     bool          `json:"success"`
	Steps       []StepSummary `json:"steps"`
	Duration    time.Duration `json:"duration_ms"`
}

// NanoAgent wraps eino's adk.Runner for the ytb2bili video pipeline.
type NanoAgent struct {
	Name   string
	Tools  []tool.BaseTool
	runner *adk.Runner
	logger *zap.Logger
	config *Config
}

// resolveBaseURL returns the effective fallback LLM base URL.
func resolveBaseURL(cfg *Config) string {
	if cfg == nil || cfg.LLM.BaseURL == "" {
		return llm.DefaultBaseURL
	}
	return cfg.LLM.BaseURL
}

// defaultModel 默认使用 gpt-4o-mini（cost-effective 且速度快）
func defaultModel() string {
	return llm.DefaultModel
}

// New creates a NanoAgent from config and the registered tools.
func New(ctx context.Context, cfg *Config, toolsList []tool.BaseTool, logger *zap.Logger) (*NanoAgent, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	name := cfg.Name
	if name == "" {
		name = "ytb2bili-agent"
	}

	configCopy := *cfg
	agentInstance := &NanoAgent{Name: name, Tools: toolsList, logger: logger, config: &configCopy}

	if cfg.LLM.APIKey != "" {
		runner, err := agentInstance.newRunnerWithConfig(ctx, cfg.LLM.Model, cfg.LLM.APIKey, resolveBaseURL(cfg))
		if err != nil {
			return nil, err
		}
		agentInstance.runner = runner
	} else {
		logger.Info("agent initialized without local LLM credentials; request-scoped LLM config required")
	}

	return agentInstance, nil
}

// HasRunner reports whether the agent has a locally configured default runner.
func (a *NanoAgent) HasRunner() bool {
	return a != nil && a.runner != nil
}

// Run executes the agent for the given user query.
func (a *NanoAgent) Run(ctx context.Context, query string) (*RunResult, error) {
	return a.runWithRunner(ctx, query, a.runner)
}

// RunWithModel executes the agent with a per-request model override.
func (a *NanoAgent) RunWithModel(ctx context.Context, query string, modelName string) (*RunResult, error) {
	if modelName == "" || a.config == nil {
		return a.Run(ctx, query)
	}

	overrideRunner, err := a.newRunner(ctx, modelName)
	if err != nil {
		return nil, err
	}

	return a.runWithRunner(ctx, query, overrideRunner)
}

// RunWithLLMConfig executes the agent with a per-request model + upstream override.
// This is used when the caller needs to route the LLM request through a user-scoped proxy.
func (a *NanoAgent) RunWithLLMConfig(ctx context.Context, query string, modelName, apiKey, baseURL string) (*RunResult, error) {
	if a.config == nil {
		return nil, fmt.Errorf("agent config is not available")
	}

	if apiKey == "" || baseURL == "" {
		if modelName != "" {
			return a.RunWithModel(ctx, query, modelName)
		}
		return a.Run(ctx, query)
	}

	overrideRunner, err := a.newRunnerWithConfig(ctx, modelName, apiKey, baseURL)
	if err != nil {
		return nil, err
	}

	return a.runWithRunner(ctx, query, overrideRunner)
}

func (a *NanoAgent) newRunner(ctx context.Context, modelName string) (*adk.Runner, error) {
	return a.newRunnerWithConfig(ctx, modelName, a.config.LLM.APIKey, resolveBaseURL(a.config))
}

func (a *NanoAgent) newRunnerWithConfig(ctx context.Context, modelName, apiKey, baseURL string) (*adk.Runner, error) {
	if a.config == nil {
		return nil, fmt.Errorf("agent config is not available")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("agent LLM API key is not configured")
	}
	if modelName == "" {
		modelName = defaultModel()
	}
	if baseURL == "" {
		baseURL = resolveBaseURL(a.config)
	}

	chatModel, err := llm.NewChatModel(ctx, apiKey, baseURL, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create override chat model: %w", err)
	}

	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        a.Name,
		Description: "YouTube-to-Bilibili video processing AI agent",
		Instruction: loadInstruction(a.config, a.logger),
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: a.Tools},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create override chat agent: %w", err)
	}

	return adk.NewRunner(ctx, adk.RunnerConfig{Agent: chatAgent}), nil
}

func (a *NanoAgent) runWithRunner(ctx context.Context, query string, runner *adk.Runner) (*RunResult, error) {
	if runner == nil {
		return nil, fmt.Errorf("agent LLM runner is unavailable")
	}

	start := time.Now()
	a.logger.Info("agent run started", zap.String("query", truncate(query, 120)))

	iter := runner.Query(ctx, query)

	var finalAnswer string
	var steps []StepSummary

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return &RunResult{Success: false, Duration: time.Since(start)},
				fmt.Errorf("agent error: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil || msg == nil {
			continue
		}
		for _, tc := range msg.ToolCalls {
			steps = append(steps, StepSummary{
				Tool:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		if msg.Content != "" {
			finalAnswer = msg.Content
		}
	}

	a.logger.Info("agent run completed",
		zap.Int("steps", len(steps)),
		zap.Duration("duration", time.Since(start)))

	return &RunResult{
		FinalAnswer: finalAnswer,
		Success:     finalAnswer != "",
		Steps:       steps,
		Duration:    time.Since(start),
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
