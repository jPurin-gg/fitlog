package main

import "testing"

func TestGenerateMonthlyPlanAvoidsRestDays(t *testing.T) {
	plan := generateMonthlyPlan("筋肉を大きく", "週3-4回", []int{3})

	if len(plan.RestDays) != 1 || plan.RestDays[0] != 3 {
		t.Fatalf("rest_days = %v, want [3]", plan.RestDays)
	}
	for _, day := range plan.RecommendedDays {
		if day == 3 {
			t.Fatalf("recommended_days includes rest day: %v", plan.RecommendedDays)
		}
	}
	if len(plan.RecommendedDays) != len(plan.WeeklyRoutine) {
		t.Fatalf("recommended_days length = %d, weekly_routine length = %d", len(plan.RecommendedDays), len(plan.WeeklyRoutine))
	}
}

func TestGenerateMonthlyPlanTrimsRoutineWhenRestDaysAreMany(t *testing.T) {
	plan := generateMonthlyPlan("本気", "週5-6回", []int{0, 1, 2, 3, 4, 5})

	if len(plan.RecommendedDays) != 1 {
		t.Fatalf("recommended_days length = %d, want 1: %v", len(plan.RecommendedDays), plan.RecommendedDays)
	}
	if len(plan.WeeklyRoutine) != 1 {
		t.Fatalf("weekly_routine length = %d, want 1", len(plan.WeeklyRoutine))
	}
	if plan.RecommendedDays[0] != 6 {
		t.Fatalf("recommended_days = %v, want [6]", plan.RecommendedDays)
	}
}

func TestValidateMonthlyPlanFromAIRequiresDictionaryIDs(t *testing.T) {
	plan := MonthlyPlanResponse{
		PlanName:        "テストプラン",
		Frequency:       "週3〜4回",
		Description:     "説明",
		Rationale:       "理由",
		RecommendedDays: []int{1},
		WeeklyRoutine: []DayRoutine{
			{
				DayName:          "1日目",
				Target:           "押す日",
				ExampleExercises: []string{"ベンチプレス"},
				ExerciseIDs:      []string{"bench"},
			},
		},
	}
	candidates := []MonthlyPlanExerciseCandidate{{ID: "bench", Name: "ベンチプレス"}}

	if err := validateMonthlyPlanFromAI(&plan, candidates); err != nil {
		t.Fatalf("validateMonthlyPlanFromAI() error = %v", err)
	}

	plan.WeeklyRoutine[0].ExerciseIDs[0] = "unknown"
	if err := validateMonthlyPlanFromAI(&plan, candidates); err == nil {
		t.Fatal("validateMonthlyPlanFromAI() error = nil, want error")
	}
}
