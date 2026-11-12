//go:build integration

package app_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
)

func TestExerciseSearchFiltersAndPagesHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	insert := func(id, name, equipment, primary, secondary string) {
		t.Helper()
		if _, err := f.db.Exec(`
			INSERT INTO exercises (id,name,level,equipment,category,instructions,primary_muscles,secondary_muscles,images)
			VALUES ($1,$2,'初級',$3,'筋力トレーニング','["手順の説明"]'::jsonb,$4::jsonb,$5::jsonb,'["a/0.jpg"]'::jsonb)
		`, id, name, equipment, primary, secondary); err != nil {
			t.Fatal(err)
		}
	}
	insert("lat_pulldown", "ラットプルダウン", "ケーブル", `["広背筋"]`, `["上腕二頭筋"]`)
	insert("seated_row", "シーテッドロー", "ケーブル", `["背中中部"]`, `[]`)
	insert("back_extension", "バックエクステンション", "自重", `["下背部"]`, `[]`)
	insert("shrug", "シュラッグ", "ダンベル", `["僧帽筋"]`, `[]`)
	insert("ball_crunch", "ボールクランチ", "バランスボール", `["腹筋"]`, `[]`)
	// Together with the two exercises of the fixture this exceeds one page.
	const fillers = 60
	for i := range fillers {
		insert(fmt.Sprintf("filler_%02d", i), "フィラー", "その他", `["肩"]`, `[]`)
	}
	const total = 2 + 5 + fillers

	search := func(query url.Values) exercise.SearchResult {
		t.Helper()
		return decodeAPI[exercise.SearchResult](t, f.request(t, "GET", "/api/exercises?"+query.Encode(), nil, "", http.StatusOK))
	}
	ids := func(page exercise.SearchResult) []string {
		result := []string{}
		for _, item := range page.Items {
			result = append(result, item.ID)
		}
		sort.Strings(result)
		return result
	}

	unfiltered := f.request(t, "GET", "/api/exercises", nil, "", http.StatusOK)
	if bytes.Contains(unfiltered, []byte("instructions")) || bytes.Contains(unfiltered, []byte("手順の説明")) || bytes.Contains(unfiltered, []byte("images")) {
		t.Fatalf("unfiltered list contains instructions or images: %s", unfiltered)
	}
	first := decodeAPI[exercise.SearchResult](t, unfiltered)
	if first.Total != total || first.Limit != exercise.DefaultSearchLimit || first.Offset != 0 || len(first.Items) != exercise.DefaultSearchLimit {
		t.Fatalf("first page: total=%d limit=%d offset=%d items=%d", first.Total, first.Limit, first.Offset, len(first.Items))
	}
	second := search(url.Values{"offset": {fmt.Sprint(exercise.DefaultSearchLimit)}})
	if second.Total != total || second.Offset != exercise.DefaultSearchLimit || len(second.Items) != total-exercise.DefaultSearchLimit {
		t.Fatalf("second page: total=%d offset=%d items=%d", second.Total, second.Offset, len(second.Items))
	}
	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		if seen[item.ID] {
			t.Fatalf("exercise %q appears on both pages", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != total {
		t.Fatalf("pages reached %d exercises, want %d", len(seen), total)
	}

	back := search(url.Values{"muscle": {"広背筋,背中中部,下背部,僧帽筋"}})
	if want := []string{"back_extension", "lat_pulldown", "seated_row", "shrug"}; back.Total != len(want) || !reflect.DeepEqual(ids(back), want) {
		t.Fatalf("back muscles: total=%d ids=%v, want %v", back.Total, ids(back), want)
	}
	if secondary := search(url.Values{"muscle": {"上腕二頭筋"}}); !reflect.DeepEqual(ids(secondary), []string{"lat_pulldown"}) {
		t.Fatalf("secondary muscle ids = %v", ids(secondary))
	}
	if ball := search(url.Values{"equipment": {"バランスボール"}}); !reflect.DeepEqual(ids(ball), []string{"ball_crunch"}) {
		t.Fatalf("exercise ball ids = %v", ids(ball))
	}
	if several := search(url.Values{"equipment": {"ケーブル,自重"}}); !reflect.DeepEqual(ids(several), []string{"back_extension", "lat_pulldown", "seated_row"}) {
		t.Fatalf("multiple equipment ids = %v", ids(several))
	}
	combined := search(url.Values{"muscle": {"広背筋,背中中部"}, "equipment": {"ケーブル"}, "name": {"ロー"}, "level": {"初級"}, "limit": {"1"}})
	if combined.Total != 1 || combined.Limit != 1 || !reflect.DeepEqual(ids(combined), []string{"seated_row"}) {
		t.Fatalf("combined filters: total=%d limit=%d ids=%v", combined.Total, combined.Limit, ids(combined))
	}
	if none := search(url.Values{"muscle": {"広背"}}); none.Total != 0 || len(none.Items) != 0 {
		t.Fatalf("partial muscle name matched %d exercises: %v", none.Total, ids(none))
	}

	detail := decodeAPI[exercise.Exercise](t, f.request(t, "GET", "/api/exercises/lat_pulldown", nil, "", http.StatusOK))
	if detail.ID != "lat_pulldown" || !reflect.DeepEqual(detail.Instructions, []string{"手順の説明"}) || !reflect.DeepEqual(detail.SecondaryMuscles, []string{"上腕二頭筋"}) {
		t.Fatalf("detail = %+v", detail)
	}
	f.request(t, "GET", "/api/exercises/unknown_exercise", nil, "", http.StatusNotFound)
	f.request(t, "GET", "/api/exercises?limit=abc", nil, "", http.StatusBadRequest)

	options := decodeAPI[exercise.FilterOptions](t, f.request(t, "GET", "/api/exercises/filters", nil, "", http.StatusOK))
	if !reflect.DeepEqual(options, exercise.AvailableFilterOptions()) {
		t.Fatalf("filter options = %+v", options)
	}
}
