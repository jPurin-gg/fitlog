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
				"SetOrder":  1,
				"Weight":    80.0,
				"Reps":      10,
				"Feeling":   "まだ余裕がある",
				"MaxWeight": 85.0,
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
			name:       "alternative",
			systemFile: "alternative_system.txt",
			userFile:   "alternative_user.txt",
			data: map[string]any{
				"Exercise":  "ベンチプレス",
				"Reason":    "マシンが空いていない",
				"DBContext": "- ID: Dumbbell_Bench_Press, Name: ダンベルベンチプレス",
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
