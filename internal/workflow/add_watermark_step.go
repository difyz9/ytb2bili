package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤: 非会员添加水印
// ============================================================================

type AddWatermarkStep struct {
	BaseStep
	ffmpegPath     string
	fontCandidates []string
	logger         *zap.Logger
}

type AddWatermarkStepParams struct {
	fx.In
	Cfg    config.WorkflowConfig
	Logger *zap.Logger
}

func NewAddWatermarkStep(params AddWatermarkStepParams) *AddWatermarkStep {
	ffmpegPath := strings.TrimSpace(params.Cfg.FFmpegPath)
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	fontCandidates := buildWatermarkFontCandidates(params.Cfg)

	return &AddWatermarkStep{
		BaseStep:       NewBaseStepWithOrder(StepNameAddWatermark, true, 8),
		ffmpegPath:     ffmpegPath,
		fontCandidates: fontCandidates,
		logger:         params.Logger,
	}
}

func (s *AddWatermarkStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	videoPath := strings.TrimSpace(vctx.VideoPath)
	if videoPath == "" {
		return vctx, nil
	}

	// 已经是水印输出文件时，避免重复处理。
	if strings.Contains(strings.ToLower(filepath.Base(videoPath)), ".watermarked") {
		return vctx, nil
	}

	userID := strings.TrimSpace(vctx.UserID)
	if userID == "" {
		userID = strings.TrimSpace(GetUserID(ctx))
	}
	if userID == "" {
		s.logger.Debug("Skip watermark: missing user id")
		return vctx, nil
	}

	// nonMember, reason := s.isNonMember(ctx, userID)
	// if !nonMember {
	// 	s.logger.Info("Skip watermark",
	// 		zap.String("user_id", userID),
	// 		zap.String("reason", reason),
	// 		zap.String("video_id", vctx.VideoID),
	// 	)
	// 	return vctx, nil
	// }

	filter := s.buildDrawtextFilter()

	outPath := watermarkedOutputPath(videoPath)
	tmpPath := watermarkTempOutputPath(outPath)

	if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return nil, fmt.Errorf("remove watermark tmp: %w", rmErr)
	}
	if rmErr := os.Remove(outPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return nil, fmt.Errorf("remove existing watermark output: %w", rmErr)
	}

	s.logger.Info("Applying watermark for non-member",
		zap.String("user_id", userID),
		zap.String("video_id", vctx.VideoID),
		zap.String("input", videoPath),
		zap.String("output", outPath),
	)

	if err := s.runFFmpegWatermark(ctx, videoPath, tmpPath, filter); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return nil, fmt.Errorf("rename watermark output: %w", err)
	}

	vctx.VideoPath = outPath
	return vctx, nil
}

func (s *AddWatermarkStep) buildDrawtextFilter() string {
	if font := s.resolveExistingFontFile(); font != "" {
		font = escapeFFmpegDrawtextValue(font)
		return "drawtext=fontfile=" + font + ":text='Powered by ytb2bili':fontsize=100:fontcolor=green@0.7:x=(w-text_w)-10:y=(h-text_h)-10:box=1:boxcolor=yellow@0.5:boxborderw=10"
	}

	// 未找到字体文件时，让 ffmpeg/fontconfig 自行选择默认字体。
	return "drawtext=text='Powered by ytb2bili':fontsize=100:fontcolor=green@0.7:x=(w-text_w)-10:y=(h-text_h)-10:box=1:boxcolor=yellow@0.5:boxborderw=10"
}

func (s *AddWatermarkStep) resolveExistingFontFile() string {
	execDir := ""
	if exe, err := os.Executable(); err == nil {
		execDir = filepath.Dir(exe)
	}

	for _, cand := range s.fontCandidates {
		cand = strings.TrimSpace(cand)
		if cand == "" {
			continue
		}
		if fileExists(cand) {
			return cand
		}
		if execDir != "" && !filepath.IsAbs(cand) {
			alt := filepath.Join(execDir, cand)
			if fileExists(alt) {
				return alt
			}
		}
	}
	return ""
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func buildWatermarkFontCandidates(cfg config.WorkflowConfig) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	add(os.Getenv("YTB2BILI_WATERMARK_FONT_FILE"))
	add(cfg.WatermarkFontFile)

	// 约定优先：容器镜像内 /app/fonts
	add("/app/fonts/watermark.ttf")
	add("/app/fonts/watermark.otf")
	add("/app/fonts/GoogleSansFlex.ttf")
	add("/app/GoogleSansFlex.ttf")

	// 常见系统字体安装路径（apt/apk）
	add("/usr/share/fonts/truetype/roboto/Roboto-Regular.ttf")
	add("/usr/share/fonts/roboto/Roboto-Regular.ttf")
	add("/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf")
	add("/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc")
	add("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")

	// 兼容：本地运行时相对路径
	add("./docker/fonts/watermark.ttf")
	add("./docker/fonts/watermark.otf")
	add("./fonts/watermark.ttf")
	add("./fonts/watermark.otf")

	return out
}

func (s *AddWatermarkStep) runFFmpegWatermark(ctx context.Context, inPath, outPath, filter string) error {
	args := []string{
		"-y",
		"-i", inPath,
		"-vf", filter,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-c:a", "copy",
		outPath,
	}

	cmd := exec.CommandContext(ctx, s.ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg watermark failed: %w: %s", err, truncateOutput(string(out), 4000))
	}
	return nil
}

func watermarkedOutputPath(videoPath string) string {
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+".watermarked.mp4")
}

func watermarkTempOutputPath(outPath string) string {
	ext := filepath.Ext(outPath)
	if ext == "" {
		return outPath + ".tmp"
	}
	name := strings.TrimSuffix(outPath, ext)
	return name + ".tmp" + ext
}

func escapeFFmpegDrawtextValue(v string) string {
	// drawtext 使用 ':' 作为分隔符；路径若包含 ':' 需要转义。
	// 同时转义 '\\'，避免被解析器吞掉。
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, ":", "\\:")
	return v
}

func truncateOutput(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
