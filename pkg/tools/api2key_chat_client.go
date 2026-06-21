package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/zap"
)

// LLMChatClient implements BatchLLMClient using an eino-based LLM client
// pointed at an OpenAI-compatible endpoint.
type LLMChatClient struct {
	client       *llm.EinoChatClient
	defaultModel string
	logger       *zap.Logger
}

// NewLLMChatClient creates an LLMChatClient backed by eino.
// baseURL should be an LLM API endpoint.
// Either apiKey or accessToken must be non-empty; apiKey takes precedence.
func NewLLMChatClient(baseURL, apiKey, accessToken, modelName string, logger *zap.Logger) (*LLMChatClient, error) {
	effectiveKey := strings.TrimSpace(apiKey)
	if effectiveKey == "" {
		effectiveKey = strings.TrimSpace(accessToken)
	}
	if effectiveKey == "" {
		return nil, fmt.Errorf("api key or access token is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	resolvedModel := strings.TrimSpace(modelName)
	if resolvedModel == "" {
		resolvedModel = llm.DefaultTranslationModel
	}

	client, err := llm.NewClient(effectiveKey, baseURL, resolvedModel, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create eino chat client: %w", err)
	}

	return &LLMChatClient{
		client:       client,
		defaultModel: resolvedModel,
		logger:       logger,
	}, nil
}

func (c *LLMChatClient) Chat(ctx context.Context, messages []TranslationChatMessage) (string, error) {
	return c.ChatWithOptions(ctx, messages, TranslationChatOptions{})
}

func (c *LLMChatClient) ChatWithOptions(ctx context.Context, messages []TranslationChatMessage, opts TranslationChatOptions) (string, error) {
	if c == nil || c.client == nil {
		return "", fmt.Errorf("LLM chat client is not initialized")
	}

	llmMessages := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    strings.TrimSpace(msg.Role),
			Content: msg.Content,
		})
	}

	modelName := strings.TrimSpace(opts.Model)
	if modelName == "" {
		modelName = c.defaultModel
	}

	chatOpts := buildLLMChatOptions(modelName, opts)

	content, err := c.client.ChatWithOptions(ctx, llmMessages, chatOpts)
	if err != nil {
		return "", err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("chat completion returned empty content")
	}
	return content, nil
}

func buildLLMChatOptions(modelName string, opts TranslationChatOptions) llm.ChatOptions {
	chatOpts := llm.ChatOptions{Model: modelName}
	if opts.Temperature != nil {
		chatOpts.Temperature = opts.Temperature
	}
	if opts.MaxTokens != nil {
		chatOpts.MaxTokens = opts.MaxTokens
	}
	return chatOpts
}

// NewLLMChatClientFromConfig creates a BatchLLMClient from the global LLMConfig.
func NewLLMChatClientFromConfig(appCfg *config.AppConfig, logger *zap.Logger) (*LLMChatClient, error) {
	return NewLLMChatClient(
		appCfg.LLMBaseURL(),
		appCfg.LLMAPIKey(),
		"",
		appCfg.LLMModel(),
		logger,
	)
}
