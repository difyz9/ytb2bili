package workflow

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 2: 下载视频
// ============================================================================

type DownloadVideoStep struct {
	*ToolStep
}

type DownloadVideoStepParams struct {
	fx.In
	Tool   *tools.DownloadVideoTool
	Logger *zap.Logger
}

func NewDownloadVideoStep(params DownloadVideoStepParams) *DownloadVideoStep {
	return &DownloadVideoStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameDownloadVideo, true, 2),
			params.Tool,
			func(vctx *VideoContext) (string, error) {
				return fmt.Sprintf(`{"video_url":%q,"format":%q}`, vctx.VideoURL, vctx.PreferredResolution), nil
			},
			func(vctx *VideoContext, result string) error {
				vctx.VideoPath = result
				if vctx.VideoID == "" {
					vctx.VideoID = extractVideoID(result)
				}
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				return vctx.VideoPath != ""
			}),
			WithRunContext(func(ctx context.Context, vctx *VideoContext) (context.Context, error) {
				tracker := GetProgressTracker(ctx)
				workflowVideoID := GetVideoID(ctx)
				return tools.WithDownloadProgressReporter(ctx, func(update tools.DownloadProgressUpdate) {
					if tracker == nil || workflowVideoID == "" {
						return
					}
					tracker.UpdateStepProgress(workflowVideoID, StepNameDownloadVideo, update.Percent, update.Message)
				}), nil
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok {
					return nil
				}
				params.Logger.Info("Video downloaded",
					zap.String("video_id", vctx.VideoID),
					zap.String("path", vctx.VideoPath))
				return nil
			}),
		),
	}
}

// extractVideoID 从视频路径提取视频 ID
func extractVideoID(videoPath string) string {
	dir := filepath.Dir(videoPath)
	return filepath.Base(dir)
}
