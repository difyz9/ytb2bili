package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
)

const imageGeneratorToolName = "generate_image"

// GenerateImageTool calls the kie.ai Nano Banana Pro API to generate images.
// Implements eino's tool.InvokableTool (Info + InvokableRun).
// Also exposes Call() for direct workflow use.
type GenerateImageTool struct {
	apiKey     string
	baseURL    string
	outputDir  string
	httpClient *http.Client
	logger     *zap.Logger
}

// GenerateImageConfig holds image generation parameters.
type GenerateImageConfig struct {
	Prompt       string `json:"prompt"`
	AspectRatio  string `json:"aspect_ratio"`
	Resolution   string `json:"resolution"`
	AutoDownload bool   `json:"auto_download"`
}

// imageTaskResponse is the API response when submitting a task.
type imageTaskResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

// taskStatusResponse is the polling response structure.
type taskStatusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Status string   `json:"status"` // pending | processing | completed | failed
		Images []string `json:"images"`
	} `json:"data"`
}

// NewGenerateImageTool creates a GenerateImageTool.
func NewGenerateImageTool(apiKey, outputDir string, logger *zap.Logger) *GenerateImageTool {
	if outputDir == "" {
		outputDir = "./generated_images"
	}
	os.MkdirAll(outputDir, 0755) //nolint:errcheck
	return &GenerateImageTool{
		apiKey:     apiKey,
		baseURL:    "https://kie.ai/api",
		outputDir:  outputDir,
		httpClient: &http.Client{Timeout: 300 * time.Second},
		logger:     logger,
	}
}

// ── eino tool.InvokableTool ───────────────────────────────────────────────────

// Info describes the tool to the LLM.
func (t *GenerateImageTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "generate_image",
		Desc: `Generate a high-quality image using the kie.ai Nano Banana Pro API. Returns task_id, image URLs, and local paths.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"prompt": {
				Type:     schema.String,
				Desc:     "Text prompt describing the image to generate (required)",
				Required: true,
			},
			"aspect_ratio": {
				Type: schema.String,
				Desc: "Aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, 21:9, 9:21, 2:3, 3:2, 5:4, 4:5 (default: 1:1)",
			},
			"resolution": {
				Type: schema.String,
				Desc: "Output resolution: 1K, 2K, 4K (default: 1K)",
			},
			"auto_download": {
				Type: schema.Boolean,
				Desc: "Whether to download generated images locally (default: true)",
			},
		}),
	}, nil
}

// InvokableRun is called by the eino agent.
func (t *GenerateImageTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	cfg, err := UnmarshalArgs[GenerateImageConfig](imageGeneratorToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	if err := RequireString(imageGeneratorToolName, "prompt", cfg.Prompt); err != nil {
		return "", err
	}
	// Apply defaults
	if cfg.AspectRatio == "" {
		cfg.AspectRatio = "1:1"
	}
	if cfg.Resolution == "" {
		cfg.Resolution = "1K"
	}
	// Delegate to Call so all logic stays in one place
	normalized, _ := json.Marshal(cfg)
	return t.Call(ctx, string(normalized))
}

// ── Direct workflow use ───────────────────────────────────────────────────────

// Call accepts either a plain prompt string or a JSON-encoded GenerateImageConfig.
func (t *GenerateImageTool) Call(ctx context.Context, input string) (string, error) {
	t.logger.Info("Generating image", zap.String("input", input))

	cfg := GenerateImageConfig{
		AspectRatio:  "1:1",
		Resolution:   "1K",
		AutoDownload: true,
	}

	if strings.HasPrefix(strings.TrimSpace(input), "{") {
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.logger.Warn("Failed to parse JSON input, treating as plain prompt", zap.Error(err))
			cfg.Prompt = input
		}
	} else {
		cfg.Prompt = strings.TrimSpace(input)
	}

	if cfg.Prompt == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	t.logger.Info("Config parsed",
		zap.String("prompt", cfg.Prompt),
		zap.String("aspect_ratio", cfg.AspectRatio),
		zap.String("resolution", cfg.Resolution))

	taskID, err := t.submitTask(ctx, cfg.Prompt, cfg.AspectRatio, cfg.Resolution)
	if err != nil {
		return "", fmt.Errorf("failed to submit task: %w", err)
	}
	t.logger.Info("Task submitted", zap.String("task_id", taskID))

	imageURLs, err := t.waitForTaskCompletion(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("task completion error: %w", err)
	}
	t.logger.Info("Task completed", zap.Strings("image_urls", imageURLs))

	var localPaths []string
	if cfg.AutoDownload && len(imageURLs) > 0 {
		localPaths, err = t.downloadImages(ctx, imageURLs, taskID)
		if err != nil {
			t.logger.Warn("Failed to download images", zap.Error(err))
		}
	}

	result := map[string]interface{}{
		"success":     true,
		"task_id":     taskID,
		"images":      imageURLs,
		"local_paths": localPaths,
		"prompt":      cfg.Prompt,
	}
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(resultJSON), nil
}

// ── Internal API calls ────────────────────────────────────────────────────────

func (t *GenerateImageTool) submitTask(ctx context.Context, prompt, aspectRatio, resolution string) (string, error) {
	url := fmt.Sprintf("%s/task/submit", t.baseURL)
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":        "nano-banana-pro",
		"prompt":       prompt,
		"aspect_ratio": aspectRatio,
		"resolution":   resolution,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}

	var taskResp imageTaskResponse
	if err := json.Unmarshal(body, &taskResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if taskResp.Code != 200 {
		return "", fmt.Errorf("API error (code=%d): %s", taskResp.Code, taskResp.Msg)
	}
	return taskResp.Data.TaskID, nil
}

func (t *GenerateImageTool) queryTaskStatus(ctx context.Context, taskID string) (*taskStatusResponse, error) {
	url := fmt.Sprintf("%s/task/query?task_id=%s", t.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}

	var statusResp taskStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if statusResp.Code != 200 {
		return nil, fmt.Errorf("API error (code=%d): %s", statusResp.Code, statusResp.Msg)
	}
	return &statusResp, nil
}

func (t *GenerateImageTool) waitForTaskCompletion(ctx context.Context, taskID string) ([]string, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("task timeout after 5 minutes")
		case <-ticker.C:
			statusResp, err := t.queryTaskStatus(ctx, taskID)
			if err != nil {
				t.logger.Warn("Failed to query task status", zap.Error(err))
				continue
			}
			t.logger.Info("Task status", zap.String("task_id", taskID), zap.String("status", statusResp.Data.Status))
			switch statusResp.Data.Status {
			case "completed":
				if len(statusResp.Data.Images) == 0 {
					return nil, fmt.Errorf("task completed but no images returned")
				}
				return statusResp.Data.Images, nil
			case "failed":
				return nil, fmt.Errorf("task failed")
			case "pending", "processing":
				// continue polling
			default:
				t.logger.Warn("Unknown task status", zap.String("status", statusResp.Data.Status))
			}
		}
	}
}

func (t *GenerateImageTool) downloadImages(ctx context.Context, imageURLs []string, taskID string) ([]string, error) {
	var localPaths []string
	for i, imageURL := range imageURLs {
		localPath := filepath.Join(t.outputDir, fmt.Sprintf("%s_%d.png", taskID, i))
		if err := t.downloadImage(ctx, imageURL, localPath); err != nil {
			t.logger.Error("Failed to download image", zap.String("url", imageURL), zap.Error(err))
			continue
		}
		localPaths = append(localPaths, localPath)
		t.logger.Info("Image downloaded", zap.String("path", localPath))
	}
	if len(localPaths) == 0 {
		return nil, fmt.Errorf("failed to download any images")
	}
	return localPaths, nil
}

func (t *GenerateImageTool) downloadImage(ctx context.Context, imageURL, localPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	if _, err = io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// NewGenerateImageToolFromAppConfig 从 AppConfig 创建 GenerateImageTool。
// outputDir 使用 DownloadDir 下的 "images" 子目录。
func NewGenerateImageToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) *GenerateImageTool {
	outputDir := filepath.Join(appCfg.Workflow.DownloadDir, "images")
	return NewGenerateImageTool(appCfg.LLM.APIKey, outputDir, logger)
}
