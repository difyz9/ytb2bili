package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const videoQueryToolName = "query_videos"

// VideoQueryTool lets the agent query the user's local video library.
type VideoQueryTool struct {
	db     *gorm.DB
	logger *zap.Logger
	userID string // 由 AgentHandler 每次请求前通过 SetUserContext 注入
}

// NewVideoQueryTool creates a VideoQueryTool.
func NewVideoQueryTool(db *gorm.DB, logger *zap.Logger) *VideoQueryTool {
	return &VideoQueryTool{db: db, logger: logger}
}

// SetUserContext implements ContextualTool — called by AgentHandler before each Run.
func (t *VideoQueryTool) SetUserContext(userID string) { t.userID = userID }

// Info describes the tool to the LLM.
func (t *VideoQueryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_videos",
		Desc: "查询用户本地视频库。支持按处理状态、平台筛选，返回视频标题、状态、是否已上传B站等信息。常用于回答「我有哪些视频」「哪些视频失败了」等问题。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"status": {
				Type: schema.String,
				Desc: "按状态筛选: completed(已完成)、processing(处理中)、failed(失败)、pending(待处理)，留空返回全部",
			},
			"platform": {
				Type: schema.String,
				Desc: "按平台筛选: youtube、bilibili，留空返回全部",
			},
			"limit": {
				Type: schema.Integer,
				Desc: "最大返回数量，默认10，最大50",
			},
			"has_bili_upload": {
				Type: schema.Boolean,
				Desc: "true 表示只返回已上传B站的视频",
			},
		}),
	}, nil
}

var videoStatusCodeMap = map[string]string{
	"completed":  "003",
	"processing": "002",
	"pending":    "001",
	"failed":     "004",
}

var videoStatusLabelMap = map[string]string{
	"001": "待处理",
	"002": "处理中",
	"003": "已完成",
	"004": "失败",
}

type queryParams struct {
	Status        string `json:"status"`
	Platform      string `json:"platform"`
	Limit         int    `json:"limit"`
	HasBiliUpload bool   `json:"has_bili_upload"`
}

// InvokableRun executes the query.
func (t *VideoQueryTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	params, err := UnmarshalArgs[queryParams](videoQueryToolName, args)
	if err != nil {
		return "", err
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	type dbRow struct {
		VideoID        string    `gorm:"column:video_id"`
		Title          string    `gorm:"column:title"`
		GeneratedTitle string    `gorm:"column:generated_title"`
		Status         string    `gorm:"column:status"`
		Platform       string    `gorm:"column:platform"`
		Duration       float64   `gorm:"column:duration"`
		BiliBVID       string    `gorm:"column:bili_bvid"`
		VideoPath      string    `gorm:"column:video_path"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}

	q := t.db.WithContext(ctx).Table("tb_videos").
		Select("video_id, title, generated_title, status, platform, duration, bili_bvid, video_path, created_at").
		Order("created_at DESC").
		Limit(params.Limit)

	if t.userID != "" {
		q = q.Where("user_id = ?", t.userID)
	}

	if s := strings.ToLower(params.Status); s != "" {
		if code, ok := videoStatusCodeMap[s]; ok {
			q = q.Where("status = ?", code)
		} else {
			q = q.Where("status = ?", s)
		}
	}
	if params.Platform != "" {
		q = q.Where("platform = ?", params.Platform)
	}
	if params.HasBiliUpload {
		q = q.Where("bili_bvid != '' AND bili_bvid IS NOT NULL")
	}

	var rows []dbRow
	if err := q.Find(&rows).Error; err != nil {
		return "", fmt.Errorf("数据库查询失败: %w", err)
	}
	if len(rows) == 0 {
		return "未找到符合条件的视频", nil
	}

	type display struct {
		VideoID        string `json:"video_id"`
		Title          string `json:"title"`
		GeneratedTitle string `json:"generated_title,omitempty"`
		Status         string `json:"status"`
		Platform       string `json:"platform"`
		Duration       string `json:"duration,omitempty"`
		BiliUploaded   bool   `json:"bili_uploaded"`
		BiliBVID       string `json:"bili_bvid,omitempty"`
		HasLocalFile   bool   `json:"has_local_file"`
		CreatedAt      string `json:"created_at"`
	}

	out := make([]display, len(rows))
	for i, r := range rows {
		label := videoStatusLabelMap[r.Status]
		if label == "" {
			label = r.Status
		}
		dur := ""
		if r.Duration > 0 {
			m := int(r.Duration) / 60
			s := int(r.Duration) % 60
			dur = fmt.Sprintf("%d:%02d", m, s)
		}
		out[i] = display{
			VideoID:        r.VideoID,
			Title:          r.Title,
			GeneratedTitle: r.GeneratedTitle,
			Status:         label,
			Platform:       r.Platform,
			Duration:       dur,
			BiliUploaded:   r.BiliBVID != "",
			BiliBVID:       r.BiliBVID,
			HasLocalFile:   r.VideoPath != "",
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	return fmt.Sprintf("找到 %d 个视频:\n%s", len(rows), string(b)), nil
}
