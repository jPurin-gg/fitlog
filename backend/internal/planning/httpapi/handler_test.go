package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
)

func TestPutMonthlyRejectsInvalidExerciseIDsWithoutChangingStoredPlan(t *testing.T) {
	tests := []struct {
		name         string
		ids          []string
		routineIndex int
	}{
		{name: "unknown ID", ids: []string{"__missing_exercise__"}},
		{name: "empty ID with a valid name", ids: []string{""}},
		{name: "blank ID with a valid name", ids: []string{" \t\n　"}},
		{name: "valid then unknown ID", ids: []string{"bench", "__missing_exercise__"}},
		{name: "unknown then valid ID", ids: []string{"__missing_exercise__", "bench"}},
		{name: "valid then blank ID", ids: []string{"bench", " "}},
		{name: "unknown ID in a later routine", ids: []string{"__missing_exercise__"}, routineIndex: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repository := newMonthlyHandler()
			put := "/api/monthly-plans/2026-08"
			original := planningRequest(t, handler, http.MethodPut, put, monthlyInput(), http.StatusOK)
			stored := repository.monthly
			input := monthlyInput()
			input.PlanName = "保存してはいけないプラン"
			routine := &input.WeeklyRoutine[test.routineIndex]
			routine.ExerciseIDs = test.ids
			routine.ExampleExercises = make([]string, len(test.ids))
			for i := range routine.ExampleExercises {
				routine.ExampleExercises[i] = "ベンチプレス"
			}

			response := planningRequest(t, handler, http.MethodPut, put, input, http.StatusBadRequest)
			var problem struct {
				Code   string            `json:"code"`
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") || problem.Code != apperr.CodeValidation || len(problem.Errors) == 0 {
				t.Fatalf("validation problem = %s", response.Body.String())
			}
			if repository.saveCalls != 1 || !reflect.DeepEqual(repository.monthly, stored) {
				t.Fatalf("invalid update changed storage: calls = %d, plan = %#v", repository.saveCalls, repository.monthly)
			}
			after := planningRequest(t, handler, http.MethodGet, put, nil, http.StatusOK)
			if after.Body.String() != original.Body.String() {
				t.Fatalf("stored plan changed: before = %s, after = %s", original.Body.String(), after.Body.String())
			}
		})
	}
}

func TestPutMonthlyNormalizesNamesAndSupportsDailyGetAndStart(t *testing.T) {
	handler, repository := newMonthlyHandler()
	response := planningRequest(t, handler, http.MethodPut, "/api/monthly-plans/2026-08", monthlyInput(), http.StatusOK)
	var saved planning.MonthlyPlan
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	for _, plan := range []planning.MonthlyPlan{saved, repository.monthly} {
		for _, routine := range plan.WeeklyRoutine {
			for i, id := range routine.ExerciseIDs {
				if routine.ExampleExercises[i] != repository.exercises[id] {
					t.Fatalf("name for %q = %q, want dictionary name", id, routine.ExampleExercises[i])
				}
			}
		}
	}

	for i, date := range []string{"2026-08-24", "2026-08-26"} {
		response = planningRequest(t, handler, http.MethodGet, "/api/workout-plans/"+date, nil, http.StatusOK)
		var daily planning.PlanSession
		if err := json.Unmarshal(response.Body.Bytes(), &daily); err != nil {
			t.Fatal(err)
		}
		routine := saved.WeeklyRoutine[i]
		if daily.Status != "draft" || len(daily.Plan.Exercises) != len(routine.ExerciseIDs) {
			t.Fatalf("daily plan = %#v", daily)
		}
		for index, exercise := range daily.Plan.Exercises {
			if exercise.ExerciseID != routine.ExerciseIDs[index] || exercise.Name != routine.ExampleExercises[index] {
				t.Fatalf("daily exercise = %#v, routine = %#v", exercise, routine)
			}
		}
		if i == 0 {
			// The clock is fixed to Monday, so start today's plan without an exercise override.
			response = planningRequest(t, handler, http.MethodPost, "/api/workout-plans/"+date+"/start", map[string]bool{"use_ai": false}, http.StatusOK)
			var started planning.PlanSession
			if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
				t.Fatal(err)
			}
			if started.WorkoutID == 0 || started.AIStatus != planning.AIStatusNotRequested || !reflect.DeepEqual(started.Plan, daily.Plan) {
				t.Fatalf("started plan = %#v, draft = %#v", started, daily)
			}
		}
	}
}

func TestPutMonthlyDoesNotSaveWhenExerciseLookupFails(t *testing.T) {
	handler, repository := newMonthlyHandler()
	path := "/api/monthly-plans/2026-08"
	planningRequest(t, handler, http.MethodPut, path, monthlyInput(), http.StatusOK)
	stored := repository.monthly
	repository.resolveErr = errors.New("dictionary unavailable")
	input := monthlyInput()
	input.PlanName = "保存してはいけないプラン"
	planningRequest(t, handler, http.MethodPut, path, input, http.StatusInternalServerError)
	if repository.saveCalls != 1 || !reflect.DeepEqual(repository.monthly, stored) {
		t.Fatalf("lookup failure changed storage: calls = %d, plan = %#v", repository.saveCalls, repository.monthly)
	}
}

func monthlyInput() planning.MonthlyPlanInput {
	return planning.MonthlyPlanInput{
		PlanName: "週2回プラン", RecommendedDays: []int{1, 3},
		WeeklyRoutine: []planning.DayRoutine{
			{DayName: "月曜", Target: "全身", ExerciseIDs: []string{"bench", "squat"}, ExampleExercises: []string{"誤った名称", "ベンチプレス"}},
			{DayName: "水曜", Target: "補助", ExerciseIDs: []string{"custom"}, ExampleExercises: []string{""}},
		},
	}
}

func newMonthlyHandler() (http.Handler, *monthlyRepository) {
	repository := &monthlyRepository{exercises: map[string]string{"bench": "ベンチプレス", "squat": "スクワット", "custom": "カスタム種目"}, daily: map[string]planning.PlanSession{}}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(planning.NewService(repository, nil, nil, nil, clock.Fixed{Time: now}, time.Second))
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/monthly-plans/{month}", handler.PutMonthly)
	mux.HandleFunc("GET /api/monthly-plans/{month}", handler.Monthly)
	mux.HandleFunc("GET /api/workout-plans/{date}", handler.Daily)
	mux.HandleFunc("POST /api/workout-plans/{date}/start", handler.StartDaily)
	return mux, repository
}

func planningRequest(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, bytes.NewReader(encoded)))
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, recorder.Code, wantStatus, recorder.Body.String())
	}
	return recorder
}

// Only persistence and dictionary access are replaced; HTTP and service logic are real.
type monthlyRepository struct {
	planning.Repository
	monthly    planning.MonthlyPlan
	exercises  map[string]string
	daily      map[string]planning.PlanSession
	resolveErr error
	saveCalls  int
}

func (r *monthlyRepository) SaveMonthly(_ context.Context, _ int, month string, plan planning.MonthlyPlan) (planning.MonthlyPlan, error) {
	r.saveCalls++
	plan.ID, plan.PlanMonth = 1, month
	r.monthly = plan
	return plan, nil
}

func (r *monthlyRepository) Monthly(context.Context, int, string) (planning.MonthlyPlan, error) {
	return r.monthly, nil
}

func (r *monthlyRepository) ResolveExercise(_ context.Context, _ int, id, name string) (planning.ExerciseStats, string, error) {
	if r.resolveErr != nil {
		return planning.ExerciseStats{}, "", r.resolveErr
	}
	if id == "" {
		for candidateID, candidateName := range r.exercises {
			if name == candidateName {
				id = candidateID
				break
			}
		}
	}
	displayName, ok := r.exercises[id]
	if !ok {
		return planning.ExerciseStats{}, "", fmt.Errorf("dictionary lookup: %w", planning.ErrExerciseNotFound)
	}
	return planning.ExerciseStats{Name: displayName, TargetSets: 3, TargetReps: 10, TargetWeight: 20}, id, nil
}

func (r *monthlyRepository) Daily(_ context.Context, _ int, date time.Time) (planning.PlanSession, error) {
	session, ok := r.daily[date.Format("2006-01-02")]
	if !ok {
		return planning.PlanSession{}, planning.ErrNotFound
	}
	return session, nil
}

func (r *monthlyRepository) CreateDailyIfMissing(_ context.Context, _ int, date time.Time, plan planning.WorkoutPlan) (bool, error) {
	key := date.Format("2006-01-02")
	if _, ok := r.daily[key]; ok {
		return false, nil
	}
	r.daily[key] = planning.PlanSession{ID: len(r.daily) + 1, PlanDate: key, Status: "active", Plan: plan}
	return true, nil
}

func (r *monthlyRepository) AttachWorkout(_ context.Context, _ int, date time.Time) (planning.PlanSession, error) {
	key := date.Format("2006-01-02")
	session := r.daily[key]
	session.WorkoutID = 9
	r.daily[key] = session
	return session, nil
}
