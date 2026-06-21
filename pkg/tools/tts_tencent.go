package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── Tencent Cloud TTS Engine ──────────────────────────────────────────────────

type TencentTTSEngine struct {
	secretID  string
	secretKey string
	region    string
	client    *http.Client
}

func NewTencentTTSEngine(secretID, secretKey, region string) *TencentTTSEngine {
	if region == "" {
		region = "ap-guangzhou"
	}
	return &TencentTTSEngine{
		secretID:  secretID,
		secretKey: secretKey,
		region:    region,
		client:    &http.Client{},
	}
}

func (e *TencentTTSEngine) Name() string {
	return "tencent"
}

func (e *TencentTTSEngine) Voices(ctx context.Context, locale string) ([]VoiceInfo, error) {
	return defaultVoicesForProvider("tencent", locale), nil
}

func (e *TencentTTSEngine) Synthesize(ctx context.Context, text, voice string, rate, volume, pitch float64) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("tencent-tts: text is empty")
	}

	voiceType := resolveTencentVoiceType(voice)

	body := map[string]interface{}{
		"Text":           text,
		"SessionId":      fmt.Sprintf("ytb2bili-%d", time.Now().UnixNano()),
		"VoiceType":      voiceType,
		"PrimaryLanguage": 1,
		"Codec":          "mp3",
		"Speed":          rate,
		"Volume":         volume / 100.0,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	host := "tts.tencentcloudapi.com"
	service := "tts"
	action := "TextToVoice"
	version := "2019-08-23"
	algorithm := "TC3-HMAC-SHA256"
	timestamp := time.Now().Unix()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://"+host, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:%s\n", host)
	signedHeaders := "content-type;host"
	payloadHash := sha256Hex(bodyBytes)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders, payloadHash)

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)))

	secretDate := hmacSHA256([]byte("TC3"+e.secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, e.secretID, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", e.region)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tencent-tts request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Response struct {
			Audio   string `json:"Audio"`
			Error   struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Tencent TTS response: %w", err)
	}
	if result.Response.Error.Code != "" {
		return nil, fmt.Errorf("tencent-tts error: %s - %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	audio, err := base64.StdEncoding.DecodeString(result.Response.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Tencent TTS audio: %w", err)
	}

	return audio, nil
}
