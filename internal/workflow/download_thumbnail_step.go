package workflow

import (
	"context"
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 3: 下载缩略图（可选）
// ============================================================================

type DownloadThumbnailStep struct {
	*ToolStep
}

type DownloadThumbnailStepParams struct {
	fx.In
	Tool   *tools.DownloadThumbnailTool
	Logger *zap.Logger
}

func NewDownloadThumbnailStep(params DownloadThumbnailStepParams) *DownloadThumbnailStep {
	return &DownloadThumbnailStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameDownloadThumbnail, false, 3),
			params.Tool,
			func(vctx *VideoContext) (string, error) {
				return fmt.Sprintf(`{"video_url":%q}`, vctx.VideoURL), nil
			},
			func(vctx *VideoContext, result string) error {
				vctx.ThumbnailPath = result
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
				return vctx.VideoURL == "" || !settings.DownloadThumbnail
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok {
					return nil
				}
				params.Logger.Info("Thumbnail downloaded", zap.String("path", vctx.ThumbnailPath))
				return nil
			}),
			WithOnError(func(ctx context.Context, err error) error {
				params.Logger.Warn("Thumbnail download failed, continuing", zap.Error(err))
				return nil
			}),
			WithSkipOnError(),
			),
		}
	}
