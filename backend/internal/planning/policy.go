package planning

import (
	"encoding/json"
	"strings"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
)

func NormalizeWeekdays(days []int) []int {
	seen := map[int]bool{}
	for _, day := range days {
		if day >= 0 && day <= 6 {
			seen[day] = true
		}
	}
	result := []int{}
	for day := 0; day <= 6; day++ {
		if seen[day] {
			result = append(result, day)
		}
	}
	return result
}

func WeekdayListText(days []int) string {
	names := map[int]string{0: "日曜", 1: "月曜", 2: "火曜", 3: "水曜", 4: "木曜", 5: "金曜", 6: "土曜"}
	labels := []string{}
	for _, day := range NormalizeWeekdays(days) {
		labels = append(labels, names[day])
	}
	return strings.Join(labels, "・")
}

func filterCandidates(candidates []Candidate, preferences profile.Preferences) []Candidate {
	result := []Candidate{}
	muscleCounts := map[string]int{}
	preferred := stringSet(preferences.PreferredEquipment)
	avoided := stringSet(preferences.AvoidedEquipment)
	for _, candidate := range candidates {
		if len(candidate.PrimaryMuscles) == 0 || avoided[candidate.Equipment] {
			continue
		}
		limit := 14
		if candidate.IsFavorite {
			limit = 30
		}
		if len(preferred) > 0 && preferred[candidate.Equipment] {
			limit = 22
		}
		if candidate.IsFavorite && len(preferred) > 0 && preferred[candidate.Equipment] {
			limit = 36
		}
		if len(preferred) > 0 && !preferred[candidate.Equipment] {
			limit = 8
		}
		primary := candidate.PrimaryMuscles[0]
		if muscleCounts[primary] >= limit {
			continue
		}
		if len(result) >= 180 {
			break
		}
		muscleCounts[primary]++
		result = append(result, candidate)
	}
	return result
}

func validateAIPlan(plan *MonthlyPlan, candidates []Candidate) error {
	if plan.PlanName == "" || plan.Description == "" || plan.Rationale == "" || len(plan.WeeklyRoutine) == 0 {
		return apperr.New(502, apperr.CodeAIUnavailable, "AIの月間プランが不完全です。")
	}
	names := map[string]string{}
	for _, candidate := range candidates {
		names[candidate.ID] = candidate.Name
	}
	for routineIndex := range plan.WeeklyRoutine {
		routine := &plan.WeeklyRoutine[routineIndex]
		if routine.DayName == "" || routine.Target == "" || len(routine.ExerciseIDs) == 0 || len(routine.ExerciseIDs) != len(routine.ExampleExercises) {
			return apperr.New(502, apperr.CodeAIUnavailable, "AIの月間プランが不完全です。")
		}
		for index, id := range routine.ExerciseIDs {
			name, exists := names[id]
			if !exists {
				return apperr.New(502, apperr.CodeAIUnavailable, "AIが辞書外の種目を選択しました。")
			}
			// IDを信頼できる辞書の正本とし、AIが返した表記ゆれは保存しない。
			routine.ExampleExercises[index] = name
		}
	}
	return nil
}

func validateSavedPlan(plan MonthlyPlan) error {
	if len(plan.RecommendedDays) != len(plan.WeeklyRoutine) {
		return apperr.Validation("実施曜日とルーティンの数を合わせてください。", map[string]string{"recommended_days": "must match weekly_routine"})
	}
	restDays := stringSetInts(plan.RestDays)
	for _, day := range plan.RecommendedDays {
		if restDays[day] {
			return apperr.Validation("休息日は実施曜日に指定できません。", map[string]string{"recommended_days": "contains a rest day"})
		}
	}
	for _, routine := range plan.WeeklyRoutine {
		if strings.TrimSpace(routine.DayName) == "" || strings.TrimSpace(routine.Target) == "" || len(routine.ExerciseIDs) == 0 || len(routine.ExerciseIDs) != len(routine.ExampleExercises) {
			return apperr.Validation("各ルーティンに種目IDと種目名が必要です。", map[string]string{"weekly_routine": "invalid routine"})
		}
	}
	return nil
}

func applyRestDays(plan *MonthlyPlan, restDays []int) {
	restDays = NormalizeWeekdays(restDays)
	plan.RestDays = restDays
	restSet := stringSetInts(restDays)
	targetCount := len(plan.WeeklyRoutine)
	if available := 7 - len(restDays); targetCount > available {
		targetCount = available
		plan.WeeklyRoutine = plan.WeeklyRoutine[:targetCount]
	}
	selected := []int{}
	seen := map[int]bool{}
	appendDay := func(day int) {
		if day >= 0 && day <= 6 && len(selected) < targetCount && !restSet[day] && !seen[day] {
			selected = append(selected, day)
			seen[day] = true
		}
	}
	for _, day := range plan.RecommendedDays {
		appendDay(day)
	}
	order := []int{1, 3, 5, 2, 4, 6, 0}
	if targetCount <= 2 {
		order = []int{2, 5, 1, 4, 6, 3, 0}
	} else if targetCount >= 5 {
		order = []int{1, 2, 3, 4, 5, 6, 0}
	}
	for _, day := range order {
		appendDay(day)
	}
	plan.RecommendedDays = selected
	if len(restDays) > 0 {
		plan.Rationale = strings.TrimSpace(plan.Rationale + " " + WeekdayListText(restDays) + "は休息日として避けて、実施曜日を調整しました。")
	}
}

func preferencesText(preferences profile.Preferences) string {
	parts := []string{}
	if preferences.TrainingEnvironment != "" {
		parts = append(parts, "環境: "+preferences.TrainingEnvironment)
	}
	if len(preferences.PreferredEquipment) > 0 {
		parts = append(parts, "優先する器具: "+strings.Join(preferences.PreferredEquipment, "、"))
	}
	if len(preferences.AvoidedEquipment) > 0 {
		parts = append(parts, "避けたい器具: "+strings.Join(preferences.AvoidedEquipment, "、"))
	}
	if preferences.Notes != "" {
		parts = append(parts, "メモ: "+preferences.Notes)
	}
	if len(parts) == 0 {
		return "指定なし"
	}
	return strings.Join(parts, "\n")
}

func marshalString(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(encoded)
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = true
		}
	}
	return result
}

func stringSetInts(values []int) map[int]bool {
	result := map[int]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
