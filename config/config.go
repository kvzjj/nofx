package config

import (
	"os"
	"strconv"
	"strings"
)

// Global configuration instance
var global *Config

// Config is the global configuration (loaded from .env)
// Only contains truly global config, trading related config is at trader/strategy level
type Config struct {
	// Service configuration
	APIServerPort       int
	JWTSecret           string
	RegistrationEnabled bool
	MaxUsers            int // Maximum number of users allowed (0 = unlimited, default = 1)

	// Security configuration
	// TransportEncryption enables browser-side encryption for API keys
	// Requires HTTPS or localhost. Set to false for HTTP access via IP.
	TransportEncryption bool

	// Quantitative data service (fund flow / OI rankings).
	// QuantDataAPIBase is the base URL of the data service; QuantDataAuthKey
	// is its credential and MUST be provided via environment. It is never
	// hardcoded so secrets do not leak through the open-source repo.
	QuantDataAPIBase string
	QuantDataAuthKey string

	// API rate limiting (requests per minute).
	RateLimitEnabled       bool
	RateLimitAuthPerMin    int // login / register / OTP / password reset, per IP
	RateLimitPublicPerMin  int // unauthenticated endpoints, per IP
	RateLimitUserPerMin    int // authenticated endpoints, per user
	RateLimitSensitivePerMin int // crypto/decrypt and similar, per IP

	// Metrics endpoint: when METRICS_TOKEN is set, /api/metrics requires it
	// (Bearer or ?token=); otherwise authenticated users can scrape.
	MetricsToken string

	// Scheduled SQLite backups.
	BackupEnabled        bool
	BackupDir            string
	BackupIntervalHours  int
	BackupRetentionCount int

	// Audit log retention in days (0 = keep forever).
	AuditRetentionDays int
	// Notification history retention in days (0 = keep forever).
	NotificationRetentionDays int
}

// Init initializes global configuration (from .env)
func Init() {
	cfg := &Config{
		APIServerPort:       8080,
		RegistrationEnabled: true,
		MaxUsers:            1, // Default: only 1 user allowed
	}

	// Load from environment variables
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = strings.TrimSpace(v)
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "default-jwt-secret-change-in-production"
	}

	if v := os.Getenv("REGISTRATION_ENABLED"); v != "" {
		cfg.RegistrationEnabled = strings.ToLower(v) == "true"
	}

	if v := os.Getenv("MAX_USERS"); v != "" {
		if maxUsers, err := strconv.Atoi(v); err == nil && maxUsers >= 0 {
			cfg.MaxUsers = maxUsers
		}
	}

	if v := os.Getenv("API_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.APIServerPort = port
		}
	}

	// Transport encryption: default false for easier deployment
	// Set TRANSPORT_ENCRYPTION=true to enable (requires HTTPS or localhost)
	if v := os.Getenv("TRANSPORT_ENCRYPTION"); v != "" {
		cfg.TransportEncryption = strings.EqualFold(strings.TrimSpace(v), "true")
	}

	// Quantitative data service. The base URL is not a secret and keeps its
	// historical default; the auth key defaults to empty so quant data stays
	// disabled until an operator explicitly configures a credential.
	cfg.QuantDataAPIBase = "http://nofxaios.com:30006"
	if v := strings.TrimSpace(os.Getenv("QUANT_DATA_API_BASE")); v != "" {
		cfg.QuantDataAPIBase = v
	}
	cfg.QuantDataAuthKey = strings.TrimSpace(os.Getenv("QUANT_DATA_AUTH_KEY"))

	// ---- Rate limiting -------------------------------------------------
	// Enabled by default: credential endpoints must be brute-force resistant.
	cfg.RateLimitEnabled = true
	if v := os.Getenv("RATE_LIMIT_ENABLED"); v != "" {
		cfg.RateLimitEnabled = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	cfg.RateLimitAuthPerMin = envInt("RATE_LIMIT_AUTH_PER_MIN", 10)
	cfg.RateLimitPublicPerMin = envInt("RATE_LIMIT_PUBLIC_PER_MIN", 240)
	cfg.RateLimitUserPerMin = envInt("RATE_LIMIT_USER_PER_MIN", 600)
	cfg.RateLimitSensitivePerMin = envInt("RATE_LIMIT_SENSITIVE_PER_MIN", 30)
	if cfg.RateLimitAuthPerMin < 1 {
		cfg.RateLimitAuthPerMin = 1
	}
	if cfg.RateLimitPublicPerMin < 1 {
		cfg.RateLimitPublicPerMin = 1
	}
	if cfg.RateLimitUserPerMin < 1 {
		cfg.RateLimitUserPerMin = 1
	}
	if cfg.RateLimitSensitivePerMin < 1 {
		cfg.RateLimitSensitivePerMin = 1
	}

	// ---- Metrics --------------------------------------------------------
	cfg.MetricsToken = strings.TrimSpace(os.Getenv("METRICS_TOKEN"))

	// ---- Backups ---------------------------------------------------------
	cfg.BackupEnabled = true
	if v := os.Getenv("BACKUP_ENABLED"); v != "" {
		cfg.BackupEnabled = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	cfg.BackupDir = strings.TrimSpace(os.Getenv("BACKUP_DIR"))
	if cfg.BackupDir == "" {
		cfg.BackupDir = "data/backups"
	}
	cfg.BackupIntervalHours = envInt("BACKUP_INTERVAL_HOURS", 24)
	if cfg.BackupIntervalHours < 1 {
		cfg.BackupIntervalHours = 24
	}
	cfg.BackupRetentionCount = envInt("BACKUP_RETENTION_COUNT", 7)
	if cfg.BackupRetentionCount < 1 {
		cfg.BackupRetentionCount = 1
	}

	// ---- Retention -------------------------------------------------------
	cfg.AuditRetentionDays = envInt("AUDIT_RETENTION_DAYS", 180)
	cfg.NotificationRetentionDays = envInt("NOTIFICATION_RETENTION_DAYS", 90)

	global = cfg
}

// envInt reads an integer env var with a default.
func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Get returns the global configuration
func Get() *Config {
	if global == nil {
		Init()
	}
	return global
}
