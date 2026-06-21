package analytics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	analytics "github.com/difyz9/go-analysis-client"
	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Client Analytics 客户端包装器
type Client struct {
	client *analytics.Client
	logger *zap.Logger
}

// Module Analytics FX 模块
var Module = fx.Module("analytics",
	fx.Provide(NewClient),
)

// NewClient 创建 Analytics 客户端
func NewClient(cfg *config.AppConfig, logger *zap.Logger, lc fx.Lifecycle) (*Client, error) {
	if !cfg.Analytics.Enabled {
		logger.Info("Analytics 数据统计已禁用")
		return &Client{logger: logger}, nil
	}

	// 创建 analytics 客户端
	client := analytics.NewClient(
		cfg.Analytics.ServerURL,
		cfg.Analytics.ProductName,
		analytics.WithDebug(cfg.Analytics.Debug),
		analytics.WithLogger(&zapAdapter{logger: logger}),
		analytics.WithBatchSize(50),
		analytics.WithFlushInterval(10*time.Second),
		analytics.WithBufferSize(1000),
	)

	// logger.Info("Analytics 客户端已初始化",
	// 	zap.String("server_url", cfg.Analytics.ServerURL),
	// 	zap.String("product_name", cfg.Analytics.ProductName),
	// 	zap.Bool("debug", cfg.Analytics.Debug))

	ac := &Client{
		client: client,
		logger: logger,
	}

	// 注册生命周期钩子
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// 上报安装信息
			if client != nil {
				client.ReportInstallWithCallback(func(err error) {
					if err != nil {
						//logger.Warn("上报安装信息失败", zap.Error(err))
					} else {
						//logger.Info("安装信息已上报")
					}
				})

				// 记录应用启动（含公网 IP，方便服务端归因）
				client.TrackAppLaunch(map[string]interface{}{
					"version":    "0.1.20",
					"server_url": cfg.Analytics.ServerURL,
					"ip_address": getPublicIP(),
				})
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if client != nil {
				// 记录应用退出
				client.TrackAppExit(map[string]interface{}{
					"reason": "normal",
				})
				
				// 在单独的 goroutine 中关闭客户端，避免阻塞
				done := make(chan error, 1)
				go func() {
					// 先尝试刷新缓冲区
					client.Flush()
					// 关闭客户端
					done <- client.Close()
				}()
				
				// 等待关闭完成或超时
				select {
				case err := <-done:
					if err != nil {
						logger.Warn("关闭 Analytics 客户端时出错", zap.Error(err))
					} else {
						logger.Info("Analytics 客户端已关闭")
					}
					return err
				case <-ctx.Done():
					logger.Warn("关闭 Analytics 客户端超时，强制继续")
					return nil // 不返回错误，让其他服务继续关闭
				case <-time.After(2 * time.Second):
					logger.Warn("Analytics 客户端关闭超过2秒，跳过等待")
					return nil
				}
			}
			return nil
		},
	})

	return ac, nil
}

// Track 发送事件（异步）
func (c *Client) Track(eventName string, properties map[string]interface{}) {
	if c.client == nil {
		return
	}
	c.client.Track(eventName, properties)
}

// TrackBatch 批量发送事件
func (c *Client) TrackBatch(events []analytics.Event) {
	if c.client == nil {
		return
	}
	c.client.TrackBatch(events)
}

// Flush 强制刷新缓冲区
func (c *Client) Flush() {
	if c.client == nil {
		return
	}
	c.client.Flush()
}

// SetUserID 设置用户ID
func (c *Client) SetUserID(userID string) {
	if c.client == nil {
		return
	}
	c.client.SetUserID(userID)
}

// GetDeviceID 获取设备ID
func (c *Client) GetDeviceID() string {
	if c.client == nil {
		return ""
	}
	return c.client.GetDeviceID()
}

// GetSessionID 获取会话ID
func (c *Client) GetSessionID() string {
	if c.client == nil {
		return ""
	}
	return c.client.GetSessionID()
}

// zapAdapter 适配 zap.Logger 到 analytics.Logger
type zapAdapter struct {
	logger *zap.Logger
}

func (z *zapAdapter) Printf(format string, v ...interface{}) {
	z.logger.Sugar().Infof(format, v...)
}

// getPublicIP 获取服务器公网 IP 地址，供启动事件上报使用。
// 依次尝试多个查询服务，任意一个成功即返回；失败时返回空字符串。
func getPublicIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for _, svc := range services {
		resp, err := client.Get(svc)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				if ip := strings.TrimSpace(string(body)); ip != "" {
					return ip
				}
			}
		}
	}
	return ""
}
