package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
)

func TestDailyPlannedSetsRange(t *testing.T) {
	for _, test := range []struct {
		sets   string
		status int
	}{
		{sets: "1", status: http.StatusOK},
		{sets: "20", status: http.StatusOK},
		{sets: "0", status: http.StatusBadRequest},
		{sets: "-1", status: http.StatusBadRequest},
		{sets: "21", status: http.StatusBadRequest},
		{sets: "4294967296", status: http.StatusBadRequest},
		{sets: "9223372036854775808", status: http.StatusBadRequest},
		{sets: "1.5", status: http.StatusBadRequest},
		{sets: "null", status: http.StatusBadRequest},
		{sets: "missing", status: http.StatusBadRequest},
	} {
		t.Run(test.sets, func(t *testing.T) {
			now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
			original := planning.PlanSession{
				ID: 7, WorkoutID: 9, PlanDate: "2026-08-24", Status: "active",
				Plan: planning.WorkoutPlan{
					WorkoutTitle: "保存済みの予定",
					Exercises:    []planning.PlanExercise{{ExerciseID: "bench", Name: "ベンチプレス", PlannedSets: 3}},
				},
			}
			repository := &dailyPlanRepository{session: original}
			service := planning.NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second)
			handler := NewHandler(service)
			mux := http.NewServeMux()
			mux.HandleFunc("PUT /api/workout-plans/{date}", handler.PutDaily)
			mux.HandleFunc("GET /api/workout-plans/{date}", handler.Daily)
			mux.HandleFunc("POST /api/workout-plans/{date}/start", handler.StartDaily)

			// The later exercise must also be validated before any plan is saved.
			setsField := ""
			if test.sets != "missing" {
				setsField = fmt.Sprintf(`,"planned_sets":%s`, test.sets)
			}
			body := fmt.Sprintf(`{"workout_title":"編集した予定","exercises":[{"exercise_id":"bench","planned_sets":1},{"exercise_id":"squat"%s}]}`, setsField)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/workout-plans/2026-08-24", strings.NewReader(body)))
			if recorder.Code != test.status {
				t.Fatalf("PUT status = %d, want %d; body = %s", recorder.Code, test.status, recorder.Body.String())
			}
			wantSaveCalls := 0
			if test.status == http.StatusOK {
				wantSaveCalls = 1
				var saved planning.PlanSession
				if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprint(saved.Plan.Exercises[1].PlannedSets); got != test.sets {
					t.Fatalf("saved planned_sets = %s, want %s", got, test.sets)
				}
			} else {
				if !strings.Contains(recorder.Body.String(), `"code":"VALIDATION_ERROR"`) {
					t.Fatalf("expected validation problem, got %s", recorder.Body.String())
				}
				if !reflect.DeepEqual(repository.session, original) {
					t.Fatalf("invalid input changed the stored plan: %#v", repository.session)
				}
			}
			if repository.saveCalls != wantSaveCalls {
				t.Fatalf("SaveDaily calls = %d, want %d", repository.saveCalls, wantSaveCalls)
			}

			// Both displaying and starting must reuse the stored plan, including
			// the original plan after a rejected update.
			for _, request := range []*http.Request{
				httptest.NewRequest(http.MethodGet, "/api/workout-plans/2026-08-24", nil),
				httptest.NewRequest(http.MethodPost, "/api/workout-plans/2026-08-24/start", strings.NewReader(`{"use_ai":false}`)),
			} {
				recorder := httptest.NewRecorder()
				mux.ServeHTTP(recorder, request)
				var result planning.PlanSession
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if recorder.Code != http.StatusOK || !reflect.DeepEqual(result.Plan, repository.session.Plan) || result.WorkoutID != 9 {
					t.Fatalf("%s = %d, %#v", request.Method, recorder.Code, result)
				}
			}
			if repository.saveCalls != wantSaveCalls {
				t.Fatal("displaying or starting rewrote the saved plan")
			}
		})
	}
}

type dailyPlanRepository struct {
	planning.Repository
	session   planning.PlanSession
	saveCalls int
}

func (r *dailyPlanRepository) Daily(context.Context, int, time.Time) (planning.PlanSession, error) {
	return r.session, nil
}

func (r *dailyPlanRepository) SaveDaily(_ context.Context, _ int, _ time.Time, plan planning.WorkoutPlan) (planning.PlanSession, error) {
	r.saveCalls++
	r.session.Plan = plan
	return r.session, nil
}

func (r *dailyPlanRepository) AttachWorkout(context.Context, int, time.Time) (planning.PlanSession, error) {
	return r.session, nil
}

func (*dailyPlanRepository) ResolveExercise(_ context.Context, _ int, exerciseID, _ string) (planning.ExerciseStats, string, error) {
	return planning.ExerciseStats{Name: exerciseID}, exerciseID, nil
}
