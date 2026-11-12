package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
)

func TestSearchPassesFiltersAndReturnsLightPage(t *testing.T) {
	equipment := "自重"
	repository := &exerciseRepository{
		items: []exercise.Summary{{ID: "pull_up", Name: "懸垂", Equipment: &equipment, PrimaryMuscles: []string{"広背筋"}}},
		total: 120,
	}
	handler := NewHandler(exercise.NewService(repository, nil, nil))
	query := url.Values{
		"name":      {" 懸垂 "},
		"muscle":    {"広背筋, 背中中部,下背部,,僧帽筋"},
		"equipment": {"自重,チューブ・バンド"},
		"level":     {"初級"},
		"limit":     {"20"},
		"offset":    {"40"},
	}

	recorder := httptest.NewRecorder()
	handler.Search(recorder, httptest.NewRequest(http.MethodGet, "/api/exercises?"+query.Encode(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Search() = %d, %s", recorder.Code, recorder.Body.String())
	}

	want := exercise.Filters{
		Name:      "懸垂",
		Muscles:   []string{"広背筋", "背中中部", "下背部", "僧帽筋"},
		Equipment: []string{"自重", "チューブ・バンド"},
		Level:     "初級",
		Limit:     20,
		Offset:    40,
	}
	if !reflect.DeepEqual(repository.filters, want) {
		t.Fatalf("repository filters = %+v, want %+v", repository.filters, want)
	}

	var page struct {
		Items  []map[string]any `json:"items"`
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v; body = %s", err, recorder.Body.String())
	}
	if page.Total != 120 || page.Limit != 20 || page.Offset != 40 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want total 120, limit 20, offset 40 and one item", page)
	}
	item := page.Items[0]
	if item["id"] != "pull_up" || item["name"] != "懸垂" || item["equipment"] != "自重" {
		t.Fatalf("item = %#v, want the stored summary", item)
	}
	for _, heavy := range []string{"instructions", "images"} {
		if _, exists := item[heavy]; exists {
			t.Fatalf("item contains %q, want list items without it: %#v", heavy, item)
		}
	}
}

func TestSearchLimitsPageSize(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "no conditions", query: "", wantLimit: exercise.DefaultSearchLimit},
		{name: "zero", query: "?limit=0", wantLimit: exercise.DefaultSearchLimit},
		{name: "too large", query: "?limit=5000", wantLimit: exercise.MaxSearchLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &exerciseRepository{}
			handler := NewHandler(exercise.NewService(repository, nil, nil))

			recorder := httptest.NewRecorder()
			handler.Search(recorder, httptest.NewRequest(http.MethodGet, "/api/exercises"+test.query, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("Search() = %d, %s", recorder.Code, recorder.Body.String())
			}
			filters := repository.filters
			if filters.Limit != test.wantLimit || filters.Offset != 0 || filters.Name != "" || filters.Level != "" || len(filters.Muscles) != 0 || len(filters.Equipment) != 0 {
				t.Fatalf("repository filters = %+v, want only limit %d", filters, test.wantLimit)
			}
			if !strings.Contains(recorder.Body.String(), `"items":[]`) {
				t.Fatalf("body = %s, want an empty items array", recorder.Body.String())
			}
		})
	}
}

func TestSearchRejectsInvalidPaging(t *testing.T) {
	for _, query := range []string{"limit=abc", "limit=-1", "offset=-5", "offset=1.5"} {
		t.Run(query, func(t *testing.T) {
			repository := &exerciseRepository{}
			handler := NewHandler(exercise.NewService(repository, nil, nil))

			recorder := httptest.NewRecorder()
			handler.Search(recorder, httptest.NewRequest(http.MethodGet, "/api/exercises?"+query, nil))
			if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" || !strings.Contains(recorder.Body.String(), `"code":"VALIDATION_ERROR"`) {
				t.Fatalf("Search() = %d, %q, %s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
			}
			if repository.searches != 0 {
				t.Fatalf("repository searches = %d, want none", repository.searches)
			}
		})
	}
}

func TestFilterOptionsReturnsTheSharedDefinition(t *testing.T) {
	handler := NewHandler(exercise.NewService(&exerciseRepository{}, nil, nil))

	recorder := httptest.NewRecorder()
	handler.FilterOptions(recorder, httptest.NewRequest(http.MethodGet, "/api/exercises/filters", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("FilterOptions() = %d, %s", recorder.Code, recorder.Body.String())
	}
	var options exercise.FilterOptions
	if err := json.Unmarshal(recorder.Body.Bytes(), &options); err != nil {
		t.Fatalf("decode options: %v; body = %s", err, recorder.Body.String())
	}
	if !reflect.DeepEqual(options, exercise.AvailableFilterOptions()) {
		t.Fatalf("FilterOptions() = %+v, want %+v", options, exercise.AvailableFilterOptions())
	}
	for _, key := range []string{`"equipment":[`, `"muscle_categories":[`, `"muscles":[`} {
		if !strings.Contains(recorder.Body.String(), key) {
			t.Fatalf("body = %s, want it to contain %s", recorder.Body.String(), key)
		}
	}
}

func TestGetReturnsInstructionsOrNotFound(t *testing.T) {
	repository := &exerciseRepository{exercises: map[string]exercise.Exercise{
		"pull_up": {ID: "pull_up", Name: "懸垂", Instructions: []string{"バーを握ります。"}, PrimaryMuscles: []string{"広背筋"}},
	}}
	handler := NewHandler(exercise.NewService(repository, nil, nil))

	found := httptest.NewRequest(http.MethodGet, "/api/exercises/pull_up", nil)
	found.SetPathValue("exerciseID", "pull_up")
	foundRecorder := httptest.NewRecorder()
	handler.Get(foundRecorder, found)
	if foundRecorder.Code != http.StatusOK || !strings.Contains(foundRecorder.Body.String(), `"instructions":["バーを握ります。"]`) {
		t.Fatalf("Get() = %d, %s", foundRecorder.Code, foundRecorder.Body.String())
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/exercises/unknown", nil)
	missing.SetPathValue("exerciseID", "unknown")
	missingRecorder := httptest.NewRecorder()
	handler.Get(missingRecorder, missing)
	if missingRecorder.Code != http.StatusNotFound || !strings.Contains(missingRecorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("Get() = %d, %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

type exerciseRepository struct {
	filters   exercise.Filters
	searches  int
	items     []exercise.Summary
	total     int
	exercises map[string]exercise.Exercise
}

func (r *exerciseRepository) Search(_ context.Context, filters exercise.Filters) ([]exercise.Summary, int, error) {
	r.searches++
	r.filters = filters
	return r.items, r.total, nil
}

func (r *exerciseRepository) Get(_ context.Context, exerciseID string) (exercise.Exercise, error) {
	item, exists := r.exercises[exerciseID]
	if !exists {
		return exercise.Exercise{}, exercise.ErrNotFound
	}
	return item, nil
}

func (*exerciseRepository) Create(context.Context, exercise.Exercise) error { return nil }

func (*exerciseRepository) Favorites(context.Context, int) ([]exercise.Exercise, error) {
	return nil, nil
}

func (*exerciseRepository) SetFavorite(context.Context, int, string, bool) error { return nil }

func (*exerciseRepository) Recent(context.Context, int) ([]exercise.Exercise, error) {
	return nil, nil
}

func (*exerciseRepository) Settings(context.Context, int, string) (exercise.Settings, error) {
	return exercise.Settings{}, nil
}

func (*exerciseRepository) SaveSettings(context.Context, int, string, exercise.Settings) error {
	return nil
}

func (*exerciseRepository) AlternativeContext(context.Context, string) (exercise.AlternativeContext, error) {
	return exercise.AlternativeContext{}, nil
}
