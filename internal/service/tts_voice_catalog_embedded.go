package service

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"go.uber.org/zap"
)

//go:embed ttsdata/*.json
var embeddedTTSVoiceSnapshots embed.FS

type voiceSnapshotEnvelope struct {
	Data struct {
		Provider string          `json:"provider"`
		Voices   json.RawMessage `json:"voices"`
	} `json:"data"`
}

type EmbeddedTTSVoiceCatalog struct {
	voices  []TTSVoiceRecord
	cascade *TTSVoiceCascade
}

func NewEmbeddedTTSVoiceCatalog(logger *zap.Logger) (TTSVoiceCatalog, error) {
	files := []string{
		"ttsdata/azure_voices.json",
		"ttsdata/tencent_voices.json",
	}

	var merged []TTSVoiceRecord
	for _, name := range files {
		b, err := fs.ReadFile(embeddedTTSVoiceSnapshots, name)
		if err != nil {
			return nil, fmt.Errorf("read embedded voice snapshot %s: %w", name, err)
		}

		var env voiceSnapshotEnvelope
		if err := json.Unmarshal(b, &env); err != nil {
			return nil, fmt.Errorf("decode embedded voice snapshot %s: %w", name, err)
		}

		provider := strings.ToLower(strings.TrimSpace(env.Data.Provider))
		var providerEnum TTSVoiceProvider
		switch provider {
		case string(TTSVoiceProviderAzure):
			providerEnum = TTSVoiceProviderAzure
		case string(TTSVoiceProviderTencent):
			providerEnum = TTSVoiceProviderTencent
		default:
			if logger != nil {
				logger.Warn("unknown TTS provider in snapshot", zap.String("file", name), zap.String("provider", provider))
			}
			continue
		}

		var voices []TTSVoiceRecord
		if err := json.Unmarshal(env.Data.Voices, &voices); err != nil {
			return nil, fmt.Errorf("decode embedded voices list %s: %w", name, err)
		}
		for i := range voices {
			voices[i].Provider = providerEnum
		}
		merged = append(merged, voices...)
	}

	seen := make(map[string]struct{}, len(merged))
	deduped := make([]TTSVoiceRecord, 0, len(merged))
	for _, v := range merged {
		shortName := strings.TrimSpace(v.ShortName)
		if shortName == "" {
			continue
		}
		key := string(v.Provider) + ":" + shortName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, v)
	}

	sort.SliceStable(deduped, func(i, j int) bool {
		a, b := deduped[i], deduped[j]
		if a.Provider != b.Provider {
			return string(a.Provider) < string(b.Provider)
		}
		if a.Locale != b.Locale {
			return a.Locale < b.Locale
		}
		nameA := strings.ToLower(strings.TrimSpace(firstNonEmpty(a.LocalName, a.DisplayName, a.ShortName)))
		nameB := strings.ToLower(strings.TrimSpace(firstNonEmpty(b.LocalName, b.DisplayName, b.ShortName)))
		if nameA != nameB {
			return nameA < nameB
		}
		return a.ShortName < b.ShortName
	})

	cascade := buildTTSVoiceCascade(deduped)
	return &EmbeddedTTSVoiceCatalog{voices: deduped, cascade: cascade}, nil
}

func (c *EmbeddedTTSVoiceCatalog) List(ctx context.Context) ([]TTSVoiceRecord, error) {
	_ = ctx
	out := make([]TTSVoiceRecord, len(c.voices))
	copy(out, c.voices)
	return out, nil
}

func (c *EmbeddedTTSVoiceCatalog) Cascade(ctx context.Context) (*TTSVoiceCascade, error) {
	_ = ctx
	return c.cascade, nil
}

func buildTTSVoiceCascade(voices []TTSVoiceRecord) *TTSVoiceCascade {
	providers := make(map[TTSVoiceProvider]map[string]*TTSVoiceLocaleGroup)
	providerOrder := make([]TTSVoiceProvider, 0, 4)

	for _, v := range voices {
		if v.Provider == "" || strings.TrimSpace(v.Locale) == "" {
			continue
		}
		m, ok := providers[v.Provider]
		if !ok {
			m = make(map[string]*TTSVoiceLocaleGroup)
			providers[v.Provider] = m
			providerOrder = append(providerOrder, v.Provider)
		}
		lg, ok := m[v.Locale]
		if !ok {
			lg = &TTSVoiceLocaleGroup{Locale: v.Locale, LocaleName: v.LocaleName}
			m[v.Locale] = lg
		}
		lg.Voices = append(lg.Voices, v)
	}

	sort.SliceStable(providerOrder, func(i, j int) bool {
		return string(providerOrder[i]) < string(providerOrder[j])
	})

	out := &TTSVoiceCascade{Providers: make([]TTSVoiceProviderGroup, 0, len(providerOrder))}
	for _, provider := range providerOrder {
		localesMap := providers[provider]
		locales := make([]TTSVoiceLocaleGroup, 0, len(localesMap))
		for _, group := range localesMap {
			locales = append(locales, *group)
		}

		sort.SliceStable(locales, func(i, j int) bool {
			return locales[i].Locale < locales[j].Locale
		})

		out.Providers = append(out.Providers, TTSVoiceProviderGroup{Provider: provider, Locales: locales})
	}

	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// VoiceEntry is a simple voice record exported for use by the TTS client.
type VoiceEntry struct {
	ShortName   string
	DisplayName string
	Locale      string
}

// LoadEmbeddedVoiceCatalog loads voice entries from the embedded catalog for a given provider.
func LoadEmbeddedVoiceCatalog(provider string) ([]VoiceEntry, error) {
	filename := fmt.Sprintf("ttsdata/%s_voices.json", strings.ToLower(strings.TrimSpace(provider)))
	data, err := embeddedTTSVoiceSnapshots.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("voice catalog not found for provider %s: %w", provider, err)
	}

	var envelope voiceSnapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse voice catalog for %s: %w", provider, err)
	}

	type rawVoice struct {
		ShortName   string `json:"ShortName"`
		DisplayName string `json:"DisplayName"`
		Locale      string `json:"Locale"`
	}

	var rawVoices []rawVoice
	if err := json.Unmarshal(envelope.Data.Voices, &rawVoices); err != nil {
		return nil, fmt.Errorf("parse voices for %s: %w", provider, err)
	}

	result := make([]VoiceEntry, 0, len(rawVoices))
	for _, v := range rawVoices {
		result = append(result, VoiceEntry{
			ShortName:   v.ShortName,
			DisplayName: v.DisplayName,
			Locale:      v.Locale,
		})
	}
	return result, nil
}
