package llm

import (
	"context"
	"fmt"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"
)

// LangChainGoClient 基于 langchaingo 的 LLM 客户端
type LangChainGoClient struct {
	llm    llms.Model
	logger *zap.Logger
}

// NewOpenAIClient 创建 OpenAI 客户端
func NewOpenAIClient(apiKey string, model string, logger *zap.Logger) (*LangChainGoClient, error) {
	if model == "" {
		model = "gpt-4o" // 默认模型
	}
	
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}
	
	logger.Info("OpenAI client created",
		zap.String("model", model))
	
	return &LangChainGoClient{
		llm:    llm,
		logger: logger,
	}, nil
}

// NewOpenAICompatibleClient 创建兼容 OpenAI API 的客户端（如 DeepSeek, 本地模型等）
func NewOpenAICompatibleClient(apiKey, baseURL, model string, logger *zap.Logger) (*LangChainGoClient, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI-compatible client: %w", err)
	}
	
	logger.Info("OpenAI-compatible client created",
		zap.String("base_url", baseURL),
		zap.String("model", model))
	
	return &LangChainGoClient{
		llm:    llm,
		logger: logger,
	}, nil
}

// Chat 同步对话接口
func (c *LangChainGoClient) Chat(ctx context.Context, messages []agentic.Message) (string, error) {
	c.logger.Debug("Starting chat",
		zap.Int("message_count", len(messages)))
	
	// 转换消息格式
	lcMessages := make([]llms.MessageContent, 0, len(messages))
	for _, msg := range messages {
		// 将 agentic.Message 转换为 langchaingo 的消息格式
		var role llms.ChatMessageType
		switch msg.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "tool":
			role = llms.ChatMessageTypeTool
		default:
			role = llms.ChatMessageTypeHuman
		}
		
		lcMessages = append(lcMessages, llms.MessageContent{
			Role: role,
			Parts: []llms.ContentPart{
				llms.TextPart(msg.Content),
			},
		})
	}
	
	// 调用 LLM
	response, err := c.llm.GenerateContent(ctx, lcMessages)
	if err != nil {
		c.logger.Error("LLM chat failed", zap.Error(err))
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}
	
	// 提取响应文本
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	
	result := response.Choices[0].Content
	
	c.logger.Debug("Chat completed",
		zap.Int("response_length", len(result)))
	
	return result, nil
}

// ChatStream 流式对话接口
func (c *LangChainGoClient) ChatStream(ctx context.Context, messages []agentic.Message) (<-chan string, error) {
	c.logger.Debug("Starting streaming chat",
		zap.Int("message_count", len(messages)))
	
	// 转换消息格式
	lcMessages := make([]llms.MessageContent, 0, len(messages))
	for _, msg := range messages {
		var role llms.ChatMessageType
		switch msg.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "tool":
			role = llms.ChatMessageTypeTool
		default:
			role = llms.ChatMessageTypeHuman
		}
		
		lcMessages = append(lcMessages, llms.MessageContent{
			Role: role,
			Parts: []llms.ContentPart{
				llms.TextPart(msg.Content),
			},
		})
	}
	
	// 创建输出通道
	outputChan := make(chan string, 100)
	
	// 在 goroutine 中处理流式响应
	go func() {
		defer close(outputChan)
		
		err := c.llm.GenerateContent(
			ctx,
			lcMessages,
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				select {
				case outputChan <- string(chunk):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}),
		)
		
		if err != nil {
			c.logger.Error("Streaming chat failed", zap.Error(err))
		}
	}()
	
	return outputChan, nil
}

// ChatWithOptions 带选项的对话
func (c *LangChainGoClient) ChatWithOptions(
	ctx context.Context,
	messages []agentic.Message,
	options ChatOptions,
) (string, error) {
	// 转换消息
	lcMessages := make([]llms.MessageContent, 0, len(messages))
	for _, msg := range messages {
		var role llms.ChatMessageType
		switch msg.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "tool":
			role = llms.ChatMessageTypeTool
		default:
			role = llms.ChatMessageTypeHuman
		}
		
		lcMessages = append(lcMessages, llms.MessageContent{
			Role: role,
			Parts: []llms.ContentPart{
				llms.TextPart(msg.Content),
			},
		})
	}
	
	// 构建 langchaingo 选项
	var callOptions []llms.CallOption
	
	if options.Temperature > 0 {
		callOptions = append(callOptions, llms.WithTemperature(options.Temperature))
	}
	if options.MaxTokens > 0 {
		callOptions = append(callOptions, llms.WithMaxTokens(options.MaxTokens))
	}
	if options.TopP > 0 {
		callOptions = append(callOptions, llms.WithTopP(options.TopP))
	}
	
	// 调用 LLM
	response, err := c.llm.GenerateContent(ctx, lcMessages, callOptions...)
	if err != nil {
		return "", fmt.Errorf("LLM generation with options failed: %w", err)
	}
	
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	
	return response.Choices[0].Content, nil
}

// ChatOptions 对话选项
type ChatOptions struct {
	Temperature float64 // 温度参数，控制随机性 (0.0-2.0)
	MaxTokens   int     // 最大 token 数
	TopP        float64 // 核采样参数 (0.0-1.0)
	Stop        []string // 停止序列
}
