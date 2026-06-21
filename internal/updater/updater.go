package updater

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minio/selfupdate"
	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

const (
	// DefaultVersion 默认版本号（如果配置中未指定）
	DefaultVersion = "0.0.1"

	// GitHub API URL
	githubAPIURL = "https://api.github.com/repos/difyz9/ytb2bili/releases/latest"
)

// Version 由构建参数注入，示例：-ldflags "-X github.com/zolagz/ytb2bili/internal/updater.Version=v1.2.3"
var Version = ""

// GitHubRelease GitHub Release API 响应结构
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	CreatedAt  string `json:"created_at"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateStatus 更新状态
type UpdateStatus struct {
	Updating            bool   `json:"updating"`       // 是否正在更新
	Progress            int    `json:"progress"`       // 进度 (0-100)
	Message             string `json:"message"`        // 状态消息
	CurrentVersion      string `json:"currentVersion"` // 当前版本
	LatestVersion       string `json:"latestVersion,omitempty"`
	LastCheckedAt       string `json:"lastCheckedAt,omitempty"`
	RestartOnSuccess    bool   `json:"restartOnSuccess"`
	RestartDelaySeconds int    `json:"restartDelaySeconds"`
}

type releaseCheckError struct {
	StatusCode int
	Message    string
}

func (e *releaseCheckError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("GitHub API 返回错误: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("GitHub API 返回错误: HTTP %d: %s", e.StatusCode, e.Message)
}

func isNonFatalReleaseCheckError(err error) bool {
	var releaseErr *releaseCheckError
	if !errors.As(err, &releaseErr) {
		return false
	}

	switch releaseErr.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// Updater 自动更新管理器
type Updater struct {
	logger           *zap.Logger
	httpClient       *http.Client
	currentVersion   string
	updateURL        string
	githubAPIToken   string
	checkInterval    time.Duration
	autoUpdate       bool
	restartOnSuccess bool
	restartDelay     time.Duration
	ytdlpPath        string
	updateStatus     atomic.Value // 存储 *UpdateStatus
	updating         atomic.Bool
	ytdlpUpdating    atomic.Bool
	restartScheduled atomic.Bool
	exitFunc         func(int)
}

// New 创建更新器
func New(logger *zap.Logger, cfg *config.UpdaterConfig, ytdlpPath string) *Updater {
	currentVersion := resolveCurrentVersion(cfg.CurrentVersion)

	// 将小时数转换为 time.Duration
	checkInterval := time.Duration(cfg.CheckInterval) * time.Hour
	if checkInterval == 0 {
		checkInterval = 24 * time.Hour // 默认每天检查一次
	}

	updater := &Updater{
		logger:           logger,
		httpClient:       &http.Client{Timeout: 5 * time.Minute},
		currentVersion:   currentVersion,
		updateURL:        cfg.UpdateURL,
		githubAPIToken:   strings.TrimSpace(cfg.GitHubAPIToken),
		checkInterval:    checkInterval,
		autoUpdate:       cfg.AutoUpdate,
		restartOnSuccess: cfg.RestartOnSuccess,
		restartDelay:     time.Duration(positiveOrDefault(cfg.RestartDelaySeconds, 5)) * time.Second,
		ytdlpPath:        strings.TrimSpace(ytdlpPath),
		exitFunc:         os.Exit,
	}

	// 初始化更新状态
	updater.storeStatus(false, 0, "就绪", "")

	return updater
}

func resolveCurrentVersion(configVersion string) string {
	if strings.TrimSpace(configVersion) != "" {
		return strings.TrimSpace(strings.TrimPrefix(configVersion, "v"))
	}
	if strings.TrimSpace(Version) != "" {
		return strings.TrimSpace(strings.TrimPrefix(Version, "v"))
	}
	return DefaultVersion
}

func (u *Updater) TriggerYtDlpBackgroundUpdate() {
	if u == nil || !u.ytdlpUpdating.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer u.ytdlpUpdating.Store(false)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := u.checkAndUpdateYtDlp(ctx); err != nil {
			u.logger.Warn("yt-dlp 静默更新失败", zap.Error(err))
		}
	}()
}

func (u *Updater) checkAndUpdateYtDlp(ctx context.Context) error {
	ytdlpPath, err := u.resolveYtDlpPath()
	if err != nil {
		return err
	}

	u.logger.Info("开始静默检查 yt-dlp 更新", zap.String("path", ytdlpPath))
	output, err := runCommand(ctx, ytdlpPath, "-U")
	if err == nil {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			trimmed = "yt-dlp 更新检查完成"
		}
		u.logger.Info("yt-dlp 更新检查完成", zap.String("output", trimmed))
		return nil
	}

	trimmedOutput := strings.TrimSpace(output)
	if !shouldTryYtDlpPipFallback(trimmedOutput) {
		return fmt.Errorf("run yt-dlp -U: %w: %s", err, trimmedOutput)
	}

	u.logger.Info("yt-dlp 不支持自更新，尝试使用 pip 静默升级", zap.String("path", ytdlpPath))
	if pipOutput, pipErr := tryPipUpdate(ctx); pipErr != nil {
		return fmt.Errorf("run yt-dlp -U: %w: %s; pip fallback failed: %v: %s", err, trimmedOutput, pipErr, strings.TrimSpace(pipOutput))
	}

	u.logger.Info("pip 已完成 yt-dlp 静默升级")
	return nil
}

func (u *Updater) resolveYtDlpPath() (string, error) {
	if strings.TrimSpace(u.ytdlpPath) != "" {
		return strings.TrimSpace(u.ytdlpPath), nil
	}

	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", fmt.Errorf("yt-dlp not found in PATH")
	}
	return path, nil
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func tryPipUpdate(ctx context.Context) (string, error) {
	commands := [][]string{
		{"python3", "-m", "pip", "install", "-U", "yt-dlp"},
		{"pip3", "install", "-U", "yt-dlp"},
		{"pip", "install", "-U", "yt-dlp"},
	}

	var lastOutput string
	var lastErr error
	for _, command := range commands {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}

		output, err := runCommand(ctx, command[0], command[1:]...)
		if err == nil {
			return output, nil
		}
		lastOutput = output
		lastErr = err
	}

	if lastErr == nil {
		return "", fmt.Errorf("no pip command available for yt-dlp update")
	}
	return lastOutput, lastErr
}

func shouldTryYtDlpPipFallback(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"use that to update",
		"installed with pip",
		"pyinstaller",
		"not a self-updatable package",
		"please use pip",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (u *Updater) storeStatus(updating bool, progress int, message string, latestVersion string) {
	status := &UpdateStatus{
		Updating:            updating,
		Progress:            progress,
		Message:             message,
		CurrentVersion:      u.currentVersion,
		LatestVersion:       strings.TrimPrefix(latestVersion, "v"),
		LastCheckedAt:       time.Now().Format(time.RFC3339),
		RestartOnSuccess:    u.restartOnSuccess,
		RestartDelaySeconds: int(u.restartDelay / time.Second),
	}
	u.updateStatus.Store(status)
}

func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// CheckForUpdates 检查是否有新版本
func (u *Updater) CheckForUpdates(ctx context.Context) (hasUpdate bool, latestVersion string, err error) {
	u.logger.Info("🔍 检查更新...", zap.String("current_version", u.currentVersion))

	// 获取 GitHub 最新 Release
	release, err := u.getLatestRelease(ctx)
	if err != nil {
		u.logger.Warn("检查更新失败", zap.Error(err))
		return false, "", err
	}

	// 跳过草稿和预发布版本
	if release.Draft || release.Prerelease {
		u.logger.Info("✅ 跳过草稿/预发布版本", zap.String("version", release.TagName))
		u.storeStatus(false, 0, "已检查：最新发布为草稿或预发布版本，已跳过", u.currentVersion)
		return false, u.currentVersion, nil
	}

	latestVersion = strings.TrimPrefix(release.TagName, "v")

	// 比较版本号
	if u.compareVersions(latestVersion, u.currentVersion) > 0 {
		u.logger.Info("✅ 发现新版本",
			zap.String("current", u.currentVersion),
			zap.String("latest", latestVersion))
		u.storeStatus(false, 0, "发现新版本，可执行更新", latestVersion)
		return true, latestVersion, nil
	}

	u.logger.Info("✅ 当前已是最新版本")
	u.storeStatus(false, 100, "当前已是最新版本", latestVersion)
	return false, u.currentVersion, nil
}

// getLatestRelease 从 GitHub API 获取最新 Release
func (u *Updater) getLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	releaseURL := u.updateURL
	if strings.TrimSpace(releaseURL) == "" {
		releaseURL = githubAPIURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置 User-Agent，GitHub API 要求
	req.Header.Set("User-Agent", "ytb2bili-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if u.githubAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.githubAPIToken)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &releaseCheckError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
		}
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &release, nil
}

// compareVersions 比较两个版本号
// 返回: 1 表示 v1 > v2, 0 表示相等, -1 表示 v1 < v2
func (u *Updater) compareVersions(v1, v2 string) int {
	nv1 := normalizeSemver(v1)
	nv2 := normalizeSemver(v2)

	switch {
	case nv1 != "" && nv2 != "":
		return semver.Compare(nv1, nv2)
	case strings.TrimPrefix(v1, "v") == strings.TrimPrefix(v2, "v"):
		return 0
	case nv1 != "" && nv2 == "":
		return 1
	case nv1 == "" && nv2 != "":
		return -1
	case strings.TrimPrefix(v1, "v") > strings.TrimPrefix(v2, "v"):
		return 1
	default:
		return -1
	}
}

func normalizeSemver(version string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if trimmed == "" {
		return ""
	}
	normalized := "v" + trimmed
	if semver.IsValid(normalized) {
		return normalized
	}
	return ""
}

func (u *Updater) ValidateUpdateEnvironment() error {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}

	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err == nil && strings.TrimSpace(resolvedPath) != "" {
		exePath = resolvedPath
	}

	mounted, err := isMountedExecutable(exePath)
	if err != nil {
		u.logger.Debug("检测可执行文件挂载状态失败", zap.String("path", exePath), zap.Error(err))
	}
	if mounted {
		return fmt.Errorf("当前运行在 Docker 挂载二进制模式，无法在容器内原地替换 %s；请在宿主机重新编译对应平台二进制并重启容器", exePath)
	}

	return nil
}

func isMountedExecutable(exePath string) (bool, error) {
	if runtime.GOOS != "linux" {
		return false, nil
	}

	content, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}

	return isMountPointInMountInfo(exePath, string(content)), nil
}

func isMountPointInMountInfo(exePath string, mountInfo string) bool {
	scanner := bufio.NewScanner(strings.NewReader(mountInfo))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) < 1 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) < 5 {
			continue
		}
		mountPoint := decodeMountInfoPath(fields[4])
		if mountPoint == exePath {
			return true
		}
	}
	return false
}

func decodeMountInfoPath(value string) string {
	decoded := value
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\\`,
	)
	decoded = replacer.Replace(decoded)

	if !strings.Contains(decoded, `\`) {
		return decoded
	}

	var builder strings.Builder
	for i := 0; i < len(decoded); i++ {
		if decoded[i] != '\\' || i+3 >= len(decoded) {
			builder.WriteByte(decoded[i])
			continue
		}
		oct := decoded[i+1 : i+4]
		value, err := strconv.ParseInt(oct, 8, 32)
		if err != nil {
			builder.WriteByte(decoded[i])
			continue
		}
		builder.WriteByte(byte(value))
		i += 3
	}
	return builder.String()
}

// DoUpdate 执行更新
func (u *Updater) DoUpdate(ctx context.Context) error {
	if !u.updating.CompareAndSwap(false, true) {
		return fmt.Errorf("更新任务已在执行中")
	}
	defer u.updating.Store(false)

	u.logger.Info("开始更新...")

	// 更新状态
	u.storeStatus(true, 10, "正在检查新版本...", "")

	if err := u.ValidateUpdateEnvironment(); err != nil {
		u.storeStatus(false, 0, err.Error(), "")
		return err
	}

	// 检查是否有新版本
	hasUpdate, latestVersion, err := u.CheckForUpdates(ctx)
	if err != nil {
		u.storeStatus(false, 0, "检查更新失败: "+err.Error(), "")
		return err
	}

	if !hasUpdate {
		u.storeStatus(false, 100, "当前已是最新版本", latestVersion)
		u.logger.Info("✅ 当前已是最新版本，无需更新")
		return nil
	}

	// 从 GitHub API 获取下载链接
	u.storeStatus(true, 30, fmt.Sprintf("正在下载版本 %s...", latestVersion), latestVersion)

	release, err := u.getLatestRelease(ctx)
	if err != nil {
		u.storeStatus(false, 0, "获取下载链接失败: "+err.Error(), latestVersion)
		return err
	}

	// 查找当前平台的二进制文件
	asset, err := u.getAssetForCurrentPlatform(release)
	if err != nil {
		u.storeStatus(false, 0, err.Error(), latestVersion)
		return err
	}

	u.logger.Info("📦 开始下载更新...",
		zap.String("from_version", u.currentVersion),
		zap.String("to_version", latestVersion),
		zap.String("asset", asset.Name),
		zap.String("url", asset.BrowserDownloadURL))

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		u.storeStatus(false, 0, "创建下载请求失败: "+err.Error(), latestVersion)
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "ytb2bili-updater")
	if u.githubAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.githubAPIToken)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		u.storeStatus(false, 0, "下载更新失败: "+err.Error(), latestVersion)
		return fmt.Errorf("下载更新失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.storeStatus(false, 0, fmt.Sprintf("下载更新失败: HTTP %d", resp.StatusCode), latestVersion)
		return fmt.Errorf("下载更新失败: HTTP %d", resp.StatusCode)
	}

	binaryReader, err := u.extractBinaryFromAsset(asset.Name, resp.Body)
	if err != nil {
		u.storeStatus(false, 0, "解析更新包失败: "+err.Error(), latestVersion)
		return fmt.Errorf("解析更新包失败: %w", err)
	}

	// 应用更新
	u.storeStatus(true, 70, "正在应用更新...", latestVersion)

	// 使用 selfupdate 应用更新
	err = selfupdate.Apply(binaryReader, selfupdate.Options{})
	if err != nil {
		// 回滚失败，尝试恢复
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			u.logger.Error("❌ 更新失败且回滚失败", zap.Error(err), zap.Error(rerr))
			u.storeStatus(false, 0, "更新失败且回滚失败: "+err.Error(), latestVersion)
			return fmt.Errorf("更新失败且回滚失败: %v, 回滚错误: %v", err, rerr)
		}
		u.logger.Warn("⚠️  更新失败，已回滚", zap.Error(err))
		u.storeStatus(false, 0, "更新失败，已回滚: "+err.Error(), latestVersion)
		return fmt.Errorf("更新失败，已回滚: %w", err)
	}

	u.currentVersion = latestVersion

	message := fmt.Sprintf("更新成功，当前版本 %s，请重启应用", latestVersion)
	if u.restartOnSuccess {
		message = fmt.Sprintf("更新成功，当前版本 %s，应用将在 %d 秒后自动重启", latestVersion, int(u.restartDelay/time.Second))
	}
	u.storeStatus(false, 100, message, latestVersion)
	if u.restartOnSuccess {
		u.scheduleRestart()
	}

	u.logger.Info("✅ 更新成功！请重启应用以应用更新")
	return nil
}

func (u *Updater) scheduleRestart() {
	if !u.restartScheduled.CompareAndSwap(false, true) {
		return
	}

	delay := u.restartDelay
	u.logger.Info("更新完成，准备自动重启进程",
		zap.Duration("delay", delay),
		zap.String("version", u.currentVersion))

	go func() {
		time.Sleep(delay)
		u.logger.Info("触发自动重启，由外部进程管理器接管拉起", zap.String("version", u.currentVersion))
		u.exitFunc(0)
	}()
}

func (u *Updater) getAssetForCurrentPlatform(release *GitHubRelease) (*struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}, error) {
	for _, expected := range getCandidateAssetNames() {
		for _, asset := range release.Assets {
			if asset.Name == expected {
				u.logger.Info("找到匹配的资源", zap.String("asset", asset.Name), zap.String("url", asset.BrowserDownloadURL))
				return &asset, nil
			}
		}
	}

	assetNames := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		assetNames = append(assetNames, asset.Name)
	}

	return nil, fmt.Errorf("未找到适用于 %s/%s 的更新包，可用资源: %s", runtime.GOOS, runtime.GOARCH, strings.Join(assetNames, ", "))
}

func getCandidateAssetNames() []string {
	baseName := fmt.Sprintf("ytb2bili-%s-%s", runtime.GOOS, runtime.GOARCH)
	switch runtime.GOOS {
	case "windows":
		return []string{baseName + ".zip", baseName + ".exe"}
	default:
		return []string{baseName + ".tar.gz", baseName + ".tgz", baseName}
	}
}

func targetBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ytb2bili.exe"
	}
	return "ytb2bili"
}

func getCandidateBinaryNames(assetName string) []string {
	baseName := strings.TrimSuffix(strings.TrimSuffix(assetName, ".tar.gz"), ".tgz")
	baseName = strings.TrimSuffix(baseName, ".zip")

	candidates := []string{}
	seen := map[string]struct{}{}
	appendCandidate := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		candidates = append(candidates, trimmed)
	}

	appendCandidate(baseName)
	appendCandidate(baseName + ".exe")
	appendCandidate(fmt.Sprintf("ytb2bili-%s-%s", runtime.GOOS, runtime.GOARCH))
	appendCandidate(targetBinaryName())
	appendCandidate("ytb2bili.exe")

	if runtime.GOOS == "windows" {
		appendCandidate(fmt.Sprintf("ytb2bili-%s-%s.exe", runtime.GOOS, runtime.GOARCH))
	}

	return candidates
}

func (u *Updater) extractBinaryFromAsset(assetName string, body io.Reader) (io.Reader, error) {
	targets := getCandidateBinaryNames(assetName)

	switch {
	case strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz"):
		return extractBinaryFromTarGz(body, targets)
	case strings.HasSuffix(assetName, ".zip"):
		return extractBinaryFromZip(body, targets)
	default:
		return body, nil
	}
}

func extractBinaryFromTarGz(body io.Reader, targets []string) (io.Reader, error) {
	gzr, err := gzip.NewReader(body)
	if err != nil {
		return nil, fmt.Errorf("解压 tar.gz 失败: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取 tar 包失败: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if !matchesBinaryCandidate(header.Name, targets) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("读取更新二进制失败: %w", err)
		}
		return bytes.NewReader(data), nil
	}

	return nil, fmt.Errorf("tar.gz 中未找到可执行文件，候选: %s", strings.Join(targets, ", "))
}

func extractBinaryFromZip(body io.Reader, targets []string) (io.Reader, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("读取 zip 数据失败: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开 zip 失败: %w", err)
	}

	for _, file := range zr.File {
		if !matchesBinaryCandidate(file.Name, targets) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("打开 zip 内文件失败: %w", err)
		}
		defer rc.Close()

		binaryData, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("读取 zip 内二进制失败: %w", err)
		}
		return bytes.NewReader(binaryData), nil
	}

	return nil, fmt.Errorf("zip 中未找到可执行文件，候选: %s", strings.Join(targets, ", "))
}

func matchesBinaryCandidate(entryName string, targets []string) bool {
	base := filepath.Base(entryName)
	for _, target := range targets {
		if base == target {
			return true
		}
	}
	return false
}

// StartAutoUpdateCheck 启动自动更新检查
func (u *Updater) StartAutoUpdateCheck(ctx context.Context) {
	if u.checkInterval == 0 {
		u.logger.Info("⏸️  自动更新检查已禁用")
		return
	}

	u.logger.Info("🚀 启动自动更新检查",
		zap.Duration("interval", u.checkInterval),
		zap.Bool("auto_update", u.autoUpdate))

	ticker := time.NewTicker(u.checkInterval)
	defer ticker.Stop()

	// 立即执行一次检查
	u.checkAndUpdate(ctx)

	for {
		select {
		case <-ctx.Done():
			u.logger.Info("⏹️  自动更新检查已停止")
			return
		case <-ticker.C:
			u.checkAndUpdate(ctx)
		}
	}
}

// checkAndUpdate 检查并执行更新
func (u *Updater) checkAndUpdate(ctx context.Context) {
	hasUpdate, _, err := u.CheckForUpdates(ctx)
	if err != nil {
		if isNonFatalReleaseCheckError(err) {
			u.logger.Warn("更新检查暂时不可用，跳过本轮", zap.Error(err))
			u.storeStatus(false, 0, "更新检查暂时不可用: "+err.Error(), "")
			return
		}
		u.logger.Error("检查更新失败", zap.Error(err))
		u.storeStatus(false, 0, "检查更新失败: "+err.Error(), "")
		return
	}

	if !hasUpdate {
		return
	}

	if u.autoUpdate {
		u.logger.Info("🔄 自动更新已启用，开始下载...")
		if err := u.DoUpdate(ctx); err != nil {
			u.logger.Error("自动更新失败", zap.Error(err))
		}
	} else {
		u.logger.Info("💡 发现新版本，但自动更新已禁用。请手动更新或启用自动更新")
	}
}

// GetCurrentVersion 获取当前版本
func (u *Updater) GetCurrentVersion() string {
	return u.currentVersion
}

// IsAutoUpdateEnabled 是否启用自动更新
func (u *Updater) IsAutoUpdateEnabled() bool {
	return u.autoUpdate
}

// IsRestartOnSuccessEnabled 是否启用更新成功后自动重启
func (u *Updater) IsRestartOnSuccessEnabled() bool {
	return u.restartOnSuccess
}

// RestartDelay 获取自动重启等待时长
func (u *Updater) RestartDelay() time.Duration {
	return u.restartDelay
}

// GetUpdateStatus 获取更新状态
func (u *Updater) GetUpdateStatus() *UpdateStatus {
	status := u.updateStatus.Load()
	if status == nil {
		return &UpdateStatus{
			Updating:       false,
			Progress:       0,
			Message:        "就绪",
			CurrentVersion: u.currentVersion,
		}
	}
	return status.(*UpdateStatus)
}
