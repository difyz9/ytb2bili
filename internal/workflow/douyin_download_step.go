package workflow

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type DownloadDouyinVideoStep struct {
	*ToolStep
}

type DownloadDouyinVideoStepParams struct {
	fx.In
	Tool   *tools.DownloadDouyinVideoTool
	Logger *zap.Logger
}

func NewDownloadDouyinVideoStep(params DownloadDouyinVideoStepParams) *DownloadDouyinVideoStep {
	return &DownloadDouyinVideoStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameDownloadDouyinVideo, true, 3),
			params.Tool,
			func(vctx *VideoContext) (string, error) {
				args := struct {
					ShareURL  string `json:"share_url,omitempty"`
					VideoInfo string `json:"video_info,omitempty"`
					FileName  string `json:"file_name,omitempty"`
				}{
					ShareURL: vctx.VideoURL,
					FileName: vctx.VideoID,
				}
				if vctx.DouyinVideoInfo != nil {
					videoInfoRaw, err := sonic.MarshalString(vctx.DouyinVideoInfo)
					if err != nil {
						return "", fmt.Errorf("marshal douyin video info: %w", err)
					}
					args.VideoInfo = videoInfoRaw
				}
				return sonic.MarshalString(args)
			},
			func(vctx *VideoContext, result string) error {
				var downloadResult tools.DouyinDownloadResult
				if err := sonic.UnmarshalString(result, &downloadResult); err != nil {
					return fmt.Errorf("unmarshal douyin download result: %w", err)
				}

				vctx.Platform = "douyin"
				vctx.VideoPath = downloadResult.FilePath
				if vctx.VideoID == "" {
					vctx.VideoID = downloadResult.VideoID
				}
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				return vctx.VideoPath != ""
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok {
					return nil
				}
				params.Logger.Info("Douyin video downloaded",
					zap.String("video_id", vctx.VideoID),
					zap.String("path", vctx.VideoPath))
				return nil
			}),
		),
	}
}
