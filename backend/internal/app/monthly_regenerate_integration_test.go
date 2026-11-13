//go:build integration

package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
)

// 月間プランを作り直しても、保存済みの日次予定（workout_plans）は作り直されずに残る。
// 予定を保存していない日は、新しい月間プランから組み立てる。
// ホームの月間プランモーダルはこの動作をそのまま説明している。
func TestRegeneratingMonthlyPlanKeepsSavedDailyPlansHTTP(t *testing.T) {
	regenerated := planning.MonthlyPlan{
		PlanName: "新プラン", Frequency: "週3-4回", Description: "脚を中心にします。", Rationale: "下半身を優先します。",
		RecommendedDays: []int{1, 3},
		WeeklyRoutine: []planning.DayRoutine{
			{DayName: "1日目", Target: "脚の日A", ExampleExercises: []string{"スクワット"}, ExerciseIDs: []string{"squat"}},
			{DayName: "2日目", Target: "脚の日B", ExampleExercises: []string{"スクワット"}, ExerciseIDs: []string{"squat"}},
		},
	}
	f := newManualAPIFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		content, _ := json.Marshal(regenerated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	})
	if _, err := f.db.Exec(`UPDATE exercises SET category='筋力トレーニング', level='初級', primary_muscles='["胸"]'::jsonb WHERE id='bench'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE exercises SET category='筋力トレーニング', level='初級', primary_muscles='["脚"]'::jsonb WHERE id='squat'`); err != nil {
		t.Fatal(err)
	}

	// 過去日の予定はカレンダー取得時に完了扱いになるため、来年8月の第2月曜と同じ週の水曜を使う。
	monday := time.Date(time.Now().UTC().Year()+1, time.August, 8, 0, 0, 0, 0, time.UTC)
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}
	month := monday.Format("2006-01")
	savedDate := monday.Format("2006-01-02")                    // カレンダーで予定を保存する日
	unsavedDate := monday.AddDate(0, 0, 2).Format("2006-01-02") // 予定を保存していない日
	f.request(t, "PUT", "/api/monthly-plans/"+month, planning.MonthlyPlanInput{
		PlanName: "旧プラン", Frequency: "週1-2回", Description: "旧", Rationale: "旧", RecommendedDays: []int{1},
		WeeklyRoutine: []planning.DayRoutine{{DayName: "1日目", Target: "胸の日", ExerciseIDs: []string{"bench"}, ExampleExercises: []string{"ベンチプレス"}}},
	}, "", http.StatusOK)
	stored := decodeAPI[planning.PlanSession](t, f.request(t, "PUT", "/api/workout-plans/"+savedDate, planning.WorkoutPlan{
		WorkoutTitle: "手で調整した胸の日", Target: "胸の日",
		Exercises: []planning.PlanExercise{{ExerciseID: "bench", PlannedSets: 5, TargetReps: 5, TargetWeight: 60}},
	}, "", http.StatusOK))

	saved := decodeAPI[planning.MonthlyPlan](t, f.request(t, "POST", "/api/monthly-plans/"+month+"/generate", planning.GenerateMonthlyInput{Frequency: "週3-4回"}, "", http.StatusOK))
	if saved.PlanName != "新プラン" || f.aiCalls.Load() != 1 {
		t.Fatalf("generated plan = %+v, AI calls = %d", saved, f.aiCalls.Load())
	}
	current := decodeAPI[planning.MonthlyPlan](t, f.request(t, "GET", "/api/monthly-plans/"+month, nil, "", http.StatusOK))
	if current.PlanName != "新プラン" || current.Description != regenerated.Description || current.Rationale != regenerated.Rationale {
		t.Fatalf("monthly plan after regeneration = %+v", current)
	}

	kept := decodeAPI[planning.PlanSession](t, f.request(t, "GET", "/api/workout-plans/"+savedDate, nil, "", http.StatusOK))
	if kept.ID != stored.ID || kept.Status != "active" || !reflect.DeepEqual(kept.Plan, stored.Plan) {
		t.Fatalf("saved daily plan changed after regeneration: got %+v, want %+v", kept, stored)
	}
	draft := decodeAPI[planning.PlanSession](t, f.request(t, "GET", "/api/workout-plans/"+unsavedDate, nil, "", http.StatusOK))
	if draft.Status != "draft" || draft.Plan.Target != "脚の日B" || draft.Plan.Exercises[0].ExerciseID != "squat" {
		t.Fatalf("unsaved day = %+v, want draft from regenerated plan", draft)
	}
	calendar := decodeAPI[reporting.Calendar](t, f.request(t, "GET", fmt.Sprintf("/api/calendar?year=%d&month=8", monday.Year()), nil, "", http.StatusOK))
	if len(calendar.PlannedDays) != 1 || calendar.PlannedDays[0].Date != monday.Day() || calendar.PlannedDays[0].PlanID != stored.ID {
		t.Fatalf("calendar planned days = %+v, want only the saved day", calendar.PlannedDays)
	}
	var dailyRows int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM workout_plans`).Scan(&dailyRows); err != nil {
		t.Fatal(err)
	}
	if dailyRows != 1 {
		t.Fatalf("workout_plans rows = %d, want 1", dailyRows)
	}
}
