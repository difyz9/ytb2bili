package bilibili

import (
	"fmt"
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SaveOrUpdateBiliAccount persists a Bilibili binding record.
func (s *Service) SaveOrUpdateBiliAccount(userID string, biliMid int64, biliName, biliFace, cookies, accessToken, refreshToken string, expiresIn int) (*model.AccountBinding, error) {
	encryptedCookies, err := s.encrypt(cookies)
	if err != nil {
		s.logger.Error("加密cookies失败", zap.Error(err))
		return nil, fmt.Errorf("加密cookies失败: %w", err)
	}

	encryptedAccessToken, err := s.encrypt(accessToken)
	if err != nil {
		s.logger.Error("加密access_token失败", zap.Error(err))
		return nil, fmt.Errorf("加密access_token失败: %w", err)
	}

	encryptedRefreshToken, err := s.encrypt(refreshToken)
	if err != nil {
		s.logger.Error("加密refresh_token失败", zap.Error(err))
		return nil, fmt.Errorf("加密refresh_token失败: %w", err)
	}

	var expiresAt *time.Time
	if expiresIn > 0 {
		expTime := time.Now().Add(time.Duration(expiresIn) * time.Second)
		expiresAt = &expTime
	}

	platformUID := fmt.Sprintf("%d", biliMid)
	platformData := &model.BiliPlatformData{BiliMid: biliMid}

	var existing model.AccountBinding
	err = s.db.Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, model.PlatformBilibili, platformUID).First(&existing).Error

	now := time.Now()

	if err == gorm.ErrRecordNotFound {
		binding := &model.AccountBinding{
			UserID:       userID,
			Platform:     model.PlatformBilibili,
			PlatformUID:  platformUID,
			Username:     biliName,
			Avatar:       biliFace,
			Cookies:      encryptedCookies,
			AccessToken:  encryptedAccessToken,
			RefreshToken: encryptedRefreshToken,
			ExpiresAt:    expiresAt,
			Status:       model.BindingStatusBound,
			LastUsedAt:   &now,
		}
		if setErr := binding.SetBiliData(platformData); setErr != nil {
			return nil, fmt.Errorf("设置B站数据失败: %w", setErr)
		}

		var count int64
		s.db.Model(&model.AccountBinding{}).
			Where("user_id = ? AND platform = ?", userID, model.PlatformBilibili).
			Count(&count)
		binding.IsPrimary = count == 0

		if err := s.db.Create(binding).Error; err != nil {
			s.logger.Error("创建B站账号记录失败", zap.Error(err))
			return nil, fmt.Errorf("创建B站账号记录失败: %w", err)
		}
		s.logger.Info("成功创建B站账号记录",
			zap.String("user_id", userID),
			zap.Int64("bili_mid", biliMid),
			zap.String("bili_name", biliName),
			zap.Bool("is_primary", binding.IsPrimary))
		return binding, nil
	}
	if err != nil {
		s.logger.Error("查询B站账号失败", zap.Error(err))
		return nil, fmt.Errorf("查询B站账号失败: %w", err)
	}

	existing.SetBiliData(platformData)
	updates := map[string]interface{}{
		"username":      biliName,
		"avatar":        biliFace,
		"cookies":       encryptedCookies,
		"access_token":  encryptedAccessToken,
		"refresh_token": encryptedRefreshToken,
		"expires_at":    expiresAt,
		"status":        model.BindingStatusBound,
		"last_used_at":  &now,
	}
	if existing.PlatformData != nil {
		updates["platform_data"] = *existing.PlatformData
	}
	if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
		s.logger.Error("更新B站账号记录失败", zap.Error(err))
		return nil, fmt.Errorf("更新B站账号记录失败: %w", err)
	}
	if err := s.db.Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, model.PlatformBilibili, platformUID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("重新加载B站账号记录失败: %w", err)
	}

	s.logger.Info("成功更新B站账号记录",
		zap.String("user_id", userID),
		zap.Int64("bili_mid", biliMid),
		zap.String("bili_name", biliName))
	return &existing, nil
}

// GetUserBiliAccounts returns all active Bilibili bindings for a user.
func (s *Service) GetUserBiliAccounts(userID string) ([]model.AccountBinding, error) {
	var accounts []model.AccountBinding
	err := s.db.Where("user_id = ? AND platform = ? AND status = ?",
		userID, model.PlatformBilibili, model.BindingStatusBound).
		Order("is_primary DESC, created_at DESC").
		Find(&accounts).Error
	if err != nil {
		s.logger.Error("获取用户B站账号列表失败", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("获取用户B站账号列表失败: %w", err)
	}
	return accounts, nil
}

// GetPrimaryBiliAccount returns the primary bound Bilibili account for a user.
func (s *Service) GetPrimaryBiliAccount(userID string) (*model.AccountBinding, error) {
	var account model.AccountBinding
	err := s.db.Where("user_id = ? AND platform = ? AND is_primary = ? AND status = ?",
		userID, model.PlatformBilibili, true, model.BindingStatusBound).
		First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		s.logger.Error("获取主B站账号失败", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("获取主B站账号失败: %w", err)
	}
	return &account, nil
}

// GetBiliAccountByID returns a specific bound Bilibili account owned by the user.
func (s *Service) GetBiliAccountByID(userID string, accountID uint) (*model.AccountBinding, error) {
	var account model.AccountBinding
	err := s.db.Where("id = ? AND user_id = ? AND platform = ? AND status = ?",
		accountID, userID, model.PlatformBilibili, model.BindingStatusBound).
		First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		s.logger.Error("获取指定B站账号失败", zap.Error(err), zap.String("user_id", userID), zap.Uint("account_id", accountID))
		return nil, fmt.Errorf("获取指定B站账号失败: %w", err)
	}
	return &account, nil
}

// SetPrimaryAccount marks one binding as primary.
func (s *Service) SetPrimaryAccount(userID string, biliMid int64) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ?", userID, model.PlatformBilibili).
		Update("is_primary", false).Error; err != nil {
		tx.Rollback()
		s.logger.Error("清除主账号标志失败", zap.Error(err), zap.String("user_id", userID))
		return fmt.Errorf("清除主账号标志失败: %w", err)
	}

	platformUID := fmt.Sprintf("%d", biliMid)
	if err := tx.Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ? AND platform_uid = ?", userID, model.PlatformBilibili, platformUID).
		Update("is_primary", true).Error; err != nil {
		tx.Rollback()
		s.logger.Error("设置主账号失败", zap.Error(err), zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
		return fmt.Errorf("设置主账号失败: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		s.logger.Error("提交事务失败", zap.Error(err))
		return fmt.Errorf("提交事务失败: %w", err)
	}

	s.logger.Info("成功设置主账号", zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
	return nil
}

// DisableBiliAccount marks a binding as unbound.
func (s *Service) DisableBiliAccount(userID string, biliMid int64) error {
	platformUID := fmt.Sprintf("%d", biliMid)
	if err := s.db.Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ? AND platform_uid = ?", userID, model.PlatformBilibili, platformUID).
		Update("status", model.BindingStatusUnbound).Error; err != nil {
		s.logger.Error("禁用B站账号失败", zap.Error(err), zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
		return fmt.Errorf("禁用B站账号失败: %w", err)
	}
	s.logger.Info("成功禁用B站账号", zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
	return nil
}

// DeleteBiliAccount soft deletes a binding.
func (s *Service) DeleteBiliAccount(userID string, biliMid int64) error {
	platformUID := fmt.Sprintf("%d", biliMid)
	if err := s.db.Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, model.PlatformBilibili, platformUID).
		Delete(&model.AccountBinding{}).Error; err != nil {
		s.logger.Error("删除B站账号失败", zap.Error(err), zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
		return fmt.Errorf("删除B站账号失败: %w", err)
	}
	s.logger.Info("成功删除B站账号", zap.String("user_id", userID), zap.Int64("bili_mid", biliMid))
	return nil
}

// UpdateLastUsed updates the last-used timestamp.
func (s *Service) UpdateLastUsed(account *model.AccountBinding) error {
	now := time.Now()
	return s.db.Model(account).Update("last_used_at", &now).Error
}