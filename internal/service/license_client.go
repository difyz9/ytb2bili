package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
)

// ── Types ────────────────────────────────────────────────────────────────────

// LicenseVerifyResponse 是 Worker 验证 License 的响应格式
type LicenseVerifyResponse struct {
	License *LicenseInfo `json:"license"`
	Error   string       `json:"error,omitempty"`
}

// LicenseInfo 从 Worker 返回的许可证信息
type LicenseInfo struct {
	LicenseKey string `json:"licenseKey"`
	Plan       string `json:"plan"`
	Status     string `json:"status"`
	ExpiresAt  string `json:"expiresAt"`
	Metadata   *struct {
		Email   string `json:"email"`
		Product string `json:"product"`
	} `json:"metadata,omitempty"`
}

// VerifyResult 业务层使用的验证结果
type VerifyResult struct {
	Valid     bool
	LicenseKey string
	Plan      string
	Tier      string
	ExpiresAt *time.Time
	Product   string
	Message   string
}

// ── LicenseClient ─────────────────────────────────────────────────────────────

// LicenseClient 验证激活码的 HTTP 客户端
type LicenseClient struct {
	verifyURL string
	adminKey  string
	http      *http.Client
	logger    *zap.Logger
}

// NewLicenseClient 创建 LicenseClient
func NewLicenseClient(cfg *config.AppConfig, logger *zap.Logger) *LicenseClient {
	baseURL := strings.TrimRight(cfg.License.VerifyURL, "/")
	return &LicenseClient{
		verifyURL: baseURL,
		adminKey:  strings.TrimSpace(cfg.License.AdminKey),
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// IsConfigured 返回是否已配置验证服务
func (c *LicenseClient) IsConfigured() bool {
	return c.verifyURL != "" && c.adminKey != ""
}

// Verify 验证激活码，返回许可证信息
func (c *LicenseClient) Verify(licenseKey string) (*VerifyResult, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("License 验证服务未配置：请在 config.toml 中设置 [license] verify_url 和 admin_key")
	}

	key := strings.TrimSpace(licenseKey)
	if key == "" {
		return nil, fmt.Errorf("激活码不能为空")
	}

	u, _ := url.Parse(c.verifyURL + "/v1/admin/licenses")
	q := u.Query()
	q.Set("licenseKey", key)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建验证请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.adminKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("验证请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 404 = 许可证不存在
	if resp.StatusCode == http.StatusNotFound {
		return &VerifyResult{
			Valid:   false,
			Message: "激活码无效，请检查后重试",
		}, nil
	}

	// 其他非 2xx 错误
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("验证服务返回错误（%d）: %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("验证服务返回状态码: %d", resp.StatusCode)
	}

	// 解析成功响应
	var result LicenseVerifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析验证响应失败: %w", err)
	}

	if result.License == nil {
		return &VerifyResult{
			Valid:   false,
			Message: "激活码未找到",
		}, nil
	}

	li := result.License

	// 检查许可证状态
	if li.Status != "active" {
		return &VerifyResult{
			Valid:      false,
			LicenseKey: li.LicenseKey,
			Plan:       li.Plan,
			Message:    fmt.Sprintf("许可证状态为 %s，不是 active", li.Status),
		}, nil
	}

	vr := &VerifyResult{
		Valid:       true,
		LicenseKey:  li.LicenseKey,
		Plan:        li.Plan,
		Tier:        mapTier(li.Plan),
		Product:     "",
		Message:     "验证成功",
	}

	// 解析产品名
	if li.Metadata != nil {
		vr.Product = li.Metadata.Product
	}

	// 解析过期时间
	if li.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, li.ExpiresAt)
		if err == nil {
			vr.ExpiresAt = &t
		}
	}

	return vr, nil
}

// mapTier 将 plan 映射为 tier
func mapTier(plan string) string {
	lower := strings.ToLower(plan)
	switch {
	case strings.Contains(lower, "enterprise"):
		return "enterprise"
	case strings.Contains(lower, "pro"):
		return "pro"
	case strings.Contains(lower, "standard"):
		return "standard"
	case strings.Contains(lower, "basic"):
		return "basic"
	default:
		return "pro"
	}
}
