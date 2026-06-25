package main

import (
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/config"
	lpdb "github.com/warkanum/lpwallet/internal/db"
	"github.com/warkanum/lpwallet/internal/handlers"
	"github.com/warkanum/lpwallet/internal/models"
	"github.com/warkanum/lpwallet/internal/routes"
)

// Version is set at build time via -ldflags "-X main.Version=<git-tag>".
var Version string

func main() {
	fmt.Printf("Starting lpwallet Server (version: %s)...\n", buildVersion())
	if err := Run(); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	db, err := lpdb.Open(cfg)
	if err != nil {
		return err
	}

	if err := seedAdmin(db, cfg, logger); err != nil {
		return err
	}

	handlers.Version = buildVersion()

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: routes.New(db, logger),
	}

	if cfg.TLSCertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return err
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		logger.Info("server starting (TLS)", "addr", cfg.ListenAddr)
		return srv.ListenAndServeTLS("", "")
	}

	logger.Info("server starting", "addr", cfg.ListenAddr)
	return srv.ListenAndServe()
}

func seedAdmin(db *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	var count int64
	db.Model(&models.ModelPublicUser{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return nil
	}
	if cfg.AdminPassword == "" {
		logger.Warn("no admin users found and ADMIN_PASSWORD not set — skipping seed")
		return nil
	}
	sum := sha256.Sum256([]byte(cfg.AdminPassword))
	hashed := hex.EncodeToString(sum[:])
	admin := models.ModelPublicUser{
		Email:    sql.NullString{String: cfg.AdminEmail, Valid: true},
		Password: sql.NullString{String: hashed, Valid: true},
		Role:     sql.NullString{String: "admin", Valid: true},
		Name:     sql.NullString{String: "Administrator", Valid: true},
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	logger.Info("seeded admin user", "email", cfg.AdminEmail)
	return nil
}

func buildVersion() string {
	if Version != "" {
		return Version
	}
	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}
	return "dev"
}
