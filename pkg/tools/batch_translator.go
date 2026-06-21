package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/zap"
)

// ── Batch Translation Constants ──────────────────────────────────────────────

const sentenceBreak = "###SENTENCE_BREAK###"

// ── BatchLLMClient interface (deprecated: prefer *llm.EinoChatClient directly) ─

// BatchLLMClient is the interface for LLM chat clients used by batch translation.
// Deprecated: New code should use *llm.EinoChatClient directly via BatchTranslator.
type BatchLLMClient interface {
	Chat(ctx context.Context, messages []TranslationChatMessage) (string, error)
	ChatWithOptions(ctx context.Context, messages []TranslationChatMessage, opts TranslationChatOptions) (string, error)
}

// TranslationChatMessage is a chat message for translation.
type TranslationChatMessage struct {
	Role    string
	Content string
}

// TranslationChatOptions controls per-request generation for translation.
type TranslationChatOptions struct {
	Model       string
	Temperature *float32
	MaxTokens   *int
}

// ── BatchTranslatorConfig ────────────────────────────────────────────────────

// BatchTranslatorConfig configures LLM-backed subtitle translation.
type BatchTranslatorConfig struct {
	SourceLang string // 源语言，默认 "en"
	TargetLang string // 目标语言，默认 "zh-Hans"
	BatchSize  int    // 每批翻译的句子数，默认 25
	MaxWorkers int    // 最大并发数，默认 3
	RetryCount int    // 重试次数，默认 2
	ContextSize int   // 上下文窗口大小（前后各取多少句），默认 2
}

// ── BatchTranslator ─────────────────────────────────────────────────────────

// BatchTranslator is the unified batch subtitle translator.
// It uses *llm.EinoChatClient directly — the caller provides a client configured
// for the desired provider (OpenAI, DeepSeek, Ollama, etc.).
//
// Usage:
//
//	client := llm.NewClientFromConfig(providerConfig, logger)
//	translator := NewBatchTranslator(client, config, logger)
//	result, err := translator.TranslateTexts(ctx, texts)
type BatchTranslator struct {
	client *llm.EinoChatClient
	config BatchTranslatorConfig
	logger *zap.Logger
}

// NewBatchTranslator creates a unified batch subtitle translator.
// If client is nil, translation will be skipped (translator is a no-op).
func NewBatchTranslator(client *llm.EinoChatClient, config BatchTranslatorConfig, logger *zap.Logger) *BatchTranslator {
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
	return &BatchTranslator{
		client: client,
		config: config,
		logger: logger,
	}
}
func (t *BatchTranslator) ModelName() string {
	if t.client == nil {
		return ""
	}
	return t.client.ModelName()
}


// TranslateTexts 批量翻译文本（并发分组处理）
func (t *BatchTranslator) TranslateTexts(ctx context.Context, texts []string) (*TranslationResult, error) {
	return t.TranslateTextsWithConfig(ctx, texts, TranslationRunConfig{})
}

// TranslateTextsWithConfig 带配置的批量翻译
// runConfig 可覆盖默认翻译参数（源语言、目标语言、模型等）
func (t *BatchTranslator) TranslateTextsWithConfig(ctx context.Context, texts []string, runConfig TranslationRunConfig) (*TranslationResult, error) {
	startTime := time.Now()

	if t.client == nil {
		return nil, fmt.Errorf("batch translator: LLM client not configured")
	}
	if len(texts) == 0 {
		return &TranslationResult{
			OriginalTexts:   texts,
			TranslatedTexts: []string{},
			Duration:        time.Since(startTime),
		}, nil
	}

	runtimeConfig := t.resolveRuntimeConfig(ctx, runConfig)

	t.logger.Info("Starting batch subtitle translation",
		zap.Int("total_texts", len(texts)),
		zap.String("model", runtimeConfig.ModelName),
		zap.String("source_lang", runtimeConfig.SourceLang),
		zap.String("target_lang", runtimeConfig.TargetLang),
		zap.Int("batch_size", t.config.BatchSize),
		zap.Int("max_workers", t.config.MaxWorkers))

	// 判断是否需要翻译
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
	if totalGroups == 0 {
		return &TranslationResult{
			OriginalTexts:   texts,
			TranslatedTexts: []string{},
			Duration:        time.Since(startTime),
		}, nil
	}

	// ── 并发分组翻译 ────────────────────────────────────────────────

	type batchTask struct {
		groupIndex  int
		texts       []string
		prevContext []string
		nextContext []string
	}

	type batchResult struct {
		groupIndex int
		texts      []string
		err        error
	}

	taskCh := make(chan batchTask, totalGroups)
	resultCh := make(chan batchResult, totalGroups)

	var wg sync.WaitGroup
	for i := 0; i < t.config.MaxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskCh {
				translated, err := t.translateGroupWithRetry(ctx, task.texts, task.prevContext, task.nextContext, runtimeConfig)
				resultCh <- batchResult{groupIndex: task.groupIndex, texts: translated, err: err}
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

			var prevContext, nextContext []string
			if i > 0 && t.config.ContextSize > 0 {
				prevStart := i - t.config.ContextSize
				if prevStart < 0 {
					prevStart = 0
				}
				prevContext = texts[prevStart:i]
			}
			if end < len(texts) && t.config.ContextSize > 0 {
				nextEnd := end + t.config.ContextSize
				if nextEnd > len(texts) {
					nextEnd = len(texts)
				}
				nextContext = texts[end:nextEnd]
			}

			taskCh <- batchTask{
				groupIndex:  i / t.config.BatchSize,
				texts:       texts[i:end],
				prevContext: prevContext,
				nextContext: nextContext,
			}
		}
		close(taskCh)
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	results := make(map[int][]string)
	var lastErr error

	for r := range resultCh {
		if r.err != nil {
			t.logger.Error("Group translation failed", zap.Int("group", r.groupIndex+1), zap.Error(r.err))
			lastErr = r.err
			continue
		}
		results[r.groupIndex] = r.texts
	}

	if lastErr != nil {
		return nil, fmt.Errorf("translation failed: %w", lastErr)
	}

	// 按顺序合并
	var allTranslated []string
	for i := 0; i < totalGroups; i++ {
		allTranslated = append(allTranslated, results[i]...)
	}

	return &TranslationResult{
		OriginalTexts:   texts,
		TranslatedTexts: allTranslated,
		Duration:        time.Since(startTime),
		DetectedLanguage: detectedLanguage,
	}, nil
}

// translateGroupWithRetry 带重试的分组翻译
func (t *BatchTranslator) translateGroupWithRetry(ctx context.Context, texts []string, prevContext, nextContext []string, runConfig TranslationRunConfig) ([]string, error) {
	var lastErr error
	for attempt := 0; attempt <= t.config.RetryCount; attempt++ {
		if attempt > 0 {
			t.logger.Warn("Retrying translation", zap.Int("attempt", attempt), zap.Int("max_retries", t.config.RetryCount))
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		result, err := t.translateGroup(ctx, texts, prevContext, nextContext, runConfig)
		if err == nil {
			return result, nil
		}
		lastErr = err
		t.logger.Warn("Translation attempt failed", zap.Int("attempt", attempt+1), zap.Error(err))
	}
	return nil, fmt.Errorf("translation failed after %d retries: %w", t.config.RetryCount, lastErr)
}

// translateGroup 翻译一组文本（单次调用 LLM）
func (t *BatchTranslator) translateGroup(ctx context.Context, texts []string, prevContext, nextContext []string, runConfig TranslationRunConfig) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// 构建包含上下文的完整文本
	var fullTexts []string
	targetStartIndex := 0
	if len(prevContext) > 0 {
		fullTexts = append(fullTexts, prevContext...)
		targetStartIndex = len(fullTexts)
	}
	fullTexts = append(fullTexts, texts...)
	targetEndIndex := len(fullTexts)
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
6. 分隔符：每句翻译用"%s"分隔

注意：只返回翻译的%s文本，不要添加序号、解释或其他内容。`,
		getLangName(runConfig.SourceLang),
		len(texts),
		contextInfo,
		getLangName(runConfig.TargetLang),
		len(texts),
		sentenceBreak,
		getLangName(runConfig.TargetLang))

	combinedText := strings.Join(fullTexts, "\n"+sentenceBreak+"\n")

	llmMessages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: combinedText},
	}

	modelName := runConfig.ModelName
	if modelName == "" {
		modelName = t.client.ModelName()
	}

	response, err := t.client.ChatWithOptions(ctx, llmMessages, llm.ChatOptions{Model: modelName})
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// 解析结果
	translatedSentences := strings.Split(response, sentenceBreak)
	for i := range translatedSentences {
		translatedSentences[i] = strings.TrimSpace(translatedSentences[i])
	}

	// 确保数量匹配
	for len(translatedSentences) < len(texts) {
		translatedSentences = append(translatedSentences, "[翻译缺失]")
	}
	if len(translatedSentences) > len(texts) {
		translatedSentences = translatedSentences[:len(texts)]
	}

	return translatedSentences, nil
}

// ── Translation decision ─────────────────────────────────────────────────────

// shouldTranslate 判断是否需要翻译
func (t *BatchTranslator) shouldTranslate(ctx context.Context, texts []string, runConfig TranslationRunConfig) (bool, string, error) {
	targetLang := strings.ToLower(runConfig.TargetLang)
	sourceLang := strings.ToLower(runConfig.SourceLang)
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

	systemPrompt := fmt.Sprintf(`你是字幕翻译前的语言判定器。请判断给定字幕样本是否需要翻译成%s。

规则：
1. 如果字幕主体已经是目标语言，needs_translation=false。
2. 如果字幕主体不是目标语言，needs_translation=true。
3. 混合语言时，以主体语言为准。
4. 只输出 JSON，不要输出解释文字。

输出格式：{"needs_translation":true,"detected_language":"en","reason":"主体为英文"}`,
		getLangName(runConfig.TargetLang))

	llmMessages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: strings.Join(sampleTexts, "\n"+sentenceBreak+"\n")},
	}

	modelName := runConfig.ModelName
	if modelName == "" {
		modelName = t.client.ModelName()
	}

	response, err := t.client.ChatWithOptions(ctx, llmMessages, llm.ChatOptions{Model: modelName})
	if err != nil {
		return false, "", fmt.Errorf("translation decision chat failed: %w", err)
	}

	var decision struct {
		NeedsTranslation bool   `json:"needs_translation"`
		DetectedLanguage string `json:"detected_language"`
		Reason           string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(extractJSON(response)), &decision); err != nil {
		return false, "", fmt.Errorf("decode translation decision: %w", err)
	}

	t.logger.Info("Translation decision completed",
		zap.Bool("needs_translation", decision.NeedsTranslation),
		zap.String("detected_language", decision.DetectedLanguage),
		zap.String("reason", decision.Reason))

	return decision.NeedsTranslation, decision.DetectedLanguage, nil
}

// ── Runtime config resolution ────────────────────────────────────────────────

func (t *BatchTranslator) resolveRuntimeConfig(ctx context.Context, runConfig TranslationRunConfig) TranslationRunConfig {
	if runConfig.SourceLang == "" {
		runConfig.SourceLang = t.config.SourceLang
	}
	if runConfig.TargetLang == "" {
		runConfig.TargetLang = t.config.TargetLang
	}
	if runConfig.ModelName == "" || isDefaultModelName(runConfig.ModelName) {
		runConfig.ModelName = t.client.ModelName()
	}
	return runConfig
}

func isDefaultModelName(name string) bool {
	return name == "gpt-4o-mini" || name == "gpt-3.5-turbo"
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func getLangName(code string) string {
	m := map[string]string{
		"en": "英文", "zh-Hans": "中文", "zh-CN": "中文", "zh": "中文",
		"ja": "日文", "ko": "韩文", "es": "西班牙文", "fr": "法文",
		"de": "德文", "ru": "俄文", "ar": "阿拉伯文", "pt": "葡萄牙文",
		"it": "意大利文", "auto": "自动检测",
	}
	if name, ok := m[code]; ok {
		return name
	}
	return code
}

func extractJSON(input string) string {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "}")
	if start >= 0 && end > start {
		return input[start : end+1]
	}
	return input
}
