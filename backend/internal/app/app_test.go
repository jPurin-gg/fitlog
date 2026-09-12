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

func TestNewHandlerRegistersProtectedRoutes(t *testing.T) {
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
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodDelete, "/api/auth/session"},
		{http.MethodGet, "/api/dashboard"},
		{http.MethodGet, "/api/calendar?year=2026&month=8"},
		{http.MethodGet, "/api/preferences"},
		{http.MethodPut, "/api/preferences"},
		{http.MethodGet, "/api/exercises"},
		{http.MethodPost, "/api/exercises"},
		{http.MethodGet, "/api/exercises/recent"},
		{http.MethodGet, "/api/exercises/favorites"},
		{http.MethodPut, "/api/exercises/bench/favorite"},
		{http.MethodDelete, "/api/exercises/bench/favorite"},
		{http.MethodGet, "/api/exercises/bench/settings"},
		{http.MethodPut, "/api/exercises/bench/settings"},
		{http.MethodPost, "/api/exercises/bench/alternatives"},
		{http.MethodGet, "/api/monthly-plans"},
		{http.MethodGet, "/api/monthly-plans/2026-08"},
		{http.MethodPut, "/api/monthly-plans/2026-08"},
		{http.MethodPost, "/api/monthly-plans/2026-08/generate"},
		{http.MethodGet, "/api/workout-plans/2026-08-24"},
		{http.MethodPut, "/api/workout-plans/2026-08-24"},
		{http.MethodPost, "/api/workout-plans/2026-08-24/start"},
		{http.MethodGet, "/api/workouts/by-date/2026-08-24"},
		{http.MethodPut, "/api/workouts/by-date/2026-08-24"},
		{http.MethodGet, "/api/workouts/42"},
		{http.MethodPost, "/api/workouts/42/sets"},
		{http.MethodPost, "/api/workouts/42/sets/7/recommendation"},
		{http.MethodPost, "/api/workouts/42/finish"},
		{http.MethodPost, "/api/workouts/42/summary-comment"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != http.StatusUnauthorized || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("%s %s = %d, %q", test.method, test.path, recorder.Code, recorder.Header().Get("Content-Type"))
		}
	}
}
