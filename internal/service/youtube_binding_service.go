package service

import (
	"context"
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type YouTubeBindingResult struct {
	ChannelID    string
	ChannelTitle string
	AvatarURL    string
	Token        *oauth2.Token
}

type YouTubeBindingService struct {
	db            *gorm.DB
	logger        *zap.Logger
	youtubeClient *YouTubeClientFactory
}

func NewYouTubeBindingService(db *gorm.DB, logger *zap.Logger, youtubeClient *YouTubeClientFactory) *YouTubeBindingService {
	return &YouTubeBindingService{
		db:            db,
		logger:        logger,
		youtubeClient: youtubeClient,
	}
}

func (s *YouTubeBindingService) CompleteOAuthBinding(ctx context.Context, userID, code string) (*YouTubeBindingResult, error) {
	if ctx == nil {
		ctx = s.youtubeClient.OAuthContext()
	}

	token, err := s.youtubeClient.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	youtubeService, err := s.youtubeClient.NewOAuthService(ctx, token)
	if err != nil {
		return nil, err
	}

	channelResponse, err := youtubeService.Channels.List([]string{"snippet"}).Mine(true).Do()
	if err != nil {
		return nil, err
	}
	if len(channelResponse.Items) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	channel := channelResponse.Items[0]

	var expiresAt *time.Time
	if !token.Expiry.IsZero() {
		expiresAt = &token.Expiry
	}

	var existingBinding model.AccountBinding
	result := s.db.WithContext(ctx).Where("user_id = ? AND platform = ?", userID, model.PlatformYoutube).First(&existingBinding)
	if result.Error == nil {
		updates := map[string]interface{}{
			"status":        model.BindingStatusBound,
			"platform_uid":  channel.Id,
			"username":      channel.Snippet.Title,
			"avatar":        channel.Snippet.Thumbnails.Default.Url,
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_at":    expiresAt,
		}
		if err := s.db.WithContext(ctx).Model(&existingBinding).Updates(updates).Error; err != nil {
			return nil, err
		}
	} else {
		binding := &model.AccountBinding{
			UserID:       userID,
			Platform:     model.PlatformYoutube,
			PlatformUID:  channel.Id,
			Username:     channel.Snippet.Title,
			Avatar:       channel.Snippet.Thumbnails.Default.Url,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			ExpiresAt:    expiresAt,
			Status:       model.BindingStatusBound,
		}

		if err := s.db.WithContext(ctx).Create(binding).Error; err != nil {
			return nil, err
		}
	}

	return &YouTubeBindingResult{
		ChannelID:    channel.Id,
		ChannelTitle: channel.Snippet.Title,
		AvatarURL:    channel.Snippet.Thumbnails.Default.Url,
		Token:        token,
	}, nil
}

func (s *YouTubeBindingService) SyncSubscriptions(ctx context.Context, userID string, token *oauth2.Token) error {
	if ctx == nil {
		ctx = context.Background()
	}

	youtubeService, err := s.youtubeClient.NewOAuthService(ctx, token)
	if err != nil {
		return err
	}

	subscriptions := make([]*model.TbSubscription, 0)
	nextPageToken := ""

	for {
		call := youtubeService.Subscriptions.List([]string{"snippet"}).Mine(true).MaxResults(50)
		if nextPageToken != "" {
			call = call.PageToken(nextPageToken)
		}

		subsResponse, err := call.Do()
		if err != nil {
			return err
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

		nextPageToken = subsResponse.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	if len(subscriptions) == 0 {
		return nil
	}

	if err := s.db.WithContext(ctx).Where("user_id = ? AND platform = ?", userID, "youtube").Delete(&model.TbSubscription{}).Error; err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Create(&subscriptions).Error; err != nil {
		return err
	}

	s.logger.Info("YouTube订阅列表已保存",
		zap.String("user_id", userID),
		zap.Int("count", len(subscriptions)))

	return nil
}
