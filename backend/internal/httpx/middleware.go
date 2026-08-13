package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 12)
		_, _ = rand.Read(bytes)
		requestID := hex.EncodeToString(bytes)
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LogRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func CORS(frontendURL string, next http.Handler) http.Handler {
	allowedOrigin := strings.TrimRight(frontendURL, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		if r.Method == http.MethodOptions {
			if origin != "" && origin != allowedOrigin {
				WriteError(w, r, apperr.Forbidden("許可されていない送信元です。"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if isUnsafeMethod(r.Method) && origin != "" && origin != allowedOrigin {
			WriteError(w, r, apperr.Forbidden("許可されていない送信元です。"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NormalizeRoutingErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &routingErrorWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.status == 0 {
			return
		}
		w.Header().Del("Content-Type")
		w.Header().Del("X-Content-Type-Options")
		if writer.status == http.StatusMethodNotAllowed {
			WriteError(w, r, apperr.MethodNotAllowed("このHTTPメソッドは使用できません。"))
			return
		}
		WriteError(w, r, apperr.NotFound("エンドポイントが見つかりません。"))
	})
}

type routingErrorWriter struct {
	http.ResponseWriter
	status int
}

func (w *routingErrorWriter) WriteHeader(status int) {
	contentType := w.Header().Get("Content-Type")
	if (status == http.StatusNotFound || status == http.StatusMethodNotAllowed) && !strings.HasPrefix(contentType, "application/problem+json") {
		w.status = status
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *routingErrorWriter) Write(body []byte) (int, error) {
	if w.status != 0 {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
