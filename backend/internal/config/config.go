package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all environment-derived runtime configuration.
type Config struct {
	DatabaseURL   string
	PublicURL     string
	JWTSigningKey string
	SMTPURL       string
	FromEmail     string
	LogLevel      string
	APIListen     string
	TestMode      bool
}

// Load reads LGV_* environment variables and returns a Config.
// Required: LGV_DATABASE_URL, LGV_JWT_SIGNING_KEY.
// Others fall back to sensible dev defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:   os.Getenv("LGV_DATABASE_URL"),
		PublicURL:     getEnv("LGV_PUBLIC_URL", "http://localhost:8080"),
		JWTSigningKey: os.Getenv("LGV_JWT_SIGNING_KEY"),
		SMTPURL:       getEnv("LGV_SMTP_URL", "smtp://mailhog:1025"),
		FromEmail:     getEnv("LGV_FROM_EMAIL", "noreply@localhost"),
		LogLevel:      getEnv("LGV_LOG_LEVEL", "info"),
		APIListen:     getEnv("LGV_API_LISTEN", ":8080"),
		TestMode:      getEnvBool("LGV_TEST_MODE", false),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("LGV_DATABASE_URL is required")
	}
	if cfg.JWTSigningKey == "" {
		return nil, errors.New("LGV_JWT_SIGNING_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return parsed
}
