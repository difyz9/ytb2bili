package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// https://github.com/JefferyHcool/BiliNote/blob/master/backend/app/transcriber/bcut.py

const (
	bcutAPIBaseURL        = "https://member.bilibili.com/x/bcut/rubick-interface"
	bcutAPIReqUpload      = bcutAPIBaseURL + "/resource/create"
	bcutAPICommitUpload   = bcutAPIBaseURL + "/resource/create/complete"
	bcutAPICreateTask     = bcutAPIBaseURL + "/task"
	bcutAPIQueryResult    = bcutAPIBaseURL + "/task/result"
	bcutModelID           = "8"
	bcutUploadPartRetries = 3
	bcutUploadRetryDelay  = 2 * time.Second
)

// BcutTranscriberTool 必剪语音识别工具
// 使用 Bilibili BCut API 进行音频转文字
type BcutTranscriberTool struct {
	name   string
	desc   string
	client *http.Client
	logger *zap.Logger
}

// Name returns the tool name.
func (t *BcutTranscriberTool) Name() string { return t.name }

// Description returns the tool description.
func (t *BcutTranscriberTool) Description() string { return t.desc }

// TranscriptSegment 转录片段
type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptResult 转录结果
type TranscriptResult struct {
	Language string              `json:"language"`
	FullText string              `json:"full_text"`
	Segments []TranscriptSegment `json:"segments"`
	SRTPath  string              `json:"srt_path,omitempty"` // SRT 字幕文件路径
}

// uploadResponse 上传响应
type uploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		InBossKey  string   `json:"in_boss_key"`
		ResourceID string   `json:"resource_id"`
		UploadID   string   `json:"upload_id"`
		UploadURLs []string `json:"upload_urls"`
		PerSize    int      `json:"per_size"`
		Size       int      `json:"size"`
	} `json:"data"`
}

// commitResponse 提交响应
type commitResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		DownloadURL string `json:"download_url"`
	} `json:"data"`
}

// taskResponse 任务响应
type taskResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

// queryResponse 查询响应
type queryResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		State  int    `json:"state"`
		Result string `json:"result"`
	} `json:"data"`
}

// utterance BCut API 返回的语句结构
type utterance struct {
	Transcript string  `json:"transcript"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
}

// bcutResult BCut API 返回的完整结果
type bcutResult struct {
	Language   string      `json:"language"`
	Utterances []utterance `json:"utterances"`
}

// NewBcutTranscriberTool 创建必剪语音识别工具
func NewBcutTranscriberTool(logger *zap.Logger) *BcutTranscriberTool {
	return &BcutTranscriberTool{
		name: "bcut_transcriber",
		desc: `使用 Bilibili 必剪 API 进行音频转文字。
功能：将音频文件转换为文字，支持中文和英文，返回带时间戳的分段结果。
输入格式：音频文件路径（字符串）
返回：JSON 格式的转录结果，包含完整文本和分段信息

示例输入: "/path/to/audio.mp3"
返回: {"language":"zh","full_text":"转录的完整文本","segments":[{"start":0.0,"end":1.5,"text":"第一段"}]}

支持的音频格式: mp3, wav, aac
注意：
1. 需要网络连接到 Bilibili API
2. 大文件会自动分片上传
3. 转录可能需要一段时间，工具会自动等待结果`,
		client: &http.Client{
			Timeout: 300 * time.Second, // 5分钟超时
		},
		logger: logger,
	}
}

// Call 执行音频转文字
func (t *BcutTranscriberTool) Call(ctx context.Context, input string) (string, error) {
	filePath := strings.TrimSpace(input)
	if filePath == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("audio file not found: %w", err)
	}

	t.logger.Info("Starting BCut transcription",
		zap.String("file", filePath))

	// 读取文件
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 1. 申请上传
	t.logger.Info("Requesting upload...")
	uploadResp, err := t.requestUpload(ctx, fileData)
	if err != nil {
		return "", fmt.Errorf("request upload failed: %w", err)
	}

	// 2. 分片上传
	t.logger.Info("Uploading file parts...",
		zap.Int("parts", len(uploadResp.Data.UploadURLs)),
		zap.Int("size_kb", uploadResp.Data.Size/1024))
	etags, err := t.uploadParts(ctx, fileData, uploadResp)
	if err != nil {
		return "", fmt.Errorf("upload parts failed: %w", err)
	}

	// 3. 提交上传
	t.logger.Info("Committing upload...")
	downloadURL, err := t.commitUpload(ctx, uploadResp, etags)
	if err != nil {
		return "", fmt.Errorf("commit upload failed: %w", err)
	}

	// 4. 创建转录任务
	t.logger.Info("Creating transcription task...")
	taskID, err := t.createTask(ctx, downloadURL)
	if err != nil {
		return "", fmt.Errorf("create task failed: %w", err)
	}

	// 5. 轮询查询结果
	t.logger.Info("Waiting for transcription result...",
		zap.String("task_id", taskID))
	result, err := t.queryResult(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("query result failed: %w", err)
	}

	// 6. 保存 SRT 字幕文件
	if len(result.Segments) > 0 {
		srtPath, err := t.saveSRT(filePath, result)
		if err != nil {
			t.logger.Warn("Failed to save SRT file", zap.Error(err))
		} else {
			result.SRTPath = srtPath
			t.logger.Info("SRT file saved", zap.String("path", srtPath))
		}
	}

	// 转换为 JSON 字符串返回
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result failed: %w", err)
	}

	t.logger.Info("Transcription completed",
		zap.String("language", result.Language),
		zap.Int("segments", len(result.Segments)))

	return string(resultJSON), nil
}

// requestUpload 申请上传
func (t *BcutTranscriberTool) requestUpload(ctx context.Context, fileData []byte) (*uploadResponse, error) {
	payload := map[string]interface{}{
		"type":             2,
		"name":             "audio.mp3",
		"size":             len(fileData),
		"ResourceFileType": "mp3",
		"model_id":         bcutModelID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bcutAPIReqUpload, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var uploadResp uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, err
	}

	if uploadResp.Code != 0 {
		return nil, fmt.Errorf("upload request failed: %s", uploadResp.Msg)
	}

	return &uploadResp, nil
}

// uploadParts 分片上传文件
func (t *BcutTranscriberTool) uploadParts(ctx context.Context, fileData []byte, uploadResp *uploadResponse) ([]string, error) {
	var etags []string
	perSize := uploadResp.Data.PerSize
	totalParts := len(uploadResp.Data.UploadURLs)

	for i := 0; i < totalParts; i++ {
		start := i * perSize
		end := (i + 1) * perSize
		if end > len(fileData) {
			end = len(fileData)
		}

		t.logger.Info("Uploading part",
			zap.Int("part", i+1),
			zap.Int("total", totalParts),
			zap.Int("start", start),
			zap.Int("end", end))

		etag, err := t.uploadPartWithRetry(ctx, uploadResp.Data.UploadURLs[i], fileData[start:end], i, totalParts)
		if err != nil {
			return nil, err
		}
		etags = append(etags, etag)

		t.logger.Info("Part uploaded successfully",
			zap.Int("part", i+1),
			zap.String("etag", etag))
	}

	return etags, nil
}

func (t *BcutTranscriberTool) uploadPartWithRetry(ctx context.Context, uploadURL string, chunk []byte, partIndex, totalParts int) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= bcutUploadPartRetries; attempt++ {
		etag, err := t.uploadSinglePart(ctx, uploadURL, chunk, partIndex)
		if err == nil {
			return etag, nil
		}

		lastErr = err
		if attempt == bcutUploadPartRetries || !isRetryableUploadPartError(err) {
			break
		}

		if t.logger != nil {
			t.logger.Warn("Upload part failed, retrying",
				zap.Int("part", partIndex+1),
				zap.Int("total", totalParts),
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", bcutUploadPartRetries),
				zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * bcutUploadRetryDelay):
		}
	}

	return "", fmt.Errorf("upload part %d failed after %d attempts: %w", partIndex, bcutUploadPartRetries, lastErr)
}

func (t *BcutTranscriberTool) uploadSinglePart(ctx context.Context, uploadURL string, chunk []byte, partIndex int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(chunk))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(bodyBytes))
		if message != "" {
			return "", fmt.Errorf("upload part %d failed: status %d body %s", partIndex, resp.StatusCode, message)
		}
		return "", fmt.Errorf("upload part %d failed: status %d", partIndex, resp.StatusCode)
	}

	return strings.Trim(resp.Header.Get("Etag"), "\""), nil
}

func isRetryableUploadPartError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	message := err.Error()
	return strings.Contains(message, "status 408") ||
		strings.Contains(message, "status 409") ||
		strings.Contains(message, "status 425") ||
		strings.Contains(message, "status 429") ||
		strings.Contains(message, "status 500") ||
		strings.Contains(message, "status 502") ||
		strings.Contains(message, "status 503") ||
		strings.Contains(message, "status 504") ||
		strings.Contains(strings.ToLower(message), "timeout") ||
		strings.Contains(strings.ToLower(message), "connection reset")
}

// commitUpload 提交上传
func (t *BcutTranscriberTool) commitUpload(ctx context.Context, uploadResp *uploadResponse, etags []string) (string, error) {
	payload := map[string]interface{}{
		"InBossKey":  uploadResp.Data.InBossKey,
		"ResourceId": uploadResp.Data.ResourceID,
		"Etags":      strings.Join(etags, ","),
		"UploadId":   uploadResp.Data.UploadID,
		"model_id":   bcutModelID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bcutAPICommitUpload, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var commitResp commitResponse
	if err := json.NewDecoder(resp.Body).Decode(&commitResp); err != nil {
		return "", err
	}

	if commitResp.Code != 0 {
		return "", fmt.Errorf("commit upload failed: %s", commitResp.Msg)
	}

	return commitResp.Data.DownloadURL, nil
}

// createTask 创建转录任务
func (t *BcutTranscriberTool) createTask(ctx context.Context, downloadURL string) (string, error) {
	payload := map[string]interface{}{
		"resource": downloadURL,
		"model_id": bcutModelID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bcutAPICreateTask, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var taskResp taskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		return "", err
	}

	if taskResp.Code != 0 {
		return "", fmt.Errorf("create task failed: %s", taskResp.Msg)
	}

	return taskResp.Data.TaskID, nil
}

// queryResult 轮询查询转录结果
func (t *BcutTranscriberTool) queryResult(ctx context.Context, taskID string) (*TranscriptResult, error) {
	maxRetries := 500
	for i := 0; i < maxRetries; i++ {
		// 每隔一段时间打印进度
		if i%10 == 0 && i > 0 {
			t.logger.Info("Transcription in progress...",
				zap.Int("attempt", i),
				zap.Int("max", maxRetries),
				zap.String("task_id", taskID))
		}

		req, err := http.NewRequestWithContext(ctx, "GET", bcutAPIQueryResult, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Add("model_id", bcutModelID)
		q.Add("task_id", taskID)
		req.URL.RawQuery = q.Encode()

		req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
		req.Header.Set("Content-Type", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			return nil, err
		}

		var queryResp queryResponse
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err := json.Unmarshal(bodyBytes, &queryResp); err != nil {
			return nil, err
		}

		if queryResp.Code != 0 {
			return nil, fmt.Errorf("query result failed: %s", queryResp.Msg)
		}

		// state: 4 = 完成, 3 = 失败
		if queryResp.Data.State == 4 {
			// 解析结果
			var bcutRes bcutResult
			if err := json.Unmarshal([]byte(queryResp.Data.Result), &bcutRes); err != nil {
				return nil, fmt.Errorf("parse result failed: %w", err)
			}

			// 转换为 TranscriptResult
			result := &TranscriptResult{
				Language: bcutRes.Language,
				Segments: make([]TranscriptSegment, 0, len(bcutRes.Utterances)),
			}

			var fullText strings.Builder
			for _, u := range bcutRes.Utterances {
				text := strings.TrimSpace(u.Transcript)
				if text != "" {
					fullText.WriteString(text)
					fullText.WriteString(" ")

					// BCut 返回的时间戳是毫秒，需要转换为秒
					result.Segments = append(result.Segments, TranscriptSegment{
						Start: u.StartTime / 1000.0,
						End:   u.EndTime / 1000.0,
						Text:  text,
					})
				}
			}

			result.FullText = strings.TrimSpace(fullText.String())
			return result, nil
		} else if queryResp.Data.State == 3 {
			return nil, fmt.Errorf("transcription task failed, state: %d", queryResp.Data.State)
		}

		if i%10 == 0 {
			t.logger.Debug("BCut task state polled",
				zap.String("task_id", taskID),
				zap.Int("state", queryResp.Data.State))
		}

		// 等待 1 秒后重试，同时响应上游取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return nil, fmt.Errorf("transcription timeout after %d retries", maxRetries)
}

// saveSRT 保存 SRT 字幕文件
func (t *BcutTranscriberTool) saveSRT(audioPath string, result *TranscriptResult) (string, error) {
	// 生成 SRT 文件路径（与音频文件同目录）
	srtPath := strings.TrimSuffix(audioPath, ".mp3") + ".srt"

	// 转换为 SRT 格式
	srtContent := t.convertToSRT(result)

	// 保存文件
	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		return "", fmt.Errorf("write SRT file failed: %w", err)
	}

	return srtPath, nil
}

// convertToSRT 将转录结果转换为 SRT 格式
func (t *BcutTranscriberTool) convertToSRT(result *TranscriptResult) string {
	var builder strings.Builder

	for i, segment := range result.Segments {
		// 序号
		builder.WriteString(fmt.Sprintf("%d\n", i+1))

		// 时间码 (格式: HH:MM:SS,mmm --> HH:MM:SS,mmm)
		startTime := t.formatSRTTime(segment.Start)
		endTime := t.formatSRTTime(segment.End)
		builder.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))

		// 字幕文本
		builder.WriteString(segment.Text)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// formatSRTTime 将秒数转换为 SRT 时间格式 (HH:MM:SS,mmm)
func (t *BcutTranscriberTool) formatSRTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}
