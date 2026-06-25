package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/warkanum/lpwallet/internal/config"
	"github.com/warkanum/lpwallet/internal/models"
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

	if err := migrate(db); err != nil {
		return nil, err
	}

	if err := db.Use(&AuditPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register audit plugin: %w", err)
	}

	if err := db.Use(&TransactionPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register transaction plugin: %w", err)
	}

	return db, nil
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.ModelPublicUser{},
		&models.ModelPublicAccount{},
		&models.ModelPublicAccountTransaction{},
		&models.ModelPublicAuditEvent{},
		&models.ModelPublicAuditDetail{},
		&models.ModelPublicUserSession{},
	); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	return nil
}
