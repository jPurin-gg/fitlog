package main

import "testing"

func TestRenderPromptPair(t *testing.T) {
	t.Setenv("PROMPT_DIR", "prompts")

	tests := []struct {
		name       string
		systemFile string
		userFile   string
		data       map[string]any
	}{
		{
			name:       "recommend",
			systemFile: "recommend_system.txt",
			userFile:   "recommend_user.txt",
			data: map[string]any{
				"ExerciseName":          "ベンチプレス",
				"SetOrder":              1,
				"Weight":                80.0,
				"Reps":                  10,
				"Feeling":               "まだ余裕がある",
				"MaxWeight":             85.0,
				"RecentExerciseHistory": "- 2026-05-10: 80.0kg x 10回 / 感想: かなりきつい",
				"TodayWorkoutContext":   "- ベンチプレス 1セット目: 80.0kg x 10回 / 感想: まだ余裕がある",
			},
		},
		{
			name:       "workout plan",
			systemFile: "workout_plan_system.txt",
			userFile:   "workout_plan_user.txt",
			data: map[string]any{
				"BasePlanJSON": `{"workout_title":"押す日","exercises":[]}`,
			},
		},
		{
			name:       "monthly plan",
			systemFile: "monthly_plan_system.txt",
			userFile:   "monthly_plan_user.txt",
			data: map[string]any{
				"Motivation":      "筋肉を大きく",
				"Frequency":       "週3-4回",
				"RestDays":        "日曜",
				"RestDaysJSON":    "[0]",
				"PreferencesText": "優先する器具: ダンベル",
				"PreferencesJSON": `{"preferred_equipment":["ダンベル"],"avoided_equipment":[],"training_environment":"ジム","notes":""}`,
				"CandidatesJSON":  `[{"id":"Barbell_Bench_Press_-_Medium_Grip","name":"ベンチプレス","equipment":"バーベル","level":"初級","category":"筋力トレーニング","primary_muscles":["大胸筋"]}]`,
			},
		},
		{
			name:       "alternative",
			systemFile: "alternative_system.txt",
			userFile:   "alternative_user.txt",
			data: map[string]any{
				"Exercise":  "ベンチプレス",
				"Reason":    "マシンが空いていない",
				"DBContext": "- ID: Dumbbell_Bench_Press, Name: ダンベルベンチプレス",
			},
		},
		{
			name:       "workout summary",
			systemFile: "workout_summary_system.txt",
			userFile:   "workout_summary_user.txt",
			data: map[string]any{
				"SummaryJSON":       `{"total_sets":3,"total_reps":30,"total_volume":2400,"duration_min":35,"pr_count":1,"exercises":[]}`,
				"WorkoutSetContext": "- ベンチプレス 1セット目: 80.0kg x 10回 / 感想: かなり効いた",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			systemPrompt, userPrompt, err := renderPromptPair(tt.systemFile, tt.userFile, tt.data)
			if err != nil {
				t.Fatalf("renderPromptPair() error = %v", err)
			}
			if systemPrompt == "" {
				t.Fatal("system prompt is empty")
			}
			if userPrompt == "" {
				t.Fatal("user prompt is empty")
			}
		})
	}
}
