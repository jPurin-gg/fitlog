package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

const maxJSONBodyBytes = 1 << 20

type Problem struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Detail    string            `json:"detail"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id"`
	Errors    map[string]string `json:"errors,omitempty"`
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperr.Validation("JSONリクエストを解析できません。", nil)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Validation("JSONは1つの値だけ送信してください。", nil)
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func Respond[T any](w http.ResponseWriter, r *http.Request, status int, value T, err error) {
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, status, value)
}

func DecodeAndRespond[Request, Response any](w http.ResponseWriter, r *http.Request, status int, operation func(Request) (Response, error)) {
	var request Request
	if err := DecodeJSON(w, r, &request); err != nil {
		WriteError(w, r, err)
		return
	}
	response, err := operation(request)
	Respond(w, r, status, response, err)
}

func RespondNoContent(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := apperr.As(err)
	problem := Problem{
		Type:      "/problems/" + strings.ToLower(strings.ReplaceAll(appErr.Code, "_", "-")),
		Title:     titleForStatus(appErr.Status),
		Status:    appErr.Status,
		Detail:    appErr.Detail,
		Code:      appErr.Code,
		RequestID: RequestID(r.Context()),
		Errors:    appErr.Fields,
	}
	WriteProblem(w, problem)
}

func WriteProblem(w http.ResponseWriter, problem Problem) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("http panic recovered", "request_id", RequestID(r.Context()), "panic", recovered)
				WriteError(w, r, apperr.Internal(errors.New("panic recovered")))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func titleForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Invalid request"
	case http.StatusUnauthorized:
		return "Authentication required"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Resource not found"
	case http.StatusMethodNotAllowed:
		return "Method not allowed"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusTooManyRequests:
		return "Rate limited"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "Service unavailable"
	default:
		return "Internal server error"
	}
}
