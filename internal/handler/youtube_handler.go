package handler

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	internalservice "github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/store"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// 共享的绑定缓存（用于OAuth回调）
var sharedBindingCache *store.CacheDict

// YouTubeFeed 表示 YouTube RSS feed 的结构
type YouTubeFeed struct {
	XMLName xml.Name       `xml:"feed"`
	Entries []YouTubeEntry `xml:"entry"`
}

// YouTubeEntry 表示 feed 中的一个视频条目
type YouTubeEntry struct {
	ID        string      `xml:"id"`
	Title     string      `xml:"title"`
	Link      Link        `xml:"link"`
	Published string      `xml:"published"`
	Updated   string      `xml:"updated"`
	VideoID   YTVideoID   `xml:"videoId"`
	ChannelID YTChannelID `xml:"channelId"`
}

// Link 表示视频链接
type Link struct {
	Href string `xml:"href,attr"`
}

// YTVideoID 表示 YouTube 视频 ID
type YTVideoID struct {
	Value string `xml:",chardata"`
}

// YTChannelID 表示 YouTube 频道 ID
type YTChannelID struct {
	Value string `xml:",chardata"`
}

// TrendingVideosRequest 热门视频请求结构
type TrendingVideosRequest struct {
	RegionCode string `json:"regionCode,omitempty" example:"US"`
	CategoryId string `json:"categoryId,omitempty" example:"10"`
	MaxResults int64  `json:"maxResults,omitempty" example:"10"`
	Chart      string `json:"chart,omitempty" example:"mostPopular"`
}

// RecommendedVideosRequest 推荐视频请求结构
type RecommendedVideosRequest struct {
	VideoId    string `json:"videoId,omitempty" example:"dQw4w9WgXcQ"`
	CategoryId string `json:"categoryId,omitempty" example:"28"`
	MaxResults int64  `json:"maxResults,omitempty" example:"10"`
	Order      string `json:"order,omitempty" example:"relevance"`
}

type updateSubscriptionStatusRequest struct {
	UserID      string `json:"user_id"`
	Status      string `json:"status"`
	SyncEnabled *bool  `json:"sync_enabled"`
}

type YouTubeHandler struct {
	logger         *zap.Logger
	youtubeClient  *internalservice.YouTubeClientFactory
	systemSettings *internalservice.SystemSettingsClient
	youtubeService *internalservice.YouTubeService
}

func NewYouTubeHandler(logger *zap.Logger, _ *config.AppConfig, youtubeClient *internalservice.YouTubeClientFactory, systemSettings *internalservice.SystemSettingsClient, youtubeService *internalservice.YouTubeService) *YouTubeHandler {
	return &YouTubeHandler{
		logger:         logger,
		youtubeClient:  youtubeClient,
		systemSettings: systemSettings,
		youtubeService: youtubeService,
	}
}

// OAuthCallback godoc
// @Summary      处理YouTube OAuth回调
// @Description  交换授权码获取访问令牌并保存用户订阅信息
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        code   query     string  true  "授权码"
// @Param        state  query     string  true  "状态值"
// @Success      302    {string}  string  "重定向到前端"
// @Failure      400    {object}  map[string]interface{}  "error: string"
// @Failure      500    {object}  map[string]interface{}  "error: string"
// @Router       /youtube/oauth/callback [get]
func (h *YouTubeHandler) OAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid state parameter",
		})
		return
	}

	// 创建带超时的 context (60秒)，并注入自定义 HTTP 客户端
	ctx, cancel := context.WithTimeout(h.youtubeClient.OAuthContext(), 60*time.Second)
	defer cancel()

	// 交换授权码获取访问令牌
	token, err := h.youtubeClient.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("Failed to exchange OAuth token",
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to exchange token: %v", err),
		})
		return
	}

	// 创建 YouTube 服务客户端（使用同一个带超时的 context）
	youtubeService, err := h.youtubeClient.NewOAuthService(ctx, token)
	if err != nil {
		h.logger.Error("Failed to create YouTube service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to create YouTube service: %v", err),
		})
		return
	}

	// 获取用户频道信息
	channelResponse, err := youtubeService.Channels.List([]string{"snippet"}).Mine(true).Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to fetch channel info: %v", err),
		})
		return
	}

	if len(channelResponse.Items) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "No channel found for this user",
		})
		return
	}

	userChannelId := channelResponse.Items[0].Id
	userChannelTitle := channelResponse.Items[0].Snippet.Title

	// state参数是qrCodeKey，从缓存中获取用户ID
	qrCodeKey := state

	// 从共享缓存中获取绑定信息
	var bindingData map[string]interface{}
	var userID string

	if sharedBindingCache != nil {
		if err := sharedBindingCache.Get(qrCodeKey, &bindingData); err == nil {
			userID = fmt.Sprintf("%v", bindingData["user_id"])
			h.logger.Info("Found binding data in cache",
				zap.String("qr_code_key", qrCodeKey),
				zap.String("user_id", userID))
		}
	}

	// 如果缓存中没有，尝试从数据库查找（兼容旧流程）
	if userID == "" {
		var binding model.AccountBinding
		if err := h.youtubeService.GetDB().Where("qr_code_key = ?", qrCodeKey).First(&binding).Error; err != nil {
			h.logger.Error("Failed to find binding record",
				zap.String("qr_code_key", qrCodeKey),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Binding record not found",
			})
			return
		}
		userID = binding.UserID
	}

	if userID == "" {
		h.logger.Error("User ID is empty", zap.String("qr_code_key", qrCodeKey))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	h.logger.Info("OAuth callback successful",
		zap.String("user_id", userID),
		zap.String("qr_code_key", qrCodeKey),
		zap.String("channel_id", userChannelId),
		zap.String("channel_title", userChannelTitle))

	// 异步获取用户订阅列表（避免阻塞用户）
	go func(userID, userChannelId, userChannelTitle string, token *oauth2.Token, logger *zap.Logger) {
		subscriptions := make([]*model.TbSubscription, 0)
		nextPageToken := ""
		totalSubscriptions := 0
		syncCompleted := true

		// 创建新的context用于订阅获取
		ctx := context.Background()
		service, err := h.youtubeClient.NewOAuthService(ctx, token)
		if err != nil {
			logger.Error("Failed to create YouTube service for subscriptions", zap.Error(err))
			return
		}

		logger.Info("Starting to fetch user subscriptions in background",
			zap.String("user_id", userID))

		for {
			call := service.Subscriptions.List([]string{"snippet"}).Mine(true).MaxResults(50)
			if nextPageToken != "" {
				call = call.PageToken(nextPageToken)
			}

			subsResponse, err := call.Do()
			if err != nil {
				logger.Error("Failed to fetch subscriptions", zap.Error(err))
				syncCompleted = false
				break
			}

			for _, item := range subsResponse.Items {
				subscription := &model.TbSubscription{
					UserID:              userID,
					ChannelID:           item.Snippet.ResourceId.ChannelId,
					Platform:            "youtube",
					ChannelTitle:        item.Snippet.Title,
					ChannelDescription:  item.Snippet.Description,
					ChannelThumbnailURL: item.Snippet.Thumbnails.Default.Url,
					SubscribedAt:        time.Now(),
					Status:              "active",
					SyncedAt:            time.Now(),
				}
				subscriptions = append(subscriptions, subscription)
			}

			totalSubscriptions += len(subsResponse.Items)
			nextPageToken = subsResponse.NextPageToken
			if nextPageToken == "" {
				break
			}
		}

		// 保存订阅列表到数据库，并保留用户已设置的同步状态
		if syncCompleted {
			if err := h.mergeUserSubscriptions(userID, subscriptions, true); err != nil {
				logger.Error("Failed to save subscriptions", zap.Error(err))
			} else {
				logger.Info("User subscriptions saved in background",
					zap.String("user_id", userID),
					zap.Int("count", totalSubscriptions))
			}
		}

		// 注意：订阅频道的视频将通过定时任务自动同步
	}(userID, userChannelId, userChannelTitle, token, h.logger)

	// 创建或更新账号绑定记录
	var expiresAt *time.Time
	if !token.Expiry.IsZero() {
		expiresAt = &token.Expiry
	}

	// 检查是否已存在绑定记录
	var existingBinding model.AccountBinding
	result := h.youtubeService.GetDB().Where("user_id = ? AND platform = ?", userID, "youtube").First(&existingBinding)

	if result.Error == nil {
		// 更新现有绑定
		updates := map[string]interface{}{
			"status":        model.BindingStatusBound,
			"platform_uid":  userChannelId,
			"username":      userChannelTitle,
			"avatar":        channelResponse.Items[0].Snippet.Thumbnails.Default.Url,
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_at":    expiresAt,
		}
		if err := h.youtubeService.GetDB().Model(&existingBinding).Updates(updates).Error; err != nil {
			h.logger.Error("Failed to update account binding", zap.Error(err))
		}
	} else {
		// 创建新绑定
		binding := &model.AccountBinding{
			UserID:       userID,
			Platform:     model.PlatformYoutube,
			PlatformUID:  userChannelId,
			Username:     userChannelTitle,
			Avatar:       channelResponse.Items[0].Snippet.Thumbnails.Default.Url,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			ExpiresAt:    expiresAt,
			Status:       model.BindingStatusBound,
		}

		if err := h.youtubeService.GetDB().Create(binding).Error; err != nil {
			h.logger.Error("Failed to create account binding", zap.Error(err))
		} else {
			h.logger.Info("Account binding created",
				zap.String("user_id", userID),
				zap.String("platform", "youtube"),
				zap.String("channel_id", userChannelId))
		}
	}

	// 清理绑定缓存
	if sharedBindingCache != nil {
		sharedBindingCache.Delete(qrCodeKey)
	}

	h.logger.Info("YouTube账号绑定成功",
		zap.String("user_id", userID),
		zap.String("channel_id", userChannelId),
		zap.String("channel_title", userChannelTitle))

	// 重定向到前端回调页面，前端弹窗页通过 postMessage 通知父窗口刷新账号列表
	redirectURL := frontendURL(c, fmt.Sprintf(
		"/dashboard/accounts/youtube-callback?success=true&platform=youtube&username=%s",
		url.QueryEscape(userChannelTitle),
	))

	c.Redirect(http.StatusFound, redirectURL)
}

// CheckAuthStatus godoc
// @Summary      检查YouTube授权状态
// @Description  检查当前用户的YouTube授权状态
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        user_id  query     string  true  "用户ID"
// @Success      200      {object}  map[string]interface{}  "authorized: bool, message: string"
// @Failure      400      {object}  map[string]interface{}  "error: string"
// @Router       /youtube/check-auth-status [get]
func (h *YouTubeHandler) CheckAuthStatus(c *gin.Context) {
	userID := c.Query("user_id")

	if userID == "" {
		BadRequest(c, ErrInvalidParams)
		return
	}

	Success(c, gin.H{
		"authorized":      true,
		"message":         "User is authenticated",
		"channels_synced": 0,
	})
}

// SearchKeyword godoc
// @Summary      根据关键字搜索YouTube视频
// @Description  使用YouTube API v3根据关键字搜索视频
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        keyword        path      string  true   "搜索关键字"
// @Param        maxResults     query     int     false  "最大结果数量(1-50)"  default(10)
// @Param        order          query     string  false  "排序方式"  default(relevance)
// @Param        type           query     string  false  "搜索类型"  default(video)
// @Param        videoDuration  query     string  false  "视频时长"  default(any)
// @Success      200            {object}  map[string]interface{}  "videos: array"
// @Failure      400            {object}  map[string]interface{}  "error: string"
// @Failure      500            {object}  map[string]interface{}  "error: string"
// @Router       /youtube/search/{keyword} [get]
func (h *YouTubeHandler) SearchKeyword(c *gin.Context) {
	keyword := c.Param("keyword")

	if keyword == "" {
		BadRequest(c, "Keyword is required")
		return
	}

	maxResults := int64(10)
	if mr := c.Query("maxResults"); mr != "" {
		if parsed, err := strconv.ParseInt(mr, 10, 64); err == nil && parsed > 0 && parsed <= 50 {
			maxResults = parsed
		}
	}

	order := c.DefaultQuery("order", "relevance")
	searchType := c.DefaultQuery("type", "video")

	// 创建 YouTube 服务
	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to create YouTube service: %v", err))
		return
	}

	// 执行搜索
	call := service.Search.List([]string{"snippet"}).
		Q(keyword).
		MaxResults(maxResults).
		Order(order).
		Type(searchType)

	response, err := call.Do()
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Search failed: %v", err))
		return
	}

	Success(c, gin.H{
		"videos":       response.Items,
		"totalResults": response.PageInfo.TotalResults,
	})
}

// GetTrendingVideos godoc
// @Summary      获取热门视频
// @Description  获取YouTube热门视频列表
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        request  body      TrendingVideosRequest  true  "热门视频请求参数"
// @Success      200      {object}  map[string]interface{}  "videos: array"
// @Failure      500      {object}  map[string]interface{}  "error: string"
// @Router       /youtube/trending [post]
func (h *YouTubeHandler) GetTrendingVideos(c *gin.Context) {
	var req TrendingVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.RegionCode == "" {
		req.RegionCode = "US"
	}
	if req.MaxResults == 0 {
		req.MaxResults = 10
	}
	if req.Chart == "" {
		req.Chart = "mostPopular"
	}

	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to create YouTube service: %v", err))
		return
	}

	call := service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).
		Chart(req.Chart).
		RegionCode(req.RegionCode).
		MaxResults(req.MaxResults)

	if req.CategoryId != "" {
		call = call.VideoCategoryId(req.CategoryId)
	}

	response, err := call.Do()
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to fetch trending videos: %v", err))
		return
	}

	Success(c, gin.H{
		"videos": response.Items,
		"total":  len(response.Items),
	})
}

// GetVideoDetails godoc
// @Summary      获取视频详情
// @Description  根据视频ID获取YouTube视频详细信息
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        videoId  path      string  true  "视频ID"
// @Success      200      {object}  map[string]interface{}  "video: object"
// @Failure      400      {object}  map[string]interface{}  "error: string"
// @Failure      500      {object}  map[string]interface{}  "error: string"
// @Router       /youtube/video/{videoId} [get]
func (h *YouTubeHandler) GetVideoDetails(c *gin.Context) {
	videoID := c.Param("videoId")

	if videoID == "" {
		BadRequest(c, "Video ID is required")
		return
	}

	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to create YouTube service: %v", err))
		return
	}

	call := service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to fetch video details: %v", err))
		return
	}

	if len(response.Items) == 0 {
		NotFound(c, ErrVideoNotFound)
		return
	}

	Success(c, gin.H{
		"video": response.Items[0],
	})
}

// GetVideoCategories godoc
// @Summary      获取视频分类
// @Description  获取YouTube视频分类列表
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        regionCode  query     string  false  "地区代码"  default(US)
// @Success      200         {object}  map[string]interface{}  "categories: array"
// @Failure      500         {object}  map[string]interface{}  "error: string"
// @Router       /youtube/categories [get]
func (h *YouTubeHandler) GetVideoCategories(c *gin.Context) {
	regionCode := c.DefaultQuery("regionCode", "US")

	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to create YouTube service: %v", err))
		return
	}

	call := service.VideoCategories.List([]string{"snippet"}).RegionCode(regionCode)
	response, err := call.Do()
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to fetch categories: %v", err))
		return
	}

	Success(c, gin.H{
		"categories": response.Items,
	})
}

// RefreshFeed godoc
// @Summary      手动刷新订阅频道
// @Description  手动触发刷新用户订阅频道的最新视频
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "message: string"
// @Router       /youtube/feed/refresh [get]
func (h *YouTubeHandler) RefreshFeed(c *gin.Context) {
	h.logger.Info("Manual feed refresh triggered")

	// 异步执行刷新任务，避免阻塞请求
	go func() {
		h.SyncYouTubeFeed()
	}()

	SuccessWithMessage(c, EmptyData{}, "Feed refresh task started")
}

// SyncYouTubeFeed runs the existing feed refresh flow outside the HTTP layer.
func (h *YouTubeHandler) SyncYouTubeFeed() {
	h.fetchYouTubeFeed()
}

// SubscriptionVideoDTO 订阅视频DTO（用于前端展示）
type SubscriptionVideoDTO struct {
	ID             uint      `json:"id"`
	VideoID        string    `json:"video_id"`
	Title          string    `json:"title"`
	VideoURL       string    `json:"video_url"`
	ChannelID      string    `json:"channel_id"`
	ChannelTitle   string    `json:"channel_title"`
	Duration       float64   `json:"duration"`
	ImgURL         string    `json:"img_url"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SubscriptionID uint      `json:"subscription_id"`
	ChannelStatus  string    `json:"channel_status"`
}

// GetLatestVideos godoc
// @Summary      获取最新视频列表
// @Description  获取订阅频道的最新视频（支持分页）
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        page      query     int  false  "页码"  default(1)
// @Param        pageSize  query     int  false  "每页数量"  default(20)
// @Success      200       {object}  map[string]interface{}  "videos: array, total: int, page: int"
// @Router       /youtube/feed/videos [get]
func (h *YouTubeHandler) GetLatestVideos(c *gin.Context) {
	// 从上下文获取用户ID（如果有认证）
	userID := ""
	if uid, exists := c.Get("uid"); exists {
		if uidStr, ok := uid.(string); ok {
			userID = uidStr
		}
	}

	// 如果没有从context获取到，尝试从query参数获取
	if userID == "" {
		userID = c.Query("user_id")
	}

	page := 1
	pageSize := 20
	searchTerm := strings.TrimSpace(c.Query("search"))
	channelStatusFilter := strings.TrimSpace(c.Query("channel_status"))

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	offset := (page - 1) * pageSize

	var total int64
	var videos []model.Video

	// 构建查询
	query := h.youtubeService.GetDB().Model(&model.Video{}).
		Where("tb_videos.video_id != ? AND tb_videos.video_id IS NOT NULL AND tb_videos.title != ? AND tb_videos.title IS NOT NULL", "", "").
		Where("tb_videos.status = ?", "synced")
	hasSubscriptionJoin := false

	// 如果有用户ID，只返回该用户的视频
	if userID != "" {
		query = query.Where("tb_videos.user_id = ?", userID)
	}

	if searchTerm != "" || channelStatusFilter == "active" || channelStatusFilter == "inactive" {
		query = query.Joins("LEFT JOIN tb_subscription ON tb_subscription.user_id = tb_videos.user_id AND tb_subscription.channel_id = tb_videos.channel_id")
		hasSubscriptionJoin = true
	}

	if searchTerm != "" {
		like := "%" + searchTerm + "%"
		if hasSubscriptionJoin {
			query = query.Where("tb_videos.title LIKE ? OR tb_videos.channel_id LIKE ? OR tb_subscription.channel_title LIKE ?", like, like, like)
		} else {
			query = query.Where("tb_videos.title LIKE ? OR tb_videos.channel_id LIKE ?", like, like)
		}
	}

	if channelStatusFilter == "active" || channelStatusFilter == "inactive" {
		query = query.Where("COALESCE(tb_subscription.status, ?) = ?", "active", channelStatusFilter)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count videos", zap.Error(err))
		InternalServerError(c, ErrDatabaseQuery)
		return
	}

	// 获取分页数据
	if err := query.Select("tb_videos.*").
		Order("tb_videos.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&videos).Error; err != nil {
		h.logger.Error("Failed to fetch videos", zap.Error(err))
		InternalServerError(c, ErrDatabaseQuery)
		return
	}

	subscriptionByChannelID := make(map[string]model.TbSubscription)
	if userID != "" {
		channelIDs := make([]string, 0, len(videos))
		seenChannelIDs := make(map[string]struct{}, len(videos))
		for _, video := range videos {
			channelID := strings.TrimSpace(video.ChannelId)
			if channelID == "" {
				continue
			}
			if _, ok := seenChannelIDs[channelID]; ok {
				continue
			}
			seenChannelIDs[channelID] = struct{}{}
			channelIDs = append(channelIDs, channelID)
		}

		if len(channelIDs) > 0 {
			var subscriptions []model.TbSubscription
			if err := h.youtubeService.GetDB().Where("user_id = ? AND channel_id IN ?", userID, channelIDs).Find(&subscriptions).Error; err != nil {
				h.logger.Warn("Failed to fetch subscriptions for videos", zap.Error(err), zap.String("user_id", userID))
			} else {
				for _, subscription := range subscriptions {
					subscriptionByChannelID[subscription.ChannelID] = subscription
				}
			}
		}
	}

	// 转换为DTO格式
	videoList := make([]SubscriptionVideoDTO, 0, len(videos))
	for _, video := range videos {
		// 生成缩略图URL
		imgURL := video.Thumbnail
		if imgURL == "" && video.VideoID != "" {
			imgURL = fmt.Sprintf("https://i.ytimg.com/vi/%s/mqdefault.jpg", video.VideoID)
		}

		createdAt := video.CreatedAt
		if video.PublishedAt != nil {
			createdAt = *video.PublishedAt
		}

		subscription := subscriptionByChannelID[video.ChannelId]
		channelStatus := subscription.Status
		if strings.TrimSpace(channelStatus) == "" {
			channelStatus = "active"
		}
		channelTitle := subscription.ChannelTitle
		if strings.TrimSpace(channelTitle) == "" {
			channelTitle = video.ChannelId
		}

		videoList = append(videoList, SubscriptionVideoDTO{
			ID:             video.ID,
			VideoID:        video.VideoID,
			Title:          video.Title,
			VideoURL:       video.URL,
			ChannelID:      video.ChannelId,
			ChannelTitle:   channelTitle,
			Duration:       video.Duration,
			ImgURL:         imgURL,
			CreatedAt:      createdAt,
			UpdatedAt:      video.UpdatedAt,
			SubscriptionID: subscription.ID,
			ChannelStatus:  channelStatus,
		})
	}

	h.logger.Info("Fetched subscription videos",
		zap.String("user_id", userID),
		zap.Int("total", int(total)),
		zap.Int("page", page),
		zap.Int("count", len(videoList)))

	SuccessWithPage(c, videoList, total, page, pageSize)
}

// parseDuration 解析 ISO 8601 时长格式为秒数
func parseDuration(duration string) float64 {
	re := regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)
	matches := re.FindStringSubmatch(duration)

	if len(matches) == 0 {
		return 0
	}

	var hours, minutes, seconds int

	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		minutes, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		seconds, _ = strconv.Atoi(matches[3])
	}

	return float64(hours*3600 + minutes*60 + seconds)
}

// getVideoDuration 通过 YouTube API 获取视频时长（秒）
func (h *YouTubeHandler) getVideoDuration(videoID string) float64 {
	ctx := context.Background()

	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		h.logger.Warn("Failed to create YouTube service", zap.Error(err))
		return 0
	}

	call := service.Videos.List([]string{"contentDetails"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		h.logger.Warn("Failed to call YouTube API", zap.Error(err))
		return 0
	}

	if len(response.Items) == 0 {
		h.logger.Warn("Video not found", zap.String("video_id", videoID))
		return 0
	}

	durationStr := response.Items[0].ContentDetails.Duration
	return parseDuration(durationStr)
}

// processVideoEntry 处理单个视频条目
func (h *YouTubeHandler) processVideoEntry(entry YouTubeEntry, channelId string, channelTitle string, lookbackDays int) {
	cutoff := time.Now().Add(-time.Duration(lookbackDays) * 24 * time.Hour)

	videoID := ""
	if len(entry.ID) > 0 {
		parts := strings.Split(entry.ID, ":")
		if len(parts) > 2 {
			videoID = parts[len(parts)-1]
		}
	}

	if videoID == "" && entry.VideoID.Value != "" {
		videoID = entry.VideoID.Value
	}

	if videoID == "" {
		h.logger.Warn("Cannot extract video ID")
		return
	}

	// 获取订阅了该频道的所有用户
	var subscriptions []model.TbSubscription
	if err := h.youtubeService.GetDB().Where("channel_id = ? AND status = ?", channelId, "active").
		Find(&subscriptions).Error; err != nil {
		h.logger.Error("Failed to fetch channel subscriptions", zap.Error(err))
		return
	}

	if len(subscriptions) == 0 {
		h.logger.Debug("No active subscriptions for channel", zap.String("channel_id", channelId))
		return
	}

	//duration := h.getVideoDuration(videoID)

	// 解析发布时间
	var publishedAt *time.Time
	if entry.Published != "" {
		if pt, err := time.Parse(time.RFC3339, entry.Published); err == nil {
			publishedAt = &pt
		}
	}

	if publishedAt == nil {
		h.logger.Debug("Skip video without published time",
			zap.String("video_id", videoID),
			zap.String("channel_id", channelId))
		return
	}

	if publishedAt.Before(cutoff) {
		h.logger.Debug("Skip video older than configured lookback window",
			zap.String("video_id", videoID),
			zap.String("channel_id", channelId),
			zap.Int("lookback_days", lookbackDays),
			zap.Time("published_at", *publishedAt),
			zap.Time("cutoff", cutoff))
		return
	}

	// 为每个订阅了该频道的用户创建视频记录
	newVideosCount := 0
	for _, sub := range subscriptions {
		// 检查该用户是否已有此视频
		var existingVideo model.Video
		err := h.youtubeService.GetDB().Where("user_id = ? AND video_id = ?", sub.UserID, videoID).First(&existingVideo).Error
		if err == nil {
			// 该用户已有此视频，跳过
			continue
		}

		video := model.Video{
			UserID:        sub.UserID,
			Title:         entry.Title,
			URL:           entry.Link.Href,
			VideoID:       videoID,
			ChannelId:     channelId,
			Duration:      0,
			Status:        "synced", // 自动同步的视频标记为 synced
			Platform:      "youtube",
			PublishedAt:   publishedAt,
			OperationType: "auto_sync", // 频道订阅自动同步
		}

		if err := h.youtubeService.GetDB().Create(&video).Error; err != nil {
			// 忽略重复键错误（并发情况下可能发生）
			if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE constraint") {
				h.logger.Debug("Video already exists for user (concurrent insert)",
					zap.String("video_id", videoID),
					zap.String("user_id", sub.UserID))
				continue
			}
			h.logger.Error("Failed to save video", zap.Error(err),
				zap.String("video_id", videoID),
				zap.String("user_id", sub.UserID))
			continue
		}

		newVideosCount++
	}

	if newVideosCount > 0 {
		h.logger.Info("New video added",
			zap.String("video_id", videoID),
			zap.String("channel", channelTitle),
			zap.String("title", entry.Title),
			zap.Int("users", newVideosCount))
	}
}

func (h *YouTubeHandler) getYouTubeFeedSyncLookbackDays() int {
	lookbackDays := model.DefaultYouTubeFeedSyncLookbackDays
	if h.systemSettings == nil || !h.systemSettings.IsEnabled() {
		return lookbackDays
	}

	settings, err := h.systemSettings.GetSettings(context.Background())
	if err != nil {
		h.logger.Warn("读取YouTube feed同步时间范围设置失败，使用默认值", zap.Error(err))
		return lookbackDays
	}

	if value := settings[model.SystemSettingKeyYouTubeFeedSyncLookback]; value != "" {
		if days, err := strconv.Atoi(value); err == nil {
			lookbackDays = model.NormalizeYouTubeFeedSyncLookbackDays(days)
		}
	}

	return lookbackDays
}

// fetchYouTubeFeed 获取所有订阅频道的最新视频（定时任务）
func (h *YouTubeHandler) fetchYouTubeFeed() {
	h.logger.Info("Fetching YouTube feed updates", zap.String("time", time.Now().Format("2006-01-02 15:04:05")))
	lookbackDays := h.getYouTubeFeedSyncLookbackDays()

	// 1. 获取所有活跃的订阅频道（去重）
	var subscriptions []model.TbSubscription
	if err := h.youtubeService.GetDB().Where("status = ?", "active").
		Distinct("channel_id", "channel_title").
		Find(&subscriptions).Error; err != nil {
		h.logger.Error("Failed to fetch subscriptions", zap.Error(err))
		return
	}

	h.logger.Info("Checking updates for channels", zap.Int("count", len(subscriptions)))

	// 2. 处理每个频道的 feed
	for _, sub := range subscriptions {
		h.processSubscriptionFeed(sub, lookbackDays)
	}
}

// processSubscriptionFeed 处理单个订阅频道的 feed
func (h *YouTubeHandler) processSubscriptionFeed(subscription model.TbSubscription, lookbackDays int) {
	// 构建 RSS feed URL
	feedURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", subscription.ChannelID)

	// 获取 feed
	resp, err := http.Get(feedURL)
	if err != nil {
		h.logger.Warn("Failed to fetch feed",
			zap.String("channel_id", subscription.ChannelID),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("Feed request failed",
			zap.String("channel_id", subscription.ChannelID),
			zap.Int("status_code", resp.StatusCode))
		return
	}

	// 解析 XML
	var feed YouTubeFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		h.logger.Error("Failed to parse feed XML",
			zap.String("channel_id", subscription.ChannelID),
			zap.Error(err))
		return
	}

	h.logger.Info("Feed fetched successfully",
		zap.String("channel", subscription.ChannelTitle),
		zap.String("channel_id", subscription.ChannelID),
		zap.Int("videos", len(feed.Entries)))

	// 处理每个视频条目
	for _, entry := range feed.Entries {
		h.processVideoEntry(entry, subscription.ChannelID, subscription.ChannelTitle, lookbackDays)
	}
}

// GetUserTbSubscriptions godoc
// @Summary      获取用户订阅列表
// @Description  获取当前用户的YouTube订阅频道列表（支持分页）
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        user_id   query     string  false  "用户ID（可选，默认从Token获取）"
// @Param        page      query     int     false  "页码"  default(1)
// @Param        page_size query     int     false  "每页数量"  default(20)
// @Security     BearerAuth
// @Success      200      {object}  map[string]interface{}  "list: array, total: int, page: int, size: int"
// @Failure      400      {object}  map[string]interface{}  "error: string"
// @Failure      500      {object}  map[string]interface{}  "error: string"
// @Router       /youtube/TbSubscriptions [get]
func (h *YouTubeHandler) GetUserTbSubscriptions(c *gin.Context) {
	// 优先从 query 参数获取 user_id
	userID := c.Query("user_id")

	// 如果没有提供 user_id，尝试从认证 context 中获取
	if userID == "" {
		if uid, exists := c.Get("uid"); exists {
			if uidStr, ok := uid.(string); ok {
				userID = uidStr
				h.logger.Info("Using auth UID as user_id", zap.String("uid", userID))
			}
		}
	}

	if userID == "" {
		BadRequest(c, "User ID is required or user not authenticated")
		return
	}

	// 分页参数
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	offset := (page - 1) * pageSize
	searchTerm := strings.TrimSpace(c.Query("search"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	var total int64
	var TbSubscriptions []model.TbSubscription
	query := h.youtubeService.GetDB().Model(&model.TbSubscription{}).Where("user_id = ?", userID)

	if statusFilter == "active" || statusFilter == "inactive" {
		query = query.Where("status = ?", statusFilter)
	}
	if searchTerm != "" {
		like := "%" + searchTerm + "%"
		query = query.Where("channel_title LIKE ? OR channel_id LIKE ?", like, like)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		InternalServerError(c, ErrDatabaseQuery)
		return
	}

	// 获取分页数据
	if err := query.
		Order("synced_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&TbSubscriptions).Error; err != nil {
		InternalServerError(c, ErrDatabaseQuery)
		return
	}

	SuccessWithPage(c, TbSubscriptions, total, page, pageSize)
}

// UpdateTbSubscriptionStatus godoc
// @Summary      更新频道同步状态
// @Description  启用或暂停某个订阅频道的自动同步
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        id      path      int   true  "订阅ID"
// @Param        request body      updateSubscriptionStatusRequest true "状态更新参数"
// @Success      200     {object}  map[string]interface{}  "subscription: object"
// @Failure      400     {object}  map[string]interface{}  "error: string"
// @Failure      403     {object}  map[string]interface{}  "error: string"
// @Failure      404     {object}  map[string]interface{}  "error: string"
// @Failure      500     {object}  map[string]interface{}  "error: string"
// @Router       /youtube/TbSubscriptions/{id}/status [patch]
func (h *YouTubeHandler) UpdateTbSubscriptionStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的订阅ID")
		return
	}

	var req updateSubscriptionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var subscription model.TbSubscription
	if err := h.youtubeService.GetDB().First(&subscription, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "订阅频道不存在")
			return
		}
		InternalServerError(c, "查询订阅频道失败")
		return
	}

	requestUserID := strings.TrimSpace(req.UserID)
	if requestUserID == "" {
		if uid, exists := c.Get("uid"); exists {
			if uidStr, ok := uid.(string); ok {
				requestUserID = uidStr
			}
		}
	}
	if requestUserID != "" && requestUserID != subscription.UserID {
		Forbidden(c, "无权操作该频道")
		return
	}

	nextStatus := strings.TrimSpace(req.Status)
	if req.SyncEnabled != nil {
		if *req.SyncEnabled {
			nextStatus = "active"
		} else {
			nextStatus = "inactive"
		}
	}
	if nextStatus != "active" && nextStatus != "inactive" {
		BadRequest(c, "状态仅支持 active 或 inactive")
		return
	}

	if err := h.youtubeService.GetDB().Model(&subscription).Update("status", nextStatus).Error; err != nil {
		InternalServerError(c, "更新频道同步状态失败")
		return
	}
	subscription.Status = nextStatus
	Success(c, gin.H{"subscription": subscription})
}

// SyncUserTbSubscriptions godoc
// @Summary      同步用户订阅
// @Description  从YouTube API同步用户的订阅频道列表
// @Tags         youtube
// @Accept       json
// @Produce      json
// @Param        user_id      query     string  true  "用户ID"
// @Param        access_token query     string  true  "访问令牌"
// @Success      200          {object}  map[string]interface{}  "synced: int"
// @Failure      400          {object}  map[string]interface{}  "error: string"
// @Failure      500          {object}  map[string]interface{}  "error: string"
// @Router       /youtube/TbSubscriptions/sync [post]
func (h *YouTubeHandler) SyncUserTbSubscriptions(c *gin.Context) {
	userID := c.Query("user_id")
	accessToken := c.Query("access_token")

	if userID == "" || accessToken == "" {
		BadRequest(c, "User ID and access token are required")
		return
	}

	// 使用访问令牌创建 YouTube 服务
	token := &oauth2.Token{AccessToken: accessToken}
	service, err := h.youtubeClient.NewOAuthService(context.Background(), token)
	if err != nil {
		InternalServerError(c, "Failed to create YouTube service")
		return
	}

	// 获取订阅列表
	subscriptions := make([]*model.TbSubscription, 0)
	nextPageToken := ""
	syncCompleted := true

	for {
		call := service.Subscriptions.List([]string{"snippet"}).Mine(true).MaxResults(50)
		if nextPageToken != "" {
			call = call.PageToken(nextPageToken)
		}

		response, err := call.Do()
		if err != nil {
			h.logger.Error("Failed to fetch subscriptions from YouTube",
				zap.String("user_id", userID),
				zap.Error(err))
			syncCompleted = false
			break
		}

		for _, item := range response.Items {
			subscription := &model.TbSubscription{
				UserID:              userID,
				ChannelID:           item.Snippet.ResourceId.ChannelId,
				Platform:            "youtube",
				ChannelTitle:        item.Snippet.Title,
				ChannelDescription:  item.Snippet.Description,
				ChannelThumbnailURL: item.Snippet.Thumbnails.Default.Url,
				SubscribedAt:        time.Now(),
				Status:              "active",
				SyncedAt:            time.Now(),
			}
			subscriptions = append(subscriptions, subscription)
		}

		nextPageToken = response.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	if syncCompleted {
		if err := h.mergeUserSubscriptions(userID, subscriptions, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to save subscriptions",
			})
			return
		}
	}

	h.logger.Info("User TbSubscriptions synced",
		zap.String("user_id", userID),
		zap.Int("count", len(subscriptions)))

	Success(c, gin.H{
		"synced": len(subscriptions),
	})
}

// getVideoTitle 获取视频标题
func (h *YouTubeHandler) getVideoTitle(videoID string) string {
	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		return ""
	}

	call := service.Videos.List([]string{"snippet"}).Id(videoID)
	response, err := call.Do()
	if err != nil || len(response.Items) == 0 {
		return ""
	}

	return response.Items[0].Snippet.Title
}

func (h *YouTubeHandler) mergeUserSubscriptions(userID string, subscriptions []*model.TbSubscription, replaceMissing bool) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user id is required")
	}

	var existing []model.TbSubscription
	if err := h.youtubeService.GetDB().Where("user_id = ?", userID).Find(&existing).Error; err != nil {
		return err
	}

	existingByChannelID := make(map[string]model.TbSubscription, len(existing))
	for _, item := range existing {
		existingByChannelID[item.ChannelID] = item
	}

	seenChannelIDs := make([]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription == nil || strings.TrimSpace(subscription.ChannelID) == "" {
			continue
		}

		seenChannelIDs = append(seenChannelIDs, subscription.ChannelID)
		if existingRecord, ok := existingByChannelID[subscription.ChannelID]; ok {
			subscription.ID = existingRecord.ID
			subscription.Status = existingRecord.Status
			if subscription.Status == "" {
				subscription.Status = "active"
			}
			if !existingRecord.SubscribedAt.IsZero() {
				subscription.SubscribedAt = existingRecord.SubscribedAt
			}
		}

		if err := h.youtubeService.GetDB().Save(subscription).Error; err != nil {
			return err
		}
	}

	if replaceMissing {
		deleteQuery := h.youtubeService.GetDB().Where("user_id = ?", userID)
		if len(seenChannelIDs) > 0 {
			deleteQuery = deleteQuery.Where("channel_id NOT IN ?", seenChannelIDs)
		}
		if err := deleteQuery.Delete(&model.TbSubscription{}).Error; err != nil {
			return err
		}
	}

	return nil
}

// RegisterRoutes 注册YouTube相关路由
func (h *YouTubeHandler) RegisterRoutes(r *gin.Engine, authMiddleware ...gin.HandlerFunc) {
	// var authMW gin.HandlerFunc
	// if len(authMiddleware) > 0 {
	// 	authMW = authMiddleware[0]
	// }

	// 原有路由 (保持向后兼容)
	api := r.Group("/api/youtube")
	{
		api.GET("/oauth/callback", h.OAuthCallback)
		api.GET("/check-auth-status", h.CheckAuthStatus)
		api.GET("/search/:keyword", h.SearchKeyword)
		api.POST("/trending", h.GetTrendingVideos)
		api.GET("/video/:videoId", h.GetVideoDetails)
		api.GET("/categories", h.GetVideoCategories)
		api.GET("/feed/refresh", h.RefreshFeed)
		api.POST("/feed/refresh", h.RefreshFeed) // 同时支持POST
		api.GET("/feed/videos", h.GetLatestVideos)

		// 需要认证的路由
		api.GET("/TbSubscriptions", h.GetUserTbSubscriptions)
		api.POST("/TbSubscriptions/sync", h.SyncUserTbSubscriptions)
		api.PATCH("/TbSubscriptions/:id/status", h.UpdateTbSubscriptionStatus)
	}

	// 新路由格式 (与 web-app 一致)
	v1 := r.Group("/api/v1")
	{
		youtube := v1.Group("/youtube")
		{
			youtube.GET("/oauth/callback", h.OAuthCallback)
			youtube.GET("/check-auth-status", h.CheckAuthStatus)
			youtube.POST("/search/page", h.SearchVideosPage) // 分页搜索
			youtube.POST("/trending", h.GetTrendingVideos)
			youtube.POST("/recommended", h.GetRecommendedVideos)
			youtube.GET("/video/:videoId", h.GetVideoDetails)
			youtube.GET("/categories", h.GetVideoCategories)

			// Feed 相关路由（同时支持GET和POST）
			youtube.GET("/feed/refresh", h.RefreshFeed)
			youtube.POST("/feed/refresh", h.RefreshFeed)
			youtube.GET("/feed/videos", h.GetLatestVideos)

			// 需要认证的路由
			youtube.GET("/TbSubscriptions", h.GetUserTbSubscriptions)
			youtube.POST("/TbSubscriptions/sync", h.SyncUserTbSubscriptions)
			youtube.PATCH("/TbSubscriptions/:id/status", h.UpdateTbSubscriptionStatus)
		}
	}
}

// SearchVideosPage godoc
// @Summary 搜索YouTube视频（分页）
// @Description 使用关键字搜索YouTube视频，支持分页
// @Tags youtube
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索参数"
// @Success 200 {object} map[string]interface{} "videos: array, total: int"
// @Failure 400 {object} map[string]interface{} "error: string"
// @Router /api/v1/youtube/search/page [post]
func (h *YouTubeHandler) SearchVideosPage(c *gin.Context) {
	var req struct {
		Keyword    string `json:"keyword" binding:"required"`
		MaxResults int64  `json:"maxResults"`
		PageToken  string `json:"pageToken"`
		Order      string `json:"order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if req.MaxResults == 0 {
		req.MaxResults = 10
	}
	if req.Order == "" {
		req.Order = "relevance"
	}

	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create YouTube service",
		})
		return
	}

	call := service.Search.List([]string{"snippet"}).
		Q(req.Keyword).
		MaxResults(req.MaxResults).
		Order(req.Order).
		Type("video")

	if req.PageToken != "" {
		call = call.PageToken(req.PageToken)
	}

	response, err := call.Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Search failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"videos":        response.Items,
			"total":         response.PageInfo.TotalResults,
			"nextPageToken": response.NextPageToken,
			"prevPageToken": response.PrevPageToken,
		},
	})
}

// GetRecommendedVideos godoc
// @Summary 获取推荐视频
// @Description 根据视频ID或分类ID获取推荐视频
// @Tags youtube
// @Accept json
// @Produce json
// @Param request body RecommendedVideosRequest true "推荐视频请求参数"
// @Success 200 {object} map[string]interface{} "videos: array"
// @Failure 500 {object} map[string]interface{} "error: string"
// @Router /api/v1/youtube/recommended [post]
func (h *YouTubeHandler) GetRecommendedVideos(c *gin.Context) {
	var req RecommendedVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if req.MaxResults == 0 {
		req.MaxResults = 10
	}
	if req.Order == "" {
		req.Order = "relevance"
	}

	ctx := context.Background()
	service, err := h.youtubeClient.NewAPIService(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create YouTube service",
		})
		return
	}

	call := service.Search.List([]string{"snippet"}).
		MaxResults(req.MaxResults).
		Order(req.Order).
		Type("video")

	// 根据参数进行搜索
	if req.VideoId != "" {
		// 搜索相关视频 (使用Q参数)
		videoTitle := h.getVideoTitle(req.VideoId)
		if videoTitle != "" {
			call = call.Q(videoTitle)
		}
	} else if req.CategoryId != "" {
		// 仅根据分类搜索
		call = call.Q("trending")
	} else {
		// 默认推荐热门视频
		call = call.Q("popular videos")
	}

	response, err := call.Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch recommended videos",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"videos": response.Items,
			"total":  len(response.Items),
		},
	})
}
