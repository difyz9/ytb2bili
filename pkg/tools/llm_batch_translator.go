package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

// LLMBatchTranslatorConfig configures LLM-backed subtitle translation.
type LLMBatchTranslatorConfig struct {
	// LLMClient is an optional prebuilt translation client.
	LLMClient BatchLLMClient
	// 用户设置客户端，用于在运行时读取用户翻译偏好
	UserSettings UserSettingsProvider

	// 翻译配置
	SourceLang string // 源语言，默认 "en"
	TargetLang string // 目标语言，默认 "zh-Hans"

	// 批处理配置
	BatchSize  int // 每批翻译的句子数，默认 25
	MaxWorkers int // 最大并发数，默认 3
	RetryCount int // 重试次数，默认 2

	// 上下文配置
	ContextSize int // 上下文窗口大小（前后各取多少句），默认 2
}

// LLMBatchTranslator batches subtitle translation through LLM.
type LLMBatchTranslator struct {
	config       LLMBatchTranslatorConfig
	userSettings UserSettingsProvider
	logger       *zap.Logger
}

// TranslationResult 翻译结果
type TranslationResult struct {
	OriginalTexts      []string
	TranslatedTexts    []string
	TotalTokens        int
	Duration           time.Duration
	Errors             []error
	SkippedTranslation bool
	DetectedLanguage   string
}

type TranslationRunConfig struct {
	SourceLang string
	TargetLang string
	ModelName  string
	UserID     string
}

// NewLLMBatchTranslator creates an LLM-backed subtitle batch translator.
func NewLLMBatchTranslator(config LLMBatchTranslatorConfig, logger *zap.Logger) *LLMBatchTranslator {
	// 设置默认值
	if config.SourceLang == "" {
		config.SourceLang = "en"
	}
	if config.TargetLang == "" {
		config.TargetLang = "zh-Hans"
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 25
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 3
	}
	if config.RetryCount <= 0 {
		config.RetryCount = 2
	}
	if config.ContextSize < 0 {
		config.ContextSize = 2
	}

	return &LLMBatchTranslator{
		config:       config,
		userSettings: config.UserSettings,
		logger:       logger,
	}
}

// translateTask 翻译任务
type translateTask struct {
	groupIndex  int
	texts       []string
	prevContext []string
	nextContext []string
}

// translateResult 翻译任务结果
type translateResult struct {
	groupIndex int
	result     []string
	err        error
}

// TranslateTexts 批量翻译文本（并发分组处理）
func (t *LLMBatchTranslator) TranslateTexts(ctx context.Context, texts []string) (*TranslationResult, error) {
	return t.TranslateTextsWithConfig(ctx, texts, TranslationRunConfig{})
}

func (t *LLMBatchTranslator) TranslateTextsWithConfig(ctx context.Context, texts []string, runConfig TranslationRunConfig) (*TranslationResult, error) {
	startTime := time.Now()
	runtimeConfig := t.resolveRuntimeConfig(ctx, runConfig)

	if len(texts) == 0 {
		return &TranslationResult{
			OriginalTexts:   texts,
			TranslatedTexts: []string{},
			Duration:        time.Since(startTime),
		}, nil
	}

	t.logger.Info("Starting LLM subtitle translation",
		zap.Int("total_texts", len(texts)),
		zap.String("model", runtimeConfig.ModelName),
		zap.String("source_lang", runtimeConfig.SourceLang),
		zap.String("target_lang", runtimeConfig.TargetLang),
		zap.Int("batch_size", t.config.BatchSize),
		zap.Int("max_workers", t.config.MaxWorkers))

	shouldTranslate, detectedLanguage, decisionErr := t.shouldTranslate(ctx, texts, runtimeConfig)
	if decisionErr != nil {
		t.logger.Warn("Translation decision failed, fallback to translate",
			zap.Error(decisionErr),
			zap.String("target_lang", runtimeConfig.TargetLang))
		shouldTranslate = true
	}
	if !shouldTranslate {
		copiedTexts := append([]string(nil), texts...)
		return &TranslationResult{
			OriginalTexts:      texts,
			TranslatedTexts:    copiedTexts,
			Duration:           time.Since(startTime),
			SkippedTranslation: true,
			DetectedLanguage:   detectedLanguage,
		}, nil
	}

	totalGroups := (len(texts) + t.config.BatchSize - 1) / t.config.BatchSize

	// 创建任务和结果通道
	taskChannel := make(chan translateTask, totalGroups)
	resultChannel := make(chan translateResult, totalGroups)

	// 启动工作者协程
	var wg sync.WaitGroup
	for i := 0; i < t.config.MaxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			t.logger.Debug("Starting translation worker", zap.Int("worker_id", workerID))

			for task := range taskChannel {
				t.logger.Info("Worker processing group",
					zap.Int("worker_id", workerID),
					zap.Int("group", task.groupIndex+1),
					zap.Int("total_groups", totalGroups),
					zap.Int("sentences", len(task.texts)))

				// 执行翻译（带重试）
				translated, err := t.translateGroupWithRetry(ctx, task.texts, task.prevContext, task.nextContext, runtimeConfig)

				resultChannel <- translateResult{
					groupIndex: task.groupIndex,
					result:     translated,
					err:        err,
				}
			}
		}(i)
	}

	// 分发任务
	go func() {
		for i := 0; i < len(texts); i += t.config.BatchSize {
			end := i + t.config.BatchSize
			if end > len(texts) {
				end = len(texts)
			}

			currentGroup := texts[i:end]

			// 准备上下文窗口
			var prevContext, nextContext []string

			// 获取前置上下文
			if i > 0 && t.config.ContextSize > 0 {
				prevStart := i - t.config.ContextSize
				if prevStart < 0 {
					prevStart = 0
				}
				prevContext = texts[prevStart:i]
			}

			// 获取后置上下文
			if end < len(texts) && t.config.ContextSize > 0 {
				nextEnd := end + t.config.ContextSize
				if nextEnd > len(texts) {
					nextEnd = len(texts)
				}
				nextContext = texts[end:nextEnd]
			}

			taskChannel <- translateTask{
				groupIndex:  i / t.config.BatchSize,
				texts:       currentGroup,
				prevContext: prevContext,
				nextContext: nextContext,
			}
		}
		close(taskChannel)
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	// 处理结果
	results := make(map[int][]string)
	var errors []error
	var lastErr error

	for result := range resultChannel {
		if result.err != nil {
			t.logger.Error("Group translation failed",
				zap.Int("group", result.groupIndex+1),
				zap.Error(result.err))
			lastErr = result.err
			errors = append(errors, result.err)
			continue
		}
		results[result.groupIndex] = result.result
	}

	if lastErr != nil {
		return nil, fmt.Errorf("translation failed: %w (total errors: %d)", lastErr, len(errors))
	}

	// 合并结果
	var allTranslated []string
	for i := 0; i < totalGroups; i++ {
		if groupResult, ok := results[i]; ok {
			allTranslated = append(allTranslated, groupResult...)
		}
	}

	duration := time.Since(startTime)

	t.logger.Info("LLM subtitle translation completed",
		zap.Int("total_texts", len(texts)),
		zap.Int("translated_count", len(allTranslated)),
		zap.Duration("duration", duration))

	return &TranslationResult{
		OriginalTexts:    texts,
		TranslatedTexts:  allTranslated,
		Duration:         duration,
		Errors:           errors,
		DetectedLanguage: detectedLanguage,
	}, nil
}

func (t *LLMBatchTranslator) mergeRunConfig(runConfig TranslationRunConfig) TranslationRunConfig {
	merged := runConfig
	if merged.SourceLang == "" {
		merged.SourceLang = t.config.SourceLang
	}
	if merged.TargetLang == "" {
		merged.TargetLang = t.config.TargetLang
	}
	if merged.ModelName == "" {
		merged.ModelName = llm.DefaultTranslationModel
	}
	return merged
}

func (t *LLMBatchTranslator) resolveRuntimeConfig(ctx context.Context, runConfig TranslationRunConfig) TranslationRunConfig {
	resolved := runConfig
	userID := strings.TrimSpace(runConfig.UserID)

	if t.userSettings != nil && t.userSettings.IsEnabled() && userID != "" {
		settings, err := t.userSettings.GetSettings(ctx, userID)
		if err != nil {
			t.logger.Warn("Failed to load translation settings from database",
				zap.String("user_id", userID),
				zap.Error(err))
		} else {
			resolved = applyStoredTranslationSettings(resolved, settings)

			t.logger.Info("Loaded translation runtime config from database",
				zap.String("user_id", userID),
				zap.String("source_lang", strings.TrimSpace(resolved.SourceLang)),
				zap.String("target_lang", strings.TrimSpace(resolved.TargetLang)),
				zap.String("model", strings.TrimSpace(resolved.ModelName)))
		}
	}

	return t.mergeRunConfig(resolved)
}

func applyStoredTranslationSettings(runConfig TranslationRunConfig, settings map[string]string) TranslationRunConfig {
	resolved := runConfig
	if settings == nil {
		return resolved
	}
	if strings.TrimSpace(resolved.SourceLang) == "" {
		if sourceLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationSourceLang]); sourceLang != "" {
			resolved.SourceLang = sourceLang
		}
	}
	if strings.TrimSpace(resolved.TargetLang) == "" {
		if targetLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationTargetLang]); targetLang != "" {
			resolved.TargetLang = targetLang
		}
	}
	if shouldLoadStoredTranslationModel(resolved.ModelName) {
		if modelName := resolvePreferredTranslationModel(settings); modelName != "" {
			resolved.ModelName = modelName
		}
	}
	return resolved
}

func shouldLoadStoredTranslationModel(modelName string) bool {
	trimmed := strings.TrimSpace(modelName)
	return trimmed == "" || trimmed == llm.DefaultTranslationModel
}

func resolvePreferredTranslationModel(settings map[string]string) string {
	if settings == nil {
		return ""
	}
	if value := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationModel]); value != "" {
		return value
	}
	return strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel])
}

func (t *LLMBatchTranslator) chatWithModel(ctx context.Context, client BatchLLMClient, messages []TranslationChatMessage, modelName string) (string, error) {
	return client.ChatWithOptions(ctx, messages, TranslationChatOptions{Model: modelName})
}

func (t *LLMBatchTranslator) resolveLLMClient(ctx context.Context, runConfig TranslationRunConfig) (BatchLLMClient, error) {
	if t.config.LLMClient != nil {
		return t.config.LLMClient, nil
	}
	return nil, fmt.Errorf("LLM subtitle translation unavailable: no client configured")
}

// translateGroupWithRetry 带重试的分组翻译
func (t *LLMBatchTranslator) translateGroupWithRetry(ctx context.Context, texts []string, prevContext, nextContext []string, runConfig TranslationRunConfig) ([]string, error) {
	var lastErr error

	for attempt := 0; attempt <= t.config.RetryCount; attempt++ {
		if attempt > 0 {
			t.logger.Warn("Retrying translation",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", t.config.RetryCount))
			// 指数退避
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		result, err := t.translateGroupWithContext(ctx, texts, prevContext, nextContext, runConfig)
		if err == nil {
			return result, nil
		}

		lastErr = err
		t.logger.Warn("Translation attempt failed",
			zap.Int("attempt", attempt+1),
			zap.Error(err))
	}

	return nil, fmt.Errorf("translation failed after %d retries: %w", t.config.RetryCount, lastErr)
}

// translateGroupWithContext 带上下文翻译一组文本
func (t *LLMBatchTranslator) translateGroupWithContext(ctx context.Context, texts []string, prevContext, nextContext []string, runConfig TranslationRunConfig) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// 构建包含上下文的完整文本
	var fullTexts []string
	targetStartIndex := 0

	// 添加前置上下文
	if len(prevContext) > 0 {
		fullTexts = append(fullTexts, prevContext...)
		targetStartIndex = len(fullTexts)
	}

	// 添加目标翻译文本
	fullTexts = append(fullTexts, texts...)
	targetEndIndex := len(fullTexts)

	// 添加后置上下文
	if len(nextContext) > 0 {
		fullTexts = append(fullTexts, nextContext...)
	}

	// 构建系统提示
	contextInfo := ""
	if len(prevContext) > 0 || len(nextContext) > 0 {
		contextInfo = fmt.Sprintf(`

上下文信息：
- 前置上下文：%d 句（仅供参考，不需要翻译）
- 目标翻译：%d 句（位于第 %d-%d 句，需要全部翻译）
- 后置上下文：%d 句（仅供参考，不需要翻译）

请只翻译目标部分（第 %d-%d 句），但要充分考虑前后文的连贯性。`,
			len(prevContext), len(texts), targetStartIndex+1, targetEndIndex,
			len(nextContext), targetStartIndex+1, targetEndIndex)
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的视频字幕翻译专家。我将给你一段连续的%s字幕，其中包含 %d 句需要翻译的内容。%s

翻译要求：
1. 自然流畅：使用口语化表达，符合%s字幕习惯
2. 上下文连贯：理解整体语境，确保翻译前后呼应
3. 准确传神：忠实原文含义，保持语气和情感
4. 简洁明了：字幕需要快速阅读，避免冗长
5. 数量严格：必须输出 %d 句翻译，不多不少
6. 分隔符：每句翻译用"###SENTENCE_BREAK###"分隔

输入格式：句子用"###SENTENCE_BREAK###"分隔
输出格式：只返回目标部分的%s翻译，用"###SENTENCE_BREAK###"分隔

注意：只返回翻译的%s文本，不要添加序号、解释或其他内容。`,
		t.getLanguageName(runConfig.SourceLang),
		len(texts),
		contextInfo,
		t.getLanguageName(runConfig.TargetLang),
		len(texts),
		t.getLanguageName(runConfig.TargetLang),
		t.getLanguageName(runConfig.TargetLang))

	// 组合输入文本
	combinedText := strings.Join(fullTexts, "\n###SENTENCE_BREAK###\n")

	// 调用LLM
	messages := []TranslationChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: combinedText},
	}

	llmClient, err := t.resolveLLMClient(ctx, runConfig)
	if err != nil {
		return nil, err
	}

	translatedText, err := t.chatWithModel(ctx, llmClient, messages, runConfig.ModelName)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// 解析结果
	translatedSentences := strings.Split(translatedText, sentenceBreak)

	// 清理和验证
	for i := range translatedSentences {
		translatedSentences[i] = strings.TrimSpace(translatedSentences[i])
	}

	// 确保数量匹配
	if len(translatedSentences) != len(texts) {
		t.logger.Warn("Translation count mismatch, fixing",
			zap.Int("expected", len(texts)),
			zap.Int("actual", len(translatedSentences)))

		// 修正数量不匹配
		for len(translatedSentences) < len(texts) {
			translatedSentences = append(translatedSentences, "[翻译缺失]")
		}
		if len(translatedSentences) > len(texts) {
			translatedSentences = translatedSentences[:len(texts)]
		}
	}

	return translatedSentences, nil
}

func (t *LLMBatchTranslator) shouldTranslate(ctx context.Context, texts []string, runConfig TranslationRunConfig) (bool, string, error) {
	targetLang := strings.TrimSpace(strings.ToLower(runConfig.TargetLang))
	sourceLang := strings.TrimSpace(strings.ToLower(runConfig.SourceLang))
	if targetLang != "" && sourceLang != "" && sourceLang == targetLang {
		return false, sourceLang, nil
	}

	var sampleTexts []string
	for _, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		sampleTexts = append(sampleTexts, trimmed)
		if len(sampleTexts) >= 8 {
			break
		}
	}
	if len(sampleTexts) == 0 {
		return false, "", nil
	}

	llmClient, err := t.resolveLLMClient(ctx, runConfig)
	if err != nil {
		return false, "", err
	}

	systemPrompt := fmt.Sprintf(`你是字幕翻译前的语言判定器。请判断给定字幕样本是否需要翻译成%s。

规则：
1. 如果字幕主体已经是目标语言，needs_translation=false。
2. 如果字幕主体不是目标语言，needs_translation=true。
3. 混合语言时，以主体语言为准。
4. 只输出 JSON，不要输出解释文字。

输出格式：{"needs_translation":true,"detected_language":"en","reason":"主体为英文"}`,
		t.getLanguageName(runConfig.TargetLang))

	messages := []TranslationChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: strings.Join(sampleTexts, "\n"+sentenceBreak+"\n")},
	}

	response, err := t.chatWithModel(ctx, llmClient, messages, runConfig.ModelName)
	if err != nil {
		return false, "", fmt.Errorf("translation decision chat failed: %w", err)
	}

	var decision struct {
		NeedsTranslation bool   `json:"needs_translation"`
		DetectedLanguage string `json:"detected_language"`
		Reason           string `json:"reason"`
	}
	decoder := json.NewDecoder(strings.NewReader(extractJSON(response)))
	if err := decoder.Decode(&decision); err != nil {
		return false, "", fmt.Errorf("decode translation decision: %w", err)
	}

	t.logger.Info("Translation decision completed",
		zap.Bool("needs_translation", decision.NeedsTranslation),
		zap.String("detected_language", decision.DetectedLanguage),
		zap.String("reason", decision.Reason),
		zap.String("target_lang", runConfig.TargetLang))

	return decision.NeedsTranslation, decision.DetectedLanguage, nil
}

// getLanguageName 获取语言名称
func (t *LLMBatchTranslator) getLanguageName(code string) string {
	langMap := map[string]string{
		"en":      "英文",
		"zh-Hans": "中文",
		"zh-CN":   "中文",
		"zh":      "中文",
		"ja":      "日文",
		"ko":      "韩文",
		"es":      "西班牙文",
		"fr":      "法文",
		"de":      "德文",
		"ru":      "俄文",
		"ar":      "阿拉伯文",
		"pt":      "葡萄牙文",
		"it":      "意大利文",
		"auto":    "自动检测",
	}

	if name, ok := langMap[code]; ok {
		return name
	}
	return code
}
