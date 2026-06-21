package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── Whisper ASR Engine ───────────────────────────────────────────────────────
// Supports two modes:
//   1. Local mode: calls whisper-cpp CLI
//   2. API mode: calls OpenAI-compatible Whisper API (WhisperX, OpenAI, etc.)
//
// Configuration via ASRConfig:
//   provider = "whisper"
//   model_path = "/path/to/models"  (local mode)
//   api_url = "http://localhost:9000/v1" (API mode, optional)

type WhisperASREngine struct {
	mode     string // "local" or "api"
	modelDir string
	apiURL   string
	apiKey   string
	client   *http.Client
}

type WhisperConfig struct {
	ModelDir string // local whisper.cpp models directory
	APIURL   string // OpenAI-compatible API endpoint (e.g. WhisperX)
	APIKey   string
}

func NewWhisperASREngine(cfg WhisperConfig) *WhisperASREngine {
	mode := "local"
	if cfg.APIURL != "" {
		mode = "api"
	}
	return &WhisperASREngine{
		mode:     mode,
		modelDir: cfg.ModelDir,
		apiURL:   strings.TrimRight(cfg.APIURL, "/"),
		apiKey:   cfg.APIKey,
		client:   &http.Client{Timeout: 10 * time.Minute},
	}
}

func (e *WhisperASREngine) Name() string {
	return "whisper"
}

func (e *WhisperASREngine) Languages() []string {
	return []string{"zh", "en", "ja", "ko", "fr", "de", "es", "ru", "ar"}
}

func (e *WhisperASREngine) Transcribe(ctx context.Context, audioPath string) (*TranscriptResult, error) {
	if _, err := os.Stat(audioPath); err != nil {
		return nil, fmt.Errorf("whisper asr: audio file not found: %w", err)
	}

	switch e.mode {
	case "local":
		return e.transcribeLocal(ctx, audioPath)
	case "api":
		return e.transcribeAPI(ctx, audioPath)
	default:
		return nil, fmt.Errorf("whisper asr: unknown mode %q", e.mode)
	}
}

// transcribeLocal uses whisper-cpp CLI: ./whisper-cli --file audio.mp3 --model base --output-json
func (e *WhisperASREngine) transcribeLocal(ctx context.Context, audioPath string) (*TranscriptResult, error) {
	// Check for whisper-cli in PATH or model directory
	whisperBin := "whisper-cli"
	if e.modelDir != "" {
		whisperBin = filepath.Join(e.modelDir, "whisper-cli")
	}

	// Output JSON to stdout
	args := []string{
		"--file", audioPath,
		"--model", "base",
		"--output-json",
		"--print-progress", "false",
	}

	cmd := exec.CommandContext(ctx, whisperBin, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("whisper-cli execution failed: %w\n%s", err, string(output))
	}

	return e.parseWhisperOutput(output)
}

// transcribeAPI uses OpenAI-compatible Whisper API:
// POST /v1/audio/transcriptions with multipart form
func (e *WhisperASREngine) transcribeAPI(ctx context.Context, audioPath string) (*TranscriptResult, error) {
	endpoint := e.apiURL + "/audio/transcriptions"

	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("whisper api: open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add model field
	if err := w.WriteField("model", "whisper-1"); err != nil {
		return nil, fmt.Errorf("whisper api: write model field: %w", err)
	}

	// Add response_format
	if err := w.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("whisper api: write format: %w", err)
	}

	// Add audio file
	part, err := w.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("whisper api: create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("whisper api: copy file: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("whisper api: create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper api: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("whisper api: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper api: status %d: %s", resp.StatusCode, string(body))
	}

	return e.parseWhisperOutput(body)
}

type whisperSegment struct {
	ID               int     `json:"id"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	AvgLogprob       float64 `json:"avg_logprob"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	CompressionRatio float64 `json:"compression_ratio"`
}

type whisperResponse struct {
	Text     string           `json:"text"`
	Language string           `json:"language"`
	Segments []whisperSegment `json:"segments"`
}

func (e *WhisperASREngine) parseWhisperOutput(data []byte) (*TranscriptResult, error) {
	var resp whisperResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("whisper asr: parse response: %w", err)
	}

	segments := make([]TranscriptSegment, 0, len(resp.Segments))
	for _, s := range resp.Segments {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		segments = append(segments, TranscriptSegment{
			Start: s.Start,
			End:   s.End,
			Text:  strings.TrimSpace(s.Text),
		})
	}

	return &TranscriptResult{
		Language: resp.Language,
		FullText: strings.TrimSpace(resp.Text),
		Segments: segments,
	}, nil
}
