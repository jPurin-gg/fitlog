package exercise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestValidateAlternativesRejectsUnknownIDsAndUsesDictionaryName(t *testing.T) {
	allowed := map[string]AlternativeCandidate{
		"dumbbell-bench": {ID: "dumbbell-bench", Name: "ダンベルベンチプレス"},
	}
	response := AlternativeResponse{Alternatives: []Alternative{{ID: "dumbbell-bench", Name: "hallucinated name"}}}
	if err := validateAlternatives(&response, allowed); err != nil {
		t.Fatalf("validateAlternatives() error = %v", err)
	}
	if response.Alternatives[0].Name != "ダンベルベンチプレス" {
		t.Fatalf("name = %q", response.Alternatives[0].Name)
	}
	response.Alternatives[0].ID = "unknown"
	if err := validateAlternatives(&response, allowed); err == nil {
		t.Fatal("validateAlternatives(unknown) error = nil")
	}
}

func TestAlternativesLogsFinalFeatureOutcome(t *testing.T) {
	validResponse := `{"alternatives":[{"id":"dumbbell-bench","name":"AI name","description":"器具を変更します。"}],"message":"候補です。"}`
	tests := []struct {
		name          string
		completion    string
		completionErr error
		wantOutcome   string
		wantErr       bool
	}{
		{name: "applied", completion: validResponse, wantOutcome: "applied"},
		{name: "provider error", completionErr: errors.New("provider unavailable"), wantOutcome: "provider_error", wantErr: true},
		{name: "invalid output", completion: "not-json", wantOutcome: "invalid_output", wantErr: true},
		{name: "unknown exercise", completion: `{"alternatives":[{"id":"unknown","name":"unknown"}]}`, wantOutcome: "invalid_output", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			repository := &stubExerciseRepository{alternativeContext: AlternativeContext{
				ExerciseName: "ベンチプレス",
				Muscles:      []string{"胸"},
				Candidates:   []AlternativeCandidate{{ID: "dumbbell-bench", Name: "ダンベルベンチプレス", Equipment: "ダンベル"}},
			}}
			service := NewService(repository, stubExerciseAI{completion: test.completion, err: test.completionErr}, stubExercisePrompts{}).WithLogger(logger)
			ctx := requestctx.WithRequestID(context.Background(), "exercise-request")

			_, err := service.Alternatives(ctx, "bench", "器具が空いていない")
			if (err != nil) != test.wantErr {
				t.Fatalf("Alternatives() error = %v, wantErr %v", err, test.wantErr)
			}

			record := decodeExerciseFeatureLog(t, output.Bytes())
			if record["stage"] != "feature" || record["request_id"] != "exercise-request" || record["task"] != string(ai.TaskAlternative) || record["outcome"] != test.wantOutcome {
				t.Fatalf("feature log = %#v", record)
			}
			if _, ok := record["total_duration_ms"]; !ok {
				t.Fatal("total_duration_ms is missing")
			}
		})
	}
}

func decodeExerciseFeatureLog(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatalf("decode feature log: %v; output = %q", err, string(data))
	}
	return record
}

type stubExerciseRepository struct {
	alternativeContext AlternativeContext
	alternativeErr     error
}

func (*stubExerciseRepository) Search(context.Context, Filters) ([]Exercise, error) { return nil, nil }
func (*stubExerciseRepository) Create(context.Context, Exercise) error              { return nil }
func (*stubExerciseRepository) Favorites(context.Context, int) ([]Exercise, error)  { return nil, nil }
func (*stubExerciseRepository) SetFavorite(context.Context, int, string, bool) error {
	return nil
}
func (*stubExerciseRepository) Recent(context.Context, int) ([]Exercise, error) { return nil, nil }
func (*stubExerciseRepository) Settings(context.Context, int, string) (Settings, error) {
	return Settings{}, nil
}
func (*stubExerciseRepository) SaveSettings(context.Context, int, string, Settings) error {
	return nil
}
func (r *stubExerciseRepository) AlternativeContext(context.Context, string) (AlternativeContext, error) {
	return r.alternativeContext, r.alternativeErr
}

type stubExerciseAI struct {
	completion string
	err        error
}

func (c stubExerciseAI) Complete(context.Context, ai.Request) (string, error) {
	return c.completion, c.err
}

type stubExercisePrompts struct{}

func (stubExercisePrompts) Pair(systemFilename, userFilename string, _ any) (string, string, error) {
	return systemFilename, userFilename, nil
}
