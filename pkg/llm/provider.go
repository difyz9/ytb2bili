// Package llm provides LLM (Large Language Model) client implementations.
// Backed by cloudwego/eino — the same framework used by the agent and all tools.
//
// Provider architecture:
//
//	ProviderConfig describes any LLM provider (openai / deepseek / ollama / ...).
//	NewClientFromConfig creates a client from a ProviderConfig.
//	The same *EinoChatClient is then used by all consumers (translator, agent, tools).
//
// Adding a new provider:
//
//	1. Add a Provider constant below.
//	2. Add a case in NewClientFromConfig / isOpenAICompatible.
//	3. (optional) Override defaults in providerDefaults().
package llm

import (
	"fmt"
	"strings"
)

// ── Provider constants ──────────────────────────────────────────────────────

const (
	ProviderOpenAI  = "openai"
	ProviderDeepSeek = "deepseek"
	ProviderOllama  = "ollama"
	ProviderQwen    = "qwen"
	ProviderZhipu   = "zhipu"
	ProviderGroq    = "groq"
	ProviderMoonshot = "moonshot"
	ProviderOpenRouter = "openrouter"
	ProviderCerebras  = "cerebras"
	ProviderCustom    = "custom"
	ProviderAnthropic = "anthropic" // future
	ProviderGemini    = "gemini"    // future
)

// ProviderConfig describes a complete LLM provider connection.
// Each use case (translation / chat / agent) gets its own ProviderConfig
// so they can route to different vendors independently.
type ProviderConfig struct {
	Provider    string   `toml:"provider"`              // openai / deepseek / ollama / …
	Model       string   `toml:"model"`                 // model name, e.g. "gpt-4o-mini", "deepseek-chat"
	BaseURL     string   `toml:"base_url,omitempty"`    // API endpoint (without /v1)
	APIKey      string   `toml:"api_key,omitempty"`     // API key (empty = no auth, e.g. local ollama)
	Temperature *float64 `toml:"temperature,omitempty"` // generation temperature
	MaxTokens   *int     `toml:"max_tokens,omitempty"`  // max tokens per request
	Timeout     *int     `toml:"timeout,omitempty"`     // request timeout in seconds
}

// IsValid returns true when the provider has an API key (or is a local-only provider like ollama).
func (p *ProviderConfig) IsValid() bool {
	if p == nil {
		return false
	}
	switch p.Provider {
	case ProviderOllama: // local-only, no key required
		return p.BaseURL != ""
	default:
		return strings.TrimSpace(p.APIKey) != ""
	}
}

// Resolve returns a copy of p with defaults filled in for empty fields.
func (p *ProviderConfig) Resolve() *ProviderConfig {
	out := &ProviderConfig{}
	if p != nil {
		*out = *p
	}
	if out.Provider == "" {
		out.Provider = ProviderOpenAI
	}
	if out.Model == "" {
		out.Model = DefaultModel
	}
	if out.BaseURL == "" {
		out.BaseURL = providerDefaults(out.Provider).baseURL
	}
	out.BaseURL = strings.TrimRight(out.BaseURL, "/")
	return out
}

// providerDefault holds the default base URL for a provider.
type providerDefault struct {
	baseURL string
}

// providerDefaults returns sensible defaults per provider type.
func providerDefaults(provider string) providerDefault {
	switch provider {
	case ProviderDeepSeek:
		return providerDefault{baseURL: "https://api.deepseek.com"}
	case ProviderOllama:
		return providerDefault{baseURL: "http://localhost:11434"}
	case ProviderQwen:
		return providerDefault{baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"}
	case ProviderZhipu:
		return providerDefault{baseURL: "https://open.bigmodel.cn/api/paas/v4/"}
	case ProviderGroq:
	return providerDefault{baseURL: "https://api.groq.com/openai/v1"}
	case ProviderOpenRouter:
		return providerDefault{baseURL: "https://openrouter.ai/api/v1"}
	default: // openai / custom / etc.
		return providerDefault{baseURL: DefaultBaseURL + "/v1"}
	}
}

// isOpenAICompatible returns true for providers that speak the OpenAI wire protocol.
func isOpenAICompatible(provider string) bool {
	switch provider {
	case ProviderOpenAI, ProviderDeepSeek, ProviderQwen, ProviderZhipu,
		ProviderGroq, ProviderMoonshot, ProviderOpenRouter, ProviderCerebras,
		ProviderCustom, ProviderOllama, "":
		return true
	}
	return false
}

// StandardModels returns a list of well-known model names for a provider.
// Used for reference / UI display.
func StandardModels(provider string) []string {
	switch provider {
	case ProviderOpenAI:
		return []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"}
	case ProviderDeepSeek:
		return []string{"deepseek-chat", "deepseek-reasoner"}
	case ProviderOllama:
		return []string{"llama3", "llama3.1", "mistral", "qwen2", "deepseek-r1"}
	default:
		return nil
	}
}

// ValidateProvider returns an error if the provider name is unknown.
func ValidateProvider(provider string) error {
	switch provider {
	case ProviderOpenAI, ProviderDeepSeek, ProviderOllama, ProviderQwen,
		ProviderZhipu, ProviderGroq, ProviderMoonshot, ProviderOpenRouter,
		ProviderCerebras, ProviderCustom, ProviderAnthropic, ProviderGemini, "":
		return nil
	}
	return fmt.Errorf("unknown LLM provider: %q (supported: openai, deepseek, ollama, qwen, zhipu, groq, ...)", provider)
}
