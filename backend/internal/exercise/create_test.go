package exercise

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestCreateStoresCatalogCategoryAndPrimaryMuscle(t *testing.T) {
	tests := []struct {
		name         string
		input        CreateInput
		wantCategory string
		wantPrimary  []string
	}{
		{
			name:         "dictionary page legacy code",
			input:        CreateInput{Name: "インクラインダンベルプレス", Category: "strength", Equipment: "ダンベル", Level: "初級", PrimaryMuscles: []string{"大胸筋"}},
			wantCategory: "筋力トレーニング",
			wantPrimary:  []string{"大胸筋"},
		},
		{
			name:         "catalog label",
			input:        CreateInput{Name: "ジャンピングジャック", Category: "有酸素運動", PrimaryMuscles: []string{"ふくらはぎ"}},
			wantCategory: "有酸素運動",
			wantPrimary:  []string{"ふくらはぎ"},
		},
		{
			name:         "legacy single primary field",
			input:        CreateInput{Name: "ボックスジャンプ改", Category: "plyometrics", Primary: " 大腿四頭筋 "},
			wantCategory: "プライオメトリクス",
			wantPrimary:  []string{"大腿四頭筋"},
		},
		{
			name:         "category omitted",
			input:        CreateInput{Name: "  ケーブルフライ改  ", PrimaryMuscles: []string{"大胸筋"}},
			wantCategory: DefaultCategory,
			wantPrimary:  []string{"大胸筋"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubExerciseRepository{}
			created, err := NewService(repository, nil, nil).Create(context.Background(), test.input)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if len(repository.created) != 1 || !reflect.DeepEqual(repository.created[0], created) {
				t.Fatalf("stored = %#v, returned = %#v", repository.created, created)
			}
			if created.Category == nil || *created.Category != test.wantCategory {
				t.Fatalf("category = %v, want %q", created.Category, test.wantCategory)
			}
			if !reflect.DeepEqual(created.PrimaryMuscles, test.wantPrimary) {
				t.Fatalf("primary muscles = %#v, want %#v", created.PrimaryMuscles, test.wantPrimary)
			}
			if created.Name != strings.TrimSpace(test.input.Name) || !strings.HasPrefix(created.ID, "custom_") {
				t.Fatalf("created = %#v", created)
			}
		})
	}
}

func TestCreateRejectsIncompleteExercise(t *testing.T) {
	tests := []struct {
		name      string
		input     CreateInput
		wantField string
		wantValue string
	}{
		{name: "missing name", input: CreateInput{Name: "  ", PrimaryMuscles: []string{"大胸筋"}}, wantField: "name", wantValue: "required"},
		{name: "selector without muscle", input: CreateInput{Name: "新しい種目", Category: "筋力トレーニング", Equipment: "その他", Primary: ""}, wantField: "primary_muscles", wantValue: "required"},
		{name: "alternative modal name only", input: CreateInput{Name: "新しい種目", Category: "strength", Equipment: "その他"}, wantField: "primary_muscles", wantValue: "required"},
		{name: "muscle outside catalog", input: CreateInput{Name: "新しい種目", PrimaryMuscles: []string{"胸"}}, wantField: "primary_muscles", wantValue: "unknown"},
		{name: "category outside catalog", input: CreateInput{Name: "新しい種目", Category: "その他", PrimaryMuscles: []string{"大胸筋"}}, wantField: "category", wantValue: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubExerciseRepository{}
			_, err := NewService(repository, nil, nil).Create(context.Background(), test.input)
			appErr := apperr.As(err)
			if err == nil || appErr.Status != http.StatusBadRequest || appErr.Fields[test.wantField] != test.wantValue {
				t.Fatalf("Create() error = %#v, want %s=%s", appErr, test.wantField, test.wantValue)
			}
			if len(repository.created) != 0 {
				t.Fatalf("created = %#v, want nothing", repository.created)
			}
		})
	}
}

func TestCreateReturnsExistingExerciseForSameNormalizedName(t *testing.T) {
	category := "筋力トレーニング"
	existing := Exercise{ID: "bench_press", Name: "ベンチプレス Ｗｉｄｅ", Category: &category, PrimaryMuscles: []string{"大胸筋"}}
	repository := &stubExerciseRepository{existing: []Exercise{existing}}

	_, err := NewService(repository, nil, nil).Create(context.Background(), CreateInput{Name: "　ベンチプレスwide ", PrimaryMuscles: []string{"大胸筋"}})

	appErr := apperr.As(err)
	if err == nil || appErr.Status != http.StatusConflict || appErr.Code != apperr.CodeConflict {
		t.Fatalf("Create() error = %#v, want conflict", appErr)
	}
	if got, ok := appErr.Extensions["existing"].(Exercise); !ok || got.ID != "bench_press" {
		t.Fatalf("existing extension = %#v", appErr.Extensions)
	}
	if !strings.Contains(appErr.Detail, existing.Name) {
		t.Fatalf("detail = %q, want existing name", appErr.Detail)
	}
	if len(repository.created) != 0 {
		t.Fatalf("created = %#v, want nothing", repository.created)
	}
}

func TestCreateReturnsInternalErrorWhenRepositoryFails(t *testing.T) {
	_, err := NewService(failingCreateRepository{&stubExerciseRepository{}}, nil, nil).Create(context.Background(), CreateInput{Name: "種目", PrimaryMuscles: []string{"肩"}})
	if appErr := apperr.As(err); appErr.Status != http.StatusInternalServerError {
		t.Fatalf("Create() error = %#v, want internal error", appErr)
	}
}

type failingCreateRepository struct{ *stubExerciseRepository }

func (failingCreateRepository) CreateUnique(context.Context, Exercise) (Exercise, error) {
	return Exercise{}, errors.New("database unavailable")
}

func TestNormalizeName(t *testing.T) {
	tests := []struct{ left, right string }{
		{"ベンチプレス", "  ベンチプレス\t"},
		{"ベンチプレス", "ベンチ　プレス"},
		{"Bench Press", "benchpress"},
		{"ＥＺバーカール", "ezバーカール"},
		{"ダンベルプレス(30°)", "ダンベルプレス（30°）"},
	}
	for _, test := range tests {
		if NormalizeName(test.left) != NormalizeName(test.right) {
			t.Errorf("NormalizeName(%q) = %q, NormalizeName(%q) = %q; want equal", test.left, NormalizeName(test.left), test.right, NormalizeName(test.right))
		}
	}
	if NormalizeName("ベンチプレス") == NormalizeName("インクラインベンチプレス") {
		t.Fatal("different exercises must not share a key")
	}
}

// カタログデータの分類・部位をすべて受け付けることを確認し、表記のずれを防ぐ。
func TestCatalogLabelsMatchSeedData(t *testing.T) {
	data, err := os.ReadFile("../../tmpkin_jp.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog []Exercise
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	categories := map[string]bool{}
	for _, item := range catalog {
		if item.Category != nil {
			categories[*item.Category] = true
		}
		for _, muscle := range item.PrimaryMuscles {
			if !isKnownMuscle(muscle) {
				t.Errorf("catalog muscle %q (%s) is not accepted", muscle, item.ID)
			}
		}
	}
	for category := range categories {
		if got, ok := NormalizeCategory(category); !ok || got != category {
			t.Errorf("NormalizeCategory(%q) = %q, %v", category, got, ok)
		}
	}
	for _, category := range Categories {
		if !categories[category] {
			t.Errorf("category %q does not appear in the catalog", category)
		}
	}
	for code, label := range legacyCategoryCodes {
		if !categories[label] {
			t.Errorf("legacy code %q maps to %q, which is not a catalog label", code, label)
		}
	}
}
