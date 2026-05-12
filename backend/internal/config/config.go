// Package config loads runtime configuration from LGV_* environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL   string
	PublicURL     string
	JWTSigningKey string
	JWTTTL        time.Duration
	SMTPURL       string
	FromEmail     string
	LogLevel      string
	APIListen     string
	TestMode      bool
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:   os.Getenv("LGV_DATABASE_URL"),
		PublicURL:     os.Getenv("LGV_PUBLIC_URL"),
		JWTSigningKey: os.Getenv("LGV_JWT_SIGNING_KEY"),
		SMTPURL:       os.Getenv("LGV_SMTP_URL"),
		FromEmail:     os.Getenv("LGV_FROM_EMAIL"),
		LogLevel:      os.Getenv("LGV_LOG_LEVEL"),
		APIListen:     os.Getenv("LGV_API_LISTEN"),
	}

	required := map[string]string{
		"LGV_DATABASE_URL":    cfg.DatabaseURL,
		"LGV_PUBLIC_URL":      cfg.PublicURL,
		"LGV_JWT_SIGNING_KEY": cfg.JWTSigningKey,
		"LGV_SMTP_URL":        cfg.SMTPURL,
		"LGV_FROM_EMAIL":      cfg.FromEmail,
		"LGV_LOG_LEVEL":       cfg.LogLevel,
		"LGV_API_LISTEN":      cfg.APIListen,
	}
	for name, value := range required {
		if value == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}

	jwtTTLStr := os.Getenv("LGV_JWT_TTL")
	if jwtTTLStr == "" {
		return nil, errors.New("LGV_JWT_TTL is required")
	}
	jwtTTL, err := time.ParseDuration(jwtTTLStr)
	if err != nil {
		return nil, fmt.Errorf("LGV_JWT_TTL: %w", err)
	}
	cfg.JWTTTL = jwtTTL

	testModeStr := os.Getenv("LGV_TEST_MODE")
	if testModeStr == "" {
		return nil, errors.New("LGV_TEST_MODE is required")
	}
	testMode, err := strconv.ParseBool(testModeStr)
	if err != nil {
		return nil, fmt.Errorf("LGV_TEST_MODE: %w", err)
	}
	cfg.TestMode = testMode

	return cfg, nil
}

func (c *Config) IsSecure() bool {
	return strings.HasPrefix(c.PublicURL, "https://")
}
