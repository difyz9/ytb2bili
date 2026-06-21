package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/internal/service"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/zap"
)

// SynthesizeSubtitleAudioStep 合成字幕音频步骤
type SynthesizeSubtitleAudioStep struct {
	BaseStep
	ttsClient    *tools.TTSClient
	userSettings *service.UserSettingsClient
	logger       *zap.Logger
}

// NewSynthesizeSubtitleAudioStep 创建合成字幕音频步骤
func NewSynthesizeSubtitleAudioStep(ttsClient *tools.TTSClient, userSettings *service.UserSettingsClient, logger *zap.Logger) *SynthesizeSubtitleAudioStep {
	return &SynthesizeSubtitleAudioStep{
		BaseStep:     NewBaseStepWithOrder(StepNameSynthesizeSubtitle, false, 7),
		ttsClient:    ttsClient,
		userSettings: userSettings,
		logger:       logger,
	}
}

// Execute 执行步骤
func (s *SynthesizeSubtitleAudioStep) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	vctx, ok := input.(*VideoContext)
	if !ok {
		return nil, fmt.Errorf("invalid input type: expected *VideoContext, got %T", input)
	}

	if len(vctx.SubtitleAudios) == 0 {
		segments := collectTranscriptTextSegments(vctx.Transcript)
		if len(segments) > 0 {
			vctx.SubtitleAudios = buildSubtitleAudiosFromTranscript(segments)
			s.logger.Info("使用转录字幕作为配音输入",
				zap.String("videoID", vctx.VideoID),
				zap.Int("subtitleCount", len(vctx.SubtitleAudios)))
		}
	}

	// 检查是否有翻译后的字幕数据
	// 翻译步骤会将结果存储在 SubtitleAudios 中
	if len(vctx.SubtitleAudios) == 0 {
		s.logger.Warn("没有翻译后的字幕数据，跳过音频合成")
		return vctx, nil
	}

	// 检查 TTS 客户端是否可用
	if s.ttsClient == nil {
		s.logger.Warn("TTS 客户端未配置，跳过音频合成")
		return vctx, nil
	}

	s.logger.Info("开始合成字幕音频",
		zap.String("videoID", vctx.VideoID),
		zap.String("voiceName", speechVoiceName(vctx.SpeechSynthesisConfig)),
		zap.Int("subtitleCount", len(vctx.SubtitleAudios)))

	// 为每条字幕合成音频
	successCount := 0
	failedCount := 0
	totalChars := 0
	tracker := GetProgressTracker(ctx)
	voiceName := speechVoiceName(vctx.SpeechSynthesisConfig)

	// 遍历已翻译的字幕（SubtitleAudios 由翻译步骤填充）
	for i := range vctx.SubtitleAudios {
		subtitle := &vctx.SubtitleAudios[i]

		// 优先使用翻译后的文本，如果没有则使用原始文本
		text := subtitle.TranslatedText
		if text == "" {
			text = subtitle.OriginalText
		}

		// 跳过空文本
		if text == "" {
			s.logger.Debug("跳过空字幕", zap.Int("index", i))
			continue
		}

		totalChars += len(text)
		if tracker != nil {
			tracker.UpdateStepProgress(vctx.VideoID, StepNameSynthesizeSubtitle, (i*100)/len(vctx.SubtitleAudios), fmt.Sprintf("使用 %s 合成 %d/%d 条字幕", voiceName, i+1, len(vctx.SubtitleAudios)))
		}

		s.logger.Debug("合成字幕音频",
			zap.Int("index", i),
			zap.String("text", text),
			zap.String("voiceName", voiceName),
			zap.Float64("startTime", subtitle.StartTime),
			zap.Float64("endTime", subtitle.EndTime))

		// 调用 TTS 服务合成音频
		resp, err := s.ttsClient.SynthesizeSubtitleAudio(ctx, vctx.UserID, text, vctx.VideoID, i, subtitleAudioDir(vctx), vctx.SpeechSynthesisConfig)
		if err != nil {
			s.logger.Error("合成字幕音频失败",
				zap.Int("index", i),
				zap.String("text", text),
				zap.Error(err))
			failedCount++
			continue
		}
		audioPath := strings.TrimSpace(resp.LocalPath)
		if audioPath == "" {
			audioPath = strings.TrimSpace(resp.AudioURL)
		}
		if audioPath == "" {
			audioPath = strings.TrimSpace(resp.CosURL)
		}
		if audioPath == "" {
			s.logger.Warn("字幕音频合成未返回可用地址",
				zap.Int("index", i),
				zap.String("provider", resp.Provider),
				zap.String("storageKey", resp.CosKey))
			failedCount++
			continue
		}

		// 更新字幕音频信息（添加音频路径）
		subtitle.AudioPath = audioPath
		successCount++

		s.logger.Info("字幕音频合成成功",
			zap.Int("index", i),
			zap.String("audioURL", audioPath),
			zap.String("localPath", resp.LocalPath),
			zap.String("storageKey", resp.CosKey),
			zap.String("provider", resp.Provider),
			zap.String("voice", resp.Voice),
			zap.Int64("fileSize", resp.FileSize))
	}

	s.logger.Info("字幕音频合成完成",
		zap.String("videoID", vctx.VideoID),
		zap.Int("total", len(vctx.SubtitleAudios)),
		zap.Int("success", successCount),
		zap.Int("failed", failedCount),
		zap.Int("total_chars", totalChars))

	// 如果全部失败，返回警告但不中断流程
	if successCount == 0 && len(vctx.SubtitleAudios) > 0 {
		s.logger.Warn("所有字幕音频合成失败，但继续执行后续步骤")
	}

	return vctx, nil
}

func speechVoiceName(config *SpeechSynthesisConfig) string {
	if config != nil && strings.TrimSpace(config.VoiceName) != "" {
		return strings.TrimSpace(config.VoiceName)
	}
	return tools.DefaultTTSVoice
}

func subtitleAudioDir(vctx *VideoContext) string {
	if vctx == nil {
		return ""
	}
	if strings.TrimSpace(vctx.VideoPath) != "" {
		return filepath.Join(filepath.Dir(vctx.VideoPath), "audio")
	}
	if strings.TrimSpace(vctx.VideoID) != "" {
		return filepath.Join("downloads", strings.TrimSpace(vctx.VideoID), "audio")
	}
	return ""
}

func (s *SynthesizeSubtitleAudioStep) ShouldSkip(ctx context.Context, input any) bool {
	if vctx, ok := input.(*VideoContext); ok {
		s.refreshTaskChainSettingsFromDB(ctx, vctx)
		settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
		return !settings.SynthesizeSubtitleAudio
	}
	return false
}

func (s *SynthesizeSubtitleAudioStep) refreshTaskChainSettingsFromDB(ctx context.Context, vctx *VideoContext) {
	if s.userSettings == nil || !s.userSettings.IsEnabled() || vctx == nil {
		return
	}

	userID := strings.TrimSpace(vctx.UserID)
	if userID == "" {
		userID = strings.TrimSpace(GetUserID(ctx))
		if userID != "" {
			vctx.UserID = userID
		}
	}
	if userID == "" {
		return
	}

	settings, err := s.userSettings.GetSettings(ctx, userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("读取用户任务链配置失败，使用当前上下文配置",
				zap.String("user_id", userID),
				zap.Error(err))
		}
		return
	}

	latest := parseWorkflowTaskChainSettings(settings[storemodel.UserSettingKeyTaskChainSettings])
	if latest == nil {
		return
	}

	vctx.TaskChainSettings = latest
	if s.logger != nil && !latest.SynthesizeSubtitleAudio {
		s.logger.Info("用户已关闭合成字幕配音，步骤将跳过",
			zap.String("user_id", userID),
			zap.String("video_id", strings.TrimSpace(vctx.VideoID)))
	}
}
