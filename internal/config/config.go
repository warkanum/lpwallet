package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	DBDriver    string `yaml:"db_driver"` // "sqlite" or "postgres"; auto-detected if empty
	DBFile      string `yaml:"db_file"`   // path to SQLite file
	DatabaseURL string `yaml:"database_url"`
	DBHost      string `yaml:"db_host"`
	DBPort      int    `yaml:"db_port"`
	DBName      string `yaml:"db_name"`
	DBUser      string `yaml:"db_user"`
	DBPassword  string `yaml:"db_password"`
	DBSSLMode   string `yaml:"db_ssl_mode"`

	TLSCertFile string `yaml:"tls_cert_file"`
	TLSKeyFile  string `yaml:"tls_key_file"`

	LogLevel string `yaml:"log_level"`

	AdminEmail    string `yaml:"admin_email"`
	AdminPassword string `yaml:"admin_password"`
}

type yamlFile struct {
	ListenAddr string `yaml:"listen_addr"`
	Database   struct {
		Driver   string `yaml:"driver"`
		File     string `yaml:"file"`
		URL      string `yaml:"url"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"database"`
	TLS struct {
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`
	LogLevel string `yaml:"log_level"`
	Admin    struct {
		Email    string `yaml:"email"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
}

func defaults() *Config {
	return &Config{
		ListenAddr: "127.0.0.1:8700",
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "lpwallet",
		DBSSLMode:  "prefer",
		LogLevel:   "INFO",
		AdminEmail: "admin@localhost",
	}
}

func Load() (*Config, error) {
	cfg := defaults()

	path := "config.yaml"
	if v := os.Getenv("CONFIG_FILE"); v != "" {
		path = v
	}

	if data, err := os.ReadFile(path); err == nil {
		var f yamlFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
		applyYAML(cfg, &f)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	applyEnv(cfg)

	if cfg.DBDriver == "" {
		switch {
		case cfg.DatabaseURL != "" || cfg.DBHost != "localhost":
			cfg.DBDriver = "postgres"
		case cfg.DBFile != "":
			cfg.DBDriver = "sqlite"
		default:
			return nil, fmt.Errorf("config: database not configured: set DATABASE_URL, DB_HOST, or DB_FILE")
		}
	}
	if cfg.DBDriver == "sqlite" && cfg.DBFile == "" {
		cfg.DBFile = "lpwallet.db"
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyYAML(cfg *Config, f *yamlFile) {
	if f.ListenAddr != "" {
		cfg.ListenAddr = f.ListenAddr
	}
	if f.Database.Driver != "" {
		cfg.DBDriver = f.Database.Driver
	}
	if f.Database.File != "" {
		cfg.DBFile = f.Database.File
	}
	if f.Database.URL != "" {
		cfg.DatabaseURL = f.Database.URL
	}
	if f.Database.Host != "" {
		cfg.DBHost = f.Database.Host
	}
	if f.Database.Port != 0 {
		cfg.DBPort = f.Database.Port
	}
	if f.Database.Name != "" {
		cfg.DBName = f.Database.Name
	}
	if f.Database.User != "" {
		cfg.DBUser = f.Database.User
	}
	if f.Database.Password != "" {
		cfg.DBPassword = f.Database.Password
	}
	if f.Database.SSLMode != "" {
		cfg.DBSSLMode = f.Database.SSLMode
	}
	if f.TLS.CertFile != "" {
		cfg.TLSCertFile = f.TLS.CertFile
	}
	if f.TLS.KeyFile != "" {
		cfg.TLSKeyFile = f.TLS.KeyFile
	}
	if f.LogLevel != "" {
		cfg.LogLevel = f.LogLevel
	}
	if f.Admin.Email != "" {
		cfg.AdminEmail = f.Admin.Email
	}
	if f.Admin.Password != "" {
		cfg.AdminPassword = f.Admin.Password
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.DBDriver = v
	}
	if v := os.Getenv("DB_FILE"); v != "" {
		cfg.DBFile = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DBPort = n
		}
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_SSL_MODE"); v != "" {
		cfg.DBSSLMode = v
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		cfg.TLSCertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		cfg.TLSKeyFile = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ADMIN_EMAIL"); v != "" {
		cfg.AdminEmail = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
}

func validate(cfg *Config) error {
	switch cfg.DBDriver {
	case "postgres":
		if cfg.DatabaseURL == "" && (cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBName == "") {
			return fmt.Errorf("config: postgres requires DATABASE_URL or db_host/db_user/db_name")
		}
	case "sqlite":
		if cfg.DBFile == "" {
			return fmt.Errorf("config: sqlite requires db_file or DB_FILE")
		}
	default:
		return fmt.Errorf("config: unknown db_driver %q (valid: sqlite, postgres)", cfg.DBDriver)
	}
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return fmt.Errorf("config: tls_cert_file and tls_key_file must both be set or both be empty")
	}
	return nil
}

func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword, c.DBSSLMode)
}
