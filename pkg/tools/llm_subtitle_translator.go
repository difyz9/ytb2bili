package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// ─────────────────────────────────────────────────────────────────────────────
// SRT 数据结构
// ─────────────────────────────────────────────────────────────────────────────

// SRTEntry 代表 SRT 字幕文件中的单个条目
type SRTEntry struct {
	Index    int
	TimeCode string
	Text     string
}

// ─────────────────────────────────────────────────────────────────────────────
// LLM subtitle translator
// ─────────────────────────────────────────────────────────────────────────────

// LLMSubtitleTranslator translates SRT subtitle files through LLM.
// 底层使用 LLMBatchTranslator 进行并发分批翻译，
// 同时实现 Translator 接口以兼容单文本翻译场景。
type LLMSubtitleTranslator struct {
	batchTranslator *LLMBatchTranslator
	logger          *zap.Logger
}

// NewLLMSubtitleTranslator 创建字幕翻译器。
// config 直接复用 LLMBatchTranslatorConfig，无需额外封装。
func NewLLMSubtitleTranslator(config LLMBatchTranslatorConfig, logger *zap.Logger) *LLMSubtitleTranslator {
	return &LLMSubtitleTranslator{
		batchTranslator: NewLLMBatchTranslator(config, logger),
		logger:          logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Translator 接口实现（单文本翻译，兼容已有调用方）
// ─────────────────────────────────────────────────────────────────────────────

// TranslateText 实现 Translator 接口，对单段文本调用 LLM 字幕翻译。
// from / to 参数当前通过 LLMBatchTranslatorConfig 中的 SourceLang / TargetLang 控制；
// 若需要动态语言对，请直接使用 TranslateSRTFile 并构造对应 config。
func (t *LLMSubtitleTranslator) TranslateText(text, from, to string) (string, error) {
	result, err := t.batchTranslator.TranslateTexts(context.Background(), []string{text})
	if err != nil {
		return "", fmt.Errorf("LLM 字幕翻译失败: %w", err)
	}
	if len(result.TranslatedTexts) == 0 {
		return "", fmt.Errorf("LLM 未返回翻译结果")
	}
	return result.TranslatedTexts[0], nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SRT 文件翻译（主要使用场景）
// ─────────────────────────────────────────────────────────────────────────────

// TranslateSRTFile 读取 inputPath 的英文（或任意源语言）SRT 文件，
// 通过 LLM 字幕翻译后将结果写入 outputPath。
func (t *LLMSubtitleTranslator) TranslateSRTFile(ctx context.Context, inputPath, outputPath string) error {
	t.logger.Info("开始翻译字幕文件",
		zap.String("input", inputPath),
		zap.String("output", outputPath))

	// 1. 读取源文件
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取字幕文件失败 (%s): %w", inputPath, err)
	}

	// 2. 解析 SRT
	entries, err := ParseSRTContent(string(raw))
	if err != nil {
		return fmt.Errorf("解析 SRT 文件失败: %w", err)
	}
	if len(entries) == 0 {
		t.logger.Warn("字幕文件为空，跳过翻译", zap.String("input", inputPath))
		return nil
	}

	t.logger.Info("解析字幕完成", zap.Int("entries", len(entries)))

	// 3. 提取纯文本
	texts := make([]string, len(entries))
	for i, e := range entries {
		texts[i] = e.Text
	}

	// 4. 批量翻译
	result, err := t.batchTranslator.TranslateTexts(ctx, texts)
	if err != nil {
		return fmt.Errorf("LLM 批量字幕翻译失败: %w", err)
	}

	t.logger.Info("翻译完成",
		zap.Int("original", len(texts)),
		zap.Int("translated", len(result.TranslatedTexts)),
		zap.Duration("duration", result.Duration))

	// 5. 生成译文 SRT
	content := GenerateSRTContent(entries, result.TranslatedTexts)

	// 6. 写入输出文件
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("保存翻译字幕失败 (%s): %w", outputPath, err)
	}

	t.logger.Info("字幕翻译完成", zap.String("output", outputPath))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SRT 解析 / 生成工具函数
// ─────────────────────────────────────────────────────────────────────────────

// ParseSRTContent 将 SRT 文本解析为 []SRTEntry。
// 支持 CRLF 和 LF 换行，容忍多余空行。
func ParseSRTContent(content string) ([]SRTEntry, error) {
	// 统一换行符
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	var entries []SRTEntry
	var current SRTEntry
	var textLines []string
	stage := 0 // 0=等待序号  1=等待时间码  2=读取文本行

	flush := func() {
		if stage == 2 && len(textLines) > 0 {
			current.Text = strings.Join(textLines, "\n")
			entries = append(entries, current)
		}
		current = SRTEntry{}
		textLines = nil
		stage = 0
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			flush()
			continue
		}

		switch stage {
		case 0: // 读取序号
			var idx int
			if _, err := fmt.Sscanf(line, "%d", &idx); err == nil {
				current.Index = idx
				stage = 1
			}
			// 无法解析为序号则忽略（容错）

		case 1: // 读取时间码
			if strings.Contains(line, "-->") {
				current.TimeCode = line
				stage = 2
			}

		case 2: // 读取文本行（可能多行）
			textLines = append(textLines, line)
		}
	}
	// 处理文件末尾没有空行的情况
	flush()

	return entries, nil
}

// SRTEntriesToTranscript 将已解析的 SRT 条目转换为 TranscriptResult。
// 常用于从磁盘 SRT 文件重建内存中的转录结果（例如步骤重试场景）。
func SRTEntriesToTranscript(entries []SRTEntry, srtPath string) *TranscriptResult {
	segments := make([]TranscriptSegment, 0, len(entries))
	var fullText strings.Builder
	for _, e := range entries {
		start, end := ParseSRTTimeCode(e.TimeCode)
		segments = append(segments, TranscriptSegment{
			Start: start,
			End:   end,
			Text:  e.Text,
		})
		if fullText.Len() > 0 {
			fullText.WriteString(" ")
		}
		fullText.WriteString(e.Text)
	}
	return &TranscriptResult{
		FullText: fullText.String(),
		Segments: segments,
		SRTPath:  srtPath,
	}
}

// ParseSRTTimeCode 解析 "HH:MM:SS,mmm --> HH:MM:SS,mmm" 时间码，返回 (start, end) 秒数。
func ParseSRTTimeCode(tc string) (float64, float64) {
	parts := strings.SplitN(tc, " --> ", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	return ParseSRTTime(strings.TrimSpace(parts[0])), ParseSRTTime(strings.TrimSpace(parts[1]))
}

// ParseSRTTime 将 "HH:MM:SS,mmm" 格式解析为秒数（float64）。
func ParseSRTTime(t string) float64 {
	t = strings.ReplaceAll(t, ",", ".")
	parts := strings.Split(t, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	s, _ := strconv.ParseFloat(parts[2], 64)
	return h*3600 + m*60 + s
}

// GenerateSRTContent 根据原始条目和译文列表生成 SRT 字符串。
// 若 translatedTexts 数量不足，对应条目保留原文。
func GenerateSRTContent(entries []SRTEntry, translatedTexts []string) string {
	var sb strings.Builder
	for i, entry := range entries {
		fmt.Fprintf(&sb, "%d\n", entry.Index)
		fmt.Fprintf(&sb, "%s\n", entry.TimeCode)
		if i < len(translatedTexts) && translatedTexts[i] != "" {
			fmt.Fprintf(&sb, "%s\n\n", translatedTexts[i])
		} else {
			fmt.Fprintf(&sb, "%s\n\n", entry.Text)
		}
	}
	return sb.String()
}
