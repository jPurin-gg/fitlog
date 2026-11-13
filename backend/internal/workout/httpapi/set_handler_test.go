package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

type setRepository struct {
	recordCalls int
	updateCalls int
	deleteCalls int
	patch       workout.SetPatch
	updated     workout.Set
	err         error
}

func (r *setRepository) RecordSet(context.Context, int, int, string, workout.SetInput) (workout.Set, bool, error) {
	r.recordCalls++
	return workout.Set{ID: 1}, false, nil
}

func (r *setRepository) UpdateSet(_ context.Context, _, _, _ int, patch workout.SetPatch) (workout.Set, error) {
	r.updateCalls++
	r.patch = patch
	return r.updated, r.err
}

func (r *setRepository) DeleteSet(context.Context, int, int, int) error {
	r.deleteCalls++
	return r.err
}

func (*setRepository) RecommendationContext(context.Context, int, int, int) (workout.RecommendationContext, error) {
	return workout.RecommendationContext{}, nil
}

func (*setRepository) Finish(context.Context, int, int) (workout.Detail, error) {
	return workout.Detail{}, nil
}

func (*setRepository) Detail(context.Context, int, int) (workout.Detail, error) {
	return workout.Detail{}, nil
}

func (*setRepository) SaveSummaryComment(context.Context, int, int, string) (string, bool, error) {
	return "", false, nil
}

func (*setRepository) CalendarWorkout(context.Context, int, time.Time) (workout.CalendarWorkout, error) {
	return workout.CalendarWorkout{}, nil
}

func (*setRepository) SaveCalendarWorkout(context.Context, int, time.Time, workout.CalendarWorkoutInput) (workout.CalendarWorkout, error) {
	return workout.CalendarWorkout{}, nil
}

func serveSetRequest(repository *setRepository, method, path, body string) *httptest.ResponseRecorder {
	handler := NewHandler(workout.NewService(repository, nil, nil, nil, 0))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workouts/{workoutID}/sets", handler.RecordSet)
	mux.HandleFunc("PATCH /api/workouts/{workoutID}/sets/{setID}", handler.UpdateSet)
	mux.HandleFunc("DELETE /api/workouts/{workoutID}/sets/{setID}", handler.DeleteSet)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request_123")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func TestRecordSetRejectsFractionalReps(t *testing.T) {
	repository := &setRepository{}
	recorder := serveSetRequest(repository, http.MethodPost, "/api/workouts/42/sets", `{"exercise_id":"bench","set_order":1,"weight":80,"reps":10.5,"feeling":""}`)
	if recorder.Code != http.StatusBadRequest || repository.recordCalls != 0 {
		t.Fatalf("POST fractional reps = %d %s; record calls = %d", recorder.Code, recorder.Body.String(), repository.recordCalls)
	}
}

func TestUpdateSetReturnsSavedSet(t *testing.T) {
	repository := &setRepository{updated: workout.Set{ID: 7, WorkoutID: 42, ExerciseID: "bench", Weight: 80, Reps: 10}}
	recorder := serveSetRequest(repository, http.MethodPatch, "/api/workouts/42/sets/7", `{"weight":80,"reps":10}`)
	if recorder.Code != http.StatusOK || repository.updateCalls != 1 {
		t.Fatalf("PATCH = %d %s; update calls = %d", recorder.Code, recorder.Body.String(), repository.updateCalls)
	}
	var body workout.Set
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.ID != 7 || body.Weight != 80 || body.Reps != 10 {
		t.Fatalf("PATCH body = %s, %v", recorder.Body.String(), err)
	}
	if repository.patch.Weight == nil || *repository.patch.Weight != 80 || repository.patch.Reps == nil || *repository.patch.Reps != 10 || repository.patch.Feeling != nil {
		t.Fatalf("PATCH patch = %#v", repository.patch)
	}
}

func TestUpdateSetRejectsInvalidRequestsBeforePersistence(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "fractional_reps", path: "/api/workouts/42/sets/7", body: `{"reps":10.5}`},
		{name: "reps_over_max", path: "/api/workouts/42/sets/7", body: `{"reps":1001}`},
		{name: "weight_over_max", path: "/api/workouts/42/sets/7", body: `{"weight":2000.5}`},
		{name: "negative_weight", path: "/api/workouts/42/sets/7", body: `{"weight":-1}`},
		{name: "empty_patch", path: "/api/workouts/42/sets/7", body: `{}`},
		{name: "exercise_cannot_change", path: "/api/workouts/42/sets/7", body: `{"exercise_id":"squat"}`},
		{name: "invalid_set_id", path: "/api/workouts/42/sets/0", body: `{"reps":8}`},
		{name: "invalid_workout_id", path: "/api/workouts/abc/sets/7", body: `{"reps":8}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &setRepository{}
			recorder := serveSetRequest(repository, http.MethodPatch, tt.path, tt.body)
			if recorder.Code != http.StatusBadRequest || repository.updateCalls != 0 {
				t.Fatalf("PATCH = %d %s; update calls = %d", recorder.Code, recorder.Body.String(), repository.updateCalls)
			}
		})
	}
}

func TestDeleteSetReturnsNoContent(t *testing.T) {
	repository := &setRepository{}
	recorder := serveSetRequest(repository, http.MethodDelete, "/api/workouts/42/sets/7", "")
	if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 || repository.deleteCalls != 1 {
		t.Fatalf("DELETE = %d %q; delete calls = %d", recorder.Code, recorder.Body.String(), repository.deleteCalls)
	}
}

func TestSetCorrectionOnCompletedWorkoutConflicts(t *testing.T) {
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		repository := &setRepository{err: workout.ErrConflict}
		recorder := serveSetRequest(repository, method, "/api/workouts/42/sets/7", `{"reps":8}`)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("%s on completed workout = %d %s", method, recorder.Code, recorder.Body.String())
		}
	}
}
