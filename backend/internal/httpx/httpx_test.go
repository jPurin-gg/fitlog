package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestDecodeAndRespondWritesDirectJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}
	type response struct {
		Greeting string `json:"greeting"`
	}

	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"Fitlog"}`))
	DecodeAndRespond(recorder, httpRequest, http.StatusCreated, func(input request) (response, error) {
		return response{Greeting: "Hello " + input.Name}, nil
	})

	if recorder.Code != http.StatusCreated || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" || recorder.Body.String() != `{"greeting":"Hello Fitlog"}`+"\n" {
		t.Fatalf("response = %d, %q, %q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestDecodeAndRespondRejectsUnknownFieldsAsProblemDetails(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}
	called := false
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"Fitlog","user_id":99}`))
	DecodeAndRespond(recorder, httpRequest, http.StatusOK, func(_ request) (request, error) {
		called = true
		return request{}, nil
	})

	if called || recorder.Code != http.StatusBadRequest || recorder.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" || !strings.Contains(recorder.Body.String(), `"code":"VALIDATION_ERROR"`) {
		t.Fatalf("called = %v, response = %d, %q, %q", called, recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestRespondUsesProblemDetailsForServiceErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodGet, "/test", nil)
	Respond(recorder, httpRequest, http.StatusOK, struct{}{}, apperr.NotFound("見つかりません。"))

	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"status":404`) || !strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("response = %d, %q", recorder.Code, recorder.Body.String())
	}
}

func TestLogRequestsRecordsRouteAndResponseMetadata(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workouts/{workoutID}/sets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "saved")
	})
	handler := WithRequestID(LogRequests(logger, mux))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workouts/private-workout-id/sets", nil)
	handler.ServeHTTP(recorder, request)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode access log: %v; output = %q", err, output.String())
	}
	requestID := recorder.Header().Get("X-Request-ID")
	if requestID == "" || record["request_id"] != requestID {
		t.Fatalf("request IDs = header %q, log %#v", requestID, record["request_id"])
	}
	if record["method"] != http.MethodPost || record["route"] != "/api/workouts/{workoutID}/sets" {
		t.Fatalf("method/route = %#v/%#v", record["method"], record["route"])
	}
	if record["status"] != float64(http.StatusCreated) || record["response_bytes"] != float64(len("saved")) {
		t.Fatalf("status/response_bytes = %#v/%#v", record["status"], record["response_bytes"])
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Fatal("duration_ms is missing")
	}
	if strings.Contains(output.String(), "private-workout-id") {
		t.Fatalf("access log contains the raw request path: %s", output.String())
	}
}

func TestLogRequestsRecordsImplicitSuccessAndUnmatchedRoute(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := LogRequests(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private/path", nil))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode access log: %v", err)
	}
	if record["status"] != float64(http.StatusOK) || record["response_bytes"] != float64(0) || record["route"] != "unmatched" {
		t.Fatalf("access log metadata = %#v", record)
	}
	if strings.Contains(output.String(), "/private/path") {
		t.Fatalf("access log contains the raw request path: %s", output.String())
	}
}

func TestLogRequestsIncludesRecoveredPanicResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	})
	handler := WithRequestID(LogRequests(logger, Recover(logger, panicHandler)))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("response status = %d", recorder.Code)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("log records = %d; output = %q", len(lines), output.String())
	}
	var accessRecord map[string]any
	if err := json.Unmarshal(lines[1], &accessRecord); err != nil {
		t.Fatalf("decode access log: %v", err)
	}
	if accessRecord["status"] != float64(http.StatusInternalServerError) || accessRecord["response_bytes"] != float64(recorder.Body.Len()) {
		t.Fatalf("panic access metadata = %#v; body bytes = %d", accessRecord, recorder.Body.Len())
	}
}
