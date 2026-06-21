package middleware

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	goauth "github.com/difyz9/go-auth"
	"github.com/gin-gonic/gin"
)

// AuthConfig 认证配置
type AuthConfig struct {
	AppID             string
	AppSecret         string
	CookiesDecryptKey string
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(config AuthConfig) gin.HandlerFunc {
	// 创建 go-auth 配置
	authConfig := goauth.NewConfig()
	authConfig.AddApp(&goauth.AppConfig{
		AppID:     config.AppID,
		AppSecret: config.AppSecret,
		Enabled:   true,
	})

	// 创建认证中间件
	authMiddleware := goauth.NewAuthMiddleware(goauth.Options{
		Config: authConfig,
	})

	// 返回 Gin 处理函数
	return authMiddleware.Authenticate()
}

// DecryptCookies 解密 cookies 中间件
func DecryptCookies(decryptKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只处理 POST 请求
		if c.Request.Method != "POST" && c.Request.Method != "PUT" {
			c.Next()
			return
		}
		
		// 读取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		
		// 如果请求体为空，直接跳过
		if len(bodyBytes) == 0 {
			c.Next()
			return
		}
		
		// 恢复请求体供后续使用
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		
		// 解析请求体，查找 meta 字段
		var data map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			c.Next()
			return
		}

		// 如果存在 meta 字段，尝试解密
		if encryptedCookies, ok := data["meta"].(string); ok && encryptedCookies != "" {
			decryptedCookies, err := decryptData(encryptedCookies, decryptKey)
			if err == nil {
				// 将解密后的 cookies 存储到 context 中
				c.Set("decryptedCookies", decryptedCookies)
			}
		}
		
		c.Next()
	}
}

// decryptData 使用 AES-GCM 解密数据
func decryptData(encryptedBase64 string, keyStr string) (string, error) {
	encryptedBase64 = strings.TrimSpace(encryptedBase64)

	// 解析 Base64（兼容标准/无填充/URL 安全编码）
	combined, err := decodeBase64Compat(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	if len(combined) < 13 {
		return "", fmt.Errorf("密文长度不足: %d", len(combined))
	}

	key := normalizeCookiesDecryptKey(keyStr)

	// 创建 cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 前端协议固定使用 12 字节 IV，后面直接拼接 ciphertext||tag。
	const nonceSize = 12
	if gcm.NonceSize() != nonceSize {
		return "", fmt.Errorf("GCM nonce size 不匹配: %d", gcm.NonceSize())
	}

	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

func normalizeCookiesDecryptKey(keyStr string) []byte {
	normalized := strings.TrimSpace(keyStr)
	if len(normalized) < 32 {
		normalized += strings.Repeat("0", 32-len(normalized))
	}
	if len(normalized) > 32 {
		normalized = normalized[:32]
	}
	return []byte(normalized)
}

func decodeBase64Compat(input string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(input); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(input); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(input); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(input)
}
