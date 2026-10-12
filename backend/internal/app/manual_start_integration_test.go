//go:build integration

package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/app"
	"github.com/jPurin-gg/myfitlog-backend/internal/config"
	"github.com/jPurin-gg/myfitlog-backend/internal/database"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

func TestManualWorkoutWithoutMonthlyPlanHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	monthPath := "/api/monthly-plans/" + date[:7]
	startPath := "/api/workout-plans/" + date + "/start"
	f.request(t, "GET", monthPath, nil, "", http.StatusNotFound)
	f.request(t, "GET", "/api/workout-plans/"+date, nil, "", http.StatusNotFound)
	f.request(t, "POST", startPath, map[string]any{"use_ai": false}, "", http.StatusNotFound)

	started := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "bench"}, "", http.StatusOK))
	if started.WorkoutID <= 0 || started.AIStatus != planning.AIStatusNotRequested {
		t.Fatalf("manual start = %+v", started)
	}
	// Choosing a different manual exercise must reuse, not replace, the saved daily plan.
	replayed := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"exercise_id": "squat"}, "", http.StatusOK))
	if replayed.WorkoutID != started.WorkoutID || !reflect.DeepEqual(replayed.Plan, started.Plan) {
		t.Fatalf("replayed start replaced the original plan: %+v", replayed)
	}

	workoutPath := fmt.Sprintf("/api/workouts/%d", started.WorkoutID)
	firstInput := workout.SetInput{ExerciseID: "bench", SetOrder: 1, Weight: 0, Reps: 12, Feeling: "普通"}
	first := decodeAPI[workout.Set](t, f.request(t, "POST", workoutPath+"/sets", firstInput, "manual_set_001", http.StatusCreated))
	repeated := decodeAPI[workout.Set](t, f.request(t, "POST", workoutPath+"/sets", firstInput, "manual_set_001", http.StatusOK))
	if first.ID != repeated.ID {
		t.Fatal("record replay created another set")
	}
	f.request(t, "POST", workoutPath+"/sets", workout.SetInput{ExerciseID: "squat", SetOrder: 1, Weight: 20, Reps: 8}, "manual_set_002", http.StatusCreated)
	finished := decodeAPI[workout.FinishResult](t, f.request(t, "POST", workoutPath+"/finish", nil, "", http.StatusOK))
	if finished.Status != "completed" || finished.Summary.TotalSets != 2 || finished.Summary.TotalReps != 20 || finished.Summary.TotalVolume != 160 || finished.Summary.AIComment != "" {
		t.Fatalf("finished summary = %+v", finished)
	}
	detail := decodeAPI[workout.Detail](t, f.request(t, "GET", workoutPath, nil, "", http.StatusOK))
	if detail.Status != "completed" || !reflect.DeepEqual(detail.Summary, finished.Summary) {
		t.Fatalf("persisted detail = %+v", detail)
	}
	history := decodeAPI[workout.CalendarWorkout](t, f.request(t, "GET", "/api/workouts/by-date/"+date, nil, "", http.StatusOK))
	if history.WorkoutID != started.WorkoutID || len(history.Sets) != 2 || history.Sets[0].Weight != 0 {
		t.Fatalf("calendar history = %+v", history)
	}
	f.request(t, "GET", monthPath, nil, "", http.StatusNotFound)
	if calls := f.aiCalls.Load(); calls != 0 {
		t.Fatalf("AI calls = %d, want zero", calls)
	}
}

func TestConcurrentManualStartsHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil)
	date := time.Now().UTC().Format("2006-01-02")
	const requests = 8
	start := make(chan struct{})
	results := make(chan apiResult, requests)
	var wait sync.WaitGroup
	for i := range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			exerciseID := []string{"bench", "squat"}[i%2]
			results <- f.do("POST", "/api/workout-plans/"+date+"/start", map[string]any{"use_ai": false, "exercise_id": exerciseID}, "")
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var winner planning.PlanSession
	for result := range results {
		if result.err != nil || result.status != http.StatusOK {
			t.Fatalf("concurrent start status=%d error=%v body=%s", result.status, result.err, result.body)
		}
		session := decodeAPI[planning.PlanSession](t, result.body)
		if winner.WorkoutID == 0 {
			winner = session
		}
		if session.WorkoutID != winner.WorkoutID || session.WorkoutID <= 0 || !reflect.DeepEqual(session.Plan, winner.Plan) {
			t.Fatalf("concurrent start did not reuse winning workout: %+v", session)
		}
	}
	var workouts, plans int
	if err := f.db.QueryRow(`SELECT (SELECT COUNT(*) FROM workouts), (SELECT COUNT(*) FROM workout_plans)`).Scan(&workouts, &plans); err != nil {
		t.Fatal(err)
	}
	if workouts != 1 || plans != 1 || f.aiCalls.Load() != 0 {
		t.Fatalf("workouts=%d plans=%d AI calls=%d, want 1/1/0", workouts, plans, f.aiCalls.Load())
	}
}

func TestLateAIStartKeepsManualPlanHTTP(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAI := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAI()
	f := newManualAPIFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		content, _ := json.Marshal(planning.WorkoutPlan{WorkoutTitle: "Late AI plan", Exercises: []planning.PlanExercise{{PlannedSets: 5, TargetReps: 6, TargetWeight: 90}}})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	})
	date := time.Now().UTC()
	startPath := "/api/workout-plans/" + date.Format("2006-01-02") + "/start"
	f.request(t, "PUT", "/api/monthly-plans/"+date.Format("2006-01"), planning.MonthlyPlanInput{
		PlanName: "Test month", Frequency: "週1-2回", RecommendedDays: []int{int(date.Weekday())},
		WeeklyRoutine: []planning.DayRoutine{{DayName: "Day 1", Target: "胸", ExerciseIDs: []string{"bench"}, ExampleExercises: []string{"ベンチプレス"}}},
	}, "", http.StatusOK)
	result := make(chan apiResult, 1)
	go func() { result <- f.do("POST", startPath, map[string]bool{"use_ai": true}, "") }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("AI request did not start")
	}
	manual := decodeAPI[planning.PlanSession](t, f.request(t, "POST", startPath, map[string]any{"use_ai": false, "exercise_id": "squat"}, "", http.StatusOK))
	releaseAI()
	late := <-result
	if late.err != nil || late.status != http.StatusOK {
		t.Fatalf("late start status=%d error=%v body=%s", late.status, late.err, late.body)
	}
	reused := decodeAPI[planning.PlanSession](t, late.body)
	if reused.WorkoutID != manual.WorkoutID || !reflect.DeepEqual(reused.Plan, manual.Plan) || reused.AIStatus != planning.AIStatusNotRequested {
		t.Fatalf("late AI replaced manual plan: %+v", reused)
	}
	stored := decodeAPI[planning.PlanSession](t, f.request(t, "GET", "/api/workout-plans/"+date.Format("2006-01-02"), nil, "", http.StatusOK))
	if !reflect.DeepEqual(stored.Plan, manual.Plan) || stored.Plan.Exercises[0].ExerciseID != "squat" || f.aiCalls.Load() != 1 {
		t.Fatalf("stored plan = %+v, AI calls=%d", stored, f.aiCalls.Load())
	}
}

type manualAPIFixture struct {
	db      *sql.DB
	server  *httptest.Server
	client  *http.Client
	aiCalls atomic.Int32
}

type apiResult struct {
	status int
	body   []byte
	err    error
}

func newManualAPIFixture(t *testing.T, aiHandler http.HandlerFunc) *manualAPIFixture {
	t.Helper()
	dsn := os.Getenv("FITLOG_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=55432 user=fitlog_test password=fitlog_test dbname=fitlog_test sslmode=disable"
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		admin.Close()
		t.Fatal("test PostgreSQL is unavailable; start compose.test.yml before running integration tests")
	}
	schema := fmt.Sprintf("fitlog_api_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+pq.QuoteIdentifier(schema)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Only the schema created by this fixture is removed; the database is never reset.
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA `+pq.QuoteIdentifier(schema)+` CASCADE`); err != nil {
			t.Errorf("clean up isolated test schema: %v", err)
		}
	})
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(20)
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO exercises (id,name) VALUES ('bench','ベンチプレス'), ('squat','スクワット')`); err != nil {
		t.Fatal(err)
	}
	f := &manualAPIFixture{db: db}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.aiCalls.Add(1)
		if aiHandler != nil {
			aiHandler(w, r)
			return
		}
		http.Error(w, "AI must not be called", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	cfg := config.Config{
		Timezone: time.UTC, PromptDir: "../../prompts", SessionSecret: []byte("integration-only-session-secret-32-bytes"),
		SessionCookieName: "test_session", SessionTTL: time.Hour,
		AI: config.AIConfig{URL: provider.URL, APIKey: "test-only", Model: "test", RPM: 60000, MaxAttempts: 1, Timeout: 10 * time.Second, OptionalTimeout: 10 * time.Second},
	}
	f.server = httptest.NewServer(app.NewHandler(db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(f.server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.client = &http.Client{Jar: jar, Timeout: 15 * time.Second}
	f.request(t, "POST", "/api/auth/login", map[string]string{"nickname": "integration-user", "password": "test-only-password"}, "", http.StatusCreated)
	f.request(t, "GET", "/api/auth/me", nil, "", http.StatusOK)
	return f
}

func (f *manualAPIFixture) do(method, path string, input any, key string) apiResult {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return apiResult{err: err}
		}
	}
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		return apiResult{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	response, err := f.client.Do(req)
	if err != nil {
		return apiResult{err: err}
	}
	defer response.Body.Close()
	body, err = io.ReadAll(response.Body)
	return apiResult{status: response.StatusCode, body: body, err: err}
}

func (f *manualAPIFixture) request(t *testing.T, method, path string, input any, key string, wantStatus int) []byte {
	t.Helper()
	result := f.do(method, path, input, key)
	if result.err != nil || result.status != wantStatus {
		t.Fatalf("%s %s: status=%d want=%d error=%v body=%s", method, path, result.status, wantStatus, result.err, result.body)
	}
	return result.body
}

func decodeAPI[T any](t *testing.T, body []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
