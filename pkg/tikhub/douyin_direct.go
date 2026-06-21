package tikhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	directResolveTimeout = 30 * time.Second
	directDLChunkSize    = int64(5 * 1024 * 1024) // 5 MB
	directDLWorkers      = 8
	directDLMaxRetries   = 3
	directDLRetryDelay   = 2 * time.Second
	directDLTimeout      = 120 * time.Second
	directDouyinMobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/121.0.2277.107 Version/17.0 Mobile/15E148 Safari/604.1"
)

// ─── DirectResolver ────────────────────────────────────────────────────────────

// DirectResolver resolves Douyin share URLs by scraping iesdouyin.com directly,
// without requiring a TikHub API key. The no-watermark video URL is obtained by
// replacing "playwm" with "play" in the page's play_addr.url_list.
type DirectResolver struct {
	http   *http.Client
	logger *zap.Logger
}

func NewDirectResolver(logger *zap.Logger) *DirectResolver {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DirectResolver{
		http:   &http.Client{Timeout: directResolveTimeout},
		logger: logger,
	}
}

// Resolve implements tikhub.Resolver.
func (r *DirectResolver) Resolve(ctx context.Context, input string) (*DouyinVideoInfo, error) {
	shareURL, err := parseDouyinShareURLInput(input)
	if err != nil {
		return nil, err
	}

	// Expand short URLs (v.douyin.com → real page URL)
	realURL := shareURL
	if strings.Contains(shareURL, "v.douyin.com") {
		realURL, err = r.expandShortURL(ctx, shareURL)
		if err != nil {
			return nil, fmt.Errorf("expand short url: %w", err)
		}
		r.logger.Debug("expanded short url", zap.String("from", shareURL), zap.String("to", realURL))
	}

	awemeID := extractDirectAwemeID(realURL)
	if awemeID == "" {
		return nil, fmt.Errorf("cannot extract aweme id from url: %s", realURL)
	}

	info, err := r.scrapeIesDY(ctx, awemeID, shareURL)
	if err != nil {
		return nil, fmt.Errorf("scrape iesdouyin.com: %w", err)
	}
	r.logger.Info("Douyin video resolved (direct)",
		zap.String("aweme_id", info.Data.AwemeID),
		zap.String("title", info.Data.Desc),
	)
	return info, nil
}

func (r *DirectResolver) expandShortURL(ctx context.Context, shortURL string) (string, error) {
	client := &http.Client{
		Timeout: directResolveTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shortURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", directDouyinMobileUA)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			return loc, nil
		}
	}
	return req.URL.String(), nil
}

func extractDirectAwemeID(rawURL string) string {
	re := regexp.MustCompile(`/video/(\d+)`)
	if m := re.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1]
	}
	u, err := url.Parse(rawURL)
	if err == nil {
		if id := u.Query().Get("modal_id"); id != "" {
			return id
		}
	}
	return ""
}

func (r *DirectResolver) scrapeIesDY(ctx context.Context, awemeID, originalShareURL string) (*DouyinVideoInfo, error) {
	scrapeURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s", awemeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scrapeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", directDouyinMobileUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract window._ROUTER_DATA = {...}</script>
	routerRe := regexp.MustCompile(`window\._ROUTER_DATA\s*=\s*(.*?)</script>`)
	matches := routerRe.FindSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("window._ROUTER_DATA not found in page")
	}

	var routerData struct {
		LoaderData map[string]json.RawMessage `json:"loaderData"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(matches[1]), &routerData); err != nil {
		return nil, fmt.Errorf("parse _ROUTER_DATA: %w", err)
	}

	var pageRaw json.RawMessage
	for _, key := range []string{"video_(id)/page", "note_(id)/page"} {
		if v, ok := routerData.LoaderData[key]; ok {
			pageRaw = v
			break
		}
	}
	if pageRaw == nil {
		return nil, fmt.Errorf("video page key not found in loaderData")
	}

	var page struct {
		VideoInfoRes struct {
			ItemList []struct {
				AwemeID string `json:"aweme_id"`
				Desc    string `json:"desc"`
				Author  struct {
					UID      string `json:"uid"`
					Nickname string `json:"nickname"`
					SecUID   string `json:"sec_uid"`
				} `json:"author"`
				Video struct {
					Cover struct {
						URLList []string `json:"url_list"`
					} `json:"cover"`
					PlayAddr struct {
						URLList []string `json:"url_list"`
					} `json:"play_addr"`
				} `json:"video"`
				CreateTime int64 `json:"create_time"`
			} `json:"item_list"`
		} `json:"videoInfoRes"`
	}
	if err := json.Unmarshal(pageRaw, &page); err != nil {
		return nil, fmt.Errorf("parse video page data: %w", err)
	}
	if len(page.VideoInfoRes.ItemList) == 0 {
		return nil, fmt.Errorf("item_list is empty in page data")
	}

	item := page.VideoInfoRes.ItemList[0]

	// No-watermark URL: replace playwm with play
	var videoURL string
	if len(item.Video.PlayAddr.URLList) > 0 {
		videoURL = strings.ReplaceAll(item.Video.PlayAddr.URLList[0], "playwm", "play")
	}

	var coverURL string
	if len(item.Video.Cover.URLList) > 0 {
		coverURL = item.Video.Cover.URLList[0]
	}

	resolvedID := strings.TrimSpace(item.AwemeID)
	if resolvedID == "" {
		resolvedID = awemeID
	}

	return &DouyinVideoInfo{
		Code: 0,
		Data: DouyinAwemeDetail{
			AwemeID:    resolvedID,
			Desc:       strings.TrimSpace(item.Desc),
			ShareURL:   fmt.Sprintf("https://www.douyin.com/video/%s", resolvedID),
			CreateTime: item.CreateTime,
			Author: DouyinAuthorInfo{
				UID:      item.Author.UID,
				Nickname: item.Author.Nickname,
				SecUID:   item.Author.SecUID,
			},
			Video: DouyinVideoData{
				Cover: DouyinImage{URLList: []string{coverURL}},
				PlayAddr: DouyinMediaURL{
					URLList: []string{videoURL},
				},
			},
		},
	}, nil
}

// ─── DirectDownloader ─────────────────────────────────────────────────────────

// DirectDownloader downloads Douyin videos using HTTP Range-based concurrent
// chunking with resume support, without requiring a TikHub API key.
// Pass any Resolver implementation to resolve share URLs.
type DirectDownloader struct {
	resolver Resolver
	logger   *zap.Logger
}

func NewDirectDownloader(resolver Resolver, logger *zap.Logger) *DirectDownloader {
	if logger == nil {
		logger = zap.NewNop()
	}
	var r Resolver
	if resolver != nil {
		r = resolver
	}
	return &DirectDownloader{resolver: r, logger: logger}
}

// Download implements tikhub.Downloader.
func (d *DirectDownloader) Download(ctx context.Context, req DownloadRequest) (*DownloadResult, error) {
	info, err := d.resolveVideoInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	videoURL, err := selectBestDouyinVideoURL(info)
	if err != nil {
		return nil, err
	}

	videoID := resolveDouyinDownloadID(req.ShareURL, info.Data.ShareURL, info.Data.AwemeID, videoURL)
	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("output_dir is required")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	fileName := normalizeDouyinFileName(req.FileName, videoID)
	filePath := filepath.Join(outputDir, fileName)
	if !filepath.IsAbs(filePath) {
		if absPath, absErr := filepath.Abs(filePath); absErr == nil {
			filePath = absPath
		}
	}

	dl := &directChunkDL{
		url:             videoURL,
		outputFile:      filePath,
		stateFile:       filePath + ".dl.json",
		numWorkers:      directDLWorkers,
		chunkSize:       directDLChunkSize,
		completedChunks: make(map[int]bool),
		logger:          d.logger,
	}
	if err := dl.run(ctx); err != nil {
		return nil, fmt.Errorf("download douyin video: %w", err)
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat downloaded file: %w", err)
	}

	result := &DownloadResult{
		VideoID:          videoID,
		Title:            strings.TrimSpace(info.Data.Desc),
		FilePath:         filePath,
		FileName:         filepath.Base(filePath),
		FileSize:         stat.Size(),
		URL:              videoURL,
		ResolvedShareURL: strings.TrimSpace(info.Data.ShareURL),
	}
	d.logger.Info("Douyin video downloaded (direct)",
		zap.String("video_id", result.VideoID),
		zap.String("path", result.FilePath),
		zap.Int64("size", result.FileSize),
	)
	return result, nil
}

func (d *DirectDownloader) resolveVideoInfo(ctx context.Context, req DownloadRequest) (*DouyinVideoInfo, error) {
	if req.VideoInfo != nil {
		return req.VideoInfo, nil
	}
	if strings.TrimSpace(req.VideoInfoRaw) != "" {
		return parseDouyinVideoInfoRaw(req.VideoInfoRaw)
	}
	if strings.TrimSpace(req.ShareURL) == "" {
		return nil, fmt.Errorf("share_url or video_info is required")
	}
	if d.resolver == nil {
		return nil, fmt.Errorf("share_url download requires a resolver")
	}
	return d.resolver.Resolve(ctx, req.ShareURL)
}

// ─── directChunkDL ────────────────────────────────────────────────────────────

type directChunkState struct {
	OutputFile      string       `json:"output_file"`
	TotalSize       int64        `json:"total_size"`
	ChunkSize       int64        `json:"chunk_size"`
	CompletedChunks map[int]bool `json:"completed_chunks"`
}

type directChunkDL struct {
	url             string
	outputFile      string
	stateFile       string
	numWorkers      int
	chunkSize       int64
	totalSize       int64
	downloaded      int64
	completedChunks map[int]bool
	mu              sync.Mutex
	logger          *zap.Logger
}

func (d *directChunkDL) run(ctx context.Context) error {
	_ = d.loadState()

	if d.totalSize == 0 {
		if err := d.fetchContentLength(ctx); err != nil {
			// Server doesn't support Range — fall back to single-thread stream
			d.logger.Debug("server doesn't support Range, using stream download")
			return d.streamDownload(ctx)
		}
	}

	chunks := d.calcChunks()
	var pending []int
	for _, idx := range chunks {
		if !d.completedChunks[idx] {
			pending = append(pending, idx)
		}
	}

	if len(pending) == 0 {
		return d.mergeChunks(len(chunks))
	}

	d.logger.Info("starting chunked download",
		zap.Float64("size_mb", float64(d.totalSize)/1024/1024),
		zap.Int("chunks", len(chunks)),
		zap.Int("workers", d.numWorkers),
		zap.Int("pending", len(pending)),
	)

	chunkCh := make(chan int, len(pending))
	errCh := make(chan error, len(pending))
	var wg sync.WaitGroup

	for i := 0; i < d.numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range chunkCh {
				if err := d.downloadChunk(ctx, idx); err != nil {
					errCh <- fmt.Errorf("chunk %d: %w", idx, err)
				}
			}
		}()
	}

	for _, idx := range pending {
		chunkCh <- idx
	}
	close(chunkCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}

	if err := d.mergeChunks(len(chunks)); err != nil {
		return fmt.Errorf("merge chunks: %w", err)
	}
	d.removeState()
	return nil
}

func (d *directChunkDL) fetchContentLength(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, d.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", directDouyinMobileUA)
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength <= 0 {
		return fmt.Errorf("invalid Content-Length: %d", resp.ContentLength)
	}
	d.totalSize = resp.ContentLength
	return nil
}

func (d *directChunkDL) calcChunks() []int {
	n := int(d.totalSize / d.chunkSize)
	if d.totalSize%d.chunkSize != 0 {
		n++
	}
	chunks := make([]int, n)
	for i := range chunks {
		chunks[i] = i
	}
	return chunks
}

func (d *directChunkDL) downloadChunk(ctx context.Context, idx int) error {
	start := int64(idx) * d.chunkSize
	end := start + d.chunkSize - 1
	if end >= d.totalSize {
		end = d.totalSize - 1
	}
	tmpFile := fmt.Sprintf("%s.part%d", d.outputFile, idx)

	// Already complete?
	if fi, err := os.Stat(tmpFile); err == nil && fi.Size() == end-start+1 {
		d.markDone(idx, fi.Size())
		return nil
	}

	var lastErr error
	for retry := 0; retry <= directDLMaxRetries; retry++ {
		if retry > 0 {
			time.Sleep(directDLRetryDelay * time.Duration(retry))
		}
		if err := d.downloadChunkOnce(ctx, idx, start, end, tmpFile); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("failed after %d retries: %w", directDLMaxRetries, lastErr)
}

func (d *directChunkDL) downloadChunkOnce(ctx context.Context, idx int, start, end int64, tmpFile string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", directDouyinMobileUA)
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := (&http.Client{Timeout: directDLTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	expect := end - start + 1
	if written != expect {
		return fmt.Errorf("size mismatch: expected %d got %d", expect, written)
	}
	d.markDone(idx, written)
	return nil
}

func (d *directChunkDL) markDone(idx int, size int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.completedChunks[idx] {
		d.completedChunks[idx] = true
		atomic.AddInt64(&d.downloaded, size)
		_ = d.saveState()
	}
}

func (d *directChunkDL) mergeChunks(n int) error {
	out, err := os.Create(d.outputFile)
	if err != nil {
		return err
	}
	defer out.Close()
	for i := 0; i < n; i++ {
		part := fmt.Sprintf("%s.part%d", d.outputFile, i)
		f, err := os.Open(part)
		if err != nil {
			return fmt.Errorf("open part%d: %w", i, err)
		}
		_, copyErr := io.Copy(out, f)
		f.Close()
		if copyErr != nil {
			return fmt.Errorf("write part%d: %w", i, copyErr)
		}
		os.Remove(part)
	}
	return nil
}

func (d *directChunkDL) streamDownload(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", directDouyinMobileUA)
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := (&http.Client{Timeout: directDLTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(d.outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (d *directChunkDL) saveState() error {
	state := directChunkState{
		OutputFile:      d.outputFile,
		TotalSize:       d.totalSize,
		ChunkSize:       d.chunkSize,
		CompletedChunks: d.completedChunks,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.stateFile, data, 0o644)
}

func (d *directChunkDL) loadState() error {
	data, err := os.ReadFile(d.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var state directChunkState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.OutputFile != d.outputFile {
		return nil // stale state from a different run
	}
	d.totalSize = state.TotalSize
	d.chunkSize = state.ChunkSize
	d.completedChunks = state.CompletedChunks

	// Rebuild downloaded byte count
	for idx := range d.completedChunks {
		start := int64(idx) * d.chunkSize
		end := start + d.chunkSize - 1
		if end >= d.totalSize {
			end = d.totalSize - 1
		}
		d.downloaded += end - start + 1
	}
	d.logger.Debug("resume download", zap.Int("completed_chunks", len(d.completedChunks)))
	return nil
}

func (d *directChunkDL) removeState() {
	os.Remove(d.stateFile)
}
