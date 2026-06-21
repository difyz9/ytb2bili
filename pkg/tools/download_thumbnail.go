package tools

import (
	"context"
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

const downloadThumbnailToolName = "download_thumbnail"

// ThumbnailQuality is a resolution tier for YouTube thumbnails.
type ThumbnailQuality string

const (
	QualityMaxRes  ThumbnailQuality = "maxresdefault" // 1920×1080
	QualitySD      ThumbnailQuality = "sddefault"     // 640×480
	QualityHigh    ThumbnailQuality = "hqdefault"     // 480×360
	QualityMedium  ThumbnailQuality = "mqdefault"     // 320×180
	QualityDefault ThumbnailQuality = "default"       // 120×90
)

// DownloadThumbnailTool downloads the cover image for a YouTube video.
// Implements eino's tool.InvokableTool (Info + InvokableRun).
// Also exposes Call() for direct workflow use.
type DownloadThumbnailTool struct {
	downloadDir string
	httpClient  *http.Client
	logger      *zap.Logger
}

// NewDownloadThumbnailTool creates a DownloadThumbnailTool.
func NewDownloadThumbnailTool(downloadDir string, logger *zap.Logger) (*DownloadThumbnailTool, error) {
	if downloadDir == "" {
		return nil, fmt.Errorf("download directory is required")
	}
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}
	return &DownloadThumbnailTool{
		downloadDir: downloadDir,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		logger:      logger,
	}, nil
}

// ── eino tool.InvokableTool ───────────────────────────────────────────────────

// Info describes the tool to the LLM.
func (t *DownloadThumbnailTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "download_thumbnail",
		Desc: "Download the thumbnail (cover image) for a YouTube video. Returns the local file path of the highest-quality thumbnail successfully downloaded.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_url": {
				Type:     schema.String,
				Desc:     "YouTube video URL or 11-character video ID",
				Required: true,
			},
			"quality": {
				Type: schema.String,
				Desc: "Preferred quality: maxresdefault, sddefault, hqdefault, mqdefault, default (auto-degrades if unavailable)",
			},
		}),
	}, nil
}

// InvokableRun is called by the eino agent.
func (t *DownloadThumbnailTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	type thumbnailArgs struct {
		VideoURL string `json:"video_url"`
		Quality  string `json:"quality,omitempty"`
	}
	args, err := UnmarshalArgs[thumbnailArgs](downloadThumbnailToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	if err := RequireString(downloadThumbnailToolName, "video_url", args.VideoURL); err != nil {
		return "", err
	}
	return t.download(ctx, args.VideoURL, ThumbnailQuality(args.Quality))
}

// ── Direct workflow use ───────────────────────────────────────────────────────

// Call accepts a plain YouTube URL or video ID (for direct workflow use).
func (t *DownloadThumbnailTool) Call(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("video URL or ID cannot be empty")
	}
	return t.download(ctx, input, "")
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (t *DownloadThumbnailTool) download(ctx context.Context, videoURL string, preferredQuality ThumbnailQuality) (string, error) {
	videoID := t.extractVideoID(videoURL)
	t.logger.Info("Starting thumbnail download", zap.String("video_id", videoID))

	videoDir := filepath.Join(t.downloadDir, videoID)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create video directory: %w", err)
	}

	// Build quality order; if a preferred quality is given, try it first.
	qualities := []ThumbnailQuality{QualityMaxRes, QualitySD, QualityHigh, QualityMedium, QualityDefault}
	if preferredQuality != "" && preferredQuality != QualityMaxRes {
		// Put preferred first, then fall back through the rest
		reordered := []ThumbnailQuality{preferredQuality}
		for _, q := range qualities {
			if q != preferredQuality {
				reordered = append(reordered, q)
			}
		}
		qualities = reordered
	}

	var lastErr error
	for _, quality := range qualities {
		thumbnailURL := fmt.Sprintf("https://img.youtube.com/vi/%s/%s.jpg", videoID, quality)
		savePath := filepath.Join(videoDir, fmt.Sprintf("thumbnail_%s.jpg", quality))

		t.logger.Debug("Trying quality", zap.String("quality", string(quality)), zap.String("url", thumbnailURL))

		if err := t.downloadFile(ctx, thumbnailURL, savePath); err == nil {
			t.logger.Info("Thumbnail downloaded", zap.String("path", savePath), zap.String("quality", string(quality)))
			return savePath, nil
		} else {
			lastErr = err
			t.logger.Debug("Quality not available", zap.String("quality", string(quality)), zap.Error(err))
		}
	}
	return "", fmt.Errorf("failed to download thumbnail at any quality: %w", lastErr)
}

func (t *DownloadThumbnailTool) downloadFile(ctx context.Context, url, savePath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
	// Very small responses are YouTube placeholder images
	if resp.ContentLength > 0 && resp.ContentLength < 1000 {
		return fmt.Errorf("thumbnail too small (placeholder): %d bytes", resp.ContentLength)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(savePath)
		return fmt.Errorf("failed to write file: %w", err)
	}
	if written < 1000 {
		os.Remove(savePath)
		return fmt.Errorf("downloaded file too small: %d bytes", written)
	}
	return nil
}

func (t *DownloadThumbnailTool) extractVideoID(input string) string {
	// Already an 11-char ID?
	if len(input) == 11 && !strings.Contains(input, "/") && !strings.Contains(input, "?") {
		return input
	}
	if strings.Contains(input, "v=") {
		parts := strings.Split(input, "v=")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "&")[0]
			if len(id) == 11 {
				return id
			}
		}
	}
	if strings.Contains(input, "youtu.be/") {
		parts := strings.Split(input, "youtu.be/")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "?")[0]
			if len(id) == 11 {
				return id
			}
		}
	}
	return input
}

// NewDownloadThumbnailToolFromAppConfig 从 AppConfig 创建 DownloadThumbnailTool。
func NewDownloadThumbnailToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) (*DownloadThumbnailTool, error) {
	return NewDownloadThumbnailTool(appCfg.Workflow.DownloadDir, logger)
}
