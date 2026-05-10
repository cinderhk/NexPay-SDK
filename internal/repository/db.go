package repository

import (
	"fmt"

	"github.com/nexpay/nexpay-sdk/internal/config"
	"github.com/nexpay/nexpay-sdk/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB 创建 *gorm.DB 连接池（不执行迁移）
//
// 迁移请单独执行 Migrate(db) 或通过命令行 `nexpay migrate` 触发，
// 避免每次进程启动都跑一次 DDL
func NewDB(cfg config.MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

// Migrate 执行所有模型的 GORM AutoMigrate
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Order{},
		&model.Refund{},
		&model.PaymentNotifyLog{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
