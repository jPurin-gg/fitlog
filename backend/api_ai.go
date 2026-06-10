package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	mathrand "math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // For JSON mode
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type aiAPIErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

type AIError struct {
	RequestID string
	Status    int
	Code      string
	Message   string
	Model     string
	Attempt   int
	Err       error
}

func (e *AIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Status > 0 {
		return fmt.Sprintf("AI request %s failed with status %d (%s)", e.RequestID, e.Status, e.Code)
	}
	if e.Code != "" {
		return fmt.Sprintf("AI request %s failed (%s)", e.RequestID, e.Code)
	}
	return fmt.Sprintf("AI request %s failed", e.RequestID)
}

func (e *AIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

var aiRateLimiter = &simpleAIRateLimiter{}

type simpleAIRateLimiter struct {
	mu   sync.Mutex
	next time.Time
}

// callAI は OpenAI API (または互換プロバイダー) を叩いて結果の文字列を返します
func callAI(systemPrompt, userPrompt string, jsonMode bool) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("OPENAI_API_KEY is not set in environment or .env")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}

	reqBody := AIRequest{
		Model: model, // デフォルトモデル
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.7,
	}

	if jsonMode {
		reqBody.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := os.Getenv("OPENAI_API_URL")
	if apiURL == "" {
		apiURL = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	}

	requestID := newAIRequestID()
	maxAttempts := envInt("AI_MAX_ATTEMPTS", 3)
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if maxAttempts > 3 {
		maxAttempts = 3
	}

	client := &http.Client{Timeout: aiHTTPTimeout()}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := aiRateLimiter.wait(context.Background(), requestID, model); err != nil {
			return "", err
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Request-ID", requestID)

		log.Printf("AI API attempt started request_id=%s model=%s attempt=%d/%d time=%s", requestID, model, attempt, maxAttempts, time.Now().UTC().Format(time.RFC3339))
		resp, err := client.Do(req)
		if err != nil {
			aiErr := &AIError{RequestID: requestID, Status: 0, Code: "REQUEST_FAILED", Message: err.Error(), Model: model, Attempt: attempt, Err: err}
			logAIError(aiErr, "")
			lastErr = aiErr
			if !isRetryableStatus(0) || attempt == maxAttempts {
				return "", aiErr
			}
			sleepBeforeRetry(nil, attempt)
			continue
		}

		bodyText, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			aiErr := &AIError{RequestID: requestID, Status: resp.StatusCode, Code: "READ_FAILED", Message: readErr.Error(), Model: model, Attempt: attempt, Err: readErr}
			logAIError(aiErr, "")
			lastErr = aiErr
			return "", aiErr
		}

		if resp.StatusCode != http.StatusOK {
			aiErr := buildAIHTTPError(requestID, model, attempt, resp.StatusCode, bodyText)
			logAIError(aiErr, string(bodyText))
			lastErr = aiErr
			if !isRetryableStatus(resp.StatusCode) || attempt == maxAttempts {
				return "", aiErr
			}
			sleepBeforeRetry(resp, attempt)
			continue
		}

		var aiResp AIResponse
		if err := json.Unmarshal(bodyText, &aiResp); err != nil {
			aiErr := &AIError{RequestID: requestID, Status: resp.StatusCode, Code: "INVALID_RESPONSE", Message: err.Error(), Model: model, Attempt: attempt, Err: err}
			logAIError(aiErr, string(bodyText))
			return "", aiErr
		}

		if len(aiResp.Choices) > 0 {
			return aiResp.Choices[0].Message.Content, nil
		}

		aiErr := &AIError{RequestID: requestID, Status: resp.StatusCode, Code: "EMPTY_RESPONSE", Message: "no choice returned by AI API", Model: model, Attempt: attempt}
		logAIError(aiErr, string(bodyText))
		return "", aiErr
	}

	return "", lastErr
}

func (l *simpleAIRateLimiter) wait(ctx context.Context, requestID, model string) error {
	rpm := envInt("AI_RATE_LIMIT_RPM", 5)
	if rpm <= 0 {
		return nil
	}
	interval := time.Minute / time.Duration(rpm)
	maxWait := time.Duration(envInt("AI_RATE_LIMIT_MAX_WAIT_MS", 15000)) * time.Millisecond

	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
	}
	if wait > maxWait {
		l.mu.Unlock()
		err := &AIError{
			RequestID: requestID,
			Status:    http.StatusTooManyRequests,
			Code:      "LOCAL_RATE_LIMIT",
			Message:   "local AI rate limit exceeded",
			Model:     model,
		}
		logAIError(err, "")
		return err
	}
	l.next = now.Add(wait).Add(interval)
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	log.Printf("AI API local rate limit wait request_id=%s model=%s wait_ms=%d time=%s", requestID, model, wait.Milliseconds(), time.Now().UTC().Format(time.RFC3339))
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildAIHTTPError(requestID, model string, attempt, status int, body []byte) *AIError {
	aiErr := &AIError{
		RequestID: requestID,
		Status:    status,
		Code:      http.StatusText(status),
		Message:   strings.TrimSpace(string(body)),
		Model:     model,
		Attempt:   attempt,
	}
	var parsed aiAPIErrorBody
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Error.Status != "" {
			aiErr.Code = parsed.Error.Status
		} else if parsed.Error.Code != "" {
			aiErr.Code = parsed.Error.Code
		}
		if parsed.Error.Message != "" {
			aiErr.Message = parsed.Error.Message
		}
	}
	return aiErr
}

func logAIError(err *AIError, responseBody string) {
	if err == nil {
		return
	}
	log.Printf(
		"AI API error time=%s request_id=%s status=%d code=%s message=%q model=%s attempt=%d response_body=%q",
		time.Now().UTC().Format(time.RFC3339),
		err.RequestID,
		err.Status,
		err.Code,
		err.Message,
		err.Model,
		err.Attempt,
		truncateForLog(responseBody, 4000),
	)
}

func isRetryableStatus(status int) bool {
	return status == 0 || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func sleepBeforeRetry(resp *http.Response, attempt int) {
	delay := retryDelay(resp, attempt)
	log.Printf("AI API retry scheduled attempt=%d wait_ms=%d time=%s", attempt+1, delay.Milliseconds(), time.Now().UTC().Format(time.RFC3339))
	time.Sleep(delay)
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After")); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if retryAt, err := http.ParseTime(retryAfter); err == nil {
				if wait := time.Until(retryAt); wait > 0 {
					return wait
				}
			}
		}
	}

	base := time.Duration(envInt("AI_RETRY_BASE_MS", 700)) * time.Millisecond
	maxDelay := time.Duration(envInt("AI_RETRY_MAX_MS", 5000)) * time.Millisecond
	if base <= 0 {
		base = 700 * time.Millisecond
	}
	delay := time.Duration(math.Pow(2, float64(attempt-1))) * base
	if delay > maxDelay {
		delay = maxDelay
	}
	jitter := time.Duration(mathrand.Int63n(int64(delay/2) + 1))
	return delay + jitter
}

func aiHTTPStatus(err error) int {
	var aiErr *AIError
	if errors.As(err, &aiErr) && aiErr.Status > 0 {
		if aiErr.Status == http.StatusTooManyRequests || aiErr.Status == http.StatusForbidden || aiErr.Status == http.StatusNotFound {
			return aiErr.Status
		}
		if aiErr.Status == http.StatusServiceUnavailable || aiErr.Status == http.StatusGatewayTimeout {
			return aiErr.Status
		}
	}
	return http.StatusServiceUnavailable
}

func aiUserMessage(err error) string {
	var aiErr *AIError
	if errors.As(err, &aiErr) {
		switch aiErr.Status {
		case http.StatusTooManyRequests:
			return "利用が集中しています。しばらくしてから再試行してください"
		case http.StatusServiceUnavailable:
			return "AIサービスが一時的に混雑しています"
		case http.StatusForbidden:
			return "APIの設定または権限に問題があります"
		case http.StatusNotFound:
			return "指定しているAIモデルまたはAPI設定が無効です"
		}
	}
	return "AIによる生成に失敗しました"
}

func writeAIError(w http.ResponseWriter, err error) {
	http.Error(w, aiUserMessage(err), aiHTTPStatus(err))
}

func newAIRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func truncateForLog(value string, limit int) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func aiHTTPTimeout() time.Duration {
	if timeoutMS := envInt("AI_HTTP_TIMEOUT_MS", 0); timeoutMS > 0 {
		return time.Duration(timeoutMS) * time.Millisecond
	}
	timeoutSeconds := envInt("AI_HTTP_TIMEOUT_SECONDS", 45)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 45
	}
	return time.Duration(timeoutSeconds) * time.Second
}
