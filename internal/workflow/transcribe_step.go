package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 5: 转录音频（可选）
// ============================================================================

type TranscribeStep struct {
	*ToolStep
}

type TranscribeStepParams struct {
	fx.In
	Tool   *tools.BcutTranscriberTool
	Logger *zap.Logger
}

func NewTranscribeStep(params TranscribeStepParams) *TranscribeStep {
	return &TranscribeStep{
		ToolStep: NewToolStep(
			NewBaseStepWithOrder(StepNameTranscribe, false, 5),
			StringCallRunner{Tool: params.Tool, JSONField: "audio_path", Name: StepNameTranscribe},
			func(vctx *VideoContext) (string, error) {
				return fmt.Sprintf(`{"audio_path":%q}`, vctx.AudioPath), nil
			},
			func(vctx *VideoContext, result string) error {
				var transcript tools.TranscriptResult
				if err := json.Unmarshal([]byte(result), &transcript); err != nil {
					return fmt.Errorf("parse transcript failed: %w", err)
				}
				vctx.Transcript = &transcript
				return nil
			},
			WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
				settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
				return !settings.Transcribe
			}),
			WithOnSuccess(func(ctx context.Context, output any) error {
				vctx, ok := output.(*VideoContext)
				if !ok || vctx.Transcript == nil {
					return nil
				}
				params.Logger.Info("Audio transcribed",
					zap.Int("segments", len(vctx.Transcript.Segments)),
					zap.String("language", vctx.Transcript.Language))
				return nil
			}),
			WithSkipOnError(),
		),
	}
}

// TranscribeStep 的包装结构
