package config

import (
	"testing"
	"time"
)

func TestLoadOptionalAITimeout(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("AI_OPTIONAL_TIMEOUT_SECONDS", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AI.OptionalTimeout != 7*time.Second {
		t.Fatalf("OptionalTimeout = %s, want 7s", cfg.AI.OptionalTimeout)
	}
}

func TestLoadOptionalAITimeoutDefaultsToFifteenSeconds(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("AI_OPTIONAL_TIMEOUT_SECONDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AI.OptionalTimeout != 15*time.Second {
		t.Fatalf("OptionalTimeout = %s, want 15s", cfg.AI.OptionalTimeout)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_TIMEZONE", "UTC")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_USER", "fitlog_test")
	t.Setenv("DB_NAME", "fitlog_test")
}
