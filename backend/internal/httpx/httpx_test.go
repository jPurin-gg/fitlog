package httpx

import (
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
