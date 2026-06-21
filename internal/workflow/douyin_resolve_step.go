package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type ResolveDouyinShareStep struct {
	*ToolStep
}

type ResolveDouyinShareStepParams struct {
	fx.In
	Tool   *tools.FetchVideoByShareURLTool
	Logger *zap.Logger
}

func NewResolveDouyinShareStep(params ResolveDouyinShareStepParams) *ResolveDouyinShareStep {
	return &ResolveDouyinShareStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameResolveDouyinShare, true, 2),
			params.Tool,
			func(vctx *VideoContext) (string, error) {
				return fmt.Sprintf(`{"share_url":%q}`, vctx.VideoURL), nil
			},
			func(vctx *VideoContext, result string) error {
				var info tools.DouyinVideoInfo
				if err := sonic.UnmarshalString(result, &info); err != nil {
					return fmt.Errorf("unmarshal douyin video info: %w", err)
				}

				vctx.Platform = "douyin"
				vctx.DouyinVideoInfo = &info
				if vctx.VideoID == "" {
					vctx.VideoID = strings.TrimSpace(info.Data.AwemeID)
				}
				if vctx.Title == "" {
					vctx.Title = strings.TrimSpace(info.Data.Desc)
				}
				if shareURL := strings.TrimSpace(info.Data.ShareURL); shareURL != "" {
					vctx.VideoURL = shareURL
				}
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				return vctx.DouyinVideoInfo != nil && strings.TrimSpace(vctx.DouyinVideoInfo.Data.AwemeID) != ""
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok {
					return nil
				}
				params.Logger.Info("Douyin share resolved",
					zap.String("video_id", vctx.VideoID),
					zap.String("title", vctx.Title))
				return nil
			}),
		),
	}
}
