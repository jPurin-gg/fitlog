package httpx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

var errPanicRecovered = errors.New("panic recovered")

type errorLogKey struct{}

type errorLogState struct {
	logger *slog.Logger
	once   sync.Once
}

func withErrorLogger(r *http.Request, logger *slog.Logger) *http.Request {
	if _, ok := r.Context().Value(errorLogKey{}).(*errorLogState); ok {
		return r
	}
	state := &errorLogState{logger: logger}
	return r.WithContext(context.WithValue(r.Context(), errorLogKey{}, state))
}

func logServerError(r *http.Request, err *apperr.Error, stage string) {
	state, _ := r.Context().Value(errorLogKey{}).(*errorLogState)
	if state == nil {
		state = &errorLogState{logger: slog.Default()}
	}
	state.once.Do(func() {
		class, sqlstate := classifyError(err)
		attributes := []any{
			"request_id", RequestID(r.Context()),
			"method", r.Method,
			"route", routePattern(r.Pattern),
			"stage", stage,
			"status", err.Status,
			"error_class", class,
		}
		if sqlstate != "" {
			attributes = append(attributes, "sqlstate", sqlstate)
		}
		// Never log Error(), driver details, panic values, or request contents.
		state.logger.ErrorContext(r.Context(), "http request failed", attributes...)
	})
}

func classifyError(err error) (class, sqlstate string) {
	var databaseError interface{ SQLState() string }
	if errors.As(err, &databaseError) {
		state := databaseError.SQLState()
		if validSQLState(state) {
			sqlstate = state
			switch state[:2] {
			case "23":
				return "database_constraint", sqlstate
			case "08":
				return "database_connection", sqlstate
			case "28":
				return "database_authentication", sqlstate
			case "40":
				return "database_transaction", sqlstate
			}
		}
		return "database", sqlstate
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return "database_connection", ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", ""
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", ""
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		if networkError.Timeout() {
			return "timeout", ""
		}
		return "network", ""
	}
	if errors.Is(err, errPanicRecovered) {
		return "panic", ""
	}
	return "internal", ""
}

// SQLSTATE is five ASCII letters/digits. Reject arbitrary driver strings rather
// than attempting to redact messages that may contain credentials or input.
func validSQLState(state string) bool {
	if len(state) != 5 {
		return false
	}
	for _, character := range state {
		if (character < '0' || character > '9') && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}
