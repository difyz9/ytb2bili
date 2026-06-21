package workflow

import (
	"context"

	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 6: 翻译字幕
// ============================================================================

type TranslateStep struct {
	BaseStep
	translator *tools.MicrosoftTranslator
	logger     *zap.Logger
}

type TranslateStepParams struct {
	fx.In
	Translator *tools.MicrosoftTranslator
	Logger     *zap.Logger
}

func NewTranslateStep(params TranslateStepParams) *TranslateStep {
	return &TranslateStep{
		BaseStep:   NewBaseStepWithOrder(StepNameTranslate, false, 6),
		translator: params.Translator,
		logger:     params.Logger,
	}
}

func (s *TranslateStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	vctx.TranslationConfig = normalizeWorkflowTranslationConfig(vctx.TranslationConfig)
	segments := collectTranscriptTextSegments(vctx.Transcript)

	// 如果没有转录结果，跳过翻译
	if len(segments) == 0 {
		s.logger.Warn("No transcript available, skipping translation")
		return vctx, nil
	}

	vctx.SubtitleAudios = make([]SubtitleAudio, 0, len(segments))

	// 记录开始时间和字符数
	totalChars := 0
	sourceLang := vctx.TranslationConfig.SourceLanguage
	targetLang := vctx.TranslationConfig.TargetLanguage

	for _, segment := range segments {
		totalChars += len(segment.Text)
		originalText := segment.Text

		// 调用翻译API
		translatedText, err := s.translator.TranslateText(originalText, sourceLang, targetLang)
		if err != nil {
			s.logger.Warn("Failed to translate segment, skipping",
				zap.String("text", originalText),
				zap.Error(err))
			continue
		}

		// 存储翻译结果
		vctx.SubtitleAudios = append(vctx.SubtitleAudios, SubtitleAudio{
			OriginalText:   originalText,
			TranslatedText: translatedText,
			StartTime:      segment.Start,
			EndTime:        segment.End,
		})
	}

	s.logger.Info("Translation completed",
		zap.Int("total_segments", len(segments)),
		zap.Int("translated_count", len(vctx.SubtitleAudios)),
		zap.Int("total_chars", totalChars))

	return vctx, nil
}

