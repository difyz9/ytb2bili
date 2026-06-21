package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// LLM 批量字幕翻译步骤（统一 BatchTranslator）
// 模型来自配置 [translation] 或 [llm]，不依赖用户设置覆盖。
// ============================================================================

type LLMTranslateStep struct {
	BaseStep
	translator  *tools.BatchTranslator
	logger      *zap.Logger
	downloadDir string
}

type LLMTranslateStepParams struct {
	fx.In
	Translator *tools.BatchTranslator `optional:"true"`
	Logger     *zap.Logger
	AppConfig  *config.AppConfig `optional:"true"`
}

func NewLLMTranslateStep(params LLMTranslateStepParams) *LLMTranslateStep {
	translator := params.Translator
	if translator == nil && params.AppConfig != nil {
		p := params.AppConfig.ResolveTranslationProvider()
		if p != nil && p.ToLLMConfig().IsValid() {
			chatLLM, err := llm.NewClientFromConfig(p.ToLLMConfig(), params.Logger)
			if err == nil {
				wf := params.AppConfig.Workflow
				translator = tools.NewBatchTranslator(chatLLM, tools.BatchTranslatorConfig{
					SourceLang:  wf.LLMTranslationSourceLang,
					TargetLang:  wf.LLMTranslationTargetLang,
					BatchSize:   wf.LLMTranslationBatchSize,
					MaxWorkers:  wf.LLMTranslationMaxWorkers,
					ContextSize: wf.LLMTranslationContextSize,
				}, params.Logger)
				params.Logger.Info("LLMTranslateStep: created runtime BatchTranslator",
					zap.String("model", chatLLM.ModelName()))
			}
		}
	}

	downloadDir := ""
	if params.AppConfig != nil {
		downloadDir = params.AppConfig.Workflow.DownloadDir
	}

	return &LLMTranslateStep{
		BaseStep:    NewBaseStepWithOrder(StepNameLLMTranslate, false, 6),
		translator:  translator,
		logger:      params.Logger,
		downloadDir: downloadDir,
	}
}

func (s *LLMTranslateStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	if s.translator == nil {
		s.logger.Warn("LLM subtitle translator unavailable, skipping translation step")
		return vctx, nil
	}

	segments := collectTranscriptTextSegments(vctx.Transcript)
	if len(segments) == 0 {
		vctx.TranslationSkipped = true
		s.logger.Warn("No transcript available, skipping LLM subtitle translation")
		return vctx, nil
	}

	s.logger.Info("Starting LLM subtitle translation",
		zap.Int("total_segments", len(segments)),
		zap.String("model", s.translator.ModelName()),
		zap.String("source_lang", resolveSourceLang(vctx)),
		zap.String("target_lang", resolveTargetLang(vctx)))

	texts := transcriptTexts(segments)
	if len(texts) == 0 {
		vctx.TranslationSkipped = true
		s.logger.Warn("No text to translate")
		return vctx, nil
	}

	runConfig := tools.TranslationRunConfig{
		SourceLang: resolveSourceLang(vctx),
		TargetLang: resolveTargetLang(vctx),
		UserID:     strings.TrimSpace(vctx.UserID),
	}

	result, err := s.translator.TranslateTextsWithConfig(ctx, texts, runConfig)
	if err != nil {
		s.logger.Error("LLM subtitle translation failed", zap.Error(err))
		return vctx, &StepSkippedError{
			Step: s.Name(), Cause: err, Output: vctx,
		}
	}
	vctx.TranslationSkipped = result.SkippedTranslation
	vctx.SubtitleAudios = buildSubtitleAudiosFromTranslations(segments, result.TranslatedTexts)

	s.logger.Info("LLM subtitle translation completed",
		zap.Int("total_segments", len(segments)),
		zap.Int("translated_count", len(vctx.SubtitleAudios)),
		zap.Bool("translation_skipped", result.SkippedTranslation),
		zap.Duration("duration", result.Duration))

	if err := s.saveTranslatedSubtitles(vctx); err != nil {
		return vctx, &StepSkippedError{Step: s.Name(), Cause: err, Output: vctx}
	}

	return vctx, nil
}

func (s *LLMTranslateStep) ShouldSkip(ctx context.Context, input any) bool {
	if vctx, ok := input.(*VideoContext); ok {
		return !NormalizeTaskChainSettings(vctx.TaskChainSettings).TranslateSubtitles
	}
	return false
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func resolveSourceLang(vctx *VideoContext) string {
	if vctx != nil && vctx.TranslationConfig != nil && vctx.TranslationConfig.SourceLanguage != "" {
		return vctx.TranslationConfig.SourceLanguage
	}
	return "en"
}

func resolveTargetLang(vctx *VideoContext) string {
	if vctx != nil && vctx.TranslationConfig != nil && vctx.TranslationConfig.TargetLanguage != "" {
		return vctx.TranslationConfig.TargetLanguage
	}
	return "zh-Hans"
}

// ── SRT file saving ──────────────────────────────────────────────────────────

func (s *LLMTranslateStep) saveTranslatedSubtitles(vctx *VideoContext) error {
	if len(vctx.SubtitleAudios) == 0 {
		return nil
	}

	videoDir := filepath.Dir(vctx.VideoPath)
	if videoDir == "." || videoDir == "" {
		if s.downloadDir != "" {
			videoDir = s.downloadDir
		} else {
			videoDir = "."
		}
		s.logger.Warn("VideoPath is empty, falling back to download directory",
			zap.String("fallback_dir", videoDir),
			zap.String("video_id", vctx.VideoID))
	}

	// Save original subtitles
	enPath := filepath.Join(videoDir, vctx.VideoID+".en.srt")
	if err := writeSRT(enPath, vctx.SubtitleAudios, false); err != nil {
		return fmt.Errorf("save English subtitles: %w", err)
	}
	s.logger.Info("Saved English subtitles", zap.String("path", enPath))

	// Save translated subtitles
	zhPath := filepath.Join(videoDir, vctx.VideoID+".zh.srt")
	if err := writeSRT(zhPath, vctx.SubtitleAudios, true); err != nil {
		return fmt.Errorf("save Chinese subtitles: %w", err)
	}
	s.logger.Info("Saved Chinese subtitles", zap.String("path", zhPath))

	// Also save without language suffix
	defPath := filepath.Join(videoDir, vctx.VideoID+".srt")
	if err := writeSRT(defPath, vctx.SubtitleAudios, true); err != nil {
		return fmt.Errorf("save default subtitles: %w", err)
	}

	return nil
}

func writeSRT(path string, subtitles []SubtitleAudio, useTranslated bool) error {
	var content strings.Builder
	for i, sub := range subtitles {
		content.WriteString(fmt.Sprintf("%d\n", i+1))
		start := formatSRTTime(sub.StartTime)
		end := formatSRTTime(sub.EndTime)
		content.WriteString(fmt.Sprintf("%s --> %s\n", start, end))
		if useTranslated {
			content.WriteString(sub.TranslatedText)
		} else {
			content.WriteString(sub.OriginalText)
		}
		content.WriteString("\n\n")
	}
	return os.WriteFile(path, []byte(content.String()), 0644)
}

func formatSRTTime(seconds float64) string {
	h := int(seconds / 3600)
	m := int((seconds - float64(h*3600)) / 60)
	s := int(seconds - float64(h*3600) - float64(m*60))
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
