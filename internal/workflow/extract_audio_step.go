package workflow

import (
	"context"
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 4: 提取音频
// ============================================================================

type ExtractAudioStep struct {
	*ToolStep
}

type ExtractAudioStepParams struct {
	fx.In
	Tool   *tools.ExtractAudioTool
	Logger *zap.Logger
}

func NewExtractAudioStep(params ExtractAudioStepParams) *ExtractAudioStep {
	return &ExtractAudioStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameExtractAudio, true, 4),
			params.Tool,
			func(vctx *VideoContext) (string, error) {
				return fmt.Sprintf(`{"video_path":%q}`, vctx.VideoPath), nil
			},
			func(vctx *VideoContext, result string) error {
				vctx.AudioPath = result
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
				return !settings.Transcribe
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok {
					return nil
				}
				params.Logger.Info("Audio extracted", zap.String("path", vctx.AudioPath))
				return nil
			}),
		),
	}
}
