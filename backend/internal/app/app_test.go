package app

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/config"
)

func TestNewHandlerRegistersAllRoutesWithoutConflicts(t *testing.T) {
	cfg := config.Config{
		FrontendURL:       "http://localhost:3000",
		Timezone:          time.UTC,
		SessionSecret:     []byte("0123456789abcdef0123456789abcdef"),
		SessionCookieName: "fitlog_session",
		SessionTTL:        30 * 24 * time.Hour,
		PromptDir:         "../../prompts",
		AI: config.AIConfig{
			Timeout: time.Second,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(&sql.DB{}, cfg, logger)
	if handler == nil {
		t.Fatal("NewHandler() = nil")
	}

	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodPost, path: "/api/dashboard", status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/api/not-found", status: http.StatusNotFound},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("%s %s = %d, %q", test.method, test.path, recorder.Code, recorder.Header().Get("Content-Type"))
		}
	}
}
