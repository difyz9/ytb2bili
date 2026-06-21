package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	clientpkg "github.com/zolagz/ytb2bili/pkg/tikhub"
	"go.uber.org/zap"
)

// FetchVideoByShareURLTool fetches Douyin video metadata from a share URL.
type FetchVideoByShareURLTool struct {
	resolver clientpkg.Resolver
	logger   *zap.Logger
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

func NewFetchVideoByShareURLTool(resolver clientpkg.Resolver, logger *zap.Logger) *FetchVideoByShareURLTool {
	return &FetchVideoByShareURLTool{resolver: resolver, logger: logger}
}

func (t *FetchVideoByShareURLTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "fetch_one_video_by_share_url",
		Desc: "根据抖音分享链接获取视频详细信息，返回 aweme_id、标题、作者、播放地址、统计数据等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"share_url": {
				Type:     schema.String,
				Desc:     "抖音分享链接，例如 https://v.douyin.com/xxxxxx/",
				Required: true,
			},
		}),
	}, nil
}

func (t *FetchVideoByShareURLTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	info, raw, err := t.fetch(ctx, argumentsInJSON, true)
	if err != nil {
		return "", err
	}
	t.logger.Info("Douyin video resolved",
		zap.String("aweme_id", info.Data.AwemeID),
		zap.String("share_url", info.Data.ShareURL))
	return raw, nil
}

func (t *FetchVideoByShareURLTool) Call(ctx context.Context, input string) (string, error) {
	_, raw, err := t.fetch(ctx, input, true)
	return raw, err
}

func (t *FetchVideoByShareURLTool) Fetch(ctx context.Context, input string) (*DouyinVideoInfo, error) {
	info, _, err := t.fetch(ctx, input, true)
	return info, err
}

func (t *FetchVideoByShareURLTool) DebugFetch(ctx context.Context, input string) (*DouyinVideoInfo, string, error) {
	return t.fetch(ctx, input, false)
}

func (t *FetchVideoByShareURLTool) fetch(ctx context.Context, input string, requireAwemeID bool) (*DouyinVideoInfo, string, error) {
	shareURL, err := parseDouyinShareURLInput(input)
	if err != nil {
		return nil, "", err
	}
	if t.resolver == nil {
		return nil, "", fmt.Errorf("douyin resolver is not configured")
	}

	resolved, err := t.resolver.Resolve(ctx, shareURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetch douyin video by share url: %w", err)
	}

	info := convertTikHubDouyinVideoInfo(resolved)
	raw, err := sonic.MarshalString(info)
	if err != nil {
		return nil, "", fmt.Errorf("marshal douyin video info: %w", err)
	}
	if requireAwemeID && strings.TrimSpace(info.Data.AwemeID) == "" {
		return nil, raw, fmt.Errorf(
			"douyin response missing aweme_id (code=%d, message=%s, message_zh=%s, raw=%s)",
			info.Code,
			strings.TrimSpace(info.Message),
			strings.TrimSpace(info.MessageZh),
			truncateDouyinDebugRaw(raw, 512),
		)
	}
	if info.Data.ShareURL == "" {
		info.Data.ShareURL = shareURL
	}
	return info, raw, nil
}

func convertTikHubDouyinVideoInfo(info *clientpkg.DouyinVideoInfo) *DouyinVideoInfo {
	if info == nil {
		return nil
	}

	return &DouyinVideoInfo{
		Code:      info.Code,
		Message:   info.Message,
		MessageZh: info.MessageZh,
		Data: DouyinAwemeDetail{
			AwemeID:    info.Data.AwemeID,
			Desc:       info.Data.Desc,
			ShareURL:   info.Data.ShareURL,
			CreateTime: info.Data.CreateTime,
			Author: DouyinAuthorInfo{
				UID:      info.Data.Author.UID,
				Nickname: info.Data.Author.Nickname,
				SecUID:   info.Data.Author.SecUID,
			},
			Video: convertTikHubDouyinVideoData(info.Data.Video),
			Statistics: DouyinStatistics{
				DiggCount:    info.Data.Statistics.DiggCount,
				CommentCount: info.Data.Statistics.CommentCount,
				ShareCount:   info.Data.Statistics.ShareCount,
				PlayCount:    info.Data.Statistics.PlayCount,
			},
		},
	}
}

func convertTikHubDouyinVideoData(video clientpkg.DouyinVideoData) DouyinVideoData {
	bitRate := make([]DouyinBitRateInfo, 0, len(video.BitRate))
	for _, item := range video.BitRate {
		bitRate = append(bitRate, DouyinBitRateInfo{
			BitRate:     item.BitRate,
			QualityType: item.QualityType,
			GearName:    item.GearName,
			PlayAddr:    convertTikHubDouyinMediaURL(item.PlayAddr),
		})
	}

	return DouyinVideoData{
		VideoID:      video.VideoID,
		Duration:     video.Duration,
		PlayAddr:     convertTikHubDouyinMediaURL(video.PlayAddr),
		PlayAddrH264: convertTikHubDouyinMediaURL(video.PlayAddrH264),
		PlayAddr265:  convertTikHubDouyinMediaURL(video.PlayAddr265),
		DownloadAddr: convertTikHubDouyinMediaURL(video.DownloadAddr),
		BitRate:      bitRate,
		Cover:        convertTikHubDouyinImage(video.Cover),
		DynamicCover: convertTikHubDouyinImage(video.DynamicCover),
		OriginCover:  convertTikHubDouyinImage(video.OriginCover),
	}
}

func convertTikHubDouyinMediaURL(media clientpkg.DouyinMediaURL) DouyinMediaURL {
	return DouyinMediaURL{
		URI:      media.URI,
		URLList:  append([]string(nil), media.URLList...),
		DataSize: media.DataSize,
		Width:    media.Width,
		Height:   media.Height,
	}
}

func convertTikHubDouyinImage(image clientpkg.DouyinImage) DouyinImage {
	return DouyinImage{
		URI:     image.URI,
		URLList: append([]string(nil), image.URLList...),
		Width:   image.Width,
		Height:  image.Height,
	}
}

func truncateDouyinDebugRaw(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if limit <= 0 || len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "..."
}

func parseDouyinVideoInfoRaw(raw string) (*DouyinVideoInfo, error) {
	var envelope douyinVideoInfoEnvelope
	if err := sonic.UnmarshalString(raw, &envelope); err != nil {
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
	if err := sonic.UnmarshalString(trimmed, &params); err == nil && strings.TrimSpace(params.ShareURL) != "" {
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
		idx := strings.Index(raw, prefix)
		if idx < 0 {
			continue
		}
		shareURL := raw[idx:]
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
