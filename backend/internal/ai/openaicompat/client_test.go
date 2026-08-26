package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestCompleteRetriesRetryableResponses(t *testing.T) {
	var calls atomic.Int32
	client := testClient(func(_ *http.Request) *http.Response {
		if calls.Add(1) == 1 {
			response := jsonResponse(http.StatusTooManyRequests, `{"error":{"status":"RESOURCE_EXHAUSTED"}}`)
			response.Header.Set("Retry-After", "0")
			return response
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"done"}}]}`)
	})

	result, err := client.Complete(context.Background(), ai.Request{})
	if err != nil || result != "done" || calls.Load() != 2 {
		t.Fatalf("Complete() = %q, %v; calls = %d", result, err, calls.Load())
	}
}

func TestCompleteDoesNotRetryForbidden(t *testing.T) {
	var calls atomic.Int32
	client := testClient(func(_ *http.Request) *http.Response {
		calls.Add(1)
		return jsonResponse(http.StatusForbidden, `{"error":{"status":"PERMISSION_DENIED"}}`)
	})

	_, err := client.Complete(context.Background(), ai.Request{})
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) || aiErr.Status != http.StatusForbidden || aiErr.Code != "PERMISSION_DENIED" || calls.Load() != 1 {
		t.Fatalf("Complete() error = %#v; calls = %d", err, calls.Load())
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

func TestCompleteLogsOneCorrelatedLogicalOutcome(t *testing.T) {
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
	if err != nil || result != "done" {
		t.Fatalf("Complete() = %q, %v", result, err)
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

func TestCompleteLogsFinalProviderFailure(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	client := New(testConfig("http://ai.test/v1/chat/completions"), logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusForbidden, `{"error":{"status":"PERMISSION_DENIED"}}`), nil
		}),
		Timeout: time.Second,
	}

	_, err := client.Complete(context.Background(), ai.Request{Task: ai.TaskMonthlyPlan})
	if err == nil {
		t.Fatal("Complete() error = nil")
	}

	record := decodeLogRecord(t, output.Bytes())
	if record["outcome"] != "provider_error" || record["attempts"] != float64(1) {
		t.Fatalf("outcome/attempts = %#v/%#v", record["outcome"], record["attempts"])
	}
	if record["provider_status"] != float64(http.StatusForbidden) || record["provider_code"] != "PERMISSION_DENIED" {
		t.Fatalf("provider metadata = %#v", record)
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
