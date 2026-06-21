package service

import (
	"context"
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BindingService 封装平台账号绑定相关的数据库操作。
// 支持 Bilibili、YouTube 等平台的 OAuth 绑定管理。
type BindingService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewBindingService(db *gorm.DB, logger *zap.Logger) *BindingService {
	return &BindingService{db: db, logger: logger}
}

// GetDB 返回底层 *gorm.DB（用于 handler 中尚未迁移的复杂查询）
func (s *BindingService) GetDB() *gorm.DB { return s.db }

// ── 查询 ─────────────────────────────────────────────────────────────────────

// GetByID 根据主键查询绑定记录
func (s *BindingService) GetByID(ctx context.Context, id uint) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	if err := s.db.WithContext(ctx).First(&binding, id).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetByField 根据字段查询绑定记录（如 qr_code_key）
func (s *BindingService) GetByField(ctx context.Context, field, value string) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	if err := s.db.WithContext(ctx).Where(field+" = ?", value).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetByUserAndPlatform 查询用户在指定平台上的绑定
func (s *BindingService) GetByUserAndPlatform(ctx context.Context, userID, platform string) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	if err := s.db.WithContext(ctx).Where("user_id = ? AND platform = ?", userID, platform).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

// ListByUser 查询用户的所有绑定
func (s *BindingService) ListByUser(ctx context.Context, userID string) ([]model.AccountBinding, error) {
	var list []model.AccountBinding
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ── 增删改 ───────────────────────────────────────────────────────────────────

// Create 创建绑定记录
func (s *BindingService) Create(ctx context.Context, binding *model.AccountBinding) error {
	return s.db.WithContext(ctx).Create(binding).Error
}

// Update 更新绑定记录字段
func (s *BindingService) Update(ctx context.Context, binding *model.AccountBinding, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(binding).Updates(updates).Error
}

// Delete 软删除绑定记录
func (s *BindingService) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.AccountBinding{}, id).Error
}

// UpdateStatus 更新绑定状态
func (s *BindingService) UpdateStatus(ctx context.Context, id uint, status string) error {
	return s.db.WithContext(ctx).Model(&model.AccountBinding{}).
		Where("id = ?", id).Update("status", status).Error
}

// SetPrimary 设置主绑定
func (s *BindingService) SetPrimary(ctx context.Context, userID string, platform string, bindingID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先取消该用户该平台的所有主绑定
		if err := tx.Model(&model.AccountBinding{}).
			Where("user_id = ? AND platform = ?", userID, platform).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		// 设置新的主绑定
		return tx.Model(&model.AccountBinding{}).
			Where("id = ?", bindingID).
			Update("is_primary", true).Error
	})
}

// FirstOrCreateByField 根据字段查找，不存在则创建
func (s *BindingService) FirstOrCreateByField(ctx context.Context, field, value string, binding *model.AccountBinding) (*model.AccountBinding, error) {
	existing, err := s.GetByField(ctx, field, value)
	if err == gorm.ErrRecordNotFound {
		if createErr := s.Create(ctx, binding); createErr != nil {
			return nil, createErr
		}
		return binding, nil
	}
	if err != nil {
		return nil, err
	}
	return existing, nil
}

// ── 检查 ─────────────────────────────────────────────────────────────────────

// ExistsByUserAndPlatform 检查用户是否已绑定平台
func (s *BindingService) ExistsByUserAndPlatform(ctx context.Context, userID, platform string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountByPlatform 统计某平台的绑定数
func (s *BindingService) CountByPlatform(ctx context.Context, platform string) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AccountBinding{}).
		Where("platform = ?", platform).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// EnsureBinding 确保绑定记录存在（创建或返回已有）
func (s *BindingService) EnsureBinding(ctx context.Context, userID, platform, platformUID string, defaults map[string]interface{}) (*model.AccountBinding, error) {
	if _, err := s.GetByField(ctx, "platform_uid", platformUID); err == nil {
		return nil, fmt.Errorf("该账号已绑定其他用户")
	}
	existing, err := s.GetByUserAndPlatform(ctx, userID, platform)
	if err == nil {
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	binding := &model.AccountBinding{
		UserID: userID, Platform: model.Platform(platform),
		PlatformUID: platformUID, Status: model.BindingStatusBound,
	}
	if err := s.Create(ctx, binding); err != nil {
		return nil, err
	}
	return binding, nil
}
