package workout

import (
	"context"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestRecordSetRequiresIdempotencyKey(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, nil, nil, nil)
	_, _, err := service.RecordSet(context.Background(), 1, 2, "short", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
	appErr := apperr.As(err)
	if appErr.Status != 400 || repository.recordCalls != 0 {
		t.Fatalf("RecordSet() error = %#v; repository calls = %d", appErr, repository.recordCalls)
	}
}

func TestRecordSetPassesStableKeyToRepository(t *testing.T) {
	repository := &stubRepository{recorded: Set{ID: 10, WorkoutID: 2, ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10}}
	service := NewService(repository, nil, nil, nil)
	result, replayed, err := service.RecordSet(context.Background(), 1, 2, "request_123", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 80, Reps: 10})
	if err != nil || replayed || result.ID != 10 || repository.key != "request_123" {
		t.Fatalf("RecordSet() = %#v, %v, %v; key = %q", result, replayed, err, repository.key)
	}
}

func TestValidateRecommendationNormalizesActionAndRejectsInvalidTargets(t *testing.T) {
	valid := Recommendation{NextAction: " adjust ", Recommendation: " 次は軽く ", Reason: " 疲労 ", TargetWeight: 20, TargetReps: 8}
	if err := validateRecommendation(&valid); err != nil || valid.NextAction != "ADJUST" {
		t.Fatalf("validateRecommendation(valid) = %#v, %v", valid, err)
	}
	invalid := Recommendation{NextAction: "CONTINUE", Recommendation: "続行", Reason: "問題なし", TargetWeight: -1, TargetReps: 8}
	if err := validateRecommendation(&invalid); err == nil {
		t.Fatal("validateRecommendation(negative weight) error = nil")
	}
}

type stubRepository struct {
	recordCalls int
	key         string
	recorded    Set
}

func (r *stubRepository) RecordSet(_ context.Context, _, _ int, key string, _ SetInput) (Set, bool, error) {
	r.recordCalls++
	r.key = key
	return r.recorded, false, nil
}

func (*stubRepository) RecommendationContext(context.Context, int, int, int) (RecommendationContext, error) {
	return RecommendationContext{}, nil
}

func (*stubRepository) Finish(context.Context, int, int) (Detail, error)           { return Detail{}, nil }
func (*stubRepository) Detail(context.Context, int, int) (Detail, error)           { return Detail{}, nil }
func (*stubRepository) SaveSummaryComment(context.Context, int, int, string) error { return nil }
func (*stubRepository) CalendarWorkout(context.Context, int, time.Time) (CalendarWorkout, error) {
	return CalendarWorkout{}, nil
}
func (*stubRepository) SaveCalendarWorkout(context.Context, int, time.Time, CalendarWorkoutInput) (CalendarWorkout, error) {
	return CalendarWorkout{}, nil
}
