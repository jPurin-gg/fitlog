package planning

import "testing"

func TestApplyRestDaysAvoidsRestDaysAndTrimsRoutine(t *testing.T) {
	plan := MonthlyPlan{
		RecommendedDays: []int{0, 1, 2, 3, 4},
		WeeklyRoutine: []DayRoutine{
			{DayName: "1"}, {DayName: "2"}, {DayName: "3"}, {DayName: "4"}, {DayName: "5"},
		},
	}
	applyRestDays(&plan, []int{0, 1, 2, 3, 4, 5})
	if len(plan.RecommendedDays) != 1 || plan.RecommendedDays[0] != 6 {
		t.Fatalf("recommended_days = %v, want [6]", plan.RecommendedDays)
	}
	if len(plan.WeeklyRoutine) != 1 {
		t.Fatalf("weekly_routine length = %d, want 1", len(plan.WeeklyRoutine))
	}
}

func TestApplyRestDaysNormalizesAndCompletesDaysWhenThereAreNoRestDays(t *testing.T) {
	plan := MonthlyPlan{
		RecommendedDays: []int{1, 1, 9},
		WeeklyRoutine: []DayRoutine{
			{DayName: "1"}, {DayName: "2"}, {DayName: "3"},
		},
	}
	applyRestDays(&plan, nil)
	if len(plan.RecommendedDays) != len(plan.WeeklyRoutine) {
		t.Fatalf("recommended_days = %v; routine length = %d", plan.RecommendedDays, len(plan.WeeklyRoutine))
	}
	seen := map[int]bool{}
	for _, day := range plan.RecommendedDays {
		if day < 0 || day > 6 || seen[day] {
			t.Fatalf("recommended_days = %v; want unique weekdays", plan.RecommendedDays)
		}
		seen[day] = true
	}
}

func TestValidateAIPlanRequiresDictionaryIDs(t *testing.T) {
	plan := MonthlyPlan{
		PlanName: "テスト", Description: "説明", Rationale: "理由",
		WeeklyRoutine: []DayRoutine{{
			DayName: "1日目", Target: "押す日",
			ExampleExercises: []string{"AIが返した別表記"}, ExerciseIDs: []string{"bench"},
		}},
	}
	candidates := []Candidate{{ID: "bench", Name: "ベンチプレス"}}
	if err := validateAIPlan(&plan, candidates); err != nil {
		t.Fatalf("validateAIPlan() error = %v", err)
	}
	if got := plan.WeeklyRoutine[0].ExampleExercises[0]; got != "ベンチプレス" {
		t.Fatalf("dictionary name = %q", got)
	}
	plan.WeeklyRoutine[0].ExerciseIDs[0] = "unknown"
	if err := validateAIPlan(&plan, candidates); err == nil {
		t.Fatal("validateAIPlan(unknown) error = nil")
	}
}
