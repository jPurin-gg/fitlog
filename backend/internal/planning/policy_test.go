package planning

import (
	"fmt"
	"slices"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
)

func TestApplyRestDaysAvoidsRestDaysAndTrimsRoutine(t *testing.T) {
	plan := MonthlyPlan{
		Rationale:       "理由",
		RecommendedDays: []int{0, 1, 2, 3, 4},
		WeeklyRoutine: []DayRoutine{
			{DayName: "1"}, {DayName: "2"}, {DayName: "3"}, {DayName: "4"}, {DayName: "5"},
		},
	}
	applyRestDays(&plan, []int{0, 1, 2, 3, 4, 5})
	if !slices.Equal(plan.RecommendedDays, []int{6}) {
		t.Fatalf("recommended_days = %v, want [6]", plan.RecommendedDays)
	}
	if len(plan.WeeklyRoutine) != 1 || plan.WeeklyRoutine[0].DayName != "1" {
		t.Fatalf("weekly_routine = %#v, want only the first routine", plan.WeeklyRoutine)
	}
	if !slices.Equal(plan.RestDays, []int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("rest_days = %v, want [0 1 2 3 4 5]", plan.RestDays)
	}
	if plan.Rationale != "理由 日曜・月曜・火曜・水曜・木曜・金曜は休息日として避けて、実施曜日を調整しました。" {
		t.Fatalf("rationale = %q", plan.Rationale)
	}
}

func TestApplyRestDaysSelectsExactDays(t *testing.T) {
	tests := []struct {
		name           string
		plan           MonthlyPlan
		restDays       []int
		wantDays       []int
		wantRestDays   []int
		wantRoutineLen int
		wantRationale  string
	}{
		{
			name:           "keeps valid input days then fills from the default order",
			plan:           MonthlyPlan{Rationale: "理由", RecommendedDays: []int{1, 1, 9}, WeeklyRoutine: []DayRoutine{{DayName: "1"}, {DayName: "2"}, {DayName: "3"}}},
			wantDays:       []int{1, 3, 5},
			wantRestDays:   []int{},
			wantRoutineLen: 3,
			wantRationale:  "理由",
		},
		{
			name:           "two routines use the spaced order",
			plan:           MonthlyPlan{Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1"}, {DayName: "2"}}},
			wantDays:       []int{2, 5},
			wantRestDays:   []int{},
			wantRoutineLen: 2,
			wantRationale:  "理由",
		},
		{
			name:           "five routines use consecutive weekdays",
			plan:           MonthlyPlan{Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1"}, {DayName: "2"}, {DayName: "3"}, {DayName: "4"}, {DayName: "5"}}},
			wantDays:       []int{1, 2, 3, 4, 5},
			wantRestDays:   []int{},
			wantRoutineLen: 5,
			wantRationale:  "理由",
		},
		{
			name:           "skips rest days while filling and explains them",
			plan:           MonthlyPlan{Rationale: "理由", RecommendedDays: []int{6}, WeeklyRoutine: []DayRoutine{{DayName: "1"}, {DayName: "2"}, {DayName: "3"}}},
			restDays:       []int{1, 3},
			wantDays:       []int{6, 5, 2},
			wantRestDays:   []int{1, 3},
			wantRoutineLen: 3,
			wantRationale:  "理由 月曜・水曜は休息日として避けて、実施曜日を調整しました。",
		},
		{
			name:           "normalizes duplicate and out-of-range rest days",
			plan:           MonthlyPlan{Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1"}, {DayName: "2"}, {DayName: "3"}}},
			restDays:       []int{3, 3, 7, -1},
			wantDays:       []int{1, 5, 2},
			wantRestDays:   []int{3},
			wantRoutineLen: 3,
			wantRationale:  "理由 水曜は休息日として避けて、実施曜日を調整しました。",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := test.plan
			applyRestDays(&plan, test.restDays)
			if !slices.Equal(plan.RecommendedDays, test.wantDays) {
				t.Fatalf("recommended_days = %v, want %v", plan.RecommendedDays, test.wantDays)
			}
			if !slices.Equal(plan.RestDays, test.wantRestDays) {
				t.Fatalf("rest_days = %v, want %v", plan.RestDays, test.wantRestDays)
			}
			if len(plan.WeeklyRoutine) != test.wantRoutineLen {
				t.Fatalf("weekly_routine length = %d, want %d", len(plan.WeeklyRoutine), test.wantRoutineLen)
			}
			if plan.Rationale != test.wantRationale {
				t.Fatalf("rationale = %q, want %q", plan.Rationale, test.wantRationale)
			}
		})
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

func TestValidateAIPlanRejectsIncompletePlans(t *testing.T) {
	candidates := []Candidate{{ID: "bench", Name: "ベンチプレス"}, {ID: "squat", Name: "スクワット"}}
	routine := DayRoutine{DayName: "1日目", Target: "押す日", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}
	tests := []struct {
		name       string
		plan       MonthlyPlan
		wantStatus int
	}{
		{name: "valid", plan: MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{routine}}},
		{name: "missing plan name", plan: MonthlyPlan{Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{routine}}, wantStatus: 502},
		{name: "missing description", plan: MonthlyPlan{PlanName: "テスト", Rationale: "理由", WeeklyRoutine: []DayRoutine{routine}}, wantStatus: 502},
		{name: "missing rationale", plan: MonthlyPlan{PlanName: "テスト", Description: "説明", WeeklyRoutine: []DayRoutine{routine}}, wantStatus: 502},
		{name: "empty routine", plan: MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由"}, wantStatus: 502},
		{
			name:       "routine without day name",
			plan:       MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{{Target: "押す日", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}}},
			wantStatus: 502,
		},
		{
			name:       "routine without target",
			plan:       MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1日目", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}}},
			wantStatus: 502,
		},
		{
			name:       "routine without exercise ids",
			plan:       MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1日目", Target: "押す日"}}},
			wantStatus: 502,
		},
		{
			name:       "routine ids and names length mismatch",
			plan:       MonthlyPlan{PlanName: "テスト", Description: "説明", Rationale: "理由", WeeklyRoutine: []DayRoutine{{DayName: "1日目", Target: "押す日", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench", "squat"}}}},
			wantStatus: 502,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := test.plan
			err := validateAIPlan(&plan, candidates)
			if test.wantStatus == 0 {
				if err != nil {
					t.Fatalf("validateAIPlan() error = %v", err)
				}
				return
			}
			appErr := apperr.As(err)
			if err == nil || appErr.Status != test.wantStatus || appErr.Code != apperr.CodeAIUnavailable {
				t.Fatalf("validateAIPlan() error = %#v, want status %d", appErr, test.wantStatus)
			}
		})
	}
}

func TestValidateSavedPlanRejectsInconsistentPlans(t *testing.T) {
	routine := DayRoutine{DayName: "1日目", Target: "胸", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}
	tests := []struct {
		name       string
		plan       MonthlyPlan
		wantStatus int
	}{
		{name: "valid", plan: MonthlyPlan{RestDays: []int{0}, RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{routine}}},
		{name: "recommended days and routine count mismatch", plan: MonthlyPlan{RecommendedDays: []int{1, 3}, WeeklyRoutine: []DayRoutine{routine}}, wantStatus: 400},
		{name: "recommended day is a rest day", plan: MonthlyPlan{RestDays: []int{1}, RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{routine}}, wantStatus: 400},
		{
			name:       "blank day name",
			plan:       MonthlyPlan{RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{{DayName: "  ", Target: "胸", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}}},
			wantStatus: 400,
		},
		{
			name:       "blank target",
			plan:       MonthlyPlan{RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{{DayName: "1日目", Target: "  ", ExampleExercises: []string{"ベンチプレス"}, ExerciseIDs: []string{"bench"}}}},
			wantStatus: 400,
		},
		{
			name:       "no exercise ids",
			plan:       MonthlyPlan{RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{{DayName: "1日目", Target: "胸"}}},
			wantStatus: 400,
		},
		{
			name:       "ids and names length mismatch",
			plan:       MonthlyPlan{RecommendedDays: []int{1}, WeeklyRoutine: []DayRoutine{{DayName: "1日目", Target: "胸", ExampleExercises: []string{"ベンチプレス", "スクワット"}, ExerciseIDs: []string{"bench"}}}},
			wantStatus: 400,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSavedPlan(test.plan)
			if test.wantStatus == 0 {
				if err != nil {
					t.Fatalf("validateSavedPlan() error = %v", err)
				}
				return
			}
			appErr := apperr.As(err)
			if err == nil || appErr.Status != test.wantStatus || appErr.Code != apperr.CodeValidation {
				t.Fatalf("validateSavedPlan() error = %#v, want status %d", appErr, test.wantStatus)
			}
		})
	}
}

func TestFilterCandidatesAppliesEquipmentAndMuscleCaps(t *testing.T) {
	build := func(count int, muscle, equipment string, favorite bool) []Candidate {
		result := []Candidate{}
		for index := 0; index < count; index++ {
			result = append(result, Candidate{ID: fmt.Sprintf("%s-%s-%d", muscle, equipment, index), Name: muscle, Equipment: equipment, PrimaryMuscles: []string{muscle}, IsFavorite: favorite})
		}
		return result
	}
	manyMuscles := []Candidate{}
	for muscle := 0; muscle < 20; muscle++ {
		manyMuscles = append(manyMuscles, build(10, fmt.Sprintf("筋群%d", muscle), "バーベル", false)...)
	}
	tests := []struct {
		name        string
		candidates  []Candidate
		preferences profile.Preferences
		wantCount   int
		wantIDs     []string
	}{
		{
			name:       "drops candidates without primary muscles",
			candidates: []Candidate{{ID: "none", Equipment: "バーベル"}, {ID: "chest", Equipment: "バーベル", PrimaryMuscles: []string{"胸"}}},
			wantCount:  1,
			wantIDs:    []string{"chest"},
		},
		{
			name:        "drops avoided equipment",
			candidates:  []Candidate{{ID: "machine", Equipment: "マシン", PrimaryMuscles: []string{"胸"}}, {ID: "barbell", Equipment: "バーベル", PrimaryMuscles: []string{"胸"}}},
			preferences: profile.Preferences{AvoidedEquipment: []string{"マシン"}},
			wantCount:   1,
			wantIDs:     []string{"barbell"},
		},
		{name: "caps plain candidates at 14 per primary muscle", candidates: build(16, "胸", "バーベル", false), wantCount: 14},
		{name: "caps favorites at 30 per primary muscle", candidates: build(32, "胸", "バーベル", true), wantCount: 30},
		{
			name:        "caps preferred equipment at 22 per primary muscle",
			candidates:  build(24, "胸", "バーベル", false),
			preferences: profile.Preferences{PreferredEquipment: []string{"バーベル"}},
			wantCount:   22,
		},
		{
			name:        "caps favorite preferred equipment at 36 per primary muscle",
			candidates:  build(38, "胸", "バーベル", true),
			preferences: profile.Preferences{PreferredEquipment: []string{"バーベル"}},
			wantCount:   36,
		},
		{
			name:        "caps non-preferred equipment at 8 when preferences exist",
			candidates:  build(10, "胸", "ダンベル", false),
			preferences: profile.Preferences{PreferredEquipment: []string{"バーベル"}},
			wantCount:   8,
		},
		{name: "caps the whole list at 180", candidates: manyMuscles, wantCount: 180},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := filterCandidates(test.candidates, test.preferences)
			if len(got) != test.wantCount {
				t.Fatalf("filterCandidates() returned %d candidates, want %d", len(got), test.wantCount)
			}
			if test.wantIDs == nil {
				return
			}
			ids := []string{}
			for _, candidate := range got {
				ids = append(ids, candidate.ID)
			}
			if !slices.Equal(ids, test.wantIDs) {
				t.Fatalf("filterCandidates() ids = %v, want %v", ids, test.wantIDs)
			}
		})
	}
}
