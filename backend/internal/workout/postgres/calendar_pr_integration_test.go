//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestSaveCalendarWorkoutPreservesHistoricalPRs(t *testing.T) {
	fixture := newIntegrationFixture(t)
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	first := fixture.saveCalendarWeights(t, date, 40)
	second := fixture.saveCalendarWeights(t, date.AddDate(0, 0, 1), 50)
	third := fixture.saveCalendarWeights(t, date.AddDate(0, 0, 2), 45)

	fixture.assertCalendarPRs(t, first.WorkoutID, []bool{true})
	fixture.assertCalendarPRs(t, second.WorkoutID, []bool{true})
	fixture.assertCalendarPRs(t, third.WorkoutID, []bool{false})
	for range 2 {
		resaved := fixture.saveCalendarWeights(t, date, 40)
		if resaved.WorkoutID != first.WorkoutID {
			t.Fatalf("resaved workout ID = %d, want %d", resaved.WorkoutID, first.WorkoutID)
		}
		fixture.assertCalendarPRs(t, first.WorkoutID, []bool{true})
		fixture.assertCalendarPRs(t, second.WorkoutID, []bool{true})
		fixture.assertCalendarPRs(t, third.WorkoutID, []bool{false})
		fixture.assertExerciseWeights(t, fixture.exerciseID, 45, 50)
	}
}

func TestSaveCalendarWorkoutRecalculatesLaterPRs(t *testing.T) {
	for _, test := range []struct {
		name              string
		initialWeight     float64
		replacementWeight float64
		wantLaterPR       bool
		wantMaxWeight     float64
	}{
		{"higher past weight invalidates later PR", 40, 60, false, 60},
		{"lower past weight restores later PR", 60, 40, true, 50},
		{"equal past weight invalidates later PR", 40, 50, false, 50},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newIntegrationFixture(t)
			date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			fixture.saveCalendarWeights(t, date, test.initialWeight)
			later := fixture.saveCalendarWeights(t, date.AddDate(0, 0, 1), 50)

			replacement := fixture.saveCalendarWeights(t, date, test.replacementWeight)
			fixture.assertCalendarPRs(t, replacement.WorkoutID, []bool{true})
			fixture.assertCalendarPRs(t, later.WorkoutID, []bool{test.wantLaterPR})
			fixture.assertExerciseWeights(t, fixture.exerciseID, 50, test.wantMaxWeight)
		})
	}
}

func TestSaveCalendarWorkoutRecalculatesSetsInRecordedOrder(t *testing.T) {
	fixture := newIntegrationFixture(t)
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	weights := []float64{0, 40, 40, 35, 50, 50, 45}
	fixture.saveCalendarWeights(t, date, weights...)
	later := fixture.saveCalendarWeights(t, date.AddDate(0, 0, 1), 55)

	resaved := fixture.saveCalendarWeights(t, date, weights...)
	fixture.assertCalendarPRs(t, resaved.WorkoutID, []bool{false, true, false, false, true, false, false})
	fixture.assertCalendarPRs(t, later.WorkoutID, []bool{true})
	fixture.assertExerciseWeights(t, fixture.exerciseID, 55, 55)
}

func TestSaveCalendarWorkoutUsesStableOrderForEqualTimestamps(t *testing.T) {
	fixture := newIntegrationFixture(t)
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	laterDate := date.AddDate(0, 0, 1)
	later := fixture.saveCalendarWeights(t, laterDate, 40, 40, 50, 50)
	if _, err := fixture.db.Exec(`UPDATE workout_sets SET created_at=$1 WHERE workout_id=$2`, laterDate.Add(12*time.Hour), later.WorkoutID); err != nil {
		t.Fatal(err)
	}

	earlier := fixture.saveCalendarWeights(t, date, 30)
	fixture.assertCalendarPRs(t, earlier.WorkoutID, []bool{true})
	fixture.assertCalendarPRs(t, later.WorkoutID, []bool{true, false, true, false})
	fixture.assertExerciseWeights(t, fixture.exerciseID, 50, 50)
}

func TestSaveCalendarWorkoutRecalculatesRemovedExercise(t *testing.T) {
	for _, hasLaterSet := range []bool{true, false} {
		name := "last set removed"
		if hasLaterSet {
			name = "later PR restored"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newIntegrationFixture(t)
			date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			original := fixture.saveCalendarWeights(t, date, 60)
			var later workout.CalendarWorkout
			if hasLaterSet {
				later = fixture.saveCalendarWeights(t, date.AddDate(0, 0, 1), 50)
				fixture.assertCalendarPRs(t, later.WorkoutID, []bool{false})
			}
			if _, err := fixture.db.Exec(`UPDATE user_exercise_stats SET target_sets=5 WHERE user_id=$1 AND exercise_id=$2`, fixture.userID, fixture.exerciseID); err != nil {
				t.Fatal(err)
			}
			replacementID := fixture.exerciseID + "_replacement"
			if _, err := fixture.db.Exec(`INSERT INTO exercises (id,name) VALUES ($1,'Replacement Exercise')`, replacementID); err != nil {
				t.Fatal(err)
			}
			_, err := fixture.repository.SaveCalendarWorkout(context.Background(), fixture.userID, date, workout.CalendarWorkoutInput{
				Title: "種目を変更",
				Sets:  []workout.CalendarSet{{ExerciseID: replacementID, Weight: 20, Reps: 8, SetOrder: 1}},
			})
			if err != nil {
				t.Fatalf("SaveCalendarWorkout() error = %v", err)
			}
			fixture.assertCalendarPRs(t, original.WorkoutID, []bool{true})
			fixture.assertExerciseWeights(t, replacementID, 20, 20)
			var latest, maxWeight sql.NullFloat64
			var targetSets int
			if err := fixture.db.QueryRow(`SELECT weight,max_weight,target_sets FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, fixture.userID, fixture.exerciseID).Scan(&latest, &maxWeight, &targetSets); err != nil {
				t.Fatal(err)
			}
			if targetSets != 5 {
				t.Errorf("target_sets = %d, want 5", targetSets)
			}
			if hasLaterSet {
				fixture.assertCalendarPRs(t, later.WorkoutID, []bool{true})
				fixture.assertExerciseWeights(t, fixture.exerciseID, 50, 50)
			} else if latest.Valid || maxWeight.Valid {
				t.Errorf("removed exercise stats = (%v, %v), want NULL weights", latest, maxWeight)
			}
		})
	}
}

func TestSaveCalendarWorkoutLeavesOtherUsersAndExercisesUnchanged(t *testing.T) {
	fixture := newIntegrationFixture(t)
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fixture.saveCalendarWeights(t, date, 40)

	var otherUserID int
	if err := fixture.db.QueryRow(`INSERT INTO users (username,password_hash) VALUES ('other_calendar_user','test') RETURNING id`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	otherExerciseID := fixture.exerciseID + "_other"
	if _, err := fixture.db.Exec(`INSERT INTO exercises (id,name) VALUES ($1,'Other Exercise')`, otherExerciseID); err != nil {
		t.Fatal(err)
	}
	otherWorkout, err := fixture.repository.SaveCalendarWorkout(context.Background(), otherUserID, date, workout.CalendarWorkoutInput{
		Title: "別ユーザーの記録",
		Sets:  []workout.CalendarSet{{ExerciseID: fixture.exerciseID, Weight: 100, Reps: 8, SetOrder: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := fixture.repository.SaveCalendarWorkout(context.Background(), fixture.userID, date.AddDate(0, 0, 1), workout.CalendarWorkoutInput{
		Title: "別種目の記録",
		Sets:  []workout.CalendarSet{{ExerciseID: otherExerciseID, Weight: 80, Reps: 8, SetOrder: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately retain non-PR flags outside the recalculation's scope.
	if _, err := fixture.db.Exec(`UPDATE workout_sets SET is_pr=false WHERE workout_id IN ($1,$2)`, otherWorkout.WorkoutID, unrelated.WorkoutID); err != nil {
		t.Fatal(err)
	}

	resaved := fixture.saveCalendarWeights(t, date, 40)
	fixture.assertCalendarPRs(t, resaved.WorkoutID, []bool{true})
	for _, workoutID := range []int{otherWorkout.WorkoutID, unrelated.WorkoutID} {
		sets := fixture.storedSets(t, workoutID)
		if len(sets) != 1 || sets[0].IsPR {
			t.Errorf("unrelated workout %d sets = %+v, want one non-PR set", workoutID, sets)
		}
	}
	fixture.assertExerciseWeights(t, fixture.exerciseID, 40, 40)
	fixture.assertExerciseWeights(t, otherExerciseID, 80, 80)
	var latest, maxWeight float64
	if err := fixture.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, otherUserID, fixture.exerciseID).Scan(&latest, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if latest != 100 || maxWeight != 100 {
		t.Errorf("other user stats = (%v, %v), want (100, 100)", latest, maxWeight)
	}
}

func (f integrationFixture) saveCalendarWeights(t *testing.T, date time.Time, weights ...float64) workout.CalendarWorkout {
	t.Helper()
	sets := make([]workout.CalendarSet, len(weights))
	for index, weight := range weights {
		sets[index] = workout.CalendarSet{ExerciseID: f.exerciseID, Weight: weight, Reps: 8, SetOrder: index + 1}
	}
	saved, err := f.repository.SaveCalendarWorkout(context.Background(), f.userID, date, workout.CalendarWorkoutInput{Title: "PR確認", Sets: sets})
	if err != nil {
		t.Fatalf("SaveCalendarWorkout() error = %v", err)
	}
	return saved
}

func (f integrationFixture) assertCalendarPRs(t *testing.T, workoutID int, want []bool) {
	t.Helper()
	sets := f.storedSets(t, workoutID)
	if len(sets) != len(want) {
		t.Fatalf("workout %d sets = %d, want %d", workoutID, len(sets), len(want))
	}
	wantCount := 0
	for index, isPR := range want {
		if sets[index].IsPR != isPR {
			t.Errorf("workout %d set %d is_pr = %v, want %v", workoutID, index, sets[index].IsPR, isPR)
		}
		if isPR {
			wantCount++
		}
	}
	detail, err := f.repository.Detail(context.Background(), f.userID, workoutID)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if detail.Summary.PRCount != wantCount {
		t.Errorf("workout %d summary PR count = %d, want %d", workoutID, detail.Summary.PRCount, wantCount)
	}
}

func (f integrationFixture) assertExerciseWeights(t *testing.T, exerciseID string, wantLatest, wantMax float64) {
	t.Helper()
	var latest, maxWeight float64
	if err := f.db.QueryRow(`SELECT weight,max_weight FROM user_exercise_stats WHERE user_id=$1 AND exercise_id=$2`, f.userID, exerciseID).Scan(&latest, &maxWeight); err != nil {
		t.Fatal(err)
	}
	if latest != wantLatest || maxWeight != wantMax {
		t.Errorf("exercise %s stats = (%v, %v), want (%v, %v)", exerciseID, latest, maxWeight, wantLatest, wantMax)
	}
}
