package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoTranscodeOptions contains the concrete ffmpeg parameters for a transcode job.
type VideoTranscodeOptions struct {
	InputPath    string
	OutputPath   string
	VideoCodec   string
	AudioCodec   string
	AudioBitrate string
	VideoFilter  string
	Preset       string
	CRF          int
	FPS          int
	Overwrite    bool
	ExtraArgs    []string
}

// TranscodeVideoWithOptions runs ffmpeg with the provided transcoding options.
func TranscodeVideoWithOptions(ctx context.Context, ffmpegPath string, opts VideoTranscodeOptions) error {
	if strings.TrimSpace(opts.InputPath) == "" {
		return fmt.Errorf("input path is required")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}

	resolvedFFmpegPath := ffmpegPath
	if resolvedFFmpegPath == "" {
		path, err := exec.LookPath("ffmpeg")
		if err != nil {
			return fmt.Errorf("ffmpeg not found in PATH: %w", err)
		}
		resolvedFFmpegPath = path
	}
	if _, err := os.Stat(resolvedFFmpegPath); err != nil {
		return fmt.Errorf("ffmpeg not found at %s: %w", resolvedFFmpegPath, err)
	}

	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input video not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	args := make([]string, 0, 24)
	if opts.Overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}
	args = append(args, "-i", opts.InputPath)

	if opts.VideoFilter != "" {
		args = append(args, "-vf", opts.VideoFilter)
	}
	if opts.FPS > 0 {
		args = append(args, "-r", strconv.Itoa(opts.FPS))
	}

	if opts.VideoCodec != "" {
		args = append(args, "-c:v", opts.VideoCodec)
		if opts.VideoCodec != "copy" {
			if opts.Preset != "" {
				args = append(args, "-preset", opts.Preset)
			}
			if opts.CRF > 0 {
				args = append(args, "-crf", strconv.Itoa(opts.CRF))
			}
		}
	}

	if opts.AudioCodec != "" {
		args = append(args, "-c:a", opts.AudioCodec)
		if opts.AudioCodec != "copy" && opts.AudioBitrate != "" {
			args = append(args, "-b:a", opts.AudioBitrate)
		}
	}

	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}
	args = append(args, opts.OutputPath)

	cmd := exec.CommandContext(ctx, resolvedFFmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg transcode failed: %w\noutput: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
