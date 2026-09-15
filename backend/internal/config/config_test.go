package config

import (
	"strings"
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

func TestLoadOptionalAITimeoutParsing(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "数値以外は既定値にフォールバック", value: "abc", want: 15 * time.Second},
		{name: "前後の空白は無視される", value: " 9 ", want: 9 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("AI_OPTIONAL_TIMEOUT_SECONDS", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.AI.OptionalTimeout != tt.want {
				t.Fatalf("OptionalTimeout = %v, want %v", cfg.AI.OptionalTimeout, tt.want)
			}
		})
	}
}

func TestLoadSessionTTLHours(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("SESSION_TTL_HOURS", "12")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("SessionTTL = %v, want 12h", cfg.SessionTTL)
	}
}

func TestLoadSessionSecretLength(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "31バイトは拒否される", secret: strings.Repeat("s", 31), wantErr: true},
		{name: "32バイトは受け入れられる", secret: strings.Repeat("s", 32), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("SESSION_SECRET", tt.secret)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want SESSION_SECRET error")
				}
				if !strings.Contains(err.Error(), "SESSION_SECRET") {
					t.Fatalf("Load() error = %v, want mention of SESSION_SECRET", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if string(cfg.SessionSecret) != tt.secret {
				t.Fatalf("SessionSecret = %q, want %q", cfg.SessionSecret, tt.secret)
			}
		})
	}
}

func TestLoadRequiresDBUserAndName(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "DB_USERが空", key: "DB_USER"},
		{name: "DB_NAMEが空", key: "DB_NAME"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(tt.key, "")

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want error for empty %s", tt.key)
			}
		})
	}
}

func TestLoadTimezone(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "存在しないタイムゾーンは拒否される", value: "Not/AZone", wantErr: true},
		{name: "有効なタイムゾーンが読み込まれる", value: "Asia/Tokyo", want: "Asia/Tokyo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("APP_TIMEZONE", tt.value)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want APP_TIMEZONE error")
				}
				if !strings.Contains(err.Error(), "APP_TIMEZONE") {
					t.Fatalf("Load() error = %v, want mention of APP_TIMEZONE", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Timezone == nil || cfg.Timezone.String() != tt.want {
				t.Fatalf("Timezone = %v, want %q", cfg.Timezone, tt.want)
			}
		})
	}
}

func TestLoadClampsAIMaxAttempts(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "0は下限1に補正", value: "0", want: 1},
		{name: "10は上限3に補正", value: "10", want: 3},
		{name: "範囲内はそのまま", value: "2", want: 2},
		{name: "未設定は既定値3", value: "", want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("AI_MAX_ATTEMPTS", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.AI.MaxAttempts != tt.want {
				t.Fatalf("MaxAttempts = %d, want %d", cfg.AI.MaxAttempts, tt.want)
			}
		})
	}
}

func TestLoadSessionCookieSecure(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "trueは有効", value: "true", want: true},
		{name: "解釈不能な値は既定値false", value: "yes", want: false},
		{name: "1は有効", value: "1", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("SESSION_COOKIE_SECURE", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.SessionCookieSecure != tt.want {
				t.Fatalf("SessionCookieSecure = %v, want %v", cfg.SessionCookieSecure, tt.want)
			}
		})
	}
}

func TestLoadAIRateLimitAndRetryKnobs(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("AI_RATE_LIMIT_RPM", "12")
	t.Setenv("AI_RATE_LIMIT_MAX_WAIT_MS", "250")
	t.Setenv("AI_RETRY_BASE_MS", "50")
	t.Setenv("AI_RETRY_MAX_MS", "900")
	t.Setenv("AI_HTTP_TIMEOUT_SECONDS", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := AIConfig{
		URL:             "https://api.x.ai/v1/chat/completions",
		Model:           "grok-4.3",
		RPM:             12,
		MaxWait:         250 * time.Millisecond,
		MaxAttempts:     3,
		Timeout:         3 * time.Second,
		OptionalTimeout: 15 * time.Second,
		RetryBase:       50 * time.Millisecond,
		RetryMaximum:    900 * time.Millisecond,
	}
	if cfg.AI != want {
		t.Fatalf("AI = %#v, want %#v", cfg.AI, want)
	}
}

func TestLoadDefaults(t *testing.T) {
	setRequiredEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, "development")
	}
	if cfg.FrontendURL != "http://localhost:3000" {
		t.Fatalf("FrontendURL = %q, want %q", cfg.FrontendURL, "http://localhost:3000")
	}
	if cfg.SessionCookieName != "fitlog_session" {
		t.Fatalf("SessionCookieName = %q, want %q", cfg.SessionCookieName, "fitlog_session")
	}
	if cfg.SessionCookieSecure {
		t.Fatalf("SessionCookieSecure = %v, want false", cfg.SessionCookieSecure)
	}
	if cfg.SessionTTL != 24*30*time.Hour {
		t.Fatalf("SessionTTL = %v, want 720h", cfg.SessionTTL)
	}
	if cfg.PromptDir != "prompts" {
		t.Fatalf("PromptDir = %q, want %q", cfg.PromptDir, "prompts")
	}
	wantDB := DBConfig{Host: "db", Port: "5432", User: "fitlog_test", Password: "", Name: "fitlog_test"}
	if cfg.DB != wantDB {
		t.Fatalf("DB = %#v, want %#v", cfg.DB, wantDB)
	}
}

func TestDBConfigDSN(t *testing.T) {
	db := DBConfig{Host: "db.example", Port: "6543", User: "alice", Password: "p@ss word", Name: "fitlog"}

	got := db.DSN()
	want := "host=db.example port=6543 user=alice password=p@ss word dbname=fitlog sslmode=disable"
	if got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_TIMEZONE", "UTC")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_USER", "fitlog_test")
	t.Setenv("DB_NAME", "fitlog_test")
	for _, name := range []string{
		"APP_ENV", "PORT", "FRONTEND_URL", "DB_HOST", "DB_PORT", "DB_PASSWORD",
		"SESSION_COOKIE_NAME", "SESSION_COOKIE_SECURE", "SESSION_TTL_HOURS", "PROMPT_DIR",
		"XAI_API_KEY", "XAI_API_URL", "XAI_MODEL",
		"AI_RATE_LIMIT_RPM", "AI_RATE_LIMIT_MAX_WAIT_MS", "AI_MAX_ATTEMPTS", "AI_HTTP_TIMEOUT_SECONDS",
		"AI_OPTIONAL_TIMEOUT_SECONDS", "AI_RETRY_BASE_MS", "AI_RETRY_MAX_MS",
	} {
		t.Setenv(name, "")
	}
}
