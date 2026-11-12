//go:build integration

package app_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

// その場で記録したワークアウトの1セットをカレンダーから修正しても、
// 開始・終了時刻、所要時間、AI総評、ほかのセットが変わらないことを実HTTPで確認する。
func TestCalendarEditKeepsRecordedTimesHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	workoutID := f.recordManualWorkout(t, date)
	workoutPath := fmt.Sprintf("/api/workouts/%d", workoutID)
	// 外部AIを呼ばずに、総評が保存済みの状態を作る。
	if _, err := f.db.Exec(`UPDATE workouts SET summary_comment='保存済みの総評' WHERE id=$1`, workoutID); err != nil {
		t.Fatal(err)
	}

	before := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if before.Status != "completed" || before.StartedAt == "" || before.EndedAt == "" || len(before.Sets) != 2 {
		t.Fatalf("detail before edit = %+v", before)
	}
	if first, second := before.Sets[0], before.Sets[1]; first.ExerciseName != "ベンチプレス" || first.Weight != 40 || first.Reps != 10 || first.Feeling != "普通" || !first.IsPR ||
		second.Weight != 45 || second.Reps != 8 || second.Feeling != "重い" || !second.IsPR {
		t.Fatalf("sets before edit = %+v", before.Sets)
	}

	calendar := decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", "/api/workouts/by-date/"+date, nil, "", http.StatusOK))
	if calendar.WorkoutID != workoutID || len(calendar.Sets) != 2 || calendar.Sets[1].ID != before.Sets[1].ID {
		t.Fatalf("calendar workout = %+v", calendar)
	}
	edited := workout.CalendarWorkoutInput{Title: calendar.Title, Sets: calendar.Sets}
	edited.Sets[1].Reps = 9
	f.request(t, "PUT", "/api/workouts/by-date/"+date, edited, "", http.StatusOK)

	after := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if after.StartedAt != before.StartedAt || after.EndedAt != before.EndedAt || after.Status != "completed" {
		t.Fatalf("times after edit = %s - %s (%s), want %s - %s", after.StartedAt, after.EndedAt, after.Status, before.StartedAt, before.EndedAt)
	}
	if after.Summary.DurationMin != before.Summary.DurationMin || after.Summary.AIComment != "保存済みの総評" || after.Summary.TotalSets != 2 || after.Summary.TotalReps != 19 || after.Summary.PRCount != before.Summary.PRCount {
		t.Fatalf("summary after edit = %+v, before = %+v", after.Summary, before.Summary)
	}
	wantEdited := before.Sets[1]
	wantEdited.Reps = 9
	if len(after.Sets) != 2 || after.Sets[0] != before.Sets[0] || after.Sets[1] != wantEdited {
		t.Fatalf("sets after edit = %+v, want %+v and %+v", after.Sets, before.Sets[0], wantEdited)
	}

	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls = %d, want zero", calls)
	}
}

// 記録を削除すると、履歴・カレンダー・ホーム・最大重量から消え、同じ日を記録し直せることを確認する。
func TestDeleteWorkoutUpdatesHistoryAndStatsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	calendarPath := fmt.Sprintf("/api/calendar?year=%d&month=%d", now.Year(), int(now.Month()))
	workoutID := f.recordManualWorkout(t, date)
	workoutPath := fmt.Sprintf("/api/workouts/%d", workoutID)

	history := decodeAPI[workout.HistoryPage](t, f.request(t, "GET", "/api/workouts?limit=1", nil, "", http.StatusOK))
	if history.Total != 1 || history.Limit != 1 || history.Offset != 0 || len(history.Workouts) != 1 {
		t.Fatalf("history before delete = %+v", history)
	}
	if entry := history.Workouts[0]; entry.ID != workoutID || entry.TotalSets != 2 || entry.TotalVolume != 760 || entry.StartedAt == "" || entry.EndedAt == "" {
		t.Fatalf("history entry = %+v", entry)
	}
	f.request(t, "GET", "/api/workouts?limit=0abc", nil, "", http.StatusBadRequest)
	f.request(t, "GET", "/api/workouts?limit=101", nil, "", http.StatusBadRequest)
	if calendar := decodeAPI[reporting.Calendar](t, f.request(t, "GET", calendarPath, nil, "", http.StatusOK)); len(calendar.WorkedOutDays) != 1 || calendar.WorkedOutDays[0].WorkoutID != workoutID {
		t.Fatalf("calendar before delete = %+v", calendar)
	}
	if dashboard := decodeAPI[reporting.Dashboard](t, f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK)); len(dashboard.RecentWorkouts) != 1 || dashboard.RecentWorkouts[0].ID != workoutID {
		t.Fatalf("recent workouts before delete = %+v", dashboard.RecentWorkouts)
	}
	if latest, best := f.exerciseStats(t, "bench"); latest != (sql.NullFloat64{Float64: 45, Valid: true}) || best != (sql.NullFloat64{Float64: 45, Valid: true}) {
		t.Fatalf("stats before delete: weight=%v max_weight=%v", latest, best)
	}

	f.request(t, "DELETE", workoutPath, nil, "", http.StatusNoContent)

	f.request(t, "GET", workoutPath, nil, "", http.StatusNotFound)
	f.request(t, "DELETE", workoutPath, nil, "", http.StatusNotFound)
	f.request(t, "GET", "/api/workouts/by-date/"+date, nil, "", http.StatusNotFound)
	if history := decodeAPI[workout.HistoryPage](t, f.request(t, "GET", "/api/workouts", nil, "", http.StatusOK)); history.Total != 0 || history.Workouts == nil || len(history.Workouts) != 0 || history.Limit != workout.DefaultHistoryLimit {
		t.Fatalf("history after delete = %+v", history)
	}
	if dashboard := decodeAPI[reporting.Dashboard](t, f.request(t, "GET", "/api/dashboard", nil, "", http.StatusOK)); len(dashboard.RecentWorkouts) != 0 {
		t.Fatalf("recent workouts after delete = %+v", dashboard.RecentWorkouts)
	}
	if calendar := decodeAPI[reporting.Calendar](t, f.request(t, "GET", calendarPath, nil, "", http.StatusOK)); len(calendar.WorkedOutDays) != 0 || len(calendar.WorkedOutDates) != 0 {
		t.Fatalf("calendar after delete = %+v", calendar)
	}
	if latest, best := f.exerciseStats(t, "bench"); latest.Valid || best.Valid {
		t.Fatalf("stats after delete: weight=%v max_weight=%v, want both cleared", latest, best)
	}

	// 削除した日の実施済み予定が残っていると、やり直した記録の終了が一意制約で失敗する。
	retried := f.recordManualWorkout(t, date)
	if retried == workoutID {
		t.Fatalf("retried workout reused the deleted ID %d", workoutID)
	}
	if history := decodeAPI[workout.HistoryPage](t, f.request(t, "GET", "/api/workouts", nil, "", http.StatusOK)); history.Total != 1 || len(history.Workouts) != 1 || history.Workouts[0].ID != retried {
		t.Fatalf("history after retry = %+v", history)
	}
	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls = %d, want zero", calls)
	}
}

// recordManualWorkout は、ベンチプレスを 40kg×10回・45kg×8回 記録して終了し、ワークアウトIDを返す。
func (f *manualAPIFixture) recordManualWorkout(t *testing.T, date string) int {
	t.Helper()
	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", "/api/workout-plans/"+date+"/start", map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	if started.WorkoutID <= 0 {
		t.Fatalf("manual start = %+v", started)
	}
	workoutPath := fmt.Sprintf("/api/workouts/%d", started.WorkoutID)
	for index, input := range []workout.SetInput{
		{ExerciseID: "bench", SetOrder: 1, Weight: 40, Reps: 10, Feeling: "普通"},
		{ExerciseID: "bench", SetOrder: 2, Weight: 45, Reps: 8, Feeling: "重い"},
	} {
		key := fmt.Sprintf("history_%d_set_%03d", started.WorkoutID, index+1)
		f.request(t, "POST", workoutPath+"/sets", input, key, http.StatusCreated)
	}
	finished := decodeAPI[workout.FinishResult](t, f.request(t, "POST", workoutPath+"/finish", nil, "", http.StatusOK))
	if finished.Status != "completed" || finished.Summary.TotalSets != 2 {
		t.Fatalf("finished workout = %+v", finished)
	}
	return started.WorkoutID
}

func (f *manualAPIFixture) exerciseStats(t *testing.T, exerciseID string) (latest, best sql.NullFloat64) {
	t.Helper()
	if err := f.db.QueryRow(`
		SELECT s.weight,s.max_weight FROM user_exercise_stats s JOIN users u ON u.id=s.user_id
		WHERE u.username='integration-user' AND s.exercise_id=$1
	`, exerciseID).Scan(&latest, &best); err != nil {
		t.Fatal(err)
	}
	return latest, best
}
