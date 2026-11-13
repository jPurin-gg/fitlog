//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	planningpostgres "github.com/jPurin-gg/myfitlog-backend/internal/planning/postgres"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestUpdateSetRecalculatesPRAndNextTargetAfterTypo(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	fixture.recordCompletedHistory(t, 80)

	workoutID := fixture.createWorkout(t)
	typo := fixture.recordSet(t, workoutID, "typo_set_key", 1, 800)
	next := fixture.recordSet(t, workoutID, "next_set_key", 2, 85)
	if !typo.IsPR || next.IsPR {
		t.Fatalf("before correction: typo.IsPR=%v next.IsPR=%v, want true/false", typo.IsPR, next.IsPR)
	}
	fixture.assertStats(t, 85, 800)

	weight := 80.0
	corrected, err := fixture.repository.UpdateSet(ctx, fixture.userID, workoutID, typo.ID, workout.SetPatch{Weight: &weight})
	if err != nil {
		t.Fatalf("UpdateSet() error = %v", err)
	}
	if corrected.Weight != 80 || corrected.Reps != typo.Reps || corrected.IsPR {
		t.Fatalf("UpdateSet() = %+v, want 80kg, unchanged reps, not a PR", corrected)
	}
	sets := fixture.storedSets(t, workoutID)
	if len(sets) != 2 || sets[0].IsPR || !sets[1].IsPR {
		t.Fatalf("stored sets after correction = %+v, want only the 85kg set as PR", sets)
	}
	fixture.assertStats(t, 85, 85)
	fixture.assertNextTarget(t, 85, 85)
}

func TestDeleteSetRecalculatesStatsWithoutTypo(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	fixture.recordCompletedHistory(t, 80)

	workoutID := fixture.createWorkout(t)
	typo := fixture.recordSet(t, workoutID, "typo_set_key", 1, 800)
	fixture.assertStats(t, 800, 800)

	if err := fixture.repository.DeleteSet(ctx, fixture.userID, workoutID, typo.ID); err != nil {
		t.Fatalf("DeleteSet() error = %v", err)
	}
	if sets := fixture.storedSets(t, workoutID); len(sets) != 0 {
		t.Fatalf("stored sets after delete = %+v, want none", sets)
	}
	fixture.assertStats(t, 80, 80)
	fixture.assertNextTarget(t, 80, 80)

	retry := fixture.recordSet(t, workoutID, "retry_set_key", 1, 82.5)
	if !retry.IsPR {
		t.Fatalf("re-recorded set = %+v, want PR over the 80kg history", retry)
	}
}

func TestDeleteOnlySetClearsStats(t *testing.T) {
	fixture := newIntegrationFixture(t)
	workoutID := fixture.createWorkout(t)
	typo := fixture.recordSet(t, workoutID, "only_set_key", 1, 800)
	if err := fixture.repository.DeleteSet(context.Background(), fixture.userID, workoutID, typo.ID); err != nil {
		t.Fatalf("DeleteSet() error = %v", err)
	}
	var weight, maxWeight sql.NullFloat64
	if err := fixture.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, fixture.userID, fixture.exerciseID).Scan(&weight, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if weight.Valid || maxWeight.Valid {
		t.Fatalf("stats after deleting the only set = (%v, %v), want NULL", weight, maxWeight)
	}
}

func TestSetCorrectionRequiresOwnActiveWorkout(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()
	workoutID := fixture.createWorkout(t)
	set := fixture.recordSet(t, workoutID, "owner_set_key", 1, 60)
	reps := 5

	var otherUserID int
	if err := fixture.db.QueryRow(`INSERT INTO users (username,password_hash) VALUES ('set_edit_other','test') RETURNING id`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.UpdateSet(ctx, otherUserID, workoutID, set.ID, workout.SetPatch{Reps: &reps}); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user UpdateSet() error = %v, want ErrNotFound", err)
	}
	if err := fixture.repository.DeleteSet(ctx, otherUserID, workoutID, set.ID); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("other user DeleteSet() error = %v, want ErrNotFound", err)
	}
	otherWorkoutID := fixture.createWorkout(t)
	if _, err := fixture.repository.UpdateSet(ctx, fixture.userID, otherWorkoutID, set.ID, workout.SetPatch{Reps: &reps}); !errors.Is(err, workout.ErrNotFound) {
		t.Fatalf("UpdateSet() through another workout error = %v, want ErrNotFound", err)
	}

	if _, err := fixture.repository.Finish(ctx, fixture.userID, workoutID); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if _, err := fixture.repository.UpdateSet(ctx, fixture.userID, workoutID, set.ID, workout.SetPatch{Reps: &reps}); !errors.Is(err, workout.ErrConflict) {
		t.Fatalf("completed UpdateSet() error = %v, want ErrConflict", err)
	}
	if err := fixture.repository.DeleteSet(ctx, fixture.userID, workoutID, set.ID); !errors.Is(err, workout.ErrConflict) {
		t.Fatalf("completed DeleteSet() error = %v, want ErrConflict", err)
	}
	if sets := fixture.storedSets(t, workoutID); len(sets) != 1 || sets[0].Reps != set.Reps {
		t.Fatalf("stored sets after rejected corrections = %+v, want unchanged", sets)
	}
}

// recordCompletedHistory stores a completed workout two days earlier.
func (f integrationFixture) recordCompletedHistory(t *testing.T, weight float64) {
	t.Helper()
	var workoutID int
	if err := f.db.QueryRow(`INSERT INTO workouts (user_id,notes,started_at) VALUES ($1,'History',CURRENT_TIMESTAMP - INTERVAL '2 days') RETURNING id`, f.userID).Scan(&workoutID); err != nil {
		t.Fatal(err)
	}
	f.recordSet(t, workoutID, "history_set_key", 1, weight)
	if _, err := f.repository.Finish(context.Background(), f.userID, workoutID); err != nil {
		t.Fatalf("Finish() history error = %v", err)
	}
}

func (f integrationFixture) recordSet(t *testing.T, workoutID int, key string, order int, weight float64) workout.Set {
	t.Helper()
	set, _, err := f.repository.RecordSet(context.Background(), f.userID, workoutID, key, workout.SetInput{ExerciseID: f.exerciseID, SetOrder: order, Weight: weight, Reps: 8})
	if err != nil {
		t.Fatalf("RecordSet(%v) error = %v", weight, err)
	}
	return set
}

func (f integrationFixture) assertStats(t *testing.T, wantWeight, wantMax float64) {
	t.Helper()
	var weight, maxWeight float64
	if err := f.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, f.userID, f.exerciseID).Scan(&weight, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if weight != wantWeight || maxWeight != wantMax {
		t.Fatalf("stats = (weight %v, max %v), want (%v, %v)", weight, maxWeight, wantWeight, wantMax)
	}
}

func (f integrationFixture) assertNextTarget(t *testing.T, wantTarget, wantMax float64) {
	t.Helper()
	stats, _, err := planningpostgres.New(f.db).ResolveExercise(context.Background(), f.userID, f.exerciseID, "")
	if err != nil {
		t.Fatalf("ResolveExercise() error = %v", err)
	}
	if stats.TargetWeight != wantTarget || stats.MaxWeight != wantMax {
		t.Fatalf("next plan stats = (target %v, max %v), want (%v, %v)", stats.TargetWeight, stats.MaxWeight, wantTarget, wantMax)
	}
}
