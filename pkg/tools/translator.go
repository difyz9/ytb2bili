package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/zolagz/ytb2bili/internal/config"
)

// TranslatorConfig 配置
// 推荐通过 config/env 传递 key/region
type TranslatorConfig struct {
	TbSubscriptionKey string
	Region            string
	Endpoint          string // 可选，默认微软官方
}

// Translator 接口
type Translator interface {
	TranslateText(text, from, to string) (string, error)
}

// MicrosoftTranslator 实现

type MicrosoftTranslator struct {
	Config TranslatorConfig
}

func NewMicrosoftTranslator(cfg TranslatorConfig) *MicrosoftTranslator {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://api.cognitive.microsofttranslator.com"
	}
	return &MicrosoftTranslator{Config: cfg}
}

// 推荐统一从 config.AppConfig 加载 key/region
func NewMicrosoftTranslatorFromAppConfig(appCfg *config.AppConfig) *MicrosoftTranslator {
	return &MicrosoftTranslator{Config: TranslatorConfig{
		TbSubscriptionKey: appCfg.Workflow.SpeechKey,
		Region:            appCfg.Workflow.SpeechRegion,
		Endpoint:          "https://api.cognitive.microsofttranslator.com",
	}}
}

// TranslateText 调用微软翻译API
func (t *MicrosoftTranslator) TranslateText(text, from, to string) (string, error) {
	uri := fmt.Sprintf("%s/translate?api-version=3.0", t.Config.Endpoint)
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	q := u.Query()
	if from != "" {
		q.Add("from", from)
	}
	q.Add("to", to)
	u.RawQuery = q.Encode()

	body := []map[string]string{{"Text": text}}
	b, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Ocp-Apim-TbSubscription-Key", t.Config.TbSubscriptionKey)
	req.Header.Add("Ocp-Apim-TbSubscription-Region", t.Config.Region)
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("translation request failed: %w", err)
	}
	defer res.Body.Close()

	var result []struct {
		Translations []struct {
			Text string `json:"text"`
			To   string `json:"to"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	if len(result) == 0 || len(result[0].Translations) == 0 {
		return "", fmt.Errorf("no translation result returned")
	}
	return result[0].Translations[0].Text, nil
}
