package openaicompat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	mathrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/config"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

type Client struct {
	config     config.AIConfig
	httpClient *http.Client
	logger     *slog.Logger
	limiter    rateLimiter
}

type rateLimiter struct {
	mu   sync.Mutex
	next time.Time
}

type requestBody struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func New(cfg config.AIConfig, logger *slog.Logger) *Client {
	return &Client{config: cfg, httpClient: &http.Client{Timeout: cfg.Timeout}, logger: logger}
}

func (c *Client) Complete(ctx context.Context, request ai.Request) (completion string, completionErr error) {
	started := time.Now()
	requestID := newRequestID()
	attempts := 0
	providerStatus := 0
	providerCode := ""
	defer func() {
		c.logCompletion(ctx, request.Task, requestID, attempts, providerStatus, providerCode, started, completionErr)
	}()

	if strings.TrimSpace(c.config.APIKey) == "" {
		return "", &ai.Error{RequestID: requestID, Code: "MISSING_API_KEY", Model: c.config.Model, Err: errors.New("OPENAI_API_KEY is not set")}
	}
	body := requestBody{
		Model: c.config.Model,
		Messages: []message{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.UserPrompt},
		},
		Temperature: 0.7,
	}
	if request.JSONMode {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 1; attempt <= c.config.MaxAttempts; attempt++ {
		if err := c.wait(ctx, requestID); err != nil {
			return "", err
		}
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(encoded))
		if err != nil {
			return "", err
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		httpRequest.Header.Set("X-Request-ID", requestID)

		attempts++
		response, err := c.httpClient.Do(httpRequest)
		if err != nil {
			aiErr := &ai.Error{RequestID: requestID, Code: "REQUEST_FAILED", Model: c.config.Model, Attempt: attempt, Err: err}
			providerStatus = 0
			providerCode = aiErr.Code
			lastErr = aiErr
			if attempt == c.config.MaxAttempts {
				return "", aiErr
			}
			if err := c.sleep(ctx, attempt, ""); err != nil {
				return "", err
			}
			continue
		}

		responseBytes, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		response.Body.Close()
		providerStatus = response.StatusCode
		if readErr != nil {
			providerCode = "READ_FAILED"
			return "", &ai.Error{RequestID: requestID, Status: response.StatusCode, Code: "READ_FAILED", Model: c.config.Model, Attempt: attempt, Err: readErr}
		}
		if response.StatusCode != http.StatusOK {
			aiErr := buildError(requestID, c.config.Model, attempt, response.StatusCode, responseBytes)
			providerCode = aiErr.Code
			lastErr = aiErr
			if !retryable(response.StatusCode) || attempt == c.config.MaxAttempts {
				return "", aiErr
			}
			if err := c.sleep(ctx, attempt, response.Header.Get("Retry-After")); err != nil {
				return "", err
			}
			continue
		}

		var decoded responseBody
		if err := json.Unmarshal(responseBytes, &decoded); err != nil {
			providerCode = "INVALID_RESPONSE"
			return "", &ai.Error{RequestID: requestID, Status: response.StatusCode, Code: "INVALID_RESPONSE", Model: c.config.Model, Attempt: attempt, Err: err}
		}
		if len(decoded.Choices) == 0 {
			providerCode = "EMPTY_RESPONSE"
			return "", &ai.Error{RequestID: requestID, Status: response.StatusCode, Code: "EMPTY_RESPONSE", Model: c.config.Model, Attempt: attempt}
		}
		providerCode = ""
		return decoded.Choices[0].Message.Content, nil
	}
	return "", lastErr
}

func (c *Client) wait(ctx context.Context, requestID string) error {
	if c.config.RPM <= 0 {
		return nil
	}
	interval := time.Minute / time.Duration(c.config.RPM)
	c.limiter.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(c.limiter.next) {
		wait = c.limiter.next.Sub(now)
	}
	if wait > c.config.MaxWait {
		c.limiter.mu.Unlock()
		return &ai.Error{RequestID: requestID, Status: http.StatusTooManyRequests, Code: "LOCAL_RATE_LIMIT", Model: c.config.Model}
	}
	c.limiter.next = now.Add(wait).Add(interval)
	c.limiter.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) sleep(ctx context.Context, attempt int, retryAfter string) error {
	delay := retryDelay(c.config, attempt, retryAfter)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(cfg config.AIConfig, attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		if delay <= cfg.RetryMaximum {
			return delay
		}
	}
	delay := time.Duration(float64(cfg.RetryBase) * math.Pow(2, float64(attempt-1)))
	if delay > cfg.RetryMaximum {
		delay = cfg.RetryMaximum
	}
	if delay <= 0 {
		return 0
	}
	return delay + time.Duration(mathrand.Int63n(int64(delay/4)+1))
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500
}

func buildError(requestID, model string, attempt, status int, body []byte) *ai.Error {
	code := http.StatusText(status)
	var decoded errorBody
	if json.Unmarshal(body, &decoded) == nil {
		if decoded.Error.Status != "" {
			code = decoded.Error.Status
		} else if decoded.Error.Code != "" {
			code = decoded.Error.Code
		}
	}
	return &ai.Error{RequestID: requestID, Status: status, Code: code, Model: model, Attempt: attempt}
}

func (c *Client) logCompletion(ctx context.Context, task ai.Task, aiRequestID string, attempts, providerStatus int, providerCode string, started time.Time, err error) {
	if providerCode == "" {
		var aiErr *ai.Error
		if errors.As(err, &aiErr) {
			providerStatus = aiErr.Status
			providerCode = aiErr.Code
		}
	}
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelWarn
	}
	c.logger.Log(
		ctx,
		level,
		"ai request finished",
		"stage", "provider",
		"request_id", requestctx.RequestID(ctx),
		"ai_request_id", aiRequestID,
		"task", task,
		"model", c.config.Model,
		"outcome", aiOutcome(err),
		"attempts", attempts,
		"total_duration_ms", time.Since(started).Milliseconds(),
		"provider_status", providerStatus,
		"provider_code", providerCode,
	)
}

func aiOutcome(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var aiErr *ai.Error
	if !errors.As(err, &aiErr) {
		return "internal_error"
	}
	switch aiErr.Code {
	case "MISSING_API_KEY":
		return "configuration_error"
	case "LOCAL_RATE_LIMIT":
		return "local_rate_limited"
	case "REQUEST_FAILED":
		return "transport_error"
	case "READ_FAILED", "INVALID_RESPONSE", "EMPTY_RESPONSE":
		return "invalid_response"
	default:
		if aiErr.Status > 0 {
			return "provider_error"
		}
		return "failed"
	}
}

func newRequestID() string {
	value := make([]byte, 12)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
