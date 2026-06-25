package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/config"
	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

func TestAuditPluginRecordsCreate(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBFile:   filepath.Join(t.TempDir(), "audit-plugin.db"),
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	caller := &models.ModelPublicUser{IDUser: 42}
	ctx := middleware.WithUser(context.Background(), caller)

	user := models.ModelPublicUser{
		Email:    sql.NullString{String: "audit@example.com", Valid: true},
		Name:     sql.NullString{String: "Audit Test", Valid: true},
		Password: sql.NullString{String: "hashed", Valid: true},
		Role:     sql.NullString{String: "member", Valid: true},
	}

	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	waitForAuditCount(t, db, 1)
}

func TestAuditPluginRecordsCreateEvenWhenOriginalTransactionRollsBack(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBFile:   filepath.Join(t.TempDir(), "audit-rollback.db"),
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	caller := &models.ModelPublicUser{IDUser: 42}
	ctx := middleware.WithUser(context.Background(), caller)

	rollbackErr := errors.New("force rollback")
	user := models.ModelPublicUser{
		Email:    sql.NullString{String: "rollback@example.com", Valid: true},
		Name:     sql.NullString{String: "Rollback Test", Valid: true},
		Password: sql.NullString{String: "hashed", Valid: true},
		Role:     sql.NullString{String: "member", Valid: true},
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error, got %v", err)
	}

	waitForAuditCount(t, db, 1)

	var userCount int64
	if err := db.Model(&models.ModelPublicUser{}).Where("email = ?", "rollback@example.com").Count(&userCount).Error; err != nil {
		t.Fatalf("count rolled back user: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("expected original user insert to roll back, found %d row(s)", userCount)
	}
}

func waitForAuditCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var count int64
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = db.Model(&models.ModelPublicAuditEvent{}).Count(&count).Error
		if lastErr == nil && count >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("count audit events: %v", lastErr)
	}
	t.Fatalf("expected at least %d audit event(s), found %d", want, count)
}
