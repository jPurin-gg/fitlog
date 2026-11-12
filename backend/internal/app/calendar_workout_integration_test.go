//go:build integration

package app_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestCalendarWorkoutRejectsActiveWorkoutHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	startPath := "/api/workout-plans/" + date + "/start"
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	workoutPath := fmt.Sprintf("/api/workouts/%d", started.WorkoutID)
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 8}, "calendar_active_001", http.StatusCreated)
	detail := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if started.Status != "active" || detail.EndedAt != "" || detail.Status != "active" {
		t.Fatalf("expected an active workout before PUT: session=%+v detail=%+v", started, detail)
	}
	before := calendarWorkoutDatabaseSnapshot(t, f)

	input := workout.CalendarWorkoutInput{Title: "カレンダーからの上書き", Sets: []workout.CalendarSet{{ExerciseID: "squat", Weight: 90, Reps: 5, SetOrder: 1}}}
	problem := decodeAPI[httpx.Problem](t, f.request(t, "PUT", "/api/workouts/by-date/"+date, input, "", http.StatusConflict))
	if problem.Status != http.StatusConflict || problem.Code != apperr.CodeConflict {
		t.Fatalf("calendar PUT problem = %+v, want 409/CONFLICT", problem)
	}
	if after := calendarWorkoutDatabaseSnapshot(t, f); after != before {
		t.Fatalf("rejected calendar PUT changed workout, plan, sets or stats: before=%s after=%s", before, after)
	}

	resumed := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]bool{"use_ai": false}, "", http.StatusOK))
	if resumed.WorkoutID != started.WorkoutID || !reflect.DeepEqual(resumed.Plan, started.Plan) {
		t.Fatalf("resume replaced active workout: got %+v, started %+v", resumed, started)
	}
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "bench", SetOrder: 2, Weight: 40, Reps: 7}, "calendar_active_002", http.StatusCreated)
	detail = decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if detail.EndedAt != "" || detail.Summary.TotalSets != 2 || detail.Summary.TotalVolume != 600 {
		t.Fatalf("workout after resume and another set = %+v", detail)
	}
	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls = %d, want zero", calls)
	}
}

func TestCalendarWorkoutSavesNonRecordingDaysHTTP(t *testing.T) {
	for _, withPlan := range []bool{false, true} {
		t.Run(fmt.Sprintf("unstarted_plan=%v", withPlan), func(t *testing.T) {
			f := newManualAPIFixture(t, nil)
			date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
			var plan planning.PlanSession
			if withPlan {
				plan = decodeAPI[planning.PlanSession](t, f.request(t, "PUT", "/api/workout-plans/"+date, planning.WorkoutPlan{
					WorkoutTitle: "未開始の予定", Exercises: []planning.PlanExercise{{ExerciseID: "bench", PlannedSets: 3, TargetWeight: 40, TargetReps: 8}},
				}, "", http.StatusOK))
				if plan.WorkoutID != 0 || plan.Status != "active" {
					t.Fatalf("unstarted plan = %+v", plan)
				}
			}
			path := "/api/workouts/by-date/" + date
			input := workout.CalendarWorkoutInput{Title: "過去の記録", Sets: []workout.CalendarSet{{ExerciseID: "bench", Weight: 40, Reps: 8, SetOrder: 1}}}
			saved := decodeAPI[workout.CalendarWorkout](t, f.request(t, "PUT", path, input, "", http.StatusOK))
			if saved.WorkoutID <= 0 || saved.Date != date || saved.Title != input.Title || len(saved.Sets) != 1 || saved.Sets[0].Weight != 40 {
				t.Fatalf("saved calendar workout = %+v", saved)
			}
			history := decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", path, nil, "", http.StatusOK))
			if !reflect.DeepEqual(history, saved) {
				t.Fatalf("calendar history = %+v, want %+v", history, saved)
			}
			input.Title = "過去の記録を修正"
			input.Sets[0].Weight = 45
			edited := decodeAPI[workout.CalendarWorkout](t, f.request(t, "PUT", path, input, "", http.StatusOK))
			if edited.WorkoutID != saved.WorkoutID || edited.Title != input.Title || len(edited.Sets) != 1 || edited.Sets[0].Weight != 45 {
				t.Fatalf("edited past workout = %+v", edited)
			}
			if withPlan {
				stored := decodeAPI[planning.PlanSession](t, f.request(t, "GET", "/api/workout-plans/"+date, nil, "", http.StatusOK))
				if !reflect.DeepEqual(stored, plan) {
					t.Fatalf("calendar save changed unstarted plan: got %+v, want %+v", stored, plan)
				}
			}
		})
	}
}

func TestCalendarWorkoutRejectsActiveWorkoutWithLaterCompletedWorkoutHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", "/api/workout-plans/"+date+"/start", map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	// A completed workout with the same timestamp and a larger ID would be the
	// usual calendar selection. It must not hide an active workout on that day.
	if _, err := f.db.Exec(`
		INSERT INTO workouts (user_id,started_at,ended_at,notes)
		SELECT user_id,started_at,started_at,'Later completed workout' FROM workouts WHERE id=$1
	`, started.WorkoutID); err != nil {
		t.Fatal(err)
	}
	before := calendarWorkoutDatabaseSnapshot(t, f)
	input := workout.CalendarWorkoutInput{Title: "日付指定の記録", Sets: []workout.CalendarSet{{ExerciseID: "bench", Weight: 40, Reps: 8, SetOrder: 1}}}
	f.request(t, "PUT", "/api/workouts/by-date/"+date, input, "", http.StatusConflict)
	if after := calendarWorkoutDatabaseSnapshot(t, f); after != before {
		t.Fatalf("rejected calendar PUT changed stored state: before=%s after=%s", before, after)
	}
}

func TestCalendarWorkoutEditsCompletedWorkoutHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", "/api/workout-plans/"+date+"/start", map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	workoutPath := fmt.Sprintf("/api/workouts/%d", started.WorkoutID)
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 8}, "calendar_finished_001", http.StatusCreated)
	f.request(t, "POST", workoutPath+"/finish", nil, "", http.StatusOK)

	input := workout.CalendarWorkoutInput{Title: "終了済みの記録を修正", Sets: []workout.CalendarSet{
		{ExerciseID: "squat", Weight: 60, Reps: 5, SetOrder: 1},
		{ExerciseID: "squat", Weight: 60, Reps: 4, SetOrder: 2},
	}}
	saved := decodeAPI[workout.CalendarWorkout](t, f.request(t, "PUT", "/api/workouts/by-date/"+date, input, "", http.StatusOK))
	if saved.WorkoutID != started.WorkoutID || saved.Title != input.Title || len(saved.Sets) != 2 || saved.Sets[0].ExerciseID != "squat" || saved.Sets[1].Reps != 4 {
		t.Fatalf("edited completed workout = %+v", saved)
	}
	detail := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if detail.Status != "completed" || detail.EndedAt == "" || detail.Summary.TotalSets != 2 || detail.Summary.TotalReps != 9 || detail.Summary.TotalVolume != 540 {
		t.Fatalf("edited completed detail = %+v", detail)
	}
	var planStatus string
	var planWorkoutID int
	if err := f.db.QueryRow(`SELECT status,workout_id FROM workout_plans WHERE id=$1`, started.ID).Scan(&planStatus, &planWorkoutID); err != nil {
		t.Fatal(err)
	}
	if planStatus != "completed" || planWorkoutID != started.WorkoutID {
		t.Fatalf("edited completed plan status=%s workout_id=%d", planStatus, planWorkoutID)
	}
}

func calendarWorkoutDatabaseSnapshot(t *testing.T, f *manualAPIFixture) string {
	t.Helper()
	var snapshot string
	if err := f.db.QueryRow(`
		SELECT json_build_object(
			'workouts', (SELECT json_agg(w ORDER BY id) FROM workouts w),
			'plans', (SELECT json_agg(p ORDER BY id) FROM workout_plans p),
			'sets', (SELECT json_agg(s ORDER BY id) FROM workout_sets s),
			'stats', (SELECT json_agg(s ORDER BY user_id,exercise_id) FROM user_exercise_stats s)
		)::text
	`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
