package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallAISuccess(t *testing.T) {
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization header was not set")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`))
	}), func() {
		got, err := callAI("system", "user", true)
		if err != nil {
			t.Fatalf("callAI() error = %v", err)
		}
		if got != `{"ok":true}` {
			t.Fatalf("callAI() = %q", got)
		}
	})
}

func TestCallAIRetries429ResourceExhausted(t *testing.T) {
	var calls int32
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"quota exceeded"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"}}]}`))
	}), func() {
		got, err := callAI("system", "user", false)
		if err != nil {
			t.Fatalf("callAI() error = %v", err)
		}
		if got != "done" {
			t.Fatalf("callAI() = %q", got)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})
}

func TestCallAIRetries503AndStopsAtMaxAttempts(t *testing.T) {
	var calls int32
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"status":"UNAVAILABLE","message":"service busy"}}`))
	}), func() {
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("callAI() error = nil")
		}
		var aiErr *AIError
		if !errors.As(err, &aiErr) {
			t.Fatalf("error type = %T, want *AIError", err)
		}
		if aiErr.Status != http.StatusServiceUnavailable || aiErr.Code != "UNAVAILABLE" {
			t.Fatalf("AIError = status %d code %q", aiErr.Status, aiErr.Code)
		}
		if calls != 3 {
			t.Fatalf("calls = %d, want 3", calls)
		}
	})
}

func TestCallAIDoesNotRetry403(t *testing.T) {
	var calls int32
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"status":"PERMISSION_DENIED","message":"bad key"}}`))
	}), func() {
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("callAI() error = nil")
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
		if aiUserMessage(err) != "APIの設定または権限に問題があります" {
			t.Fatalf("aiUserMessage() = %q", aiUserMessage(err))
		}
	})
}

func TestCallAI404ModelMessage(t *testing.T) {
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","message":"model not found"}}`))
	}), func() {
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("callAI() error = nil")
		}
		if aiUserMessage(err) != "指定しているAIモデルまたはAPI設定が無効です" {
			t.Fatalf("aiUserMessage() = %q", aiUserMessage(err))
		}
	})
}

func TestCallAITimeout(t *testing.T) {
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}), func() {
		t.Setenv("AI_HTTP_TIMEOUT_MS", "1")
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("callAI() error = nil")
		}
		var aiErr *AIError
		if !errors.As(err, &aiErr) || aiErr.Code != "REQUEST_FAILED" {
			t.Fatalf("error = %#v, want REQUEST_FAILED AIError", err)
		}
	})
}

func TestCallAIInvalidResponse(t *testing.T) {
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}), func() {
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("callAI() error = nil")
		}
		var aiErr *AIError
		if !errors.As(err, &aiErr) || aiErr.Code != "INVALID_RESPONSE" {
			t.Fatalf("error = %#v, want INVALID_RESPONSE AIError", err)
		}
	})
}

func TestLocalRateLimitReturns429WhenWaitIsTooLong(t *testing.T) {
	withAITestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"}}]}`))
	}), func() {
		t.Setenv("AI_RATE_LIMIT_RPM", "1")
		t.Setenv("AI_RATE_LIMIT_MAX_WAIT_MS", "0")
		aiRateLimiter = &simpleAIRateLimiter{}
		if _, err := callAI("system", "user", false); err != nil {
			t.Fatalf("first callAI() error = %v", err)
		}
		_, err := callAI("system", "user", false)
		if err == nil {
			t.Fatal("second callAI() error = nil")
		}
		var aiErr *AIError
		if !errors.As(err, &aiErr) || aiErr.Status != http.StatusTooManyRequests || aiErr.Code != "LOCAL_RATE_LIMIT" {
			t.Fatalf("error = %#v, want local 429", err)
		}
	})
}

func TestAPIKeyIsNotAcceptedFromPublicEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NEXT_PUBLIC_OPENAI_API_KEY", "public-key")
	_, err := callAI("system", "user", false)
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("callAI() error = %v, want missing OPENAI_API_KEY", err)
	}
}

func withAITestEnv(t *testing.T, handler http.Handler, fn func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "gemini-2.5-flash")
	t.Setenv("OPENAI_API_URL", server.URL)
	t.Setenv("AI_RATE_LIMIT_RPM", "0")
	t.Setenv("AI_MAX_ATTEMPTS", "3")
	t.Setenv("AI_RETRY_BASE_MS", "1")
	t.Setenv("AI_RETRY_MAX_MS", "2")
	t.Setenv("AI_HTTP_TIMEOUT_SECONDS", "5")
	aiRateLimiter = &simpleAIRateLimiter{}
	defer func() {
		aiRateLimiter = &simpleAIRateLimiter{}
		_ = os.Unsetenv("NEXT_PUBLIC_OPENAI_API_KEY")
	}()

	fn()
}
