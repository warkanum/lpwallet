package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/warkanum/lpwallet/internal/config"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	switch cfg.DBDriver {
	case "sqlite":
		return openSQLite(cfg)
	default:
		return openPostgres(cfg)
	}
}

func openPostgres(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("db: open postgres: %w", err)
	}

	if err := db.Use(&AuditPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register audit plugin: %w", err)
	}

	if err := db.Use(&TransactionPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register transaction plugin: %w", err)
	}

	return db, nil
}
