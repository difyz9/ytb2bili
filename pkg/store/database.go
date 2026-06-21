package store

import (
	"fmt"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/fx"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Module provides the database connection
var Module = fx.Options(
	fx.Provide(
		NewDatabase,
	),
)

// NewDatabase creates a new GORM database connection.
// Supported: postgres/mysql/sqlite.
func NewDatabase(appCfg *config.AppConfig) (*gorm.DB, error) {
	// GORM configuration
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   appCfg.Database.TablePrefix,
			SingularTable: false,
		},
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用自动外键约束（使用字符串UID而非自增ID）
	}

	// Logging
	if appCfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	// Create connection
	var (
		db  *gorm.DB
		err error
	)

	switch appCfg.Database.Type {
	case "postgres", "postgresql":
		dsn := appCfg.Database.GetDSN()
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
	case "mysql":
		dsn := appCfg.Database.GetDSN()
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: postgres, mysql)", appCfg.Database.Type)
	}

	// Connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移表结构
	fmt.Println("🚀 开始数据库迁移...")
	if err := MigrateDatabase(db); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	fmt.Println("✅ 数据库迁移完成!")

	return db, nil
}
