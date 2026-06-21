package agent

import "github.com/zolagz/ytb2bili/pkg/llm"

// Config holds agent configuration. Matches the [agent] section of config.toml (unchanged schema).
type Config struct {
	Name          string           `toml:"name"`
	MaxIterations int              `toml:"max_iterations"`
	APIKey        string           `toml:"api_key"`
	BaseURL       string           `toml:"base_url"`
	LLM           LLMConfig        `toml:"llm"`

	// InstructionFile 指定外部 System Prompt 文件路径（相对于工作目录或绝对路径）。
	// 留空时使用内置默认提示词；如需覆盖，再显式指定外部文件路径。
	InstructionFile string `toml:"instruction_file"`
}

// LLMConfig holds optional server-side fallback settings for the agent.
// Provider and Model allow independent routing (e.g. agent uses GPT-4o while translation uses DeepSeek).
type LLMConfig struct {
	Provider string `toml:"provider,omitempty"` // openai / deepseek / ollama / … (optional; if empty, uses global fallback)
	Model    string `toml:"model,omitempty"`    // model name (optional; if empty, uses default model)
	APIKey  string `toml:"api_key"`  // 可选：服务端兜底 API Key
	BaseURL string `toml:"base_url"` // 可选兜底上游，留空则使用 pkg/llm 默认地址
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Name:          "ytb2bili-agent",
		MaxIterations: 10,
		BaseURL:       llm.DefaultBaseURL,
		LLM: LLMConfig{
			BaseURL: llm.DefaultBaseURL,
		},
	}
}

func (c *Config) Normalize() {
	if c == nil {
		return
	}
	if c.APIKey == "" {
		c.APIKey = c.LLM.APIKey
	}
	if c.BaseURL == "" {
		c.BaseURL = c.LLM.BaseURL
	}
	if c.LLM.APIKey == "" {
		c.LLM.APIKey = c.APIKey
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = c.BaseURL
	}
	if c.BaseURL == "" {
		c.BaseURL = llm.DefaultBaseURL
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = c.BaseURL
	}
}
