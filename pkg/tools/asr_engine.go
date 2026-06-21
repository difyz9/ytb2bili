package tools

import (
	"context"
	"fmt"
)

// ── ASR Engine Interface ─────────────────────────────────────────────────────

// ASREngine 是语音识别/音频转文字（ASR）供应商的统一接口。
// 每个 provider（bcut, whisper）实现此接口。
type ASREngine interface {
	// Name 返回供应商名称
	Name() string

	// Transcribe 将音频文件转写为字幕文本，返回带时间戳的片段
	Transcribe(ctx context.Context, audioPath string) (*TranscriptResult, error)

	// Languages 返回该引擎支持的语言列表
	Languages() []string
}

// ── ASR Engine Registry ─────────────────────────────────────────────────────

type asrEngineRegistry struct {
	engines map[string]ASREngine
}

func newASREngineRegistry() *asrEngineRegistry {
	return &asrEngineRegistry{engines: make(map[string]ASREngine)}
}

func (r *asrEngineRegistry) Register(engine ASREngine) {
	r.engines[engine.Name()] = engine
}

func (r *asrEngineRegistry) Get(name string) (ASREngine, error) {
	engine, ok := r.engines[name]
	if !ok {
		return nil, fmt.Errorf("unsupported ASR provider: %q (supported: bcut, whisper)", name)
	}
	return engine, nil
}

func (r *asrEngineRegistry) All() []ASREngine {
	var result []ASREngine
	for _, e := range r.engines {
		result = append(result, e)
	}
	return result
}
