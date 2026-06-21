package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
)

const extractAudioToolName = "extract_audio"

// AudioFormat is the output format for audio extraction.
type AudioFormat string

const (
	FormatMP3 AudioFormat = "mp3"
	FormatWAV AudioFormat = "wav"
	FormatAAC AudioFormat = "aac"
	FormatOGG AudioFormat = "ogg"
)

// ExtractAudioTool extracts audio from a local video file using FFmpeg.
// Implements eino's tool.InvokableTool (Info + InvokableRun).
// Also exposes Call() / ExtractWAV() for direct workflow use.
type ExtractAudioTool struct {
	ffmpegPath string
	logger     *zap.Logger
}

// NewExtractAudioTool creates an ExtractAudioTool.
func NewExtractAudioTool(ffmpegPath string, logger *zap.Logger) (*ExtractAudioTool, error) {
	if ffmpegPath == "" {
		path, err := exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found in PATH, please install it or specify path")
		}
		ffmpegPath = path
	}
	if _, err := os.Stat(ffmpegPath); err != nil {
		return nil, fmt.Errorf("ffmpeg not found at %s: %w", ffmpegPath, err)
	}
	return &ExtractAudioTool{ffmpegPath: ffmpegPath, logger: logger}, nil
}

// ── eino tool.InvokableTool ───────────────────────────────────────────────────

// Info describes the tool to the LLM via eino's schema system.
func (t *ExtractAudioTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "extract_audio",
		Desc: "Extract the audio track from a local video file using FFmpeg. Returns the absolute path of the audio file.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_path": {
				Type:     schema.String,
				Desc:     "Absolute local path of the video file (output of download_video)",
				Required: true,
			},
			"format": {
				Type: schema.String,
				Desc: "Output audio format: mp3, wav, aac, ogg (default: mp3)",
			},
		}),
	}, nil
}

// InvokableRun is called by the eino agent when the LLM issues a tool call.
func (t *ExtractAudioTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	type audioArgs struct {
		VideoPath string `json:"video_path"`
		Format    string `json:"format,omitempty"`
	}
	args, err := UnmarshalArgs[audioArgs](extractAudioToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	if err := RequireString(extractAudioToolName, "video_path", args.VideoPath); err != nil {
		return "", err
	}
	format := AudioFormat(args.Format)
	if format == "" {
		format = FormatMP3
	}
	return t.extract(ctx, args.VideoPath, format)
}

// ── Direct workflow use ───────────────────────────────────────────────────────

// Call accepts a plain video path string (for direct workflow use).
func (t *ExtractAudioTool) Call(ctx context.Context, input string) (string, error) {
	videoPath := strings.TrimSpace(input)
	if videoPath == "" {
		return "", fmt.Errorf("video path cannot be empty")
	}
	return t.extract(ctx, videoPath, FormatMP3)
}

// ExtractWAV extracts WAV audio (16 kHz mono) suitable for speech recognition.
func (t *ExtractAudioTool) ExtractWAV(ctx context.Context, videoPath string) (string, error) {
	if videoPath == "" {
		return "", fmt.Errorf("video path cannot be empty")
	}
	audioPath := t.outputPath(videoPath, FormatWAV)
	if _, err := os.Stat(audioPath); err == nil {
		return audioPath, nil
	}
	args := []string{"-i", videoPath, "-vn", "-acodec", "pcm_s16le", "-ar", "16000", "-ac", "1", "-y", audioPath}
	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("WAV extraction failed: %w\noutput: %s", err, out)
	}
	return audioPath, nil
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (t *ExtractAudioTool) extract(ctx context.Context, videoPath string, format AudioFormat) (string, error) {
	if _, err := os.Stat(videoPath); err != nil {
		return "", fmt.Errorf("video file not found: %w", err)
	}

	audioPath := t.outputPath(videoPath, format)
	if _, err := os.Stat(audioPath); err == nil {
		t.logger.Info("audio already exists, skipping extraction", zap.String("path", audioPath))
		return audioPath, nil
	}

	t.logger.Info("extracting audio", zap.String("video", videoPath), zap.String("format", string(format)))

	var ffmpegArgs []string
	switch format {
	case FormatWAV:
		ffmpegArgs = []string{"-i", videoPath, "-vn", "-acodec", "pcm_s16le", "-ar", "44100", "-ac", "2", "-y", audioPath}
	case FormatAAC:
		ffmpegArgs = []string{"-i", videoPath, "-vn", "-acodec", "aac", "-ab", "192k", "-y", audioPath}
	case FormatOGG:
		ffmpegArgs = []string{"-i", videoPath, "-vn", "-acodec", "libvorbis", "-ab", "192k", "-y", audioPath}
	default: // mp3
		ffmpegArgs = []string{"-i", videoPath, "-vn", "-acodec", "libmp3lame", "-ab", "192k", "-ar", "44100", "-ac", "2", "-y", audioPath}
	}

	cmd := exec.CommandContext(ctx, t.ffmpegPath, ffmpegArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("audio extraction failed: %w\noutput: %s", err, out)
	}

	info, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("output file not found after extraction: %w", err)
	}
	t.logger.Info("audio extracted", zap.String("path", audioPath), zap.Int64("size_mb", info.Size()/(1024*1024)))
	return audioPath, nil
}

func (t *ExtractAudioTool) outputPath(videoPath string, format AudioFormat) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + "." + string(format)
}

// NewExtractAudioToolFromAppConfig 从 AppConfig 创建 ExtractAudioTool。
func NewExtractAudioToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) (*ExtractAudioTool, error) {
	return NewExtractAudioTool(appCfg.Workflow.FFmpegPath, logger)
}
