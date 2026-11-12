//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/database"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestSameDayRestartAppendsWorkoutHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	today := time.Now().UTC()
	date := today.Format("2006-01-02")
	startPath := "/api/workout-plans/" + date + "/start"
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	workoutPath := fmt.Sprintf("/api/workouts/%d", started.WorkoutID)
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "bench", SetOrder: 3, Weight: 10, Reps: 12}, "restart_set_001", http.StatusCreated)
	firstFinish := decodeAPI[workout.FinishResult](t, f.request(t, "POST", workoutPath+"/finish", nil, "", http.StatusOK))
	if firstFinish.Summary.TotalSets != 1 || firstFinish.Summary.TotalReps != 12 || len(firstFinish.Summary.Exercises) != 1 || firstFinish.Summary.Exercises[0].LastSetOrder != 3 {
		t.Fatalf("first summary = %+v", firstFinish)
	}
	// A summary generated before the restart must not describe the appended workout.
	if _, err := f.db.Exec(`UPDATE workouts SET summary_comment='old summary' WHERE id=$1`, started.WorkoutID); err != nil {
		t.Fatal(err)
	}
	resumed := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "squat"}, "", http.StatusOK))
	if !resumed.Resumed || resumed.WorkoutID != started.WorkoutID || resumed.ID != started.ID || !reflect.DeepEqual(resumed.Plan, started.Plan) {
		t.Fatalf("same-day restart replaced the original record: %+v", resumed)
	}
	detail := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if detail.Status != "active" || detail.EndedAt != "" || detail.Summary.TotalSets != 1 || detail.Summary.AIComment != "" {
		t.Fatalf("resumed detail = %+v", detail)
	}
	if len(detail.Summary.Exercises) != 1 || detail.Summary.Exercises[0].LastSetOrder != 3 {
		t.Fatalf("resumed set order must retain gaps: %+v", detail.Summary.Exercises)
	}
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "bench", SetOrder: 4, Weight: 20, Reps: 8}, "restart_set_002", http.StatusCreated)
	finished := decodeAPI[workout.FinishResult](t, f.request(t, "POST", workoutPath+"/finish", nil, "", http.StatusOK))
	if finished.Status != "completed" || finished.Summary.TotalSets != 2 || finished.Summary.TotalReps != 20 || finished.Summary.TotalVolume != 280 {
		t.Fatalf("second summary = %+v", finished)
	}
	if len(finished.Summary.Exercises) != 1 || finished.Summary.Exercises[0].Sets != 2 || finished.Summary.Exercises[0].LastSetOrder != 4 {
		t.Fatalf("second summary must distinguish set count from last order: %+v", finished.Summary.Exercises)
	}
	history := decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", "/api/workouts/by-date/"+date, nil, "", http.StatusOK))
	if history.WorkoutID != started.WorkoutID || len(history.Sets) != 2 || history.Sets[0].SetOrder != 3 || history.Sets[1].SetOrder != 4 {
		t.Fatalf("same-day history = %+v", history)
	}
	var workouts, plans int
	if err := f.db.QueryRow(`SELECT (SELECT COUNT(*) FROM workouts), (SELECT COUNT(*) FROM workout_plans)`).Scan(&workouts, &plans); err != nil {
		t.Fatal(err)
	}
	if workouts != 1 || plans != 1 {
		t.Fatalf("workouts=%d plans=%d, want one combined record", workouts, plans)
	}

	// The app uses the system clock. Move only this fixture's records back one
	// day to model the next day's HTTP reads without waiting or changing it.
	if _, err := f.db.Exec(`
		UPDATE workouts SET started_at=started_at-INTERVAL '1 day', ended_at=ended_at-INTERVAL '1 day';
		UPDATE workout_sets SET created_at=created_at-INTERVAL '1 day';
		UPDATE workout_plans SET plan_date=plan_date-1;
	`); err != nil {
		t.Fatal(err)
	}
	yesterday := today.AddDate(0, 0, -1)
	dashboard := decodeAPI[reporting.Dashboard](t, f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK))
	if len(dashboard.RecentWorkouts) != 1 || dashboard.RecentWorkouts[0].ID != started.WorkoutID || dashboard.ChartData[5] != 2 {
		t.Fatalf("next-day dashboard = %+v", dashboard)
	}
	calendar := decodeAPI[reporting.Calendar](t, f.request(t, "GET", fmt.Sprintf("/api/calendar?year=%d&month=%d", yesterday.Year(), yesterday.Month()), nil, "", http.StatusOK))
	if len(calendar.WorkedOutDays) != 1 || calendar.WorkedOutDays[0].WorkoutID != started.WorkoutID || calendar.WorkedOutDays[0].Date != yesterday.Day() || len(calendar.PlannedDays) != 0 {
		t.Fatalf("next-day calendar = %+v", calendar)
	}
	history = decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", "/api/workouts/by-date/"+yesterday.Format("2006-01-02"), nil, "", http.StatusOK))
	if history.WorkoutID != started.WorkoutID || len(history.Sets) != 2 || f.aiCalls.Load() != 0 {
		t.Fatalf("next-day history = %+v; AI calls=%d", history, f.aiCalls.Load())
	}
}

func TestConcurrentSameDayResumesHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	startPath := "/api/workout-plans/" + date + "/start"
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	f.request(t, "POST", fmt.Sprintf("/api/workouts/%d/finish", started.WorkoutID), nil, "", http.StatusOK)
	const requests = 8
	start := make(chan struct{})
	results := make(chan apiResult, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- f.do("POST", startPath, map[string]any{"use_ai": false, "exercise_id": "squat"}, "")
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for result := range results {
		if result.err != nil || result.status != http.StatusOK {
			t.Fatalf("concurrent resume status=%d error=%v body=%s", result.status, result.err, result.body)
		}
		resumed := decodeAPI[planning.PlanSession](t, result.body)
		if resumed.WorkoutID != started.WorkoutID || !reflect.DeepEqual(resumed.Plan, started.Plan) {
			t.Fatalf("concurrent resume created another record: %+v", resumed)
		}
	}
	var workouts, plans int
	if err := f.db.QueryRow(`SELECT (SELECT COUNT(*) FROM workouts), (SELECT COUNT(*) FROM workout_plans)`).Scan(&workouts, &plans); err != nil {
		t.Fatal(err)
	}
	if workouts != 1 || plans != 1 {
		t.Fatalf("workouts=%d plans=%d after concurrent resume", workouts, plans)
	}
}

func TestLegacySameDayCollisionMigrationHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	ctx := context.Background()
	// Recreate the old schema and migration state only in the fixture's
	// isolated schema. This also verifies that the new partial index is created.
	if _, err := f.db.ExecContext(ctx, `
		DROP INDEX workout_plans_user_date_active_unique;
		ALTER TABLE workout_plans ADD CONSTRAINT workout_plans_user_id_plan_date_status_key UNIQUE (user_id,plan_date,status);
		DELETE FROM schema_migrations WHERE version='00003_workout_plans_active_unique.sql';
	`); err != nil {
		t.Fatal(err)
	}
	var userID int
	if err := f.db.QueryRow(`SELECT id FROM users WHERE username='integration-user'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC()
	tests := []struct {
		name          string
		date          time.Time
		alreadyEnded  bool
		wantCompleted bool
	}{
		{name: "failed finish left workout open", date: today.AddDate(0, 0, -3), wantCompleted: true},
		{name: "stale cleanup left only plan open", date: today.AddDate(0, 0, -2), alreadyEnded: true, wantCompleted: true},
		{name: "yesterday waits for timezone-aware cleanup", date: today.AddDate(0, 0, -1), wantCompleted: false},
		{name: "current recording is preserved", date: today, wantCompleted: false},
	}
	workoutIDs := make([]int, len(tests))
	for i, test := range tests {
		startedAt := time.Date(test.date.Year(), test.date.Month(), test.date.Day(), 10, 0, 0, 0, time.UTC)
		var completedID int
		if err := f.db.QueryRow(`INSERT INTO workouts (user_id,started_at,ended_at,notes) VALUES ($1,$2,$2,'first') RETURNING id`, userID, startedAt).Scan(&completedID); err != nil {
			t.Fatal(err)
		}
		var endedAt any
		if test.alreadyEnded {
			endedAt = startedAt.Add(time.Hour)
		}
		if err := f.db.QueryRow(`INSERT INTO workouts (user_id,started_at,ended_at,notes) VALUES ($1,$2,$3,'second') RETURNING id`, userID, startedAt.Add(time.Hour), endedAt).Scan(&workoutIDs[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Exec(`
			INSERT INTO workout_plans (workout_id,user_id,plan_date,title,status,plan) VALUES
			($1,$3,$4::date,'first','completed','{}'),($2,$3,$4::date,'second','active','{}');
		`, completedID, workoutIDs[i], userID, test.date.Format("2006-01-02")); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Exec(`INSERT INTO workout_sets (workout_id,exercise_id,weight,reps,set_order,created_at) VALUES ($1,'bench',20,8,1,$2)`, workoutIDs[i], startedAt.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Migrate(ctx, f.db); err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var status string
			var endedAt sql.NullTime
			var sets int
			if err := f.db.QueryRow(`SELECT p.status,w.ended_at,(SELECT COUNT(*) FROM workout_sets WHERE workout_id=w.id) FROM workouts w JOIN workout_plans p ON p.workout_id=w.id WHERE w.id=$1`, workoutIDs[i]).Scan(&status, &endedAt, &sets); err != nil {
				t.Fatal(err)
			}
			if (status == "completed") != test.wantCompleted || endedAt.Valid != test.wantCompleted || sets != 1 {
				t.Fatalf("status=%s ended=%v sets=%d; want completed=%v, original set retained", status, endedAt.Valid, sets, test.wantCompleted)
			}
			if test.wantCompleted && !test.alreadyEnded {
				wantEnded := time.Date(test.date.Year(), test.date.Month(), test.date.Day(), 12, 0, 0, 0, time.UTC)
				if !endedAt.Time.Equal(wantEnded) {
					t.Fatalf("repaired ended_at=%v, want last set time %v", endedAt.Time, wantEnded)
				}
			}
		})
	}
	_, err := f.db.Exec(`INSERT INTO workout_plans (workout_id,user_id,plan_date,title,status,plan) SELECT workout_id,user_id,plan_date,title,status,plan FROM workout_plans WHERE workout_id=$1`, workoutIDs[3])
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" || pqErr.Constraint != "workout_plans_user_date_active_unique" {
		t.Fatalf("duplicate active plan error=%v, want the partial unique index violation", err)
	}
	// A previously failed second finish now succeeds under the partial index.
	f.request(t, "POST", fmt.Sprintf("/api/workouts/%d/finish", workoutIDs[3]), nil, "", http.StatusOK)
	f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK)
	f.request(t, "GET", fmt.Sprintf("/api/calendar?year=%d&month=%d", today.Year(), today.Month()), nil, "", http.StatusOK)
	var cleaned bool
	if err := f.db.QueryRow(`SELECT p.status='completed' AND w.ended_at IS NOT NULL FROM workouts w JOIN workout_plans p ON p.workout_id=w.id WHERE w.id=$1`, workoutIDs[2]).Scan(&cleaned); err != nil {
		t.Fatal(err)
	}
	if !cleaned {
		t.Fatal("yesterday's active/completed collision was not closed by the reporting read")
	}
}
