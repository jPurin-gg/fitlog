package exercise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai"
	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/requestctx"
)

func TestAlternativesValidatesResponseAndLogsOutcome(t *testing.T) {
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

			response, err := service.Alternatives(ctx, "bench", "器具が空いていない")
			if (err != nil) != test.wantErr {
				t.Fatalf("Alternatives() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr {
				if len(response.Alternatives) != 1 {
					t.Fatalf("alternatives = %#v, want one candidate", response.Alternatives)
				}
				alternative := response.Alternatives[0]
				if alternative.ID != "dumbbell-bench" || alternative.Name != "ダンベルベンチプレス" {
					t.Fatalf("alternative = %#v, want the dictionary ID and name", alternative)
				}
			}

			record := decodeExerciseFeatureLog(t, output.Bytes())
			if record["request_id"] != "exercise-request" || record["task"] != string(ai.TaskAlternative) || record["outcome"] != test.wantOutcome {
				t.Fatalf("feature log = %#v", record)
			}
		})
	}
}

func TestSearchBoundsPageSizeAndReportsPage(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "default limit", limit: 0, offset: 0, wantLimit: DefaultSearchLimit, wantOffset: 0},
		{name: "requested page", limit: 20, offset: 40, wantLimit: 20, wantOffset: 40},
		{name: "maximum limit", limit: MaxSearchLimit + 1, offset: 100, wantLimit: MaxSearchLimit, wantOffset: 100},
		{name: "negative values", limit: -5, offset: -1, wantLimit: DefaultSearchLimit, wantOffset: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubExerciseRepository{searchItems: []Summary{{ID: "pull_up", Name: "懸垂"}}, searchTotal: 873}
			service := NewService(repository, nil, nil)

			result, err := service.Search(context.Background(), Filters{Muscles: []string{"広背筋", "僧帽筋"}, Limit: test.limit, Offset: test.offset})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if repository.searchFilters.Limit != test.wantLimit || repository.searchFilters.Offset != test.wantOffset || len(repository.searchFilters.Muscles) != 2 {
				t.Fatalf("repository filters = %+v, want limit %d and offset %d", repository.searchFilters, test.wantLimit, test.wantOffset)
			}
			if result.Limit != test.wantLimit || result.Offset != test.wantOffset || result.Total != 873 || len(result.Items) != 1 {
				t.Fatalf("Search() = %+v, want limit %d, offset %d and total 873", result, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestSearchEncodesAnEmptyPageAsAnArray(t *testing.T) {
	service := NewService(&stubExerciseRepository{}, nil, nil)

	result, err := service.Search(context.Background(), Filters{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if want := `{"items":[],"total":0,"limit":50,"offset":0}`; string(encoded) != want {
		t.Fatalf("Search() JSON = %s, want %s", encoded, want)
	}
}

func TestSearchAndGetMapRepositoryErrors(t *testing.T) {
	service := NewService(&stubExerciseRepository{searchErr: errors.New("database unavailable"), getErr: ErrNotFound}, nil, nil)

	_, err := service.Search(context.Background(), Filters{})
	if appErr := apperr.As(err); appErr == nil || appErr.Status != http.StatusInternalServerError {
		t.Fatalf("Search() error = %v, want an internal error", err)
	}
	_, err = service.Get(context.Background(), "unknown")
	if appErr := apperr.As(err); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("Get() error = %v, want not found", err)
	}
}

func TestGetReturnsTheFullExercise(t *testing.T) {
	stored := Exercise{ID: "pull_up", Name: "懸垂", Instructions: []string{"バーを握ります。"}, PrimaryMuscles: []string{"広背筋"}}
	service := NewService(&stubExerciseRepository{exercise: stored}, nil, nil)

	item, err := service.Get(context.Background(), "pull_up")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item.ID != stored.ID || len(item.Instructions) != 1 || item.Instructions[0] != "バーを握ります。" {
		t.Fatalf("Get() = %+v, want the stored exercise", item)
	}
}

// "背中" is the category from the bug report: it has to reach the exercises
// of every back muscle, not only those of the first one.
func TestBackCategoryListsEveryBackMuscle(t *testing.T) {
	var back []string
	for _, category := range AvailableFilterOptions().MuscleCategories {
		if category.Name == "背中" {
			back = category.Muscles
		}
	}
	for _, muscle := range []string{"広背筋", "背中中部", "下背部", "僧帽筋"} {
		if !slices.Contains(back, muscle) {
			t.Errorf("category 背中 = %v, want it to include %q", back, muscle)
		}
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
	searchFilters      Filters
	searchItems        []Summary
	searchTotal        int
	searchErr          error
	exercise           Exercise
	getErr             error
}

func (r *stubExerciseRepository) Search(_ context.Context, filters Filters) ([]Summary, int, error) {
	r.searchFilters = filters
	return r.searchItems, r.searchTotal, r.searchErr
}
func (r *stubExerciseRepository) Get(context.Context, string) (Exercise, error) {
	return r.exercise, r.getErr
}
func (*stubExerciseRepository) Create(context.Context, Exercise) error             { return nil }
func (*stubExerciseRepository) Favorites(context.Context, int) ([]Exercise, error) { return nil, nil }
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
