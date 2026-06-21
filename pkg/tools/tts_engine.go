package tools

import (
	"context"
	"fmt"
)

// ── TTS Engine Interface ─────────────────────────────────────────────────────

// TTSEngine 是 TTS 供应商的统一接口。
// 每个 provider（azure / edge / openai / tencent）实现此接口。
type TTSEngine interface {
	// Synthesize 合成语音，返回音频字节。
	Synthesize(ctx context.Context, text, voice string, rate, volume, pitch float64) ([]byte, error)

	// Name 返回供应商名称（用于日志和配置识别）
	Name() string

	// Voices 返回该供应商支持的音色列表
	Voices(ctx context.Context, locale string) ([]VoiceInfo, error)
}

// ── Engine Registry ──────────────────────────────────────────────────────────

// ttsEngineRegistry 全局引擎注册表
type ttsEngineRegistry struct {
	engines map[string]TTSEngine
}

func newTTSEngineRegistry() *ttsEngineRegistry {
	return &ttsEngineRegistry{engines: make(map[string]TTSEngine)}
}

func (r *ttsEngineRegistry) Register(engine TTSEngine) {
	r.engines[engine.Name()] = engine
}

func (r *ttsEngineRegistry) Get(name string) (TTSEngine, error) {
	engine, ok := r.engines[name]
	if !ok {
		return nil, fmt.Errorf("unsupported TTS provider: %q (supported: azure, edge, openai, tencent)", name)
	}
	return engine, nil
}

func (r *ttsEngineRegistry) All() []TTSEngine {
	var result []TTSEngine
	for _, e := range r.engines {
		result = append(result, e)
	}
	return result
}
