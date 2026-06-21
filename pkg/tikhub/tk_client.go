package tikhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(cfg *config.AppConfig) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.TikHub.BaseURL, "/"),
		apiKey:  cfg.TikHub.APIKey,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func NewClientWithCredentials(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.baseURL) != "" && strings.TrimSpace(c.apiKey) != ""
}

type Request struct {
	Method  string
	Path    string
	Query   map[string]any
	Headers map[string]string
	Body    json.RawMessage
}

func (c *Client) Do(ctx context.Context, req Request) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("TIKHUB_API_KEY is required to call TikHub endpoints")
	}
	if req.Method == "" {
		return "", fmt.Errorf("method is required")
	}
	if req.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	u, err := url.Parse(c.baseURL + req.Path)
	if err != nil {
		return "", fmt.Errorf("build url: %w", err)
	}

	q := u.Query()
	for k, v := range req.Query {
		if v == nil {
			continue
		}
		// 处理不同类型的值
		switch val := v.(type) {
		case string:
			q.Set(k, val)
		case []byte:
			q.Set(k, string(val))
		default:
			// 对于数字、布尔值等，使用 Sprint
			q.Set(k, fmt.Sprint(val))
		}
	}
	u.RawQuery = q.Encode()

	var body io.Reader
	if len(req.Body) > 0 && strings.ToUpper(req.Method) != http.MethodGet {
		body = bytes.NewReader(req.Body)
	}

	h, err := http.NewRequestWithContext(ctx, req.Method, u.String(), body)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	h.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		h.Header.Set("Content-Type", "application/json")
	}
	for k, v := range req.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		h.Header.Set(k, v)
	}

	resp, err := c.http.Do(h)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tikhub http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return string(b), nil
}
