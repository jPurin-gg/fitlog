package httpx

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestWriteErrorLogsSafeDiagnosticsAndCorrelatedAccessLog(t *testing.T) {
	const sensitive = "test-only-sensitive-input"
	databaseError := func(state string) error {
		return fmt.Errorf("query parameters=%s: %w", sensitive, &pq.Error{
			Code: pq.ErrorCode(state), Message: sensitive, Detail: sensitive,
			Hint: sensitive, Schema: sensitive, Table: sensitive,
			Column: sensitive, Constraint: sensitive, InternalQuery: sensitive,
		})
	}
	tests := []struct {
		name     string
		err      error
		class    string
		sqlstate string
	}{
		{"unique constraint", databaseError("23505"), "database_constraint", "23505"},
		{"foreign key constraint", databaseError("23503"), "database_constraint", "23503"},
		{"database connection", databaseError("08006"), "database_connection", "08006"},
		{"database authentication", databaseError("28P01"), "database_authentication", "28P01"},
		{"database transaction", databaseError("40001"), "database_transaction", "40001"},
		{"database query", databaseError("42601"), "database", "42601"},
		{"invalid sqlstate", databaseError(sensitive), "database", ""},
		{"invalid sqlstate characters", databaseError("23\n01"), "database", ""},
		{"bad connection", fmt.Errorf("%s: %w", sensitive, driver.ErrBadConn), "database_connection", ""},
		{"closed connection", sql.ErrConnDone, "database_connection", ""},
		{"dial failure", &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 5432}, Err: syscall.ECONNREFUSED}, "network", ""},
		{"network timeout", &net.DNSError{Name: sensitive, IsTimeout: true}, "timeout", ""},
		{"deadline", fmt.Errorf("%s: %w", sensitive, context.DeadlineExceeded), "timeout", ""},
		{"canceled", context.Canceled, "canceled", ""},
		{"unknown error", errors.New("postgres://test-user:test-password@invalid/db?input=" + sensitive), "internal", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/workouts/{workoutID}/sets", func(w http.ResponseWriter, r *http.Request) {
				Respond(w, r, http.StatusCreated, struct{}{}, fmt.Errorf("service: %w", apperr.Internal(test.err)))
			})
			handler := WithRequestID(LogRequests(logger, Recover(logger, mux)))
			request := httptest.NewRequest(http.MethodPost, "/api/workouts/private-workout-id/sets?input="+sensitive, strings.NewReader(sensitive))
			request.Header.Set("Authorization", "Bearer test-only-token")
			request.Header.Set("Cookie", "test-only-cookie")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			var problem Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			requestID := recorder.Header().Get("X-Request-ID")
			if recorder.Code != http.StatusInternalServerError || problem.Status != http.StatusInternalServerError || problem.Code != apperr.CodeInternal || problem.Detail != "処理中にエラーが発生しました。" || problem.RequestID != requestID || requestID == "" || len(problem.Errors) != 0 {
				t.Fatalf("unexpected problem: %#v", problem)
			}
			records := decodeLogRecords(t, &output)
			if len(records) != 2 {
				t.Fatalf("log count = %d, want one diagnostic and one access log", len(records))
			}
			diagnostic, access := records[0], records[1]
			if diagnostic["level"] != "ERROR" || diagnostic["stage"] != "handler" || diagnostic["error_class"] != test.class {
				t.Fatalf("diagnostic metadata = %#v", diagnostic)
			}
			if test.sqlstate == "" {
				if _, exists := diagnostic["sqlstate"]; exists {
					t.Fatal("unexpected sqlstate field")
				}
			} else if diagnostic["sqlstate"] != test.sqlstate {
				t.Fatalf("sqlstate = %#v, want %q", diagnostic["sqlstate"], test.sqlstate)
			}
			for _, record := range records {
				if record["request_id"] != requestID || record["method"] != http.MethodPost || record["route"] != "/api/workouts/{workoutID}/sets" || record["status"] != float64(http.StatusInternalServerError) {
					t.Fatalf("correlation metadata = %#v", record)
				}
			}
			if access["msg"] != "http request" || access["response_bytes"] != float64(recorder.Body.Len()) {
				t.Fatalf("access metadata = %#v", access)
			}
			for _, forbidden := range []string{sensitive, "private-workout-id", "test-user", "test-password", "192.0.2.1", "test-only-token", "test-only-cookie"} {
				if strings.Contains(output.String(), forbidden) || strings.Contains(recorder.Body.String(), forbidden) {
					t.Fatalf("response or log contains forbidden test marker %q", forbidden)
				}
			}
		})
	}
}

func TestWriteErrorDoesNotLogClientErrorsAsInternalFailures(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 409, 429} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			handler := WithRequestID(LogRequests(logger, Recover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteError(w, r, apperr.Wrap(errors.New("test-only-sensitive-cause"), status, "CLIENT_ERROR", "Client error"))
			}))))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/unmatched-private-path", nil))
			records := decodeLogRecords(t, &output)
			if recorder.Code != status || len(records) != 1 || records[0]["msg"] != "http request" || records[0]["status"] != float64(status) || records[0]["route"] != "unmatched" {
				t.Fatalf("status = %d, logs = %#v", recorder.Code, records)
			}
			if strings.Contains(output.String(), "private-path") || strings.Contains(output.String(), "sensitive-cause") {
				t.Fatal("access log contains private request or error data")
			}
		})
	}
}

func TestWriteErrorLogsOtherServerErrors(t *testing.T) {
	for _, status := range []int{502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			handler := LogRequests(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteError(w, r, apperr.Wrap(context.DeadlineExceeded, status, apperr.CodeAIUnavailable, "Service unavailable"))
			}))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			records := decodeLogRecords(t, &output)
			if len(records) != 2 || records[0]["status"] != float64(status) || records[0]["error_class"] != "timeout" || recorder.Code != status {
				t.Fatalf("status = %d, logs = %#v", recorder.Code, records)
			}
		})
	}
}

func TestWriteErrorLogsOnlyOneFailurePerRequest(t *testing.T) {
	for _, panicAfterError := range []bool{false, true} {
		t.Run(fmt.Sprint(panicAfterError), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			handler := LogRequests(logger, Recover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteError(w, r, errors.New("first failure"))
				if panicAfterError {
					panic("test-only-sensitive-panic")
				}
				WriteError(w, r, errors.New("second failure"))
			})))
			// Reuse the handler to ensure deduplication is scoped to each request.
			for range 2 {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			}
			records := decodeLogRecords(t, &output)
			if len(records) != 4 || records[0]["level"] != "ERROR" || records[1]["level"] != "INFO" || records[2]["level"] != "ERROR" || records[3]["level"] != "INFO" {
				t.Fatalf("expected one diagnostic and one access log per request: %#v", records)
			}
			if strings.Contains(output.String(), "sensitive-panic") {
				t.Fatal("log contains panic value")
			}
		})
	}
}

func TestRecoverLogsSafePanicWithRouteAndRequestID(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/workouts/{workoutID}", func(http.ResponseWriter, *http.Request) {
		panic(errors.New("test-only-sensitive-panic"))
	})
	handler := WithRequestID(LogRequests(logger, Recover(logger, mux)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/workouts/private-workout-id", nil))
	records := decodeLogRecords(t, &output)
	if len(records) != 2 || records[0]["error_class"] != "panic" || records[0]["stage"] != "recovery" {
		t.Fatalf("panic records = %#v", records)
	}
	for _, record := range records {
		if record["route"] != "/api/workouts/{workoutID}" || record["request_id"] != recorder.Header().Get("X-Request-ID") {
			t.Fatalf("panic correlation = %#v", record)
		}
	}
	if strings.Contains(output.String(), "sensitive-panic") || strings.Contains(output.String(), "private-workout-id") || strings.Contains(recorder.Body.String(), "sensitive-panic") {
		t.Fatal("panic value or raw path leaked")
	}
}

func decodeLogRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}
