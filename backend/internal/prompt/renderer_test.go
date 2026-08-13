package prompt

import (
	"path/filepath"
	"testing"
)

func TestRendererRendersEveryPromptPair(t *testing.T) {
	renderer := NewRenderer(filepath.Join("..", "..", "prompts"))
	tests := []struct {
		system string
		user   string
		data   map[string]any
	}{
		{"recommend_system.txt", "recommend_user.txt", map[string]any{"ExerciseName": "ベンチプレス", "SetOrder": 1, "Weight": 80, "Reps": 10, "Feeling": "余裕", "MaxWeight": 85, "RecentExerciseHistory": "なし", "TodayWorkoutContext": "なし"}},
		{"workout_plan_system.txt", "workout_plan_user.txt", map[string]any{"BasePlanJSON": `{}`}},
		{"monthly_plan_system.txt", "monthly_plan_user.txt", map[string]any{"Motivation": "健康", "Frequency": "週3回", "RestDays": "日曜", "RestDaysJSON": "[0]", "PreferencesText": "なし", "PreferencesJSON": `{}`, "CandidatesJSON": `[]`}},
		{"alternative_system.txt", "alternative_user.txt", map[string]any{"Exercise": "ベンチプレス", "Reason": "混雑", "DBContext": "候補"}},
		{"workout_summary_system.txt", "workout_summary_user.txt", map[string]any{"SummaryJSON": `{}`, "WorkoutSetContext": "なし"}},
	}
	for _, test := range tests {
		system, user, err := renderer.Pair(test.system, test.user, test.data)
		if err != nil {
			t.Fatalf("Pair(%s, %s) error = %v", test.system, test.user, err)
		}
		if system == "" || user == "" {
			t.Fatalf("Pair(%s, %s) rendered an empty prompt", test.system, test.user)
		}
	}
}

func TestJSONTextRemovesJSONFence(t *testing.T) {
	if got := JSONText("```json\n{\"ok\":true}\n```"); got != `{"ok":true}` {
		t.Fatalf("JSONText() = %q", got)
	}
}
