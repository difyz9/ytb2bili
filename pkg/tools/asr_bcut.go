package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// ── Bcut ASR Engine ──────────────────────────────────────────────────────────
// Uses Bilibili BCut API for speech-to-text (requires network).

type BcutASREngine struct {
	inner *BcutTranscriberTool
}

func NewBcutASREngine(logger *zap.Logger) *BcutASREngine {
	return &BcutASREngine{
		inner: NewBcutTranscriberTool(logger),
	}
}

func (e *BcutASREngine) Name() string {
	return "bcut"
}

func (e *BcutASREngine) Languages() []string {
	return []string{"zh", "en", "ja", "ko"}
}

func (e *BcutASREngine) Transcribe(ctx context.Context, audioPath string) (*TranscriptResult, error) {
	input := fmt.Sprintf(`{"audio_path":%q}`, audioPath)
	result, err := e.inner.Call(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bcut asr failed: %w", err)
	}

	var transcript TranscriptResult
	if err := json.Unmarshal([]byte(result), &transcript); err != nil {
		return nil, fmt.Errorf("bcut asr parse result: %w", err)
	}

	return &transcript, nil
}

var _ ASREngine = (*BcutASREngine)(nil)
