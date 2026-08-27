package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Environment         string
	Port                string
	FrontendURL         string
	Timezone            *time.Location
	DB                  DBConfig
	AI                  AIConfig
	SessionSecret       []byte
	SessionCookieName   string
	SessionCookieSecure bool
	SessionTTL          time.Duration
	PromptDir           string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", c.Host, c.Port, c.User, c.Password, c.Name)
}

type AIConfig struct {
	APIKey          string
	URL             string
	Model           string
	RPM             int
	MaxWait         time.Duration
	MaxAttempts     int
	Timeout         time.Duration
	OptionalTimeout time.Duration
	RetryBase       time.Duration
	RetryMaximum    time.Duration
}

func Load() (Config, error) {
	locationName := envString("APP_TIMEZONE", "Asia/Tokyo")
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_TIMEZONE %q: %w", locationName, err)
	}

	secret := []byte(os.Getenv("SESSION_SECRET"))
	if len(secret) < 32 {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}

	cfg := Config{
		Environment:         envString("APP_ENV", "development"),
		Port:                envString("PORT", "8080"),
		FrontendURL:         envString("FRONTEND_URL", "http://localhost:3000"),
		Timezone:            location,
		SessionSecret:       secret,
		SessionCookieName:   envString("SESSION_COOKIE_NAME", "fitlog_session"),
		SessionCookieSecure: envBool("SESSION_COOKIE_SECURE", false),
		SessionTTL:          time.Duration(envInt("SESSION_TTL_HOURS", 24*30)) * time.Hour,
		PromptDir:           envString("PROMPT_DIR", "prompts"),
		DB: DBConfig{
			Host:     envString("DB_HOST", "db"),
			Port:     envString("DB_PORT", "5432"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
		},
		AI: AIConfig{
			APIKey:          os.Getenv("XAI_API_KEY"),
			URL:             envString("XAI_API_URL", "https://api.x.ai/v1/chat/completions"),
			Model:           envString("XAI_MODEL", "grok-4.3"),
			RPM:             envInt("AI_RATE_LIMIT_RPM", 5),
			MaxWait:         time.Duration(envInt("AI_RATE_LIMIT_MAX_WAIT_MS", 15000)) * time.Millisecond,
			MaxAttempts:     clamp(envInt("AI_MAX_ATTEMPTS", 3), 1, 3),
			Timeout:         time.Duration(envInt("AI_HTTP_TIMEOUT_SECONDS", 30)) * time.Second,
			OptionalTimeout: time.Duration(envInt("AI_OPTIONAL_TIMEOUT_SECONDS", 15)) * time.Second,
			RetryBase:       time.Duration(envInt("AI_RETRY_BASE_MS", 500)) * time.Millisecond,
			RetryMaximum:    time.Duration(envInt("AI_RETRY_MAX_MS", 5000)) * time.Millisecond,
		},
	}

	if cfg.DB.User == "" || cfg.DB.Name == "" {
		return Config{}, fmt.Errorf("DB_USER and DB_NAME are required")
	}
	return cfg, nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
