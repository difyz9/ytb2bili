package bilibili

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type credentialFile struct {
	UserID       string `json:"user_id"`
	PlatformUID  string `json:"platform_uid"`
	Username     string `json:"username"`
	Avatar       string `json:"avatar"`
	Cookies      string `json:"cookies"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IsPrimary    bool   `json:"is_primary"`
	Status       string `json:"status"`
	PlatformData string `json:"platform_data,omitempty"`
	UpdatedAt    string `json:"updated_at"`
}

func credentialFilePath(dir, userID string) string {
	return filepath.Join(dir, userID, "bilibili.json")
}

// SaveCredentialToFile persists encrypted credentials to local storage.
func (s *Service) SaveCredentialToFile(account *model.AccountBinding) error {
	if s.credentialsDir == "" {
		return nil
	}

	userDir := filepath.Join(s.credentialsDir, account.UserID)
	if err := os.MkdirAll(userDir, 0700); err != nil {
		return fmt.Errorf("创建凭证目录失败: %w", err)
	}

	var platformData string
	if account.PlatformData != nil {
		platformData = *account.PlatformData
	}

	record := credentialFile{
		UserID:       account.UserID,
		PlatformUID:  account.PlatformUID,
		Username:     account.Username,
		Avatar:       account.Avatar,
		Cookies:      account.Cookies,
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		IsPrimary:    account.IsPrimary,
		Status:       string(account.Status),
		PlatformData: platformData,
		UpdatedAt:    time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}

	path := credentialFilePath(s.credentialsDir, account.UserID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入凭证文件失败: %w", err)
	}

	return nil
}

// LoadCredentialFromFile loads a user's encrypted Bilibili binding from a local file.
func (s *Service) LoadCredentialFromFile(userID string) (*model.AccountBinding, error) {
	if s.credentialsDir == "" {
		return nil, nil
	}

	path := credentialFilePath(s.credentialsDir, userID)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取凭证文件失败: %w", err)
	}

	var record credentialFile
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("解析凭证文件失败: %w", err)
	}

	binding := &model.AccountBinding{
		UserID:       record.UserID,
		Platform:     model.PlatformBilibili,
		PlatformUID:  record.PlatformUID,
		Username:     record.Username,
		Avatar:       record.Avatar,
		Cookies:      record.Cookies,
		AccessToken:  record.AccessToken,
		RefreshToken: record.RefreshToken,
		IsPrimary:    record.IsPrimary,
		Status:       model.BindingStatus(record.Status),
	}
	if record.PlatformData != "" {
		platformData := record.PlatformData
		binding.PlatformData = &platformData
	}

	s.logger.Info("已从本地文件加载B站凭证",
		zap.String("user_id", userID),
		zap.String("username", record.Username),
		zap.String("path", path))
	return binding, nil
}

// GetPrimaryBiliAccountWithFile prefers a local credential file and falls back to the database.
func (s *Service) GetPrimaryBiliAccountWithFile(userID string) (*model.AccountBinding, error) {
	account, err := s.LoadCredentialFromFile(userID)
	if err != nil {
		s.logger.Warn("从文件加载B站凭证失败，回退到数据库", zap.String("user_id", userID), zap.Error(err))
	}
	if account != nil {
		return account, nil
	}
	return s.GetPrimaryBiliAccount(userID)
}

// GetAnyActiveBiliAccountWithFile returns the first available bound account without requiring a user ID.
func (s *Service) GetAnyActiveBiliAccountWithFile() (*model.AccountBinding, error) {
	if s.credentialsDir != "" {
		entries, err := os.ReadDir(s.credentialsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				account, loadErr := s.LoadCredentialFromFile(entry.Name())
				if loadErr == nil && account != nil {
					s.logger.Info("从凭证目录加载B站账号",
						zap.String("user_id", account.UserID),
						zap.String("username", account.Username))
					return account, nil
				}
			}
		}
	}

	var account model.AccountBinding
	err := s.db.Where("platform = ? AND status = ?", model.PlatformBilibili, model.BindingStatusBound).
		Order("is_primary DESC, last_used_at DESC").
		First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询B站账号失败: %w", err)
	}
	return &account, nil
}