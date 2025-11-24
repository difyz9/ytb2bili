package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeepSeekClient DeepSeek API客户端
type DeepSeekClient struct {
	APIKey     string
	BaseURL    string
	Client     *http.Client
	MaxRetries int
	RetryDelay time.Duration
}

// DeepSeekRequest API请求结构
type DeepSeekRequest struct {
	Model    string            `json:"model"`
	Messages []DeepSeekMessage `json:"messages"`
	Stream   bool              `json:"stream"`
	Settings *DeepSeekSettings `json:"settings,omitempty"`
}

// DeepSeekMessage 消息结构
type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeekSettings API设置
type DeepSeekSettings struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// DeepSeekResponse API响应结构
type DeepSeekResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []DeepSeekChoice `json:"choices"`
	Usage   DeepSeekUsage    `json:"usage"`
}

// DeepSeekChoice 选择结构
type DeepSeekChoice struct {
	Index        int             `json:"index"`
	Message      DeepSeekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// DeepSeekUsage 使用量统计
type DeepSeekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewDeepSeekClient 创建DeepSeek客户端
func NewDeepSeekClient(apiKey string) *DeepSeekClient {
	return &DeepSeekClient{
		APIKey:     apiKey,
		BaseURL:    "https://api.deepseek.com/v1/chat/completions",
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatCompletion 执行对话补全（带重试机制）
func (c *DeepSeekClient) ChatCompletion(systemPrompt, userPrompt string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.RetryDelay * time.Duration(attempt))
		}

		result, err := c.doRequest(systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 如果是API限制错误，延长等待时间
		if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "429") {
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
		}
	}

	return "", fmt.Errorf("重试 %d 次后仍然失败: %v", c.MaxRetries, lastErr)
}

// doRequest 执行单次API请求
func (c *DeepSeekClient) doRequest(systemPrompt, userPrompt string) (string, error) {
	request := DeepSeekRequest{
		Model: "deepseek-chat",
		Messages: []DeepSeekMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false,
		Settings: &DeepSeekSettings{
			Temperature: 0.3,  // 降低随机性，提高一致性
			MaxTokens:   4000, // 增加最大token数
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	var response DeepSeekResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("API响应中没有结果")
	}

	return response.Choices[0].Message.Content, nil
}

// ChatCompletionWithUsage 执行对话补全并返回使用量统计
func (c *DeepSeekClient) ChatCompletionWithUsage(systemPrompt, userPrompt string) (string, *DeepSeekUsage, error) {
	request := DeepSeekRequest{
		Model: "deepseek-chat",
		Messages: []DeepSeekMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	var response DeepSeekResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", nil, fmt.Errorf("API响应中没有结果")
	}

	return response.Choices[0].Message.Content, &response.Usage, nil
}
