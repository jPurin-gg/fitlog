//go:build integration

package postgres_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestSetLimitBoundariesRemainReadableAndFinishable(t *testing.T) {
	fixture := newIntegrationFixture(t)
	service := workout.NewService(fixture.repository, nil, nil, clock.Fixed{Time: time.Now()}, 0)
	workoutID := fixture.createWorkout(t)
	for _, key := range []string{"boundary_set_one", "boundary_set_two"} {
		_, _, err := service.RecordSet(context.Background(), fixture.userID, workoutID, key, workout.SetInput{
			ExerciseID: fixture.exerciseID, SetOrder: 1, Reps: workout.MaxReps, Weight: workout.MaxWeight,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	assertSummary := func(detail workout.Detail, err error) {
		t.Helper()
		if err != nil || detail.Summary.TotalReps != 2000 || detail.Summary.TotalVolume != 4000000 {
			t.Fatalf("detail = %+v, error = %v", detail, err)
		}
	}
	assertSummary(fixture.repository.Detail(context.Background(), fixture.userID, workoutID))
	assertSummary(fixture.repository.Finish(context.Background(), fixture.userID, workoutID))
	assertSummary(fixture.repository.Detail(context.Background(), fixture.userID, workoutID))

	calendar, err := service.SaveCalendarWorkout(context.Background(), fixture.userID, "2026-10-01", workout.CalendarWorkoutInput{Sets: []workout.CalendarSet{
		{ExerciseID: fixture.exerciseID, Reps: workout.MaxReps, Weight: workout.MaxWeight},
		{ExerciseID: fixture.exerciseID, Reps: workout.MaxReps, Weight: workout.MaxWeight},
	}})
	if err != nil {
		t.Fatal(err)
	}
	assertSummary(fixture.repository.Detail(context.Background(), fixture.userID, calendar.WorkoutID))
}

func TestLegacyRepetitionTotalsRemainReadableAndFinishable(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	// Seed values accepted before the API limit was introduced.
	if _, err := fixture.db.Exec(`
		INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order)
		VALUES ($1,$2,80,2147483647,1),($1,$2,80,2147483647,2)
	`, workoutID, fixture.exerciseID); err != nil {
		t.Fatal(err)
	}
	for _, finish := range []bool{false, true, false, true} {
		read := fixture.repository.Detail
		if finish {
			read = fixture.repository.Finish
		}
		detail, err := read(context.Background(), fixture.userID, workoutID)
		if err != nil || detail.Summary.TotalReps != 4294967294 || len(detail.Summary.Exercises) != 1 || detail.Summary.Exercises[0].TotalReps != 4294967294 || detail.Summary.TotalVolume != 343597383520 {
			t.Fatalf("finish=%v: detail = %+v, error = %v", finish, detail, err)
		}
	}
}

func TestFinishRollsBackCompletionWhenLegacyVolumeIsUnsafe(t *testing.T) {
	fixture := newIntegrationFixture(t)
	for index, weight := range []float64{math.MaxFloat64, math.Inf(1), math.NaN()} {
		workoutID := fixture.createWorkout(t)
		if _, err := fixture.db.Exec(`
			INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order) VALUES ($1,$2,$3,2,1)
		`, workoutID, fixture.exerciseID, weight); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`
			INSERT INTO workout_plans (workout_id,user_id,plan_date,title,status,plan)
			VALUES ($1,$2,$3::date,'Unsafe workout','active','{}')
		`, workoutID, fixture.userID, time.Date(2026, 11, 13+index, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := fixture.repository.Finish(context.Background(), fixture.userID, workoutID); err == nil {
				t.Fatal("Finish() succeeded with an unsafe legacy volume")
			}
			var endedAt any
			var status string
			if err := fixture.db.QueryRow(`
				SELECT w.ended_at,p.status FROM workouts w JOIN workout_plans p ON p.workout_id=w.id WHERE w.id=$1
			`, workoutID).Scan(&endedAt, &status); err != nil {
				t.Fatal(err)
			}
			if endedAt != nil || status != "active" {
				t.Fatalf("failed Finish() persisted completion: ended_at=%v, status=%s", endedAt, status)
			}
		}
	}
}
