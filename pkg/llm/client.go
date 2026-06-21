// Package llm provides LLM (Large Language Model) client implementations.
// Backed by cloudwego/eino — the same framework used by the agent and all tools.
//
// File layout:
//
//	client.go            — core type, NewClient constructor, NewClientFromConfig
//	chat.go              — Chat / ChatStream / ChatWithOptions methods
//	provider.go          — ProviderConfig, provider constants, validation
//	types.go             — Message, ChatOptions, helper types
//	zap_helpers.go       — zap.Error field helper
package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"
)

const (
	// DefaultBaseURL points to OpenAI's API by default. Users can override via config.
	DefaultBaseURL = "https://api.openai.com"
	DefaultModel   = "gpt-4o-mini"
	// DefaultTranslationModel is the default model for subtitle translation.
	DefaultTranslationModel = "gpt-4o-mini"
	// DefaultTimeout is the default per-request timeout in seconds.
	DefaultTimeout = 120
)

// EinoChatClient is an eino-based LLM client backed by einoopenai.
type EinoChatClient struct {
	chatModel model.ToolCallingChatModel
	apiKey    string
	baseURL   string
	modelName string
	timeout   time.Duration
	logger    *zap.Logger
}

func normalizeConfig(baseURL, modelName string) (string, string) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	// Ensure the base URL includes an API version path prefix.
	// The go-openai library constructs the final URL as: {baseURL}/{endpoint_suffix}
	// Example: baseURL="https://api.deepseek.com" → "https://api.deepseek.com/chat/completions" (WRONG - 404)
	// The correct URL requires a version prefix: "https://api.deepseek.com/v1/chat/completions"
	if !strings.Contains(baseURL, "/v1") && !strings.Contains(baseURL, "/v4") {
		baseURL = baseURL + "/v1"
	}
	if modelName == "" {
		modelName = DefaultModel
	}
	return baseURL, modelName
}

func resolveTimeout(timeoutSeconds *int) time.Duration {
	if timeoutSeconds != nil && *timeoutSeconds > 0 {
		return time.Duration(*timeoutSeconds) * time.Second
	}
	return time.Duration(DefaultTimeout) * time.Second
}

// createChatModel creates an eino OpenAI-compatible chat model.
func createChatModel(ctx context.Context, apiKey, baseURL, modelName string, timeout time.Duration) (model.ToolCallingChatModel, error) {
	baseURL, modelName = normalizeConfig(baseURL, modelName)

	m, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		Model:   modelName,
		APIKey:  apiKey,
		BaseURL: baseURL,
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Eino chat model: %w", err)
	}
	return m, nil
}

// NewChatModel creates a raw eino ToolCallingChatModel from explicit credentials.
func NewChatModel(ctx context.Context, apiKey, baseURL, modelName string) (model.ToolCallingChatModel, error) {
	return createChatModel(ctx, apiKey, baseURL, modelName, time.Duration(DefaultTimeout)*time.Second)
}

// ── Constructors ─────────────────────────────────────────────────────────────

// NewClient creates an EinoChatClient from explicit credentials.
// Kept for backward compatibility; prefer NewClientFromConfig for new code.
func NewClient(apiKey, baseURL, modelName string, logger *zap.Logger) (*EinoChatClient, error) {
	baseURL, modelName = normalizeConfig(baseURL, modelName)
	if logger == nil {
		logger = zap.NewNop()
	}
	timeout := time.Duration(DefaultTimeout) * time.Second

	m, err := createChatModel(context.Background(), apiKey, baseURL, modelName, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	logger.Info("LLM client created",
		zap.String("model", modelName),
		zap.String("base_url", baseURL))

	return &EinoChatClient{
		chatModel: m,
		apiKey:    apiKey,
		baseURL:   baseURL,
		modelName: modelName,
		timeout:   timeout,
		logger:    logger,
	}, nil
}

// NewClientFromConfig creates an EinoChatClient from a ProviderConfig.
// This is the recommended constructor for new code — it handles provider-specific
// defaults (base URL, model name) and supports all OpenAI-compatible providers.
//
// Usage:
//
//	client, err := llm.NewClientFromConfig(&ProviderConfig{
//	    Provider: "deepseek",
//	    Model:    "deepseek-chat",
//	    APIKey:   "sk-...",
//	}, logger)
//
//	client, err := llm.NewClientFromConfig(&ProviderConfig{
//	    Provider: "ollama",
//	    Model:    "llama3",
//	    BaseURL:  "http://localhost:11434",
//	    // APIKey is optional for ollama
//	}, logger)
func NewClientFromConfig(cfg *ProviderConfig, logger *zap.Logger) (*EinoChatClient, error) {
	resolved := cfg.Resolve()

	if err := ValidateProvider(resolved.Provider); err != nil {
		return nil, err
	}

	if !isOpenAICompatible(resolved.Provider) {
		return nil, fmt.Errorf("provider %q is not yet supported via OpenAI-compatible mode; "+
			"currently supported: openai, deepseek, ollama, qwen, zhipu, groq, custom", resolved.Provider)
	}

		baseURL := resolved.BaseURL
		modelName := resolved.Model

		if resolved.Provider == ProviderOllama {
			baseURL = strings.TrimRight(baseURL, "/") + "/v1"
		} else {
			baseURL, modelName = normalizeConfig(baseURL, modelName)
		}

	if logger == nil {
		logger = zap.NewNop()
	}
	timeout := resolveTimeout(resolved.Timeout)

	m, err := createChatModel(context.Background(), resolved.APIKey, baseURL, modelName, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client for provider %q: %w", resolved.Provider, err)
	}

	logger.Info("LLM client created from config",
		zap.String("provider", resolved.Provider),
		zap.String("model", modelName),
		zap.String("base_url", baseURL))

	return &EinoChatClient{
		chatModel: m,
		apiKey:    resolved.APIKey,
		baseURL:   baseURL,
		modelName: modelName,
		timeout:   timeout,
		logger:    logger,
	}, nil
}

// ChatModel returns the underlying Eino chat model so callers like ADK agents
// can share the same supplier configuration as the plain LLM client.
func (c *EinoChatClient) ChatModel() model.ToolCallingChatModel {
	if c == nil {
		return nil
	}
	return c.chatModel
}

// ModelName returns the configured model name.
func (c *EinoChatClient) ModelName() string {
	if c == nil {
		return ""
	}
	return c.modelName
}

// BaseURL returns the configured API base URL.
func (c *EinoChatClient) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}
