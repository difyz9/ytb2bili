package service

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const defaultYouTubeAPIKey = "AIzaSyAS5ZTsMgLDw5xfUWQkOvTDIDu_LlZrWuU"

type YouTubeClientFactory struct {
	logger      *zap.Logger
	oauthConfig *oauth2.Config
	httpClient  *http.Client
	oauthCtx    context.Context
	apiKey      string
}

func NewYouTubeClientFactory(logger *zap.Logger, cfg *config.AppConfig) *YouTubeClientFactory {
	transport := &http.Transport{
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}

	if cfg != nil && cfg.Workflow.ProxyURL != "" {
		if proxyURL, err := url.Parse(cfg.Workflow.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			logger.Info("YouTube OAuth 使用代理", zap.String("proxy", cfg.Workflow.ProxyURL))
		} else {
			logger.Warn("代理地址解析失败，不使用代理", zap.String("proxy_url", cfg.Workflow.ProxyURL), zap.Error(err))
		}
	}

	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	oauthConfig := &oauth2.Config{
		ClientID:     "",
 		ClientSecret: "",
		RedirectURL:  "",
		Scopes:       []string{youtube.YoutubeReadonlyScope},
		Endpoint:     google.Endpoint,
	}

	oauthCtx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	return &YouTubeClientFactory{
		logger:      logger,
		oauthConfig: oauthConfig,
		httpClient:  httpClient,
		oauthCtx:    oauthCtx,
		apiKey:      defaultYouTubeAPIKey,
	}
}

func (f *YouTubeClientFactory) OAuthConfig() *oauth2.Config {
	if f == nil {
		return nil
	}
	return f.oauthConfig
}

func (f *YouTubeClientFactory) OAuthContext() context.Context {
	if f == nil || f.oauthCtx == nil {
		return context.Background()
	}
	return f.oauthCtx
}

func (f *YouTubeClientFactory) RedirectURL() string {
	if f == nil || f.oauthConfig == nil {
		return ""
	}
	return f.oauthConfig.RedirectURL
}

func (f *YouTubeClientFactory) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	if f == nil || f.oauthConfig == nil {
		return ""
	}
	return f.oauthConfig.AuthCodeURL(state, opts...)
}

func (f *YouTubeClientFactory) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if ctx == nil {
		ctx = f.OAuthContext()
	}
	return f.oauthConfig.Exchange(ctx, code)
}

func (f *YouTubeClientFactory) NewOAuthService(ctx context.Context, token *oauth2.Token) (*youtube.Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := f.oauthConfig.Client(ctx, token)
	return youtube.NewService(ctx, option.WithHTTPClient(client))
}

func (f *YouTubeClientFactory) NewAPIService(ctx context.Context) (*youtube.Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return youtube.NewService(ctx, option.WithAPIKey(f.apiKey))
}
