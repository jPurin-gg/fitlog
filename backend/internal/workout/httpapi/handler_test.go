package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestSetLimitsReturnBadRequestBeforePersistence(t *testing.T) {
	// A nil repository makes any attempt to persist a rejected input fail the test.
	handler := NewHandler(workout.NewService(nil, nil, nil, clock.Fixed{Time: time.Now()}, 0))
	for _, values := range []string{
		`"weight":80,"reps":1001`,
		`"weight":80,"reps":2147483647`,
		`"weight":2000.01,"reps":10`,
		`"weight":1e308,"reps":1000`,
	} {
		set := `{"exercise_id":"bench","set_order":1,` + values + `}`
		for _, calendar := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/calendar=%v", values, calendar), func(t *testing.T) {
				body, method := set, http.MethodPost
				if calendar {
					body, method = `{"sets":[`+set+`]}`, http.MethodPut
				}
				request := httptest.NewRequest(method, "/", strings.NewReader(body))
				request.SetPathValue("workoutID", "2")
				request.SetPathValue("date", "2026-11-13")
				request.Header.Set("Idempotency-Key", "request_123")
				recorder := httptest.NewRecorder()
				if calendar {
					handler.PutCalendarWorkout(recorder, request)
				} else {
					handler.RecordSet(recorder, request)
				}
				if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"VALIDATION_ERROR"`) {
					t.Fatalf("response = %d, %s", recorder.Code, recorder.Body.String())
				}
			})
		}
	}
}
