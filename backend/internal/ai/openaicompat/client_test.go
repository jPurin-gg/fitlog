package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/config"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestCompleteSendsOpenAICompatibleRequest(t *testing.T) {
	client := testClient(func(r *http.Request) *http.Response {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			return jsonResponse(http.StatusUnauthorized, `{}`)
		}
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ResponseFormat == nil || body.ResponseFormat.Type != "json_object" {
			return jsonResponse(http.StatusBadRequest, `{}`)
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`)
	})

	result, err := client.Complete(context.Background(), ai.Request{Task: ai.TaskRecommendation, SystemPrompt: "system", UserPrompt: "user", JSONMode: true})
	if err != nil || result != `{"ok":true}` {
		t.Fatalf("Complete() = %q, %v", result, err)
	}
}

func TestCompleteRejectsInvalidResponse(t *testing.T) {
	client := testClient(func(_ *http.Request) *http.Response { return jsonResponse(http.StatusOK, `not json`) })
	_, err := client.Complete(context.Background(), ai.Request{})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Code != "INVALID_RESPONSE" {
		t.Fatalf("Complete() error = %#v", err)
	}
}

func TestCompleteEnforcesLocalRateLimit(t *testing.T) {
	client := testClient(func(_ *http.Request) *http.Response {
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`)
	})
	client.config.RPM = 1
	client.config.MaxWait = 0
	if _, err := client.Complete(context.Background(), ai.Request{}); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	_, err := client.Complete(context.Background(), ai.Request{})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Status != http.StatusTooManyRequests || aiErr.Code != "LOCAL_RATE_LIMIT" {
		t.Fatalf("second Complete() error = %#v", err)
	}
}

func TestCompleteRequiresServerSideAPIKey(t *testing.T) {
	cfg := testConfig("http://example.invalid")
	cfg.APIKey = ""
	_, err := New(cfg, discardLogger()).Complete(context.Background(), ai.Request{})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Code != "MISSING_API_KEY" {
		t.Fatalf("Complete() error = %#v", err)
	}
}

func TestCompleteRetriesAndLogsFinalSuccess(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(testConfig("http://ai.test/v1/chat/completions"), logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				response := jsonResponse(http.StatusTooManyRequests, `{"error":{"status":"RESOURCE_EXHAUSTED"}}`)
				response.Header.Set("Retry-After", "0")
				return response, nil
			}
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`), nil
		}),
		Timeout: time.Second,
	}
	ctx := requestctx.WithRequestID(context.Background(), "http-request-123")

	result, err := client.Complete(ctx, ai.Request{
		Task:         ai.TaskRecommendation,
		SystemPrompt: "private-system-prompt",
		UserPrompt:   "private-user-input",
	})
	if err != nil || result != "done" || calls.Load() != 2 {
		t.Fatalf("Complete() = %q, %v; calls = %d", result, err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["request_id"] != "http-request-123" || record["outcome"] != "success" {
		t.Fatalf("correlation/outcome = %#v/%#v", record["request_id"], record["outcome"])
	}
	if record["attempts"] != float64(2) || record["provider_status"] != float64(http.StatusOK) || record["provider_code"] != "" {
		t.Fatalf("attempt/provider metadata = %#v", record)
	}
	if _, ok := record["total_duration_ms"]; !ok {
		t.Fatal("total_duration_ms is missing")
	}
	if strings.Contains(output.String(), "private-system-prompt") || strings.Contains(output.String(), "private-user-input") || strings.Contains(output.String(), "test-key") {
		t.Fatalf("AI log contains private request data: %s", output.String())
	}
}

func TestCompleteRejectsForbiddenWithoutRetryAndLogsFailure(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(testConfig("http://ai.test/v1/chat/completions"), logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls.Add(1)
			return jsonResponse(http.StatusForbidden, `{"error":{"status":"PERMISSION_DENIED"}}`), nil
		}),
		Timeout: time.Second,
	}

	_, err := client.Complete(context.Background(), ai.Request{Task: ai.TaskMonthlyPlan})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Status != http.StatusForbidden || aiErr.Code != "PERMISSION_DENIED" || calls.Load() != 1 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "provider_error" || record["attempts"] != float64(1) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(http.StatusForbidden) || record["provider_code"] != "PERMISSION_DENIED" {
		t.Fatalf("provider metadata = %#v", record)
	}
}

func TestCompleteRetriesRetryableStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "internal server error", status: http.StatusInternalServerError},
		{name: "request timeout", status: http.StatusRequestTimeout},
		{name: "bad gateway", status: http.StatusBadGateway},
		{name: "service unavailable", status: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(func(_ *http.Request) *http.Response {
				if calls.Add(1) == 1 {
					return jsonResponse(test.status, `{"error":{"message":"temporary"}}`)
				}
				return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`)
			})

			result, err := client.Complete(context.Background(), ai.Request{})
			if err != nil || result != "done" || calls.Load() != 2 {
				t.Fatalf("Complete() = %q, %v; calls = %d", result, err, calls.Load())
			}
		})
	}
}

func TestCompleteDoesNotRetryBadRequest(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{name: "status field", body: `{"error":{"status":"INVALID_ARGUMENT","code":"ignored"}}`, wantCode: "INVALID_ARGUMENT"},
		{name: "code field", body: `{"error":{"code":"invalid_request_error","message":"bad prompt"}}`, wantCode: "invalid_request_error"},
		{name: "json without error fields", body: `{}`, wantCode: "Bad Request"},
		{name: "non json body", body: `<html>bad request</html>`, wantCode: "Bad Request"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(func(_ *http.Request) *http.Response {
				calls.Add(1)
				return jsonResponse(http.StatusBadRequest, test.body)
			})

			_, err := client.Complete(context.Background(), ai.Request{})
			var aiErr *ai.Error
			if !errors.As(err, &aiErr) || aiErr.Status != http.StatusBadRequest || aiErr.Code != test.wantCode || aiErr.Attempt != 1 || calls.Load() != 1 {
				t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
			}
		})
	}
}

func TestCompleteReturnsTransportErrorAfterAllAttempts(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(testConfig("http://ai.test/v1/chat/completions"), logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("connection refused")
		}),
		Timeout: time.Second,
	}

	_, err := client.Complete(context.Background(), ai.Request{Task: ai.TaskRecommendation})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Code != "REQUEST_FAILED" || aiErr.Status != 0 || aiErr.Attempt != 3 || calls.Load() != 3 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "transport_error" || record["attempts"] != float64(3) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(0) || record["provider_code"] != "REQUEST_FAILED" {
		t.Fatalf("provider metadata = %#v", record)
	}
}

func TestCompleteGivesUpAfterMaxAttemptsOnRateLimit(t *testing.T) {
	var calls atomic.Int32
	client := testClient(func(_ *http.Request) *http.Response {
		calls.Add(1)
		return jsonResponse(http.StatusTooManyRequests, `{"error":{"status":"RESOURCE_EXHAUSTED"}}`)
	})

	_, err := client.Complete(context.Background(), ai.Request{})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Status != http.StatusTooManyRequests || aiErr.Code != "RESOURCE_EXHAUSTED" || aiErr.Attempt != 3 || calls.Load() != 3 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
	}
}

func TestCompleteRejectsEmptyChoicesAndLogsInvalidResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(testConfig("http://ai.test/v1/chat/completions"), logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls.Add(1)
			return jsonResponse(http.StatusOK, `{"choices":[]}`), nil
		}),
		Timeout: time.Second,
	}

	_, err := client.Complete(context.Background(), ai.Request{Task: ai.TaskRecommendation})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Code != "EMPTY_RESPONSE" || aiErr.Status != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "invalid_response" || record["attempts"] != float64(1) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(http.StatusOK) || record["provider_code"] != "EMPTY_RESPONSE" {
		t.Fatalf("provider metadata = %#v", record)
	}
}

func TestCompleteWaitsBackoffBetweenAttempts(t *testing.T) {
	cfg := testConfig("http://ai.test/v1/chat/completions")
	cfg.RetryBase = 50 * time.Millisecond
	cfg.RetryMaximum = 100 * time.Millisecond
	var calls atomic.Int32
	client := New(cfg, discardLogger())
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				return jsonResponse(http.StatusInternalServerError, `{}`), nil
			}
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`), nil
		}),
		Timeout: time.Second,
	}

	started := time.Now()
	result, err := client.Complete(context.Background(), ai.Request{})
	elapsed := time.Since(started)
	if err != nil || result != "done" || calls.Load() != 2 {
		t.Fatalf("Complete() = %q, %v; calls = %d", result, err, calls.Load())
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("elapsed = %v; want >= 50ms", elapsed)
	}
}

func TestCompleteHonorsRetryAfterHeader(t *testing.T) {
	cfg := testConfig("http://ai.test/v1/chat/completions")
	cfg.RetryMaximum = 2 * time.Second
	var calls atomic.Int32
	client := New(cfg, discardLogger())
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				response := jsonResponse(http.StatusTooManyRequests, `{}`)
				response.Header.Set("Retry-After", "1")
				return response, nil
			}
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`), nil
		}),
		Timeout: 3 * time.Second,
	}

	started := time.Now()
	result, err := client.Complete(context.Background(), ai.Request{})
	if err != nil || result != "done" || calls.Load() != 2 {
		t.Fatalf("Complete() = %q, %v; calls = %d", result, err, calls.Load())
	}
	if elapsed := time.Since(started); elapsed < time.Second {
		t.Fatalf("elapsed = %v; want >= 1s from Retry-After", elapsed)
	}
}

func TestCompleteStopsRetryingWhenContextCanceledDuringBackoff(t *testing.T) {
	cfg := testConfig("http://ai.test/v1/chat/completions")
	cfg.RetryBase = time.Second
	cfg.RetryMaximum = time.Hour
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(cfg, logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls.Add(1)
			return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
		}),
		Timeout: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(20*time.Millisecond, cancel)

	_, err := client.Complete(ctx, ai.Request{Task: ai.TaskRecommendation})
	if !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "canceled" || record["attempts"] != float64(1) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(http.StatusServiceUnavailable) || record["provider_code"] != "Service Unavailable" {
		t.Fatalf("provider metadata = %#v", record)
	}
}

func TestCompleteSkipsRateLimiterWhenRPMIsZero(t *testing.T) {
	var calls atomic.Int32
	client := testClient(func(_ *http.Request) *http.Response {
		calls.Add(1)
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`)
	})
	client.config.RPM = 0
	client.config.MaxWait = 0

	started := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := client.Complete(context.Background(), ai.Request{}); err != nil {
			t.Fatalf("Complete() #%d error = %v", i+1, err)
		}
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond || calls.Load() != 5 {
		t.Fatalf("elapsed = %v; calls = %d", elapsed, calls.Load())
	}
}

func TestCompleteSpacesRequestsByRPM(t *testing.T) {
	var calls atomic.Int32
	client := testClient(func(_ *http.Request) *http.Response {
		calls.Add(1)
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`)
	})
	client.config.RPM = 600
	client.config.MaxWait = time.Second

	started := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := client.Complete(context.Background(), ai.Request{}); err != nil {
			t.Fatalf("Complete() #%d error = %v", i+1, err)
		}
	}
	elapsed := time.Since(started)
	if calls.Load() != 2 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("elapsed = %v; want >= 80ms", elapsed)
	}
}

func TestCompleteReturnsCanceledWhileWaitingForRateLimiter(t *testing.T) {
	cfg := testConfig("http://ai.test/v1/chat/completions")
	cfg.RPM = 1
	cfg.MaxWait = time.Hour
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var calls atomic.Int32
	client := New(cfg, logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls.Add(1)
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`), nil
		}),
		Timeout: time.Second,
	}
	if _, err := client.Complete(context.Background(), ai.Request{}); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	output.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(20*time.Millisecond, cancel)

	_, err := client.Complete(ctx, ai.Request{Task: ai.TaskRecommendation})
	if !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("second Complete() error = %#v; calls = %d", err, calls.Load())
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "canceled" || record["attempts"] != float64(0) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(0) || record["provider_code"] != "" {
		t.Fatalf("provider metadata = %#v", record)
	}
}

func TestRetryDelay(t *testing.T) {
	cfg := config.AIConfig{RetryBase: 100 * time.Millisecond, RetryMaximum: time.Second}
	generous := cfg
	generous.RetryMaximum = 5 * time.Second
	noBase := cfg
	noBase.RetryBase = 0
	noMaximum := cfg
	noMaximum.RetryMaximum = 0
	tests := []struct {
		name       string
		cfg        config.AIConfig
		attempt    int
		retryAfter string
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{name: "retry-after within maximum", cfg: generous, attempt: 1, retryAfter: "2", wantMin: 2 * time.Second, wantMax: 2 * time.Second},
		{name: "retry-after zero", cfg: cfg, attempt: 3, retryAfter: "0", wantMin: 0, wantMax: 0},
		{name: "retry-after with spaces", cfg: generous, attempt: 1, retryAfter: " 3 ", wantMin: 3 * time.Second, wantMax: 3 * time.Second},
		{name: "retry-after above maximum falls back to backoff", cfg: cfg, attempt: 5, retryAfter: "10", wantMin: time.Second, wantMax: 1250 * time.Millisecond},
		{name: "retry-after not a number", cfg: cfg, attempt: 1, retryAfter: "abc", wantMin: 100 * time.Millisecond, wantMax: 125 * time.Millisecond},
		{name: "retry-after negative", cfg: cfg, attempt: 1, retryAfter: "-1", wantMin: 100 * time.Millisecond, wantMax: 125 * time.Millisecond},
		{name: "attempt 1", cfg: cfg, attempt: 1, wantMin: 100 * time.Millisecond, wantMax: 125 * time.Millisecond},
		{name: "attempt 2", cfg: cfg, attempt: 2, wantMin: 200 * time.Millisecond, wantMax: 250 * time.Millisecond},
		{name: "attempt 4", cfg: cfg, attempt: 4, wantMin: 800 * time.Millisecond, wantMax: time.Second},
		{name: "attempt 5 capped", cfg: cfg, attempt: 5, wantMin: time.Second, wantMax: 1250 * time.Millisecond},
		{name: "no retry base", cfg: noBase, attempt: 3, wantMin: 0, wantMax: 0},
		{name: "no retry maximum", cfg: noMaximum, attempt: 3, wantMin: 0, wantMax: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for i := 0; i < 20; i++ {
				delay := retryDelay(test.cfg, test.attempt, test.retryAfter)
				if delay < test.wantMin || delay > test.wantMax {
					t.Fatalf("retryDelay(attempt %d, retryAfter %q) = %v; want [%v, %v]", test.attempt, test.retryAfter, delay, test.wantMin, test.wantMax)
				}
			}
		})
	}
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "too many requests", status: http.StatusTooManyRequests, want: true},
		{name: "request timeout", status: http.StatusRequestTimeout, want: true},
		{name: "internal server error", status: http.StatusInternalServerError, want: true},
		{name: "bad gateway", status: http.StatusBadGateway, want: true},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: true},
		{name: "bad request", status: http.StatusBadRequest, want: false},
		{name: "unauthorized", status: http.StatusUnauthorized, want: false},
		{name: "forbidden", status: http.StatusForbidden, want: false},
		{name: "not found", status: http.StatusNotFound, want: false},
		{name: "ok", status: http.StatusOK, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := retryable(test.status); got != test.want {
				t.Fatalf("retryable(%d) = %v; want %v", test.status, got, test.want)
			}
		})
	}
}

func TestAIOutcome(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", err: nil, want: "success"},
		{name: "canceled", err: context.Canceled, want: "canceled"},
		{name: "timeout", err: context.DeadlineExceeded, want: "timeout"},
		{name: "canceled inside ai error", err: &ai.Error{Code: "REQUEST_FAILED", Err: context.Canceled}, want: "canceled"},
		{name: "missing api key", err: &ai.Error{Code: "MISSING_API_KEY"}, want: "configuration_error"},
		{name: "local rate limit", err: &ai.Error{Status: http.StatusTooManyRequests, Code: "LOCAL_RATE_LIMIT"}, want: "local_rate_limited"},
		{name: "request failed", err: &ai.Error{Code: "REQUEST_FAILED"}, want: "transport_error"},
		{name: "read failed", err: &ai.Error{Status: http.StatusOK, Code: "READ_FAILED"}, want: "invalid_response"},
		{name: "invalid response", err: &ai.Error{Status: http.StatusOK, Code: "INVALID_RESPONSE"}, want: "invalid_response"},
		{name: "empty response", err: &ai.Error{Status: http.StatusOK, Code: "EMPTY_RESPONSE"}, want: "invalid_response"},
		{name: "provider status", err: &ai.Error{Status: http.StatusForbidden, Code: "PERMISSION_DENIED"}, want: "provider_error"},
		{name: "wrapped provider status", err: fmt.Errorf("wrapped: %w", &ai.Error{Status: http.StatusBadGateway, Code: "Bad Gateway"}), want: "provider_error"},
		{name: "unknown code without status", err: &ai.Error{Code: "UNKNOWN"}, want: "failed"},
		{name: "non ai error", err: errors.New("boom"), want: "internal_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := aiOutcome(test.err); got != test.want {
				t.Fatalf("aiOutcome(%#v) = %q; want %q", test.err, got, test.want)
			}
		})
	}
}

func decodeLogRecord(t *testing.T, data []byte) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("log records = %d; output = %q", len(lines), string(data))
	}
	var record map[string]any
	if err := json.Unmarshal(lines[0], &record); err != nil {
		t.Fatalf("decode log record: %v; output = %q", err, string(data))
	}
	return record
}

func testConfig(url string) config.AIConfig {
	return config.AIConfig{
		APIKey: "test-key", URL: url, Model: "test-model", MaxAttempts: 3,
		Timeout: time.Second, RetryBase: 0, RetryMaximum: 0,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClient(respond func(*http.Request) *http.Response) *Client {
	client := New(testConfig("http://ai.test/v1/chat/completions"), discardLogger())
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return respond(request), nil
		}),
		Timeout: time.Second,
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
