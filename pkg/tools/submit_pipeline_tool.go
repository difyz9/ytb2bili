package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
)

const submitPipelineToolName = "submit_pipeline"

var youtubeRe = regexp.MustCompile(`(?:youtube\.com/(?:watch\?[^\s]*v=|shorts/)|youtu\.be/)([A-Za-z0-9_-]{11})`)

// SubmitPipelineTool submits a YouTube URL, YouTube video ID, or Douyin share URL to the full processing pipeline.
type SubmitPipelineTool struct {
	serverPort int
	logger     *zap.Logger
	client     *http.Client
}

// NewSubmitPipelineTool creates a SubmitPipelineTool that posts to the local server.
func NewSubmitPipelineTool(serverPort int, logger *zap.Logger) *SubmitPipelineTool {
	return &SubmitPipelineTool{
		serverPort: serverPort,
		logger:     logger,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Info describes the tool to the LLM.
func (t *SubmitPipelineTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "submit_pipeline",
		Desc: "将 YouTube 视频链接、YouTube 视频 ID 或抖音分享链接提交到完整处理流水线（下载→转录→翻译字幕→合成字幕音频→生成元数据→准备上传B站）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {
				Type:     schema.String,
				Desc:     "YouTube 视频 URL / 视频 ID，或抖音分享链接，如 https://www.youtube.com/watch?v=xxx、https://youtu.be/xxx、https://v.douyin.com/xxxxx/",
				Required: true,
			},
			"user_id": {
				Type: schema.String,
				Desc: "用户ID（可选，留空使用系统默认）",
			},
			"voice_name": {
				Type: schema.String,
				Desc: "字幕音频合成所用音色（可选），默认 zh-CN-XiaoxiaoNeural",
			},
		}),
	}, nil
}

type pipelineParams struct {
	URL       string `json:"url"`
	UserID    string `json:"user_id"`
	VoiceName string `json:"voice_name"`
}

// InvokableRun submits the URL to the pipeline.
func (t *SubmitPipelineTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	params, err := UnmarshalArgs[pipelineParams](submitPipelineToolName, args)
	if err != nil {
		return "", err
	}
	if err := RequireString(submitPipelineToolName, "url", params.URL); err != nil {
		return "", err
	}

	// Normalise: if someone passes only a video ID, build the full URL
	if m := youtubeRe.FindStringSubmatch(params.URL); m == nil {
		// Might be a bare YouTube video ID (11 chars)
		if len(params.URL) == 11 {
			params.URL = "https://www.youtube.com/watch?v=" + params.URL
		}
	}

	voiceName := params.VoiceName
	if voiceName == "" {
		voiceName = "zh-CN-XiaoxiaoNeural"
	}

	payload := map[string]any{
		"url":     params.URL,
		"user_id": params.UserID,
		"task_chain_settings": map[string]bool{
			"download_thumbnail":        true,
			"transcribe":                true,
			"translate_subtitles":       true,
			"synthesize_subtitle_audio": true,
		},
		"speech_synthesis_config": map[string]string{
			"language":   "zh-CN",
			"voice_name": voiceName,
			"format":     "mp3",
		},
	}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/api/v1/video-process/async-submit-link", t.serverPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("提交失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			VideoID string `json:"video_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("响应解析失败: %w\n原始响应: %s", err, respBody)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("提交失败: %s", result.Message)
	}

	t.logger.Info("视频已提交处理流水线", zap.String("video_id", result.Data.VideoID), zap.String("url", params.URL))
	return fmt.Sprintf("✅ 视频已加入处理队列！\n视频ID: %s\n\n处理流程（下载→转录→翻译→合成字幕音频）将在后台进行，可在「任务队列」页面查看进度。", result.Data.VideoID), nil
}

// NewSubmitPipelineToolFromAppConfig 从 AppConfig 创建 SubmitPipelineTool。
func NewSubmitPipelineToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) *SubmitPipelineTool {
	return NewSubmitPipelineTool(appCfg.Server.Port, logger)
}
