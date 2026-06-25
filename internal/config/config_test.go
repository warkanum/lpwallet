package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lpwallet")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8700", cfg.ListenAddr)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "admin@localhost", cfg.AdminEmail)
}

func TestDSN_URL(t *testing.T) {
	cfg := &Config{DatabaseURL: "postgres://user:pass@host/db"}
	assert.Equal(t, "postgres://user:pass@host/db", cfg.DSN())
}

func TestDSN_Fields(t *testing.T) {
	cfg := &Config{
		DBHost: "localhost", DBPort: 5432, DBName: "lpwallet",
		DBUser: "user", DBPassword: "pass", DBSSLMode: "disable",
	}
	assert.Equal(t, "host=localhost port=5432 dbname=lpwallet user=user password=pass sslmode=disable", cfg.DSN())
}

func TestValidate_NoDatabase(t *testing.T) {
	clearEnv(t)
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database not configured")
}

func TestValidate_TLSPartial(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lpwallet")
	t.Setenv("TLS_CERT_FILE", "/some/cert.pem")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls_cert_file and tls_key_file")
}

func TestEnvOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lpwallet")
	t.Setenv("LISTEN_ADDR", "0.0.0.0:9000")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9000", cfg.ListenAddr)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CONFIG_FILE", "LISTEN_ADDR", "DATABASE_URL", "DB_HOST", "DB_PORT",
		"DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE",
		"TLS_CERT_FILE", "TLS_KEY_FILE", "LOG_LEVEL", "ADMIN_EMAIL", "ADMIN_PASSWORD",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck
	}
	// Point config file at a nonexistent path so YAML loading is skipped
	t.Setenv("CONFIG_FILE", "/nonexistent/config.yaml")
}
