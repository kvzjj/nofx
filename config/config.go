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

	global = cfg
}

// Get returns the global configuration
func Get() *Config {
	if global == nil {
		Init()
	}
	return global
}
