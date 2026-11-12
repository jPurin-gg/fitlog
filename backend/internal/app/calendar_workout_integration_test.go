//go:build integration

package app_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestConcurrentCalendarWorkoutPutsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := "2026-08-10"
	path := "/api/workouts/by-date/" + date
	f.request(t, "GET", path, nil, "", http.StatusNotFound)
	inputs := []workout.CalendarWorkoutInput{
		{Title: "ベンチの日", Sets: []workout.CalendarSet{
			{ExerciseID: "bench", Weight: 40, Reps: 8, SetOrder: 1, Feeling: "普通"},
			{ExerciseID: "bench", Weight: 35, Reps: 10, SetOrder: 2, Feeling: "余裕"},
		}},
		{Title: "スクワットの日", Sets: []workout.CalendarSet{
			{ExerciseID: "squat", Weight: 60, Reps: 5, SetOrder: 1, Feeling: "重い"},
		}},
	}
	// Both old requests must pass the missing-workout lookup before INSERT can
	// proceed. With serialization, only the first reaches INSERT and the other
	// waits on its date lock instead.
	release := holdWorkoutWrites(t, f)
	defer release()
	results := make(chan apiResult, len(inputs))
	for _, input := range inputs {
		go func() { results <- f.do("PUT", path, input, "") }()
	}
	waitForWorkoutLockWaiters(t, f, len(inputs))
	release()

	workoutID := 0
	for range inputs {
		result := <-results
		if result.err != nil || result.status != http.StatusOK {
			t.Fatalf("concurrent PUT status=%d error=%v body=%s", result.status, result.err, result.body)
		}
		saved := decodeAPI[workout.CalendarWorkout](t, result.body)
		if workoutID == 0 {
			workoutID = saved.WorkoutID
		}
		if saved.WorkoutID <= 0 || saved.WorkoutID != workoutID {
			t.Fatalf("concurrent PUT workout_id=%d, want %d", saved.WorkoutID, workoutID)
		}
	}
	assertSingleCalendarWorkout(t, f, path, workoutID, inputs)
}

func TestConcurrentCalendarWorkoutPutAndStartHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	path := "/api/workouts/by-date/" + date
	input := workout.CalendarWorkoutInput{Title: "保存した記録", Sets: []workout.CalendarSet{
		{ExerciseID: "bench", Weight: 40, Reps: 8, SetOrder: 1},
	}}
	release := holdWorkoutWrites(t, f)
	defer release()
	putResult := make(chan apiResult, 1)
	startResult := make(chan apiResult, 1)
	go func() { putResult <- f.do("PUT", path, input, "") }()
	go func() {
		startResult <- f.do("POST", "/api/workout-plans/"+date+"/start", map[string]any{"use_ai": false, "exercise_id": "bench"}, "")
	}()
	waitForWorkoutLockWaiters(t, f, 2)
	release()

	put := <-putResult
	if put.err != nil || put.status != http.StatusOK {
		t.Fatalf("concurrent PUT status=%d error=%v body=%s", put.status, put.err, put.body)
	}
	saved := decodeAPI[workout.CalendarWorkout](t, put.body)
	start := <-startResult
	if start.err != nil || (start.status != http.StatusOK && start.status != http.StatusConflict) {
		t.Fatalf("concurrent start status=%d error=%v body=%s", start.status, start.err, start.body)
	}
	if start.status == http.StatusOK {
		session := decodeAPI[planning.PlanSession](t, start.body)
		if session.WorkoutID != saved.WorkoutID {
			t.Fatalf("start workout_id=%d, want PUT workout_id=%d", session.WorkoutID, saved.WorkoutID)
		}
	}
	assertSingleCalendarWorkout(t, f, path, saved.WorkoutID, []workout.CalendarWorkoutInput{input})
}

func TestCalendarWorkoutPutAndStartOrderHTTP(t *testing.T) {
	for _, putFirst := range []bool{true, false} {
		name := "start then PUT"
		if putFirst {
			name = "PUT then start"
		}
		t.Run(name, func(t *testing.T) {
			f := newManualAPIFixture(t, nil)
			date := time.Now().UTC().Format("2006-01-02")
			path := "/api/workouts/by-date/" + date
			startPath := "/api/workout-plans/" + date + "/start"
			startInput := map[string]any{"use_ai": false, "exercise_id": "bench"}
			input := workout.CalendarWorkoutInput{Title: "カレンダーから保存", Sets: []workout.CalendarSet{
				{ExerciseID: "squat", Weight: 50, Reps: 8, SetOrder: 1},
			}}
			startedID := 0
			if !putFirst {
				session := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, startInput, "", http.StatusOK))
				startedID = session.WorkoutID
			}
			saved := decodeAPI[workout.CalendarWorkout](t, f.request(t, "PUT", path, input, "", http.StatusOK))
			if startedID != 0 && saved.WorkoutID != startedID {
				t.Fatalf("PUT workout_id=%d, want started workout_id=%d", saved.WorkoutID, startedID)
			}
			// A calendar PUT saves a completed record. Starting it must not create
			// a second workout or overwrite the saved title, in either order.
			f.request(t, "POST", startPath, startInput, "", http.StatusConflict)
			assertSingleCalendarWorkout(t, f, path, saved.WorkoutID, []workout.CalendarWorkoutInput{input})
		})
	}
}

func holdWorkoutWrites(t *testing.T, f *manualAPIFixture) func() {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.Exec(`LOCK TABLE workouts IN SHARE MODE`); err != nil {
		t.Fatal(err)
	}
	return func() { _ = tx.Rollback() }
}

func waitForWorkoutLockWaiters(t *testing.T, f *manualAPIFixture, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var waiting int
		if err := f.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE application_name=current_setting('application_name') AND wait_event_type='Lock'
		`).Scan(&waiting); err != nil {
			t.Fatalf("wait for %d concurrent requests to block: %v", want, err)
		}
		if waiting == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("requests waiting on locks=%d, want %d", waiting, want)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func assertSingleCalendarWorkout(t *testing.T, f *manualAPIFixture, path string, workoutID int, inputs []workout.CalendarWorkoutInput) {
	t.Helper()
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("workouts count=%d, want 1", count)
	}
	stored := decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", path, nil, "", http.StatusOK))
	if workoutID <= 0 || stored.WorkoutID != workoutID {
		t.Fatalf("GET workout_id=%d, want %d", stored.WorkoutID, workoutID)
	}
	sets := append([]workout.CalendarSet(nil), stored.Sets...)
	for i := range sets {
		sets[i].ID = 0
		sets[i].ExerciseName = ""
	}
	matched := false
	for _, input := range inputs {
		if stored.Title == input.Title && reflect.DeepEqual(sets, input.Sets) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("saved title and sets do not match a complete input: %+v", stored)
	}
	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls=%d, want zero", calls)
	}
}
