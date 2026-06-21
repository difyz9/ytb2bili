package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// DownloadVideoTool downloads YouTube videos via yt-dlp.
// Implements eino's tool.InvokableTool (Info + InvokableRun).
// Also exposes Call() for direct use by the YouTube workflow.
type DownloadVideoTool struct {
	ytdlpPath   string
	downloadDir string
	cookiesDir  string // downloadDir/cookies – where the browser extension saves files
	cookiesFile string
	proxyURL    string
	logger      *zap.Logger
}

type downloadStrategy struct {
	label string
	extra []string
}

const downloadVideoToolName = "download_video"
const minDownloadHeight = 720
const ytDLPProgressPrefix = "ytb2bili-progress:"

type downloadProgressContextKey string

const downloadProgressReporterKey downloadProgressContextKey = "download_progress_reporter"

type DownloadRequest struct {
	Input               string
	PreferredResolution string
}

type PlaylistOptions struct {
	Enabled    bool
	StartIndex int
	MaxItems   int
}

type PlaylistEntry struct {
	VideoID    string
	Title      string
	URL        string
	PlaylistID string
	Position   int
}

type PlaylistResult struct {
	PlaylistID string
	Title      string
	Entries    []PlaylistEntry
}

type DownloadResult struct {
	VideoPath  string
	Client     string
	FormatID   string
	Resolution string
	Format     string
	Ext        string
	Width      int
	Height     int
}

type DownloadProgressUpdate struct {
	Percent    int
	Message    string
	FormatID   string
	Resolution string
	Client     string
}

type DownloadProgressReporter func(DownloadProgressUpdate)

type ytDLPSelection struct {
	FormatID   string
	Format     string
	Resolution string
	Width      int
	Height     int
	Ext        string
}

type ytDLPProbe struct {
	ID                 string                 `json:"id"`
	Title              string                 `json:"title"`
	FormatID           string                 `json:"format_id"`
	Format             string                 `json:"format"`
	Resolution         string                 `json:"resolution"`
	Width              int                    `json:"width"`
	Height             int                    `json:"height"`
	Ext                string                 `json:"ext"`
	Entries            []ytDLPPlaylistEntry   `json:"entries"`
	RequestedFormats   []ytDLPRequestedFormat `json:"requested_formats"`
	RequestedDownloads []ytDLPRequestedFormat `json:"requested_downloads"`
}

type ytDLPPlaylistEntry struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	PlaylistIndex int    `json:"playlist_index"`
}

type ytDLPRequestedFormat struct {
	FormatID   string `json:"format_id"`
	Format     string `json:"format"`
	Resolution string `json:"resolution"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Ext        string `json:"ext"`
	VCodec     string `json:"vcodec"`
	ACodec     string `json:"acodec"`
}

type downloadArgs struct {
	VideoURL  string `json:"video_url"`
	Format    string `json:"format,omitempty"`
	AudioOnly bool   `json:"audio_only,omitempty"`
}

// WithDownloadProgressReporter stores a progress callback in context.
func WithDownloadProgressReporter(ctx context.Context, reporter DownloadProgressReporter) context.Context {
	if reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, downloadProgressReporterKey, reporter)
}

func reportDownloadProgress(ctx context.Context, update DownloadProgressUpdate) {
	reporter, ok := ctx.Value(downloadProgressReporterKey).(DownloadProgressReporter)
	if !ok || reporter == nil {
		return
	}
	reporter(update)
}

// DownloadVideoConfig holds constructor options.
type DownloadVideoConfig struct {
	YtDlpPath   string
	DownloadDir string
	CookiesDir  string // directory scanned for *.txt cookie files; defaults to /tmp/cookies
	CookiesFile string
	ProxyURL    string
}

// NewDownloadVideoTool creates a DownloadVideoTool.
func NewDownloadVideoTool(config DownloadVideoConfig, logger *zap.Logger) (*DownloadVideoTool, error) {
	if config.DownloadDir == "" {
		return nil, fmt.Errorf("download directory is required")
	}
	if err := os.MkdirAll(config.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

	ytdlpPath := config.YtDlpPath
	if ytdlpPath == "" {
		path, err := exec.LookPath("yt-dlp")
		if err != nil {
			return nil, fmt.Errorf("yt-dlp not found, please install: pip install yt-dlp")
		}
		ytdlpPath = path
	}

	cookiesDir := config.CookiesDir
	if cookiesDir == "" {
		cookiesDir = "/tmp/cookies"
	}
	if err := os.MkdirAll(cookiesDir, 0755); err != nil {
		logger.Warn("Failed to create cookies directory", zap.String("path", cookiesDir), zap.Error(err))
	}

	cookiesFile := config.CookiesFile
	if cookiesFile == "" {
		cookiesFile = findLatestCookiesFile(cookiesDir)
	}
	if cookiesFile != "" {
		logger.Info("Using cookies file", zap.String("path", cookiesFile))
	}

	return &DownloadVideoTool{
		ytdlpPath:   ytdlpPath,
		downloadDir: config.DownloadDir,
		cookiesDir:  cookiesDir,
		cookiesFile: cookiesFile,
		proxyURL:    config.ProxyURL,
		logger:      logger,
	}, nil
}

// ── eino tool.InvokableTool ───────────────────────────────────────────────────

// Info describes the tool to the LLM via eino's schema system.
func (t *DownloadVideoTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "download_video",
		Desc: "Download a YouTube video to local storage using yt-dlp. Returns the absolute path of the downloaded file.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_url": {
				Type:     schema.String,
				Desc:     "YouTube video URL or video ID (e.g. https://www.youtube.com/watch?v=dQw4w9WgXcQ)",
				Required: true,
			},
			"format": {
				Type: schema.String,
				Desc: "Video quality: best, 720p, 1080p, worst (default: best)",
			},
			"audio_only": {
				Type: schema.Boolean,
				Desc: "Download audio only as MP3 (default: false)",
			},
		}),
	}, nil
}

// InvokableRun is called by the eino agent when the LLM issues a tool call.
func (t *DownloadVideoTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	args, err := UnmarshalArgs[downloadArgs](downloadVideoToolName, argumentsInJSON)
	if err != nil {
		return "", err
	}
	if err := RequireString(downloadVideoToolName, "video_url", args.VideoURL); err != nil {
		return "", err
	}
	result, err := t.Download(ctx, DownloadRequest{Input: args.VideoURL, PreferredResolution: args.Format})
	if err != nil {
		return "", err
	}
	return result.VideoPath, nil
}

// ── Direct workflow use ───────────────────────────────────────────────────────

// Call accepts a plain URL/ID string (for direct workflow use).
func (t *DownloadVideoTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.Download(ctx, DownloadRequest{Input: input})
	if err != nil {
		return "", err
	}
	return result.VideoPath, nil
}

// Download executes yt-dlp and returns the downloaded file plus the selected format metadata.
func (t *DownloadVideoTool) Download(ctx context.Context, req DownloadRequest) (*DownloadResult, error) {
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		return nil, fmt.Errorf("video URL or ID cannot be empty")
	}
	return t.call(ctx, req)
}

// ListPlaylistEntries resolves a YouTube playlist into individual video tasks.
func (t *DownloadVideoTool) ListPlaylistEntries(ctx context.Context, input string, options PlaylistOptions) (*PlaylistResult, error) {
	playlistID := ExtractYouTubePlaylistID(input)
	if playlistID == "" {
		return nil, fmt.Errorf("playlist URL is required")
	}

	if latest := findLatestCookiesFile(t.cookiesDir); latest != "" && latest != t.cookiesFile {
		t.cookiesFile = latest
	}

	startIndex := options.StartIndex
	if startIndex < 1 {
		startIndex = 1
	}
	maxItems := options.MaxItems
	if maxItems < 1 {
		maxItems = 10
	}
	endIndex := startIndex + maxItems - 1

	args := []string{
		"--flat-playlist",
		"--dump-single-json",
		"--skip-download",
		"--no-warnings",
		"--playlist-start", strconv.Itoa(startIndex),
		"--playlist-end", strconv.Itoa(endIndex),
	}
	if t.proxyURL != "" {
		args = append(args, "--proxy", t.proxyURL)
	}
	if t.cookiesFile != "" {
		args = append(args, "--cookies", t.cookiesFile)
	}
	args = append(args, normalizePlaylistURL(input, playlistID))

	cmd := exec.CommandContext(ctx, t.ytdlpPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, handleDownloadError(err, string(out))
	}

	var probe ytDLPProbe
	if err := sonic.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("parse playlist metadata: %w", err)
	}

	result := &PlaylistResult{
		PlaylistID: strings.TrimSpace(probe.ID),
		Title:      strings.TrimSpace(probe.Title),
	}
	if result.PlaylistID == "" {
		result.PlaylistID = playlistID
	}

	for index, entry := range probe.Entries {
		videoID := strings.TrimSpace(entry.ID)
		if videoID == "" {
			videoID = extractVideoID(entry.URL)
		}
		if videoID == "" {
			continue
		}

		position := entry.PlaylistIndex
		if position <= 0 {
			position = startIndex + index
		}

		result.Entries = append(result.Entries, PlaylistEntry{
			VideoID:    videoID,
			Title:      strings.TrimSpace(entry.Title),
			URL:        normalizeURL(videoID),
			PlaylistID: result.PlaylistID,
			Position:   position,
		})
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("playlist has no downloadable entries")
	}

	return result, nil
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (t *DownloadVideoTool) call(ctx context.Context, req DownloadRequest) (*DownloadResult, error) {
	videoID := extractVideoID(req.Input)
	t.logger.Info("Starting video download", zap.String("video_id", videoID))

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if latest := findLatestCookiesFile(t.cookiesDir); latest != "" && latest != t.cookiesFile {
			t.cookiesFile = latest
			t.logger.Debug("Switched to newer cookies", zap.String("path", latest))
		}
		if attempt > 1 {
			t.logger.Info("Retrying download", zap.Int("attempt", attempt))
			time.Sleep(5 * time.Second)
		}
		result, err := t.download(ctx, videoID, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if isCookiesError(err) {
			break
		}
	}
	return nil, lastErr
}

func (t *DownloadVideoTool) download(ctx context.Context, videoID string, req DownloadRequest) (*DownloadResult, error) {
	videoDir := filepath.Join(t.downloadDir, videoID)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create video directory: %w", err)
	}

	preferredResolution := normalizePreferredResolution(req.PreferredResolution)
	formatSelector := formatSelectorForResolution(preferredResolution)

	baseArgs := []string{
		"-P", videoDir,
		"-o", "%(id)s.%(ext)s",
		"-f", formatSelector,

		"--merge-output-format", "mp4",
		"--no-playlist",
	}
	if t.proxyURL != "" {
		baseArgs = append(baseArgs, "--proxy", t.proxyURL)
	}
	url := normalizeURL(req.Input)
	requiredMinHeight := minimumHeightForResolution(preferredResolution)

	// tryClient runs yt-dlp with the given extra args and returns (videoPath, nil) on success.
	tryClient := func(label string, extra []string) (*DownloadResult, error) {
		strategyArgs := append(append([]string{}, baseArgs...), extra...)
		selection, err := t.probeDownloadSelection(ctx, strategyArgs, url)
		if err != nil {
			return nil, fmt.Errorf("probe failed: %w", err)
		}
		if selection.Height > 0 && selection.Height < requiredMinHeight {
			err := fmt.Errorf("selected video resolution too low for client %s: got %s (%s), minimum is %dp", label, selection.resolutionLabel(), selection.formatIDLabel(), requiredMinHeight)
			t.logger.Info("Skipping low-resolution client",
				zap.String("client", label),
				zap.String("format_id", selection.formatIDLabel()),
				zap.String("resolution", selection.resolutionLabel()),
				zap.Int("minimum_height", requiredMinHeight),
			)
			return nil, err
		}
		reportDownloadProgress(ctx, DownloadProgressUpdate{
			Percent:    0,
			Message:    fmt.Sprintf("已选格式 %s (%s)", selection.resolutionLabel(), selection.formatIDLabel()),
			FormatID:   selection.formatIDLabel(),
			Resolution: selection.resolutionLabel(),
			Client:     label,
		})
		t.logger.Info("Selected video format",
			zap.String("client", label),
			zap.String("format_id", selection.formatIDLabel()),
			zap.String("resolution", selection.resolutionLabel()),
			zap.String("format", selection.formatLabel()),
			zap.String("ext", selection.extLabel()),
		)

		args := append(strategyArgs, url)
		t.logger.Debug("Attempting download", zap.String("client", label), zap.String("video_id", videoID))
		if err := t.runDownloadCommand(ctx, label, videoID, args, selection); err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}
		vp := findVideoFile(videoDir)
		if vp == "" {
			return nil, fmt.Errorf("download completed but video file not found")
		}
		t.logger.Info("Video downloaded",
			zap.String("client", label),
			zap.String("format_id", selection.formatIDLabel()),
			zap.String("resolution", selection.resolutionLabel()),
			zap.String("path", vp),
		)
		reportDownloadProgress(ctx, DownloadProgressUpdate{
			Percent:    100,
			Message:    fmt.Sprintf("下载完成 %s", selection.resolutionLabel()),
			FormatID:   selection.formatIDLabel(),
			Resolution: selection.resolutionLabel(),
			Client:     label,
		})
		return &DownloadResult{
			VideoPath:  vp,
			Client:     label,
			FormatID:   selection.formatIDLabel(),
			Resolution: selection.resolutionLabel(),
			Format:     selection.formatLabel(),
			Ext:        selection.extLabel(),
			Width:      selection.Width,
			Height:     selection.Height,
		}, nil
	}

	// Build cookies args once.
	var cookieArgs []string
	if t.cookiesFile != "" {
		cookieArgs = []string{"--cookies", t.cookiesFile}
		t.logger.Debug("Using cookies file", zap.String("path", t.cookiesFile))
	} else {
		t.logger.Warn("No cookies file found — YouTube bot detection may block the download.",
			zap.String("cookies_dir", t.cookiesDir),
			zap.String("hint", "export YouTube cookies from Chrome/Firefox with the 'Get cookies.txt LOCALLY' extension and place the .txt file in cookies_dir"),
		)
	}

	strategies := buildDownloadStrategies(cookieArgs)
	var lastErr error
	for index, strategy := range strategies {
		result, err := tryClient(strategy.label, strategy.extra)
		if err == nil {
			return result, nil
		}
		nextClient := ""
		if index+1 < len(strategies) {
			nextClient = strategies[index+1].label
		}
		if nextClient != "" {
			t.logger.Info("Client attempt failed, trying next client",
				zap.String("client", strategy.label),
				zap.String("next_client", nextClient),
				zap.String("reason", compactErrorMessage(err)),
			)
		} else {
			t.logger.Info("Client attempt failed",
				zap.String("client", strategy.label),
				zap.String("reason", compactErrorMessage(err)),
			)
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("download failed: no available download strategy")
}

func buildDownloadStrategies(cookieArgs []string) []downloadStrategy {
	hasCookies := len(cookieArgs) > 0
	if hasCookies {
		return []downloadStrategy{
			{
				label: "default+cookies",
				extra: copyArgs(cookieArgs),
			},
			{
				label: "web_creator+cookies",
				extra: append(copyArgs(cookieArgs), "--extractor-args", "youtube:player_client=web_creator"),
			},
			{
				label: "tv+cookies",
				extra: append(copyArgs(cookieArgs), "--extractor-args", "youtube:player_client=tv"),
			},
			{
				label: "tv_embedded+cookies",
				extra: append(copyArgs(cookieArgs), "--extractor-args", "youtube:player_client=tv_embedded"),
			},
			{
				label: "mweb+cookies",
				extra: append(copyArgs(cookieArgs), "--extractor-args", "youtube:player_client=mweb"),
			},
		}
	}

	return []downloadStrategy{
		{
			label: "default",
			extra: nil,
		},
		{
			label: "tv_embedded",
			extra: []string{"--extractor-args", "youtube:player_client=tv_embedded"},
		},
		{
			label: "mweb",
			extra: []string{"--extractor-args", "youtube:player_client=mweb"},
		},
	}
}

func copyArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	dup := make([]string, len(args))
	copy(dup, args)
	return dup
}

func compactErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if newline := strings.IndexByte(msg, '\n'); newline >= 0 {
		msg = msg[:newline]
	}
	return msg
}

func normalizePreferredResolution(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "best", "auto":
		return "best"
	case "720", "720p":
		return "720p"
	case "1080", "1080p", "fullhd":
		return "1080p"
	case "1440", "1440p", "2k":
		return "1440p"
	case "2160", "2160p", "4k":
		return "2160p"
	default:
		return "best"
	}
}

func preferredResolutionHeight(value string) int {
	switch normalizePreferredResolution(value) {
	case "720p":
		return 720
	case "1080p":
		return 1080
	case "1440p":
		return 1440
	case "2160p":
		return 2160
	default:
		return 0
	}
}

func minimumHeightForResolution(value string) int {
	if height := preferredResolutionHeight(value); height > 0 {
		if height < minDownloadHeight {
			return height
		}
		return minDownloadHeight
	}
	return minDownloadHeight
}

func formatSelectorForResolution(value string) string {
	if height := preferredResolutionHeight(value); height > 0 {
		return fmt.Sprintf("bestvideo*[height<=%d]+bestaudio/best[height<=%d]", height, height)
	}
	return "bestvideo*+bestaudio/best"
}

func (t *DownloadVideoTool) runDownloadCommand(ctx context.Context, label, videoID string, args []string, selection ytDLPSelection) error {
	progressArgs := append(copyArgs(args),
		"--newline",
		"--progress-template", "download:"+ytDLPProgressPrefix+"%(progress._percent_str)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.eta)s",
	)
	cmd := exec.CommandContext(ctx, t.ytdlpPath, progressArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open yt-dlp stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("open yt-dlp stderr: %w", err)
	}

	var outputMu sync.Mutex
	var output strings.Builder
	handleLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		outputMu.Lock()
		output.WriteString(line)
		output.WriteByte('\n')
		outputMu.Unlock()
		if update, ok := parseDownloadProgressLine(line, label, selection); ok {
			reportDownloadProgress(ctx, update)
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scanOutput(&wg, stdout, handleLine)
	go scanOutput(&wg, stderr, handleLine)

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		outputMu.Lock()
		combined := output.String()
		outputMu.Unlock()
		return handleDownloadError(err, combined)
	}

	t.logger.Debug("yt-dlp download command completed",
		zap.String("client", label),
		zap.String("video_id", videoID),
	)
	return nil
}

func scanOutput(wg *sync.WaitGroup, pipe io.Reader, handleLine func(string)) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		handleLine(scanner.Text())
	}
}

func parseDownloadProgressLine(line, client string, selection ytDLPSelection) (DownloadProgressUpdate, bool) {
	idx := strings.Index(line, ytDLPProgressPrefix)
	if idx < 0 {
		return DownloadProgressUpdate{}, false
	}
	payload := strings.TrimSpace(line[idx+len(ytDLPProgressPrefix):])
	parts := strings.Split(payload, "|")
	if len(parts) < 5 {
		return DownloadProgressUpdate{}, false
	}
	percent := parseProgressPercent(parts[0])
	downloaded := parseProgressInt64(parts[1])
	total := parseProgressInt64(parts[2])
	if total == 0 {
		total = parseProgressInt64(parts[3])
	}
	eta := strings.TrimSpace(parts[4])
	messageParts := []string{fmt.Sprintf("下载中 %d%%", percent)}
	if selection.resolutionLabel() != "unknown" {
		messageParts = append(messageParts, selection.resolutionLabel())
	}
	if downloaded > 0 && total > 0 {
		messageParts = append(messageParts, humanizeBytes(downloaded)+"/"+humanizeBytes(total))
	}
	if eta != "" && eta != "NA" {
		messageParts = append(messageParts, "ETA "+eta+"s")
	}
	return DownloadProgressUpdate{
		Percent:    percent,
		Message:    strings.Join(messageParts, " · "),
		FormatID:   selection.formatIDLabel(),
		Resolution: selection.resolutionLabel(),
		Client:     client,
	}, true
}

func parseProgressPercent(value string) int {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" || value == "NA" {
		return 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int(floatValue + 0.5)
}

func parseProgressInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "NA" {
		return 0
	}
	intValue, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return intValue
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(floatValue)
}

func humanizeBytes(value int64) string {
	if value <= 0 {
		return "0B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	floatValue := float64(value)
	unitIndex := 0
	for floatValue >= 1024 && unitIndex < len(units)-1 {
		floatValue /= 1024
		unitIndex++
	}
	if floatValue >= 100 || unitIndex == 0 {
		return fmt.Sprintf("%.0f%s", floatValue, units[unitIndex])
	}
	return fmt.Sprintf("%.1f%s", floatValue, units[unitIndex])
}

func (t *DownloadVideoTool) probeDownloadSelection(ctx context.Context, strategyArgs []string, url string) (ytDLPSelection, error) {
	probeArgs := append(copyArgs(strategyArgs), "--dump-single-json", "--skip-download", "--no-warnings")
	probeArgs = append(probeArgs, url)

	cmd := exec.CommandContext(ctx, t.ytdlpPath, probeArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ytDLPSelection{}, handleDownloadError(err, string(out))
	}

	var probe ytDLPProbe
	if err := sonic.UnmarshalString(strings.TrimSpace(string(out)), &probe); err != nil {
		return ytDLPSelection{}, fmt.Errorf("probe yt-dlp selection: %w", err)
	}

	selection := probe.selectedVideoFormat()
	if selection.Height == 0 {
		selection.Height = parseResolutionHeight(selection.Resolution)
	}
	return selection, nil
}

func (p ytDLPProbe) selectedVideoFormat() ytDLPSelection {
	for _, candidate := range p.RequestedFormats {
		if candidate.VCodec != "" && candidate.VCodec != "none" {
			return candidate.toSelection()
		}
	}
	for _, candidate := range p.RequestedDownloads {
		if candidate.VCodec != "" && candidate.VCodec != "none" {
			return candidate.toSelection()
		}
	}
	for _, candidate := range p.RequestedFormats {
		return candidate.toSelection()
	}
	for _, candidate := range p.RequestedDownloads {
		return candidate.toSelection()
	}
	return ytDLPSelection{
		FormatID:   p.FormatID,
		Format:     p.Format,
		Resolution: p.Resolution,
		Width:      p.Width,
		Height:     p.Height,
		Ext:        p.Ext,
	}
}

func (f ytDLPRequestedFormat) toSelection() ytDLPSelection {
	return ytDLPSelection{
		FormatID:   f.FormatID,
		Format:     f.Format,
		Resolution: f.Resolution,
		Width:      f.Width,
		Height:     f.Height,
		Ext:        f.Ext,
	}
}

func (s ytDLPSelection) resolutionLabel() string {
	if s.Width > 0 && s.Height > 0 {
		return fmt.Sprintf("%dx%d", s.Width, s.Height)
	}
	if s.Resolution != "" {
		return s.Resolution
	}
	if s.Height > 0 {
		return fmt.Sprintf("%dp", s.Height)
	}
	return "unknown"
}

func (s ytDLPSelection) formatIDLabel() string {
	if s.FormatID != "" {
		return s.FormatID
	}
	return "unknown"
}

func (s ytDLPSelection) formatLabel() string {
	if s.Format != "" {
		return s.Format
	}
	return "unknown"
}

func (s ytDLPSelection) extLabel() string {
	if s.Ext != "" {
		return s.Ext
	}
	return "unknown"
}

func parseResolutionHeight(resolution string) int {
	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return 0
	}
	if strings.HasSuffix(resolution, "p") {
		var height int
		if _, err := fmt.Sscanf(resolution, "%dp", &height); err == nil {
			return height
		}
	}
	var width, height int
	if _, err := fmt.Sscanf(resolution, "%dx%d", &width, &height); err == nil {
		return height
	}
	return 0
}

// ── Package-level helpers ─────────────────────────────────────────────────────

func handleDownloadError(err error, output string) error {
	switch {
	case strings.Contains(output, "Sign in to confirm") || strings.Contains(output, "not a bot"):
		return fmt.Errorf("YouTube bot-detection triggered: export cookies from a logged-in browser " +
			"(Chrome/Firefox → 'Get cookies.txt LOCALLY' extension) and place the .txt file in the cookies_dir; " +
			"also run: yt-dlp -U to ensure yt-dlp is up to date")
	case strings.Contains(output, "cookies are no longer valid") || strings.Contains(output, "HTTP Error 401"):
		return fmt.Errorf("cookies expired - re-export browser cookies and replace the file in cookies_dir")
	case strings.Contains(output, "Video unavailable"):
		return fmt.Errorf("video unavailable or deleted")
	case strings.Contains(output, "Private video"):
		return fmt.Errorf("private video - login required")
	case strings.Contains(output, "n challenge") || strings.Contains(output, "JS Challenge") || strings.Contains(output, "no solutions"):
		return fmt.Errorf("YouTube n-challenge failed: yt-dlp needs update or JS runtime (deno/node) in PATH")
	default:
		return fmt.Errorf("download failed: %w\noutput: %s", err, output)
	}
}

func isCookiesError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "cookies") || strings.Contains(msg, "authentication")
}

func extractVideoID(input string) string {
	if len(input) == 11 && !strings.ContainsAny(input, "/?") {
		return input
	}
	if strings.Contains(input, "v=") {
		parts := strings.Split(input, "v=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	if strings.Contains(input, "youtu.be/") {
		parts := strings.Split(input, "youtu.be/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}
	return input
}

func normalizeURL(input string) string {
	if strings.HasPrefix(input, "http") {
		return input
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", extractVideoID(input))
}

func normalizePlaylistURL(input, playlistID string) string {
	if strings.HasPrefix(strings.TrimSpace(input), "http") {
		return input
	}
	values := url.Values{}
	values.Set("list", playlistID)
	return "https://www.youtube.com/playlist?" + values.Encode()
}

func ExtractYouTubePlaylistID(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + strings.TrimLeft(trimmed, "/")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || (!strings.HasSuffix(host, "youtube.com") && !strings.HasSuffix(host, "youtu.be")) {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("list"))
}

func findVideoFile(dir string) string {
	for _, ext := range []string{"*.mp4", "*.webm", "*.mkv"} {
		files, _ := filepath.Glob(filepath.Join(dir, ext))
		if len(files) > 0 {
			return files[0]
		}
	}
	return ""
}

// findLatestCookiesFile searches dirs in order and returns the newest valid Netscape cookies file.
// Typical dirs: downloadDir/cookies (browser extension saves here), fallback: ./cookies.
func findLatestCookiesFile(dirs ...string) string {
	var allFiles []string
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
		if err == nil {
			allFiles = append(allFiles, files...)
		}
	}
	if len(allFiles) == 0 {
		return ""
	}
	var latestFile string
	var latestTime int64
	for _, f := range allFiles {
		if !isValidCookiesFile(f) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latestFile = f
		}
	}
	return latestFile
}

func isValidCookiesFile(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil || len(data) < 10 {
		return false
	}
	content := string(data)
	return strings.Contains(content, "# Netscape HTTP Cookie File") || strings.Contains(content, ".youtube.com")
}
