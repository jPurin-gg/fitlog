package workout

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

func TestRecordSetEnforcesSharedInputLimits(t *testing.T) {
	tests := []struct {
		name      string
		weight    float64
		reps      int
		wantField string
	}{
		{name: "max_reps_accepted", weight: 80, reps: MaxReps},
		{name: "reps_over_max_rejected", weight: 80, reps: MaxReps + 1, wantField: "reps"},
		{name: "max_weight_accepted", weight: MaxWeight, reps: 1},
		{name: "weight_over_max_rejected", weight: MaxWeight + 0.5, reps: 10, wantField: "weight"},
		{name: "typo_weight_over_max_rejected", weight: 8000, reps: 10, wantField: "weight"},
		{name: "nan_weight_rejected", weight: math.NaN(), reps: 10, wantField: "weight"},
		{name: "infinite_weight_rejected", weight: math.Inf(1), reps: 10, wantField: "weight"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &stubRepository{}
			service := NewService(repository, nil, nil, nil, 0)
			_, _, err := service.RecordSet(context.Background(), 1, 2, "request_123", SetInput{ExerciseID: "bench", SetOrder: 1, Weight: tt.weight, Reps: tt.reps})
			if tt.wantField == "" {
				if err != nil || repository.recordCalls != 1 {
					t.Fatalf("RecordSet() error = %v; repository calls = %d", err, repository.recordCalls)
				}
				return
			}
			appErr := apperr.As(err)
			if appErr.Status != http.StatusBadRequest || appErr.Fields[tt.wantField] == "" || repository.recordCalls != 0 {
				t.Fatalf("RecordSet() error = %#v; repository calls = %d", appErr, repository.recordCalls)
			}
		})
	}
}

func TestSaveCalendarWorkoutEnforcesSharedInputLimits(t *testing.T) {
	tests := []struct {
		name    string
		set     CalendarSet
		wantErr bool
	}{
		{name: "limits_accepted", set: CalendarSet{ExerciseID: "bench", Weight: MaxWeight, Reps: MaxReps}},
		{name: "reps_over_max_rejected", set: CalendarSet{ExerciseID: "bench", Weight: 80, Reps: MaxReps + 1}, wantErr: true},
		{name: "weight_over_max_rejected", set: CalendarSet{ExerciseID: "bench", Weight: MaxWeight + 1, Reps: 10}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(&stubRepository{}, nil, nil, clock.Fixed{Time: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}, 0)
			_, err := service.SaveCalendarWorkout(context.Background(), 1, "2026-11-12", CalendarWorkoutInput{Sets: []CalendarSet{tt.set}})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("SaveCalendarWorkout() error = %v", err)
				}
				return
			}
			if appErr := apperr.As(err); appErr.Status != http.StatusBadRequest {
				t.Fatalf("SaveCalendarWorkout() error = %#v", appErr)
			}
		})
	}
}
