package store

import (
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabase 自动迁移数据库表
func MigrateDatabase(db *gorm.DB) error {
	fmt.Println("📦 开始 GORM AutoMigrate...")

	// 1. 先运行 AutoMigrate 创建表
	if err := db.AutoMigrate(
		&model.User{},
		&model.Video{},             // 视频元数据
		&model.TaskStep{},          // 任务步骤
		&model.App{},               // 应用
		&model.UserToken{},         // 用户令牌
		&model.UserPreference{},    // 用户偏好设置
		&model.UserSettings{},      // 用户设置
		&model.SystemSettings{},    // 系统设置
		&model.EmailVerification{}, // 邮箱验证码
		&model.LicenseActivation{}, // 许可证激活记录
		&model.UserMembership{},    // 用户会员信息
		&model.TbSubscription{},    // 用户订阅频道
		&model.Channel{},           // 频道信息
		&model.AccountBinding{},     // 账号绑定
		&model.BiliSubtitleUpload{}, // B站字幕上传状态
		&model.AgentClient{},       // 第三方 agent 应用
		&model.AgentAPIKey{},       // agent API key
		&model.AgentRequestLog{},   // agent 请求日志
		&model.AgentJob{},          // agent 异步作业
	); err != nil {
		return err
	}

	fmt.Println("✅ AutoMigrate 完成")

	// 2. 修正可能以 VARCHAR/TEXT 建立的大字段（AutoMigrate 不会缩小/扩大已有列类型）
	if err := db.Exec(`ALTER TABLE tb_videos MODIFY COLUMN subtitles MEDIUMTEXT`).Error; err != nil {
		// 表不存在或列不存在时会报错，忽略即可
		fmt.Printf("⚠️  修正 subtitles 列类型时跳过: %v\n", err)
	}

	// 2b. 补全新增的字幕上传标记列（老库升级时 AutoMigrate 不会添加带 DEFAULT 的布尔列）
	if err := db.Exec(`ALTER TABLE tb_videos ADD COLUMN IF NOT EXISTS bili_subtitle_uploaded TINYINT(1) NOT NULL DEFAULT 0`).Error; err != nil {
		fmt.Printf("⚠️  添加 bili_subtitle_uploaded 列时跳过: %v\n", err)
	}

	// 3. 初始化种子数据（如初始用户）
	fmt.Println("📝 检查初始数据...")
	return seedInitialData(db)
}

// seedInitialData 初始化种子数据
func seedInitialData(db *gorm.DB) error {
	fmt.Println("🔍 检查用户数量...")

	// 检查是否有用户
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		fmt.Printf("⚠️ 检查用户数量失败: %v\n", err)
		return fmt.Errorf("检查用户数量失败: %w", err)
	}

	fmt.Printf("📊 当前用户数: %d\n", count)

	if count == 0 {
		fmt.Println("🌟 数据库为空，正在创建初始管理员用户...")
		user := model.User{
			Username: "admin",
			Email:    "admin@gmail.com",
			Role:     "admin",
			Status:   1,
		}
		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("创建初始用户失败: %w", err)
		}
		fmt.Println("✅ 初始管理员创建成功: admin@gmail.com")
	} else {
		fmt.Printf("✅ 已有 %d 个用户，跳过初始化\n", count)
	}
	return nil
}

// runCustomMigrations 运行需要手动处理的迁移
func runCustomMigrations(db *gorm.DB) error {
	// 添加 role 字段到 users 表（如果不存在）
	return addRoleFieldIfNotExists(db)
}

// addRoleFieldIfNotExists 添加 role 字段到 users 表（如果不存在）
func addRoleFieldIfNotExists(db *gorm.DB) error {
	// 检查字段是否已存在
	var columnExists bool
	checkSQL := `
		SELECT COUNT(*) as count
		FROM information_schema.columns
		WHERE table_name = 'tb_users'
		AND column_name = 'role'
		AND table_schema = DATABASE()
	`

	if db.Dialector.Name() == "sqlite" {
		checkSQL = `
			SELECT COUNT(*) as count
			FROM pragma_table_info('tb_users')
			WHERE name = 'role'
		`
	}

	result := db.Raw(checkSQL).Scan(&columnExists)
	if result.Error != nil {
		return fmt.Errorf("检查 role 字段失败: %w", result.Error)
	}

	if columnExists {
		// 字段已存在，检查是否需要设置管理员
		var adminCount int64
		if err := db.Table("tb_users").Where("role = ? OR role IS NULL", "admin").Count(&adminCount).Error; err != nil {
			return fmt.Errorf("检查管理员数量失败: %w", err)
		}

		// 检查是否有空角色需要填充默认值
		var emptyRoleCount int64
		if err := db.Table("tb_users").Where("role = ? OR role IS NULL", "").Count(&emptyRoleCount).Error; err == nil && emptyRoleCount > 0 {
			fmt.Printf("📝 发现 %d 个用户的 role 字段为空，填充默认值 'user'...\n", emptyRoleCount)
			db.Table("tb_users").Where("role = ? OR role IS NULL", "").Update("role", "user")
		}

		if adminCount == 0 {
			// 如果没有管理员，将第一个用户设为管理员
			var userID uint
			if err := db.Table("tb_users").Order("id ASC").Limit(1).Select("id").Scan(&userID).Error; err != nil {
				return fmt.Errorf("查询第一个用户失败: %w", err)
			}

			if userID > 0 {
				if err := db.Table("tb_users").Where("id = ?", userID).Update("role", "admin").Error; err != nil {
					return fmt.Errorf("设置管理员失败: %w", err)
				}
				fmt.Printf("✅ 用户 ID=%d 已设为管理员\n", userID)
			}
		} else {
			fmt.Printf("✅ 已有 %d 个管理员，跳过管理员设置\n", adminCount)
		}
		return nil
	}

	// 添加字段
	fmt.Println("📝 添加 role 字段到 tb_users 表...")
	alterSQL := `
		ALTER TABLE tb_users
		ADD COLUMN role VARCHAR(20) DEFAULT 'user'
		COMMENT '用户角色: admin/user'
	`

	if db.Dialector.Name() == "sqlite" {
		alterSQL = `
			ALTER TABLE tb_users
			ADD COLUMN role VARCHAR(20) DEFAULT 'user'
		`
	}

	if err := db.Exec(alterSQL).Error; err != nil {
		return fmt.Errorf("添加 role 字段失败: %w", err)
	}

	// 创建索引
	fmt.Println("📝 创建 role 索引...")
	if db.Dialector.Name() != "sqlite" {
		indexSQL := `CREATE INDEX idx_users_role ON tb_users(role)`
		if err := db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("创建 role 索引失败: %w", err)
		}
	} else {
		fmt.Println("✅ SQLite 索引已自动创建")
	}

	// 将第一个用户设为管理员
	var userID uint
	if err := db.Order("id ASC").Limit(1).Select("id").Scan(&userID).Error; err == nil && userID > 0 {
		db.Table("tb_users").Where("id = ?", userID).Update("role", "admin")
		fmt.Printf("✅ 用户 ID=%d 已设为管理员\n", userID)
	}

	fmt.Println("✅ role 字段迁移完成!")
	return nil
}
