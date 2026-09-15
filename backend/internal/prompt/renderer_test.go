package prompt

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
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

func TestRendererRejectsMissingTemplateData(t *testing.T) {
	renderer := NewRenderer(filepath.Join("..", "..", "prompts"))
	if _, _, err := renderer.Pair("recommend_system.txt", "recommend_user.txt", map[string]any{}); err == nil {
		t.Fatal("Pair() error = nil; want missing template data error")
	}
}

func TestRendererRejectsPathEscapingPromptDirectory(t *testing.T) {
	renderer := NewRenderer(filepath.Join("..", "..", "prompts"))
	_, _, err := renderer.Pair("../go.mod", "../go.mod", nil)
	if err == nil || !strings.Contains(err.Error(), "parse prompt") {
		t.Fatalf("Pair(../go.mod) error = %v; want parse error inside prompt directory", err)
	}
}

func TestJSONTextStripsMarkdownFence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no fence", input: "  {\"ok\":true}\n", want: `{"ok":true}`},
		{name: "json fence", input: "```json\n{\n  \"ok\": true\n}\n```", want: "{\n  \"ok\": true\n}"},
		{name: "plain fence", input: "```\n{\"ok\":true}\n```", want: `{"ok":true}`},
		{name: "uppercase fence", input: "```JSON\n{\"ok\":true}\n```", want: `{"ok":true}`},
		{name: "fence with trailing space", input: "```json \n{\"ok\":true}\n```", want: `{"ok":true}`},
		{name: "crlf fence", input: "```json\r\n{\"ok\":true}\r\n```", want: `{"ok":true}`},
		{name: "blank lines around fence", input: "\n\n```json\n{\"ok\":true}\n```\n\n", want: `{"ok":true}`},
		{name: "backticks inside string without fence", input: "{\"note\":\"use ``` for code\"}", want: "{\"note\":\"use ``` for code\"}"},
		{name: "closing fence with trailing newline", input: "```json\n{\"ok\":true}\n```\n", want: `{"ok":true}`},
		{name: "single line json fence", input: "```json{\"ok\":true}```", want: `{"ok":true}`},
		{name: "body on fence line without tag", input: "```{\"ok\":true}\n```", want: `{"ok":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := JSONText(test.input); got != test.want {
				t.Fatalf("JSONText(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestJSONTextFencedOutputUnmarshalsLikeBody(t *testing.T) {
	body := "{\"ok\":true,\"sets\":[{\"weight\":80,\"reps\":10}],\"note\":\"use ``` for code\"}"
	var want map[string]any
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatalf("json.Unmarshal(body) error = %v", err)
	}
	tests := []struct {
		name  string
		input string
	}{
		{name: "json fence", input: "```json\n" + body + "\n```"},
		{name: "plain fence", input: "```\n" + body + "\n```"},
		{name: "uppercase fence", input: "```JSON\n" + body + "\n```"},
		{name: "fence with trailing space", input: "```json \n" + body + "\n```"},
		{name: "crlf fence", input: "```json\r\n" + body + "\r\n```"},
		{name: "blank lines around fence", input: "\n\n```json\n" + body + "\n```\n\n"},
		{name: "closing fence with trailing newline", input: "```json\n" + body + "\n```\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got map[string]any
			if err := json.Unmarshal([]byte(JSONText(test.input)), &got); err != nil {
				t.Fatalf("json.Unmarshal(JSONText(%q)) error = %v", test.input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSONText(%q) unmarshaled = %#v, want %#v", test.input, got, want)
			}
		})
	}
}
