package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/utils"
	"go.uber.org/zap"
)

const transcodeVideoToolName = "transcode_video"

const (
	transcodeResolutionSource = "source"
	transcodeFormatMP4        = "mp4"
	transcodeFormatMKV        = "mkv"
	transcodeFormatMOV        = "mov"
	transcodeFormatWEBM       = "webm"
	transcodeCodecH264        = "h264"
	transcodeCodecH265        = "h265"
	transcodeCodecVP9         = "vp9"
	transcodeCodecCopy        = "copy"
	transcodeDefaultPreset    = "medium"
	transcodeDefaultCRF       = 23
	transcodeDefaultBitrate   = "192k"
)

var (
	supportedTranscodeFormats = map[string]struct{}{
		transcodeFormatMP4:  {},
		transcodeFormatMKV:  {},
		transcodeFormatMOV:  {},
		transcodeFormatWEBM: {},
	}
	supportedTranscodeCodecs = map[string]struct{}{
		transcodeCodecH264: {},
		transcodeCodecH265: {},
		transcodeCodecVP9:  {},
		transcodeCodecCopy: {},
	}
	supportedTranscodePresets = map[string]struct{}{
		"ultrafast": {}, "superfast": {}, "veryfast": {}, "faster": {}, "fast": {},
		transcodeDefaultPreset: {}, "slow": {}, "slower": {}, "veryslow": {},
	}
	transcodeResolutionHeights = map[string]int{
		"480p":  480,
		"720p":  720,
		"1080p": 1080,
		"1440p": 1440,
		"2160p": 2160,
	}
)

// TranscodeVideoTool converts a local video into another codec, format or resolution using FFmpeg.
type TranscodeVideoTool struct {
	ffmpegPath string
	logger     *zap.Logger
}

// NewTranscodeVideoTool creates a TranscodeVideoTool.
func NewTranscodeVideoTool(ffmpegPath string, logger *zap.Logger) (*TranscodeVideoTool, error) {
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
	return &TranscodeVideoTool{ffmpegPath: ffmpegPath, logger: logger}, nil
}

// Info describes the tool to the LLM.
func (t *TranscodeVideoTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "transcode_video",
		Desc: "Transcode a local video file with FFmpeg. Supports format conversion, codec conversion, resolution downscaling and FPS changes. Returns the absolute path of the transcoded video.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_path": {
				Type:     schema.String,
				Desc:     "Absolute local path of the source video file",
				Required: true,
			},
			"output_format": {
				Type: schema.String,
				Desc: "Target container format: mp4, mkv, mov, webm (default: mp4)",
			},
			"resolution": {
				Type: schema.String,
				Desc: "Target max height: source, 480p, 720p, 1080p, 1440p, 2160p (default: source)",
			},
			"video_codec": {
				Type: schema.String,
				Desc: "Video codec: h264, h265, vp9, copy (default: h264; webm defaults to vp9)",
			},
			"preset": {
				Type: schema.String,
				Desc: "Encoder preset: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow (default: medium)",
			},
			"crf": {
				Type: schema.Integer,
				Desc: "CRF quality value 18-36 for re-encoding (default: 23, lower is higher quality)",
			},
			"audio_bitrate": {
				Type: schema.String,
				Desc: "Audio bitrate for re-encoding, such as 128k or 192k (default: 192k)",
			},
			"fps": {
				Type: schema.Integer,
				Desc: "Target frame rate. Leave empty to keep source FPS.",
			},
			"output_path": {
				Type: schema.String,
				Desc: "Optional absolute or relative output file path. If omitted, a new file is created next to the source video.",
			},
		}),
	}, nil
}

// InvokableRun handles LLM tool calls.
func (t *TranscodeVideoTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	type transcodeArgs struct {
		VideoPath    string `json:"video_path"`
		OutputFormat string `json:"output_format,omitempty"`
		Resolution   string `json:"resolution,omitempty"`
		VideoCodec   string `json:"video_codec,omitempty"`
		Preset       string `json:"preset,omitempty"`
		CRF          int    `json:"crf,omitempty"`
		AudioBitrate string `json:"audio_bitrate,omitempty"`
		FPS          int    `json:"fps,omitempty"`
		OutputPath   string `json:"output_path,omitempty"`
	}
	args, err := UnmarshalArgs[transcodeArgs](transcodeVideoToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	if err := RequireString(transcodeVideoToolName, "video_path", args.VideoPath); err != nil {
		return "", err
	}
	return t.transcode(ctx, transcodeRequest{
		VideoPath:    args.VideoPath,
		OutputFormat: args.OutputFormat,
		Resolution:   args.Resolution,
		VideoCodec:   args.VideoCodec,
		Preset:       args.Preset,
		CRF:          args.CRF,
		AudioBitrate: args.AudioBitrate,
		FPS:          args.FPS,
		OutputPath:   args.OutputPath,
	})
}

// Call transcodes a local video path using the default MP4/H.264 settings.
func (t *TranscodeVideoTool) Call(ctx context.Context, input string) (string, error) {
	videoPath := strings.TrimSpace(input)
	if videoPath == "" {
		return "", fmt.Errorf("video path cannot be empty")
	}
	return t.transcode(ctx, transcodeRequest{VideoPath: videoPath, OutputFormat: transcodeFormatMP4})
}

type transcodeRequest struct {
	VideoPath    string
	OutputFormat string
	Resolution   string
	VideoCodec   string
	Preset       string
	CRF          int
	AudioBitrate string
	FPS          int
	OutputPath   string
}

func (t *TranscodeVideoTool) transcode(ctx context.Context, req transcodeRequest) (string, error) {
	videoPath, err := filepath.Abs(strings.TrimSpace(req.VideoPath))
	if err != nil {
		return "", fmt.Errorf("resolve video_path: %w", err)
	}
	if _, err := os.Stat(videoPath); err != nil {
		return "", fmt.Errorf("video file not found: %w", err)
	}

	outputFormat, err := normalizeTranscodeFormat(req.OutputFormat)
	if err != nil {
		return "", err
	}
	resolution, err := normalizeTranscodeResolution(req.Resolution)
	if err != nil {
		return "", err
	}
	videoCodec, err := normalizeTranscodeCodec(req.VideoCodec, outputFormat)
	if err != nil {
		return "", err
	}
	preset, err := normalizeTranscodePreset(req.Preset)
	if err != nil {
		return "", err
	}
	crf, err := normalizeTranscodeCRF(req.CRF)
	if err != nil {
		return "", err
	}
	audioBitrate, err := normalizeTranscodeAudioBitrate(req.AudioBitrate)
	if err != nil {
		return "", err
	}
	if req.FPS < 0 {
		return "", fmt.Errorf("fps must be greater than 0")
	}

	inputFormat := normalizeInputExtension(videoPath)
	if err := validateTranscodeCombination(inputFormat, outputFormat, resolution, videoCodec, req.FPS); err != nil {
		return "", err
	}

	outputPath, err := resolveTranscodeOutputPath(videoPath, req.OutputPath, outputFormat, resolution, videoCodec, req.FPS)
	if err != nil {
		return "", err
	}

	videoFilter := buildTranscodeScaleFilter(resolution)
	audioCodec := defaultAudioCodec(outputFormat)

	t.logger.Info("transcoding video",
		zap.String("input", videoPath),
		zap.String("output", outputPath),
		zap.String("format", outputFormat),
		zap.String("resolution", resolution),
		zap.String("video_codec", videoCodec),
		zap.Int("fps", req.FPS),
	)

	err = utils.TranscodeVideoWithOptions(ctx, t.ffmpegPath, utils.VideoTranscodeOptions{
		InputPath:    videoPath,
		OutputPath:   outputPath,
		VideoCodec:   ffmpegVideoCodec(videoCodec),
		AudioCodec:   audioCodec,
		AudioBitrate: audioBitrate,
		VideoFilter:  videoFilter,
		Preset:       preset,
		CRF:          crf,
		FPS:          req.FPS,
		Overwrite:    false,
	})
	if err != nil {
		return "", err
	}

	t.logger.Info("video transcoded",
		zap.String("output", outputPath),
		zap.String("format", outputFormat),
		zap.String("resolution", resolution),
		zap.String("video_codec", videoCodec),
	)
	return outputPath, nil
}

func normalizeTranscodeFormat(input string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(input))
	if format == "" {
		return transcodeFormatMP4, nil
	}
	if _, ok := supportedTranscodeFormats[format]; !ok {
		return "", fmt.Errorf("unsupported output_format %q, allowed: mp4, mkv, mov, webm", input)
	}
	return format, nil
}

func normalizeTranscodeResolution(input string) (string, error) {
	resolution := strings.ToLower(strings.TrimSpace(input))
	if resolution == "" {
		return transcodeResolutionSource, nil
	}
	if resolution == transcodeResolutionSource {
		return resolution, nil
	}
	if _, ok := transcodeResolutionHeights[resolution]; !ok {
		return "", fmt.Errorf("unsupported resolution %q, allowed: source, 480p, 720p, 1080p, 1440p, 2160p", input)
	}
	return resolution, nil
}

func normalizeTranscodeCodec(input, outputFormat string) (string, error) {
	codec := strings.ToLower(strings.TrimSpace(input))
	if codec == "" {
		if outputFormat == transcodeFormatWEBM {
			return transcodeCodecVP9, nil
		}
		return transcodeCodecH264, nil
	}
	if _, ok := supportedTranscodeCodecs[codec]; !ok {
		return "", fmt.Errorf("unsupported video_codec %q, allowed: h264, h265, vp9, copy", input)
	}
	return codec, nil
}

func normalizeTranscodePreset(input string) (string, error) {
	preset := strings.ToLower(strings.TrimSpace(input))
	if preset == "" {
		return transcodeDefaultPreset, nil
	}
	if _, ok := supportedTranscodePresets[preset]; !ok {
		return "", fmt.Errorf("unsupported preset %q", input)
	}
	return preset, nil
}

func normalizeTranscodeCRF(input int) (int, error) {
	if input == 0 {
		return transcodeDefaultCRF, nil
	}
	if input < 18 || input > 36 {
		return 0, fmt.Errorf("crf must be between 18 and 36")
	}
	return input, nil
}

func normalizeTranscodeAudioBitrate(input string) (string, error) {
	bitrate := strings.ToLower(strings.TrimSpace(input))
	if bitrate == "" {
		return transcodeDefaultBitrate, nil
	}
	if len(bitrate) < 2 || !strings.HasSuffix(bitrate, "k") {
		return "", fmt.Errorf("audio_bitrate must look like 128k or 192k")
	}
	value, err := strconv.Atoi(strings.TrimSuffix(bitrate, "k"))
	if err != nil || value < 32 || value > 512 {
		return "", fmt.Errorf("audio_bitrate must be between 32k and 512k")
	}
	return bitrate, nil
}

func validateTranscodeCombination(inputFormat, outputFormat, resolution, videoCodec string, fps int) error {
	if outputFormat == transcodeFormatWEBM && (videoCodec == transcodeCodecH264 || videoCodec == transcodeCodecH265) {
		return fmt.Errorf("webm output only supports vp9 or copy video codecs")
	}
	if videoCodec != transcodeCodecCopy {
		return nil
	}
	if resolution != transcodeResolutionSource || fps > 0 {
		return fmt.Errorf("video_codec=copy cannot be used together with resolution or fps changes")
	}
	if inputFormat != "" && inputFormat != outputFormat {
		return fmt.Errorf("video_codec=copy requires output_format to match the source container (%s)", inputFormat)
	}
	return nil
}

func resolveTranscodeOutputPath(videoPath, requestedOutputPath, outputFormat, resolution, videoCodec string, fps int) (string, error) {
	if strings.TrimSpace(requestedOutputPath) != "" {
		outputPath, err := filepath.Abs(strings.TrimSpace(requestedOutputPath))
		if err != nil {
			return "", fmt.Errorf("resolve output_path: %w", err)
		}
		if outputPath == videoPath {
			return "", fmt.Errorf("output_path must be different from video_path")
		}
		return outputPath, nil
	}

	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	suffixParts := []string{"transcoded", videoCodec}
	if resolution != transcodeResolutionSource {
		suffixParts = append(suffixParts, resolution)
	}
	if fps > 0 {
		suffixParts = append(suffixParts, fmt.Sprintf("%dfps", fps))
	}
	fileName := base + "." + strings.Join(suffixParts, ".") + "." + outputFormat
	outputPath := filepath.Join(dir, fileName)
	if outputPath == videoPath {
		outputPath = filepath.Join(dir, base+".transcoded.1."+outputFormat)
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return outputPath, nil
	}

	for index := 1; index <= 999; index++ {
		candidate := filepath.Join(dir, base+"."+strings.Join(suffixParts, ".")+fmt.Sprintf(".%d.%s", index, outputFormat))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to allocate output path for transcoded video")
}

func buildTranscodeScaleFilter(resolution string) string {
	height, ok := transcodeResolutionHeights[resolution]
	if !ok {
		return ""
	}
	return fmt.Sprintf("scale=w=-2:h=%d:force_original_aspect_ratio=decrease", height)
}

func ffmpegVideoCodec(codec string) string {
	switch codec {
	case transcodeCodecH265:
		return "libx265"
	case transcodeCodecVP9:
		return "libvpx-vp9"
	case transcodeCodecCopy:
		return "copy"
	default:
		return "libx264"
	}
}

func defaultAudioCodec(outputFormat string) string {
	if outputFormat == transcodeFormatWEBM {
		return "libopus"
	}
	return "aac"
}

func normalizeInputExtension(videoPath string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(videoPath)), ".")
	if _, ok := supportedTranscodeFormats[ext]; ok {
		return ext
	}
	return ""
}

// NewTranscodeVideoToolFromAppConfig 从 AppConfig 创建 TranscodeVideoTool。
func NewTranscodeVideoToolFromAppConfig(appCfg *config.AppConfig, logger *zap.Logger) (*TranscodeVideoTool, error) {
	return NewTranscodeVideoTool(appCfg.Workflow.FFmpegPath, logger)
}
