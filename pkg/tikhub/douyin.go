package tikhub

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type Resolver interface {
	Resolve(context.Context, string) (*DouyinVideoInfo, error)
}

type Downloader interface {
	Download(context.Context, DownloadRequest) (*DownloadResult, error)
}

type DouyinVideoInfoInput struct {
	ShareURL string `json:"share_url"`
}

type DouyinVideoInfo struct {
	Code      int               `json:"code"`
	Message   string            `json:"message"`
	MessageZh string            `json:"message_zh"`
	Data      DouyinAwemeDetail `json:"data"`
}

type douyinVideoInfoEnvelope struct {
	Code      int                     `json:"code"`
	Message   string                  `json:"message"`
	MessageZh string                  `json:"message_zh"`
	Data      douyinVideoDataEnvelope `json:"data"`
}

type douyinVideoDataEnvelope struct {
	AwemeID     string            `json:"aweme_id"`
	Desc        string            `json:"desc"`
	ShareURL    string            `json:"share_url"`
	CreateTime  int64             `json:"create_time"`
	Author      DouyinAuthorInfo  `json:"author"`
	Video       DouyinVideoData   `json:"video"`
	Statistics  DouyinStatistics  `json:"statistics"`
	AwemeDetail DouyinAwemeDetail `json:"aweme_detail"`
}

type DouyinAwemeDetail struct {
	AwemeID    string           `json:"aweme_id"`
	Desc       string           `json:"desc"`
	ShareURL   string           `json:"share_url"`
	CreateTime int64            `json:"create_time"`
	Author     DouyinAuthorInfo `json:"author"`
	Video      DouyinVideoData  `json:"video"`
	Statistics DouyinStatistics `json:"statistics"`
}

type DouyinAuthorInfo struct {
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	SecUID   string `json:"sec_uid"`
}

type DouyinVideoData struct {
	VideoID      string              `json:"video_id"`
	Duration     int                 `json:"duration"`
	PlayAddr     DouyinMediaURL      `json:"play_addr"`
	PlayAddrH264 DouyinMediaURL      `json:"play_addr_h264"`
	PlayAddr265  DouyinMediaURL      `json:"play_addr_265"`
	DownloadAddr DouyinMediaURL      `json:"download_addr"`
	BitRate      []DouyinBitRateInfo `json:"bit_rate"`
	Cover        DouyinImage         `json:"cover"`
	DynamicCover DouyinImage         `json:"dynamic_cover"`
	OriginCover  DouyinImage         `json:"origin_cover"`
}

type DouyinBitRateInfo struct {
	BitRate     int            `json:"bit_rate"`
	QualityType int            `json:"quality_type"`
	GearName    string         `json:"gear_name"`
	PlayAddr    DouyinMediaURL `json:"play_addr"`
}

type DouyinMediaURL struct {
	URI      string   `json:"uri"`
	URLList  []string `json:"url_list"`
	DataSize int64    `json:"data_size"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
}

type DouyinImage struct {
	URI     string   `json:"uri"`
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type DouyinStatistics struct {
	DiggCount    int `json:"digg_count"`
	CommentCount int `json:"comment_count"`
	ShareCount   int `json:"share_count"`
	PlayCount    int `json:"play_count"`
}

type DownloadRequest struct {
	ShareURL     string
	VideoInfo    *DouyinVideoInfo
	VideoInfoRaw string
	FileName     string
	OutputDir    string
}

type DownloadResult struct {
	VideoID          string `json:"video_id"`
	Title            string `json:"title,omitempty"`
	FilePath         string `json:"file_path"`
	FileName         string `json:"file_name"`
	FileSize         int64  `json:"file_size"`
	URL              string `json:"url"`
	ResolvedShareURL string `json:"resolved_share_url,omitempty"`
}

func selectBestDouyinVideoURL(info *DouyinVideoInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("douyin video info is nil")
	}

	bestURL := ""
	bestBitRate := -1
	for _, item := range info.Data.Video.BitRate {
		if len(item.PlayAddr.URLList) == 0 {
			continue
		}
		if item.BitRate > bestBitRate {
			bestBitRate = item.BitRate
			bestURL = item.PlayAddr.URLList[0]
		}
	}
	if bestURL != "" {
		return bestURL, nil
	}

	for _, candidate := range []DouyinMediaURL{
		info.Data.Video.PlayAddr265,
		info.Data.Video.PlayAddrH264,
		info.Data.Video.PlayAddr,
		info.Data.Video.DownloadAddr,
	} {
		if len(candidate.URLList) > 0 {
			return candidate.URLList[0], nil
		}
	}

	return "", fmt.Errorf("no downloadable douyin video url found")
}

func normalizeDouyinFileName(fileName, videoID string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = videoID + ".mp4"
	}
	if filepath.Ext(name) == "" {
		name += ".mp4"
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return videoID + ".mp4"
	}
	return name
}

func resolveDouyinDownloadID(inputShareURL, resolvedShareURL, awemeID, videoURL string) string {
	for _, candidate := range []string{inputShareURL, resolvedShareURL} {
		if downloadID := extractDouyinDownloadID(candidate); downloadID != "" {
			return downloadID
		}
	}

	if awemeID = strings.TrimSpace(awemeID); awemeID != "" {
		return awemeID
	}

	hash := md5.Sum([]byte(videoURL))
	return fmt.Sprintf("video_%x", hash[:8])
}

func extractDouyinDownloadID(raw string) string {
	shareURL := strings.TrimSpace(raw)
	if shareURL == "" {
		return ""
	}

	parsed, err := url.Parse(shareURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Host)
	pathParts := splitDouyinPathSegments(parsed.Path)
	if len(pathParts) == 0 {
		return ""
	}

	switch {
	case strings.Contains(host, "v.douyin.com"):
		return sanitizeDouyinDownloadID(pathParts[0])
	case strings.Contains(host, "iesdouyin.com"):
		if len(pathParts) >= 3 && pathParts[0] == "share" && pathParts[1] == "video" {
			return sanitizeDouyinDownloadID(pathParts[2])
		}
	case strings.Contains(host, "douyin.com"):
		for index := 0; index < len(pathParts)-1; index++ {
			if pathParts[index] == "video" {
				return sanitizeDouyinDownloadID(pathParts[index+1])
			}
		}
		return sanitizeDouyinDownloadID(pathParts[len(pathParts)-1])
	}

	return ""
}

func splitDouyinPathSegments(path string) []string {
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

func sanitizeDouyinDownloadID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "/")
	value = strings.TrimRight(value, ".,，。；;!！?？)）]】>》")
	return value
}

func parseDouyinVideoInfoRaw(raw string) (*DouyinVideoInfo, error) {
	var envelope douyinVideoInfoEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, err
	}

	detail := envelope.Data.AwemeDetail
	if strings.TrimSpace(detail.AwemeID) == "" {
		detail = DouyinAwemeDetail{
			AwemeID:    envelope.Data.AwemeID,
			Desc:       envelope.Data.Desc,
			ShareURL:   envelope.Data.ShareURL,
			CreateTime: envelope.Data.CreateTime,
			Author:     envelope.Data.Author,
			Video:      envelope.Data.Video,
			Statistics: envelope.Data.Statistics,
		}
	}

	return &DouyinVideoInfo{
		Code:      envelope.Code,
		Message:   envelope.Message,
		MessageZh: envelope.MessageZh,
		Data:      detail,
	}, nil
}

func parseDouyinShareURLInput(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("share_url is required")
	}

	var params DouyinVideoInfoInput
	if err := json.Unmarshal([]byte(trimmed), &params); err == nil && strings.TrimSpace(params.ShareURL) != "" {
		trimmed = params.ShareURL
	}

	if extracted := extractDouyinShareURL(trimmed); extracted != "" {
		return extracted, nil
	}
	if strings.Contains(trimmed, "douyin.com") || strings.Contains(trimmed, "iesdouyin.com") {
		return strings.TrimSpace(trimmed), nil
	}
	return "", fmt.Errorf("invalid douyin share_url")
}

func extractDouyinShareURL(raw string) string {
	for _, prefix := range []string{"https://v.douyin.com/", "http://v.douyin.com/", "https://www.douyin.com/", "http://www.douyin.com/", "https://iesdouyin.com/", "http://iesdouyin.com/"} {
		index := strings.Index(raw, prefix)
		if index < 0 {
			continue
		}
		shareURL := raw[index:]
		for i, r := range shareURL {
			if r == ' ' || r == '\n' || r == '\t' || r == '"' || r == '\'' || r == '<' || r == '>' {
				shareURL = shareURL[:i]
				break
			}
		}
		return strings.TrimRight(shareURL, ",，。；;!！?？)）]】>》")
	}
	return ""
}
