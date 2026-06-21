package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/internal/config"
	clientpkg "github.com/zolagz/ytb2bili/pkg/tikhub"
	"go.uber.org/zap"
)

const douyinDownloadToolName = "download_douyin_video"

type DownloadDouyinVideoTool struct {
	downloadDir string
	resolver    clientpkg.Resolver
	logger      *zap.Logger
}

type douyinDownloadArgs struct {
	ShareURL   string `json:"share_url"`
	VideoInfo  string `json:"video_info"`
	FileName   string `json:"file_name"`
	NumWorkers int    `json:"num_workers"`
	ChunkSize  int64  `json:"chunk_size"`
}

type DouyinDownloadRequest struct {
	ShareURL     string
	VideoInfo    *DouyinVideoInfo
	VideoInfoRaw string
	FileName     string
}

type DouyinDownloadResult struct {
	VideoID  string `json:"video_id"`
	Title    string `json:"title,omitempty"`
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	URL      string `json:"url"`
}

func NewDownloadDouyinVideoTool(downloadDir string, logger *zap.Logger) (*DownloadDouyinVideoTool, error) {
	if downloadDir == "" {
		return nil, fmt.Errorf("download directory is required")
	}
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DownloadDouyinVideoTool{
		downloadDir: downloadDir,
		resolver:    clientpkg.NewDirectResolver(logger),
		logger:      logger,
	}, nil
}

func (t *DownloadDouyinVideoTool) SetResolver(resolver clientpkg.Resolver) {
	t.resolver = resolver
}

func (t *DownloadDouyinVideoTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "download_douyin_video",
		Desc: "下载抖音视频到本地，支持并发分块下载和断点续传。可传 share_url，也可传 fetch_one_video_by_share_url 的完整返回结果作为 video_info。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"share_url": {
				Type: schema.String,
				Desc: "抖音分享链接。与 video_info 二选一。",
			},
			"video_info": {
				Type: schema.String,
				Desc: "fetch_one_video_by_share_url 返回的完整 JSON 字符串。与 share_url 二选一。",
			},
			"file_name": {
				Type: schema.String,
				Desc: "自定义文件名，可选。",
			},
			"num_workers": {
				Type: schema.Integer,
				Desc: "并发下载线程数，可选，默认 8。",
			},
			"chunk_size": {
				Type: schema.Integer,
				Desc: "分块大小，单位字节，可选，默认 5MB。",
			},
		}),
	}, nil
}

func (t *DownloadDouyinVideoTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	args, err := UnmarshalArgs[douyinDownloadArgs](douyinDownloadToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	result, err := t.Download(ctx, DouyinDownloadRequest{
		ShareURL:     args.ShareURL,
		VideoInfoRaw: args.VideoInfo,
		FileName:     args.FileName,
	})
	if err != nil {
		return "", err
	}
	data, err := sonic.MarshalString(result)
	if err != nil {
		return "", fmt.Errorf("marshal download result: %w", err)
	}
	return data, nil
}

func (t *DownloadDouyinVideoTool) Download(ctx context.Context, req DouyinDownloadRequest) (*DouyinDownloadResult, error) {
	if t.resolver == nil {
		return nil, fmt.Errorf("douyin resolver is not configured")
	}

	input := clientpkg.DownloadRequest{
		ShareURL:     req.ShareURL,
		VideoInfoRaw: req.VideoInfoRaw,
		FileName:     req.FileName,
		OutputDir:    t.downloadDir,
	}
	if req.VideoInfo != nil {
		payload, err := sonic.MarshalString(req.VideoInfo)
		if err != nil {
			return nil, fmt.Errorf("marshal video_info: %w", err)
		}
		input.VideoInfoRaw = payload
	}

	result, err := clientpkg.NewDirectDownloader(t.resolver, t.logger).Download(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("download douyin video: %w", err)
	}

	return &DouyinDownloadResult{
		VideoID:  result.VideoID,
		Title:    strings.TrimSpace(result.Title),
		FilePath: result.FilePath,
		FileName: result.FileName,
		FileSize: result.FileSize,
		URL:      result.URL,
	}, nil
}

// NewDownloadDouyinVideoToolFromAppConfig 从 AppConfig 创建 DownloadDouyinVideoTool。
func NewDownloadDouyinVideoToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) (*DownloadDouyinVideoTool, error) {
	return NewDownloadDouyinVideoTool(appCfg.Workflow.DownloadDir, logger)
}
