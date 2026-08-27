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

func TestLoadXAIConfig(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("XAI_API_KEY", "test-xai-key")
	t.Setenv("XAI_API_URL", "https://xai.example/v1/chat/completions")
	t.Setenv("XAI_MODEL", "grok-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AI.APIKey != "test-xai-key" {
		t.Fatal("APIKey was not loaded from XAI_API_KEY")
	}
	if cfg.AI.URL != "https://xai.example/v1/chat/completions" {
		t.Fatalf("URL = %q", cfg.AI.URL)
	}
	if cfg.AI.Model != "grok-test" {
		t.Fatalf("Model = %q", cfg.AI.Model)
	}
}

func TestLoadDefaultsToXAIChatCompletions(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("XAI_API_URL", "")
	t.Setenv("XAI_MODEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AI.URL != "https://api.x.ai/v1/chat/completions" {
		t.Fatalf("URL = %q", cfg.AI.URL)
	}
	if cfg.AI.Model != "grok-4.3" {
		t.Fatalf("Model = %q", cfg.AI.Model)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_TIMEZONE", "UTC")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_USER", "fitlog_test")
	t.Setenv("DB_NAME", "fitlog_test")
}
